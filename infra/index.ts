import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";
import * as cloudflare from "@pulumi/cloudflare";
import * as fs from "node:fs";
import * as path from "node:path";
const cfg = new pulumi.Config();
const domain = cfg.get("domain") ?? "xitlaliabigail.com";
const zoneId = cfg.require("cloudflareZoneId");
const adminKey = cfg.requireSecret("adminKey");
const east = new aws.Provider("east", { region: "us-east-1" });
const table = new aws.dynamodb.Table(
  "families",
  {
    billingMode: "PAY_PER_REQUEST",
    hashKey: "pk",
    attributes: [{ name: "pk", type: "S" }],
    pointInTimeRecovery: { enabled: true },
    serverSideEncryption: { enabled: true },
    deletionProtectionEnabled: true,
  },
  { protect: true },
);
const bucket = new aws.s3.BucketV2("website", {});
new aws.s3.BucketPublicAccessBlock("private-site", {
  bucket: bucket.id,
  blockPublicAcls: true,
  blockPublicPolicy: true,
  ignorePublicAcls: true,
  restrictPublicBuckets: true,
});
new aws.s3.BucketServerSideEncryptionConfigurationV2("site-encryption", {
  bucket: bucket.id,
  rules: [{ applyServerSideEncryptionByDefault: { sseAlgorithm: "AES256" } }],
});
const cert = new aws.acm.Certificate(
  "tls",
  { domainName: domain, validationMethod: "DNS" },
  { provider: east },
);
const validationRecord = new cloudflare.DnsRecord("certificate-validation", {
  zoneId,
  name: cert.domainValidationOptions.apply((v) => v[0].resourceRecordName),
  type: "CNAME",
  content: cert.domainValidationOptions.apply((v) => v[0].resourceRecordValue),
  ttl: 60,
  proxied: false,
});
const validated = new aws.acm.CertificateValidation(
  "tls-validated",
  { certificateArn: cert.arn, validationRecordFqdns: [validationRecord.name] },
  { provider: east },
);
const role = new aws.iam.Role("api-role", {
  assumeRolePolicy: aws.iam.assumeRolePolicyForPrincipal({
    Service: "lambda.amazonaws.com",
  }),
});
const logs = new aws.cloudwatch.LogGroup("api-logs", { retentionInDays: 14 });
const policy = new aws.iam.RolePolicy("api-permissions", {
  role: role.id,
  policy: pulumi.all([table.arn, logs.arn]).apply(([db, log]) =>
    JSON.stringify({
      Version: "2012-10-17",
      Statement: [
        {
          Effect: "Allow",
          Action: ["dynamodb:GetItem", "dynamodb:PutItem", "dynamodb:Scan"],
          Resource: db,
        },
        {
          Effect: "Allow",
          Action: ["logs:CreateLogStream", "logs:PutLogEvents"],
          Resource: `${log}:*`,
        },
      ],
    }),
  ),
});
const fn = new aws.lambda.Function(
  "api",
  {
    role: role.arn,
    runtime: "provided.al2023",
    architectures: ["arm64"],
    handler: "bootstrap",
    code: new pulumi.asset.FileArchive("../build/lambda.zip"),
    memorySize: 256,
    timeout: 15,
    loggingConfig: { logFormat: "Text", logGroup: logs.name },
    environment: {
      variables: {
        TABLE_NAME: table.name,
        ADMIN_KEY: adminKey,
        SITE_URL: `https://${domain}`,
      },
    },
  },
  { dependsOn: [policy] },
);
const api = new aws.apigatewayv2.Api("http-api", { protocolType: "HTTP" });
const integration = new aws.apigatewayv2.Integration("lambda-integration", {
  apiId: api.id,
  integrationType: "AWS_PROXY",
  integrationUri: fn.arn,
  payloadFormatVersion: "2.0",
});
new aws.apigatewayv2.Route("api-route", {
  apiId: api.id,
  routeKey: "ANY /api/{proxy+}",
  target: pulumi.interpolate`integrations/${integration.id}`,
});
new aws.apigatewayv2.Stage("default", {
  apiId: api.id,
  name: "$default",
  autoDeploy: true,
  defaultRouteSettings: { throttlingBurstLimit: 20, throttlingRateLimit: 10 },
});
new aws.lambda.Permission("invoke", {
  action: "lambda:InvokeFunction",
  function: fn.name,
  principal: "apigateway.amazonaws.com",
  sourceArn: pulumi.interpolate`${api.executionArn}/*/*`,
});
const oac = new aws.cloudfront.OriginAccessControl("site-access", {
  originAccessControlOriginType: "s3",
  signingBehavior: "always",
  signingProtocol: "sigv4",
});
const headers = new aws.cloudfront.ResponseHeadersPolicy("security", {
  securityHeadersConfig: {
    contentTypeOptions: { override: true },
    frameOptions: { frameOption: "DENY", override: true },
    referrerPolicy: { referrerPolicy: "no-referrer", override: true },
    strictTransportSecurity: {
      accessControlMaxAgeSec: 31536000,
      includeSubdomains: true,
      override: true,
    },
    contentSecurityPolicy: {
      override: true,
      contentSecurityPolicy:
        "default-src 'self'; script-src 'self'; style-src 'self' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data:; connect-src 'self'; frame-src https://maps.google.com; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'",
    },
  },
});
const distribution = new aws.cloudfront.Distribution("cdn", {
  enabled: true,
  aliases: [domain],
  defaultRootObject: "index.html",
  priceClass: "PriceClass_100",
  origins: [
    {
      originId: "site",
      domainName: bucket.bucketRegionalDomainName,
      originAccessControlId: oac.id,
    },
    {
      originId: "api",
      domainName: api.apiEndpoint.apply((u) => new URL(u).hostname),
      customOriginConfig: {
        httpPort: 80,
        httpsPort: 443,
        originProtocolPolicy: "https-only",
        originSslProtocols: ["TLSv1.2"],
      },
    },
  ],
  defaultCacheBehavior: {
    targetOriginId: "site",
    viewerProtocolPolicy: "redirect-to-https",
    allowedMethods: ["GET", "HEAD", "OPTIONS"],
    cachedMethods: ["GET", "HEAD"],
    compress: true,
    cachePolicyId: "658327ea-f89d-4fab-a63d-7e88639e58f6",
    responseHeadersPolicyId: headers.id,
  },
  orderedCacheBehaviors: [
    {
      pathPattern: "/api/*",
      targetOriginId: "api",
      viewerProtocolPolicy: "https-only",
      allowedMethods: [
        "GET",
        "HEAD",
        "OPTIONS",
        "PUT",
        "POST",
        "PATCH",
        "DELETE",
      ],
      cachedMethods: ["GET", "HEAD"],
      cachePolicyId: "413f160d-7a38-4b3d-9b16-12e677c2308c",
      originRequestPolicyId: "b689b0a8-53d0-40ab-baf2-68738e2966ac",
      responseHeadersPolicyId: headers.id,
    },
  ],
  restrictions: { geoRestriction: { restrictionType: "none" } },
  viewerCertificate: {
    acmCertificateArn: validated.certificateArn,
    sslSupportMethod: "sni-only",
    minimumProtocolVersion: "TLSv1.2_2021",
  },
});
new aws.s3.BucketPolicy("cdn-read", {
  bucket: bucket.id,
  policy: pulumi
    .all([bucket.arn, distribution.arn])
    .apply(([b, d]) =>
      JSON.stringify({
        Version: "2012-10-17",
        Statement: [
          {
            Effect: "Allow",
            Principal: { Service: "cloudfront.amazonaws.com" },
            Action: "s3:GetObject",
            Resource: `${b}/*`,
            Condition: { StringEquals: { "AWS:SourceArn": d } },
          },
        ],
      }),
    ),
});
new cloudflare.DnsRecord("site-dns", {
  zoneId,
  name: domain,
  type: "CNAME",
  content: distribution.domainName,
  proxied: false,
  ttl: 300,
});
const mime: Record<string, string> = {
  ".html": "text/html; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".webp": "image/webp",
  ".svg": "image/svg+xml",
};
function upload(dir: string, prefix = "") {
  for (const file of fs.readdirSync(dir, { withFileTypes: true })) {
    const key = prefix + file.name,
      local = path.join(dir, file.name);
    if (file.isDirectory()) {
      upload(local, key + "/");
      continue;
    }
    new aws.s3.BucketObjectv2("file-" + key.replace(/[^a-zA-Z0-9-]/g, "-"), {
      bucket: bucket.id,
      key,
      source: new pulumi.asset.FileAsset(local),
      contentType: mime[path.extname(key)] ?? "application/octet-stream",
      cacheControl: "public, max-age=300",
    });
  }
}
upload("../web");
export const siteUrl = pulumi.interpolate`https://${domain}`;
export const tableName = table.name;
export const distributionId = distribution.id;
export const apiUrl = api.apiEndpoint;
