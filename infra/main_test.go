package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"sync"
	"testing"

	pr "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type graphMocks struct {
	mu        sync.Mutex
	resources map[string]pulumi.MockResourceArgs
}

func (m *graphMocks) Call(a pulumi.MockCallArgs) (pr.PropertyMap, error) { return a.Args, nil }
func (m *graphMocks) NewResource(a pulumi.MockResourceArgs) (string, pr.PropertyMap, error) {
	m.mu.Lock()
	m.resources[a.Name] = a
	m.mu.Unlock()
	out := a.Inputs.Copy()
	for _, key := range []pr.PropertyKey{"arn", "name", "certificateArn", "executionArn", "domainName", "bucketRegionalDomainName"} {
		if _, ok := out[key]; !ok {
			out[key] = pr.NewStringProperty(a.Name + "-" + string(key))
		}
	}
	out["apiEndpoint"] = pr.NewStringProperty("https://api.example.com")
	if a.Name == "tls" {
		out["domainValidationOptions"] = pr.NewArrayProperty([]pr.PropertyValue{pr.NewObjectProperty(pr.PropertyMap{"resourceRecordName": pr.NewStringProperty("_validate.example.com"), "resourceRecordValue": pr.NewStringProperty("_acm.example.com")})})
	}
	return a.Name + "-id", out, nil
}
func object(p pr.PropertyValue) pr.PropertyMap {
	if p.IsSecret() {
		p = p.SecretValue().Element
	}
	return p.ObjectValue()
}
func TestInfrastructureProtectionAndRouting(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"infra", "web", "build"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(root, "web", "index.html"), []byte("test"), 0600)
	f, err := os.Create(filepath.Join(root, "build", "lambda.zip"))
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	w, _ := z.Create("bootstrap")
	w.Write([]byte("test"))
	z.Close()
	f.Close()
	t.Chdir(filepath.Join(root, "infra"))
	mocks := &graphMocks{resources: map[string]pulumi.MockResourceArgs{}}
	err = pulumi.RunErr(define, pulumi.WithMocks("birthday-invitation", "test", mocks), func(info *pulumi.RunInfo) {
		info.Config = map[string]string{"birthday-invitation:cloudflareZoneId": "test-zone", "birthday-invitation:adminKey": "test-secret-which-is-at-least-32-characters"}
		info.ConfigSecretKeys = []string{"birthday-invitation:adminKey"}
	})
	if err != nil {
		t.Fatal(err)
	}
	table := mocks.resources["families"]
	if !table.Inputs["deletionProtectionEnabled"].BoolValue() || table.RegisterRPC == nil || !table.RegisterRPC.GetProtect() {
		t.Fatal("guest database must remain protected")
	}
	private := mocks.resources["private-site"].Inputs
	for _, key := range []pr.PropertyKey{"blockPublicAcls", "blockPublicPolicy", "ignorePublicAcls", "restrictPublicBuckets"} {
		if !private[key].BoolValue() {
			t.Errorf("S3 protection disabled: %s", key)
		}
	}
	cdn := mocks.resources["cdn"].Inputs
	// IDs verified against AWS's managed-policy catalog:
	// https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/using-managed-cache-policies.html
	if object(cdn["defaultCacheBehavior"])["cachePolicyId"].StringValue() != "658327ea-f89d-4fab-a63d-7e88639e58f6" {
		t.Fatal("static assets must use AWS Managed-CachingOptimized")
	}
	api := object(cdn["orderedCacheBehaviors"].ArrayValue()[0])
	if api["cachePolicyId"].StringValue() != "4135ea2d-6df8-44a3-9df3-4b5a84be39ad" {
		t.Fatal("private API must use AWS Managed-CachingDisabled")
	}
	if api["originRequestPolicyId"].StringValue() != "b689b0a8-53d0-40ab-baf2-68738e2966ac" {
		t.Fatal("API must forward auth and cookies")
	}
	fn := mocks.resources["api"].Inputs
	if !fn["environment"].ContainsSecrets() {
		t.Fatal("admin key lost Pulumi secret tracking")
	}
	if fn["runtime"].StringValue() != "provided.al2023" || fn["architectures"].ArrayValue()[0].StringValue() != "arm64" {
		t.Fatal("wrong Lambda target")
	}
	if mocks.resources["site-dns"].Inputs["proxied"].BoolValue() {
		t.Fatal("Cloudflare record must be DNS-only")
	}
	if mocks.resources["file-index-html"].Inputs["key"].StringValue() != "index.html" {
		t.Fatal("static asset key changed")
	}
	if mocks.resources["tls"].Provider == "" {
		t.Fatal("certificate needs explicit us-east-1 provider")
	}
	if mocks.resources["east"].Inputs["region"].StringValue() != "us-east-1" {
		t.Fatal("wrong certificate region")
	}
}
