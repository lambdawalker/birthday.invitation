package deploytools

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// DeploymentDefaults contains only non-secret prompt defaults.
type DeploymentDefaults struct {
	ContactPhone      string `yaml:"contact_phone"`
	AWSProfile        string `yaml:"aws_profile"`
	AWSRegion         string `yaml:"aws_region"`
	StateBucket       string `yaml:"state_bucket"`
	CreateStateBucket string `yaml:"create_state_bucket"`
	Stack             string `yaml:"stack"`
	Domain            string `yaml:"domain"`
	CloudflareZoneID  string `yaml:"cloudflare_zone_id"`
	Deploy            string `yaml:"deploy"`
}

func loadDefaults(root string) (DeploymentDefaults, error) {
	cfg := DeploymentDefaults{
		AWSProfile: "default", AWSRegion: "us-east-1",
		StateBucket: "abi-pulumi-{account}-{region}", CreateStateBucket: "CREATE",
		Stack: "production", Domain: "xitlaliabigail.com", CloudflareZoneID: "auto", Deploy: "DEPLOY",
	}
	file, err := os.Open(filepath.Join(root, "deploy.defaults.yaml"))
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("cannot read deploy.defaults.yaml: %w", err)
	}
	defer file.Close()
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		// Do not echo YAML parser details, which can include accidentally pasted secrets.
		return cfg, errors.New("invalid deploy.defaults.yaml: use the documented non-secret fields and valid YAML")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return cfg, errors.New("deploy.defaults.yaml must contain exactly one YAML document")
	}
	for _, value := range []string{cfg.AWSProfile, cfg.AWSRegion, cfg.StateBucket, cfg.CreateStateBucket, cfg.Stack, cfg.Domain, cfg.CloudflareZoneID, cfg.Deploy} {
		if strings.TrimSpace(value) == "" {
			return cfg, errors.New("deploy.defaults.yaml values must not be empty")
		}
	}
	if cfg.CreateStateBucket != "CREATE" && cfg.CreateStateBucket != "CANCEL" {
		return cfg, errors.New("create_state_bucket must be CREATE or CANCEL")
	}
	if cfg.Deploy != "DEPLOY" && cfg.Deploy != "CANCEL" {
		return cfg, errors.New("deploy must be DEPLOY or CANCEL")
	}
	if cfg.ContactPhone != "" {
		cfg.ContactPhone, err = NormalizeContactPhone(cfg.ContactPhone)
		if err != nil {
			return cfg, err
		}
	}
	return cfg, nil
}

func NormalizeContactPhone(value string) (string, error) {
	value = strings.NewReplacer(" ", "", "-", "", "(", "", ")", "", ".", "").Replace(strings.TrimSpace(value))
	if !regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`).MatchString(value) {
		return "", errors.New("contact_phone must include + and the country code, for example +52 followed by 10 digits or +1 followed by 10 digits")
	}
	return value, nil
}

func (c DeploymentDefaults) bucket(account, region string) string {
	return strings.NewReplacer("{account}", account, "{region}", region).Replace(c.StateBucket)
}
