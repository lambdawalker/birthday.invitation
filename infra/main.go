package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// Provider resource tokens and logical names match the original stack exactly.
// RegisterResource is Pulumi's core Go API; explicit plugin versions keep the
// provider schemas pinned without requiring Node or generated provider SDKs.
const awsVersion = "7.51.1"
const cloudflareVersion = "6.22.1"

type resource struct {
	pulumi.CustomResourceState
	Arn                      pulumi.StringOutput `pulumi:"arn"`
	Name                     pulumi.StringOutput `pulumi:"name"`
	BucketRegionalDomainName pulumi.StringOutput `pulumi:"bucketRegionalDomainName"`
	DomainValidationOptions  pulumi.ArrayOutput  `pulumi:"domainValidationOptions"`
	CertificateArn           pulumi.StringOutput `pulumi:"certificateArn"`
	ApiEndpoint              pulumi.StringOutput `pulumi:"apiEndpoint"`
	ExecutionArn             pulumi.StringOutput `pulumi:"executionArn"`
	DomainName               pulumi.StringOutput `pulumi:"domainName"`
}
type provider struct{ pulumi.ProviderResourceState }
type M = map[string]any
type A = []any

func inputs(m M) pulumi.Map {
	out := pulumi.Map{}
	for k, v := range m {
		out[k] = pulumi.ToOutput(v)
	}
	return out
}
func add(ctx *pulumi.Context, token, name string, args M, opts ...pulumi.ResourceOption) *resource {
	r := &resource{}
	version := awsVersion
	if strings.HasPrefix(token, "cloudflare:") {
		version = cloudflareVersion
	}
	opts = append(opts, pulumi.Version(version))
	if err := ctx.RegisterResource(token, name, inputs(args), r, opts...); err != nil {
		panic(err)
	}
	return r
}
func jsonOutput(values []pulumi.Input, fn func([]any) any) pulumi.StringOutput {
	raw := make([]any, len(values))
	for i, v := range values {
		raw[i] = v
	}
	return pulumi.All(raw...).ApplyT(func(v []any) (string, error) { b, err := json.Marshal(fn(v)); return string(b), err }).(pulumi.StringOutput)
}
func recordField(options pulumi.ArrayOutput, key string) pulumi.StringOutput {
	return options.ApplyT(func(v []any) (string, error) {
		if len(v) == 0 {
			return "", fmt.Errorf("ACM returned no DNS validation options")
		}
		row, ok := v[0].(map[string]any)
		if !ok {
			return "", fmt.Errorf("unexpected ACM validation options")
		}
		value, ok := row[key].(string)
		if !ok {
			return "", fmt.Errorf("missing ACM %s", key)
		}
		return value, nil
	}).(pulumi.StringOutput)
}
func define(ctx *pulumi.Context) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = e
			} else {
				panic(r)
			}
		}
	}()
	cfg := config.New(ctx, "")
	domain := cfg.Get("domain")
	if domain == "" {
		domain = "xitlaliabigail.com"
	}
	zone := cfg.Require("cloudflareZoneId")
	admin := cfg.RequireSecret("adminKey")
	east := &provider{}
	if err = ctx.RegisterResource("pulumi:providers:aws", "east", inputs(M{"region": "us-east-1"}), east, pulumi.Version(awsVersion)); err != nil {
		return err
	}
	table := add(ctx, "aws:dynamodb/table:Table", "families", M{"billingMode": "PAY_PER_REQUEST", "hashKey": "pk", "attributes": A{M{"name": "pk", "type": "S"}}, "pointInTimeRecovery": M{"enabled": true}, "serverSideEncryption": M{"enabled": true}, "deletionProtectionEnabled": true}, pulumi.Protect(true))
	bucket := add(ctx, "aws:s3/bucketV2:BucketV2", "website", M{})
	add(ctx, "aws:s3/bucketPublicAccessBlock:BucketPublicAccessBlock", "private-site", M{"bucket": bucket.ID(), "blockPublicAcls": true, "blockPublicPolicy": true, "ignorePublicAcls": true, "restrictPublicBuckets": true})
	add(ctx, "aws:s3/bucketServerSideEncryptionConfigurationV2:BucketServerSideEncryptionConfigurationV2", "site-encryption", M{"bucket": bucket.ID(), "rules": A{M{"applyServerSideEncryptionByDefault": M{"sseAlgorithm": "AES256"}}}})
	cert := add(ctx, "aws:acm/certificate:Certificate", "tls", M{"domainName": domain, "validationMethod": "DNS"}, pulumi.Provider(east))
	validation := add(ctx, "cloudflare:index/dnsRecord:DnsRecord", "certificate-validation", M{"zoneId": zone, "name": recordField(cert.DomainValidationOptions, "resourceRecordName"), "type": "CNAME", "content": recordField(cert.DomainValidationOptions, "resourceRecordValue"), "ttl": 60, "proxied": false})
	validated := add(ctx, "aws:acm/certificateValidation:CertificateValidation", "tls-validated", M{"certificateArn": cert.Arn, "validationRecordFqdns": A{validation.Name}}, pulumi.Provider(east))
	role := add(ctx, "aws:iam/role:Role", "api-role", M{"assumeRolePolicy": `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"lambda.amazonaws.com"},"Action":"sts:AssumeRole"}]}`})
	logs := add(ctx, "aws:cloudwatch/logGroup:LogGroup", "api-logs", M{"retentionInDays": 14})
	policy := add(ctx, "aws:iam/rolePolicy:RolePolicy", "api-permissions", M{"role": role.ID(), "policy": jsonOutput([]pulumi.Input{table.Arn, logs.Arn}, func(v []any) any {
		return M{"Version": "2012-10-17", "Statement": A{M{"Effect": "Allow", "Action": A{"dynamodb:GetItem", "dynamodb:PutItem", "dynamodb:DeleteItem", "dynamodb:Scan"}, "Resource": v[0]}, M{"Effect": "Allow", "Action": A{"logs:CreateLogStream", "logs:PutLogEvents"}, "Resource": fmt.Sprint(v[1]) + ":*"}}}
	})})
	fn := add(ctx, "aws:lambda/function:Function", "api", M{"role": role.Arn, "runtime": "provided.al2023", "architectures": A{"arm64"}, "handler": "bootstrap", "code": pulumi.NewFileArchive("../build/lambda.zip"), "memorySize": 256, "timeout": 15, "loggingConfig": M{"logFormat": "Text", "logGroup": logs.Name}, "environment": M{"variables": M{"TABLE_NAME": table.Name, "ADMIN_KEY": admin, "SITE_URL": "https://" + domain}}}, pulumi.DependsOn([]pulumi.Resource{policy}))
	api := add(ctx, "aws:apigatewayv2/api:Api", "http-api", M{"protocolType": "HTTP"})
	integration := add(ctx, "aws:apigatewayv2/integration:Integration", "lambda-integration", M{"apiId": api.ID(), "integrationType": "AWS_PROXY", "integrationUri": fn.Arn, "payloadFormatVersion": "2.0"})
	add(ctx, "aws:apigatewayv2/route:Route", "api-route", M{"apiId": api.ID(), "routeKey": "ANY /api/{proxy+}", "target": pulumi.Sprintf("integrations/%s", integration.ID())})
	add(ctx, "aws:apigatewayv2/stage:Stage", "default", M{"apiId": api.ID(), "name": "$default", "autoDeploy": true, "defaultRouteSettings": M{"throttlingBurstLimit": 20, "throttlingRateLimit": 10}})
	add(ctx, "aws:lambda/permission:Permission", "invoke", M{"action": "lambda:InvokeFunction", "function": fn.Name, "principal": "apigateway.amazonaws.com", "sourceArn": pulumi.Sprintf("%s/*/*", api.ExecutionArn)})
	oac := add(ctx, "aws:cloudfront/originAccessControl:OriginAccessControl", "site-access", M{"originAccessControlOriginType": "s3", "signingBehavior": "always", "signingProtocol": "sigv4"})
	headers := add(ctx, "aws:cloudfront/responseHeadersPolicy:ResponseHeadersPolicy", "security", M{"securityHeadersConfig": M{
		"contentTypeOptions": M{"override": true}, "frameOptions": M{"frameOption": "DENY", "override": true}, "referrerPolicy": M{"referrerPolicy": "no-referrer", "override": true}, "strictTransportSecurity": M{"accessControlMaxAgeSec": 31536000, "includeSubdomains": true, "override": true},
		"contentSecurityPolicy": M{"override": true, "contentSecurityPolicy": "default-src 'self'; script-src 'self'; style-src 'self' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data:; connect-src 'self'; frame-src https://maps.google.com; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'"}}})
	distribution := add(ctx, "aws:cloudfront/distribution:Distribution", "cdn", M{"enabled": true, "aliases": A{domain}, "defaultRootObject": "index.html", "priceClass": "PriceClass_100",
		"origins": A{M{"originId": "site", "domainName": bucket.BucketRegionalDomainName, "originAccessControlId": oac.ID()}, M{"originId": "api", "domainName": api.ApiEndpoint.ApplyT(func(s string) (string, error) {
			u, err := url.Parse(s)
			if err != nil {
				return "", err
			}
			return u.Hostname(), nil
		}), "customOriginConfig": M{"httpPort": 80, "httpsPort": 443, "originProtocolPolicy": "https-only", "originSslProtocols": A{"TLSv1.2"}}}},
		"defaultCacheBehavior":  M{"targetOriginId": "site", "viewerProtocolPolicy": "redirect-to-https", "allowedMethods": A{"GET", "HEAD", "OPTIONS"}, "cachedMethods": A{"GET", "HEAD"}, "compress": true, "cachePolicyId": "658327ea-f89d-4fab-a63d-7e88639e58f6", "responseHeadersPolicyId": headers.ID()},
		"orderedCacheBehaviors": A{M{"pathPattern": "/api/*", "targetOriginId": "api", "viewerProtocolPolicy": "https-only", "allowedMethods": A{"GET", "HEAD", "OPTIONS", "PUT", "POST", "PATCH", "DELETE"}, "cachedMethods": A{"GET", "HEAD"}, "cachePolicyId": "4135ea2d-6df8-44a3-9df3-4b5a84be39ad", "originRequestPolicyId": "b689b0a8-53d0-40ab-baf2-68738e2966ac", "responseHeadersPolicyId": headers.ID()}},
		"restrictions":          M{"geoRestriction": M{"restrictionType": "none"}}, "viewerCertificate": M{"acmCertificateArn": validated.CertificateArn, "sslSupportMethod": "sni-only", "minimumProtocolVersion": "TLSv1.2_2021"}})
	add(ctx, "aws:s3/bucketPolicy:BucketPolicy", "cdn-read", M{"bucket": bucket.ID(), "policy": jsonOutput([]pulumi.Input{bucket.Arn, distribution.Arn}, func(v []any) any {
		return M{"Version": "2012-10-17", "Statement": A{M{"Effect": "Allow", "Principal": M{"Service": "cloudfront.amazonaws.com"}, "Action": "s3:GetObject", "Resource": fmt.Sprint(v[0]) + "/*", "Condition": M{"StringEquals": M{"AWS:SourceArn": v[1]}}}}}
	})})
	add(ctx, "cloudflare:index/dnsRecord:DnsRecord", "site-dns", M{"zoneId": zone, "name": domain, "type": "CNAME", "content": distribution.DomainName, "proxied": false, "ttl": 300})
	mime := map[string]string{".html": "text/html; charset=utf-8", ".css": "text/css; charset=utf-8", ".js": "text/javascript; charset=utf-8", ".webp": "image/webp", ".svg": "image/svg+xml"}
	safe := regexp.MustCompile(`[^a-zA-Z0-9-]`)
	err = filepath.WalkDir("../web", func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		key, e := filepath.Rel("../web", path)
		if e != nil {
			return e
		}
		key = filepath.ToSlash(key)
		contentType := mime[filepath.Ext(key)]
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		add(ctx, "aws:s3/bucketObjectv2:BucketObjectv2", "file-"+safe.ReplaceAllString(key, "-"), M{"bucket": bucket.ID(), "key": key, "source": pulumi.NewFileAsset(path), "contentType": contentType, "cacheControl": "public, max-age=300"})
		return nil
	})
	if err != nil {
		return err
	}
	ctx.Export("siteUrl", pulumi.String("https://"+domain))
	ctx.Export("tableName", table.Name)
	ctx.Export("distributionId", distribution.ID())
	ctx.Export("apiUrl", api.ApiEndpoint)
	return nil
}
func main() { pulumi.Run(define) }
