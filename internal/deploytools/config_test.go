package deploytools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeploymentDefaults(t *testing.T) {
	root := t.TempDir()
	cfg, err := loadDefaults(root)
	if err != nil || cfg.Domain != "xitlaliabigail.com" || cfg.bucket("123", "us-east-1") != "abi-pulumi-123-us-east-1" {
		t.Fatalf("%+v %v", cfg, err)
	}
	err = os.WriteFile(filepath.Join(root, "deploy.defaults.yaml"), []byte("aws_profile: attestra\ncreate_state_bucket: CANCEL\nstate_bucket: custom-{account}-{region}\n"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err = loadDefaults(root)
	if err != nil || cfg.AWSProfile != "attestra" || cfg.Domain != "xitlaliabigail.com" || cfg.CreateStateBucket != "CANCEL" || cfg.bucket("123", "us-west-2") != "custom-123-us-west-2" {
		t.Fatalf("%+v %v", cfg, err)
	}
}

func TestInvalidDeploymentDefaults(t *testing.T) {
	for _, input := range []string{"aws_profile: [", "access_token: secret", "domain: ''", "deploy: yes", "create_state_bucket: yes", "domain: a\ndomain: b", "domain: example.com\n---\nstack: other"} {
		t.Run(input, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "deploy.defaults.yaml"), []byte(input), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := loadDefaults(root); err == nil {
				t.Fatal("expected invalid config error")
			}
		})
	}
}

func TestCheckedInDeploymentDefaults(t *testing.T) {
	cfg, err := loadDefaults(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Stack != "production" || cfg.CloudflareZoneID != "auto" {
		t.Fatalf("%+v", cfg)
	}
}
