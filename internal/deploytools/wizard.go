package deploytools

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/term"
)

var bucketPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)

func ValidBucket(s string) bool {
	if !bucketPattern.MatchString(s) || strings.Contains(s, "..") || strings.Contains(s, ".-") || strings.Contains(s, "-.") || net.ParseIP(s) != nil {
		return false
	}
	for _, p := range []string{"xn--", "sthree-", "amzn-s3-demo-"} {
		if strings.HasPrefix(s, p) {
			return false
		}
	}
	for _, p := range []string{"-s3alias", "--ol-s3", ".mrap", "--x-s3", "--table-s3"} {
		if strings.HasSuffix(s, p) {
			return false
		}
	}
	return true
}
func CheckTools() error {
	missing := []string{}
	for _, name := range []string{"go", "aws", "pulumi"} {
		if _, err := exec.LookPath(name); err != nil {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("install these tools first: %s. See README.md", strings.Join(missing, ", "))
	}
	return nil
}

type commandResult struct {
	stdout, stderr string
	err            error
}
type wizard struct {
	root       string
	input      io.Reader
	output     io.Writer
	newCommand func(string, ...string) *exec.Cmd
}
type aborted struct{ err error }

func stop(err error) {
	if err != nil {
		panic(aborted{err})
	}
}
func (w *wizard) step(n int, title string) {
	if w.output == os.Stdout && term.IsTerminal(int(os.Stdout.Fd())) && os.Getenv("NO_COLOR") == "" {
		fmt.Fprintf(w.output, "\n\033[1;32m%d  %s\033[0m\n%s\n", n, title, strings.Repeat("─", 55))
	} else {
		fmt.Fprintf(w.output, "\n%d  %s\n%s\n", n, title, strings.Repeat("─", 55))
	}
}
func (w *wizard) ask(prompt, def string) string {
	fmt.Fprint(w.output, prompt)
	if def != "" {
		fmt.Fprintf(w.output, " [%s]", def)
	}
	fmt.Fprint(w.output, ": ")
	// Read exactly one line; do not prefetch data intended for a hidden prompt.
	var b strings.Builder
	one := make([]byte, 1)
	for {
		n, err := w.input.Read(one)
		if n > 0 {
			if one[0] == '\n' {
				break
			}
			b.WriteByte(one[0])
		}
		if err != nil {
			stop(fmt.Errorf("input cancelled: %w", err))
		}
	}
	answer := strings.TrimSpace(b.String())
	if answer == "" {
		return def
	}
	return answer
}
func (w *wizard) secret(env, label string) string {
	if s := os.Getenv(env); s != "" {
		return s
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		stop(fmt.Errorf("%s requires a terminal for hidden input; alternatively, set %s", label, env))
	}
	fmt.Fprintf(w.output, "%s (hidden): ", label)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(w.output)
	stop(err)
	if len(b) == 0 {
		stop(errors.New("the secret must not be empty"))
	}
	return string(b)
}
func (w *wizard) command(dir, input string, capture bool, args ...string) commandResult {
	// Machine-readable output must not inherit a user's text/table/yaml profile.
	// Command-line options override AWS_DEFAULT_OUTPUT without modifying it.
	if capture && args[0] == "aws" {
		args = append(append([]string(nil), args...), "--output", "json", "--no-cli-pager", "--color", "off")
	}
	// The wizard owns prompts. Pulumi must not enter first-login onboarding
	// or consume answers intended for subsequent wizard prompts.
	if args[0] == "pulumi" {
		args = append(append([]string(nil), args...), "--non-interactive")
	}
	newCommand := w.newCommand
	if newCommand == nil {
		newCommand = exec.Command
	}
	cmd := newCommand(args[0], args[1:]...)
	cmd.Dir = dir
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	} else if args[0] != "pulumi" {
		cmd.Stdin = os.Stdin
	}
	var out, stderr bytes.Buffer
	if capture {
		cmd.Stdout = &out
		cmd.Stderr = &stderr
	} else {
		cmd.Stdout = w.output
		cmd.Stderr = w.output
	}
	err := cmd.Run()
	return commandResult{out.String(), stderr.String(), err}
}
func (w *wizard) must(input string, capture bool, args ...string) string {
	r := w.command(filepath.Join(w.root, "infra"), input, capture, args...)
	if r.err != nil {
		stop(fmt.Errorf("%s failed: %w. Check credentials, permissions and stack state; resources were not rolled back", args[0], r.err))
	}
	return r.stdout
}

func Deploy(root string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if a, ok := r.(aborted); ok {
				err = a.err
			} else {
				panic(r)
			}
		}
	}()
	defaults, configErr := loadDefaults(root)
	stop(configErr)
	w := &wizard{root: root, input: os.Stdin, output: os.Stdout}
	fmt.Fprintln(w.output, "\n✦ ABI · Deployment wizard\n  Go + AWS + Cloudflare · private S3 state")
	w.step(1, "Check prerequisites")
	stop(CheckTools())
	w.step(2, "Connect to AWS")
	profile := w.ask("AWS SSO profile", envDefault("AWS_PROFILE", defaults.AWSProfile))
	stop(os.Setenv("AWS_PROFILE", profile))
	stop(os.Setenv("AWS_PAGER", ""))
	if r := w.command(root, "", true, "aws", "sts", "get-caller-identity"); r.err != nil {
		fmt.Fprintln(w.output, "AWS credentials are unavailable or expired. Starting SSO login. If this profile is not configured, run aws configure sso first.")
		w.must("", false, "aws", "sso", "login", "--profile", profile)
	}
	var identity struct{ Account string }
	stop(decodeCommandJSON("aws sts get-caller-identity", w.must("", true, "aws", "sts", "get-caller-identity"), &identity))
	if identity.Account == "" {
		stop(errors.New("AWS did not return an account ID"))
	}
	fmt.Fprintln(w.output, "AWS account:", identity.Account)
	region := w.ask("AWS region", defaults.AWSRegion)
	stop(os.Setenv("AWS_REGION", region))
	stop(os.Setenv("AWS_DEFAULT_REGION", region))
	w.step(3, "Prepare private Pulumi state")
	bucket := w.ask("State bucket", defaults.bucket(identity.Account, region))
	if !ValidBucket(bucket) {
		stop(errors.New("invalid bucket name"))
	}
	head := w.command(root, "", true, "aws", "s3api", "head-bucket", "--bucket", bucket)
	if head.err != nil {
		if !strings.Contains(head.stderr, "404") && !strings.Contains(head.stderr, "Not Found") {
			stop(errors.New("cannot access the state bucket; check permissions. It will not be created without confirming that it is missing"))
		}
		if w.ask("Create the state bucket (creates AWS resources)? enter CREATE or CANCEL", defaults.CreateStateBucket) != "CREATE" {
			return nil
		}
		args := []string{"aws", "s3api", "create-bucket", "--bucket", bucket}
		if region != "us-east-1" {
			args = append(args, "--create-bucket-configuration", "LocationConstraint="+region)
		}
		w.must("", true, args...)
	}
	w.must("", true, "aws", "s3api", "put-public-access-block", "--bucket", bucket, "--public-access-block-configuration", "BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true")
	w.must("", true, "aws", "s3api", "put-bucket-versioning", "--bucket", bucket, "--versioning-configuration", "Status=Enabled")
	w.must("", true, "aws", "s3api", "put-bucket-encryption", "--bucket", bucket, "--server-side-encryption-configuration", `{"Rules":[{"ApplyServerSideEncryptionByDefault":{"SSEAlgorithm":"AES256"}}]}`)
	stop(os.Setenv("PULUMI_CONFIG_PASSPHRASE", w.secret("PULUMI_CONFIG_PASSPHRASE", "Pulumi encryption passphrase (save it and reuse it)")))
	w.must("", false, "pulumi", "login", "s3://"+bucket+"/birthday-invitation")
	stack := w.ask("Stack", defaults.Stack)
	if !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(stack) {
		stop(errors.New("invalid stack name"))
	}
	// A failed select can mean a network/auth error, not a missing stack.
	var stacks []struct {
		Name string `json:"name"`
	}
	stop(decodeCommandJSON("pulumi stack ls --json", w.must("", true, "pulumi", "stack", "ls", "--json"), &stacks))
	exists := false
	for _, s := range stacks {
		if s.Name == stack || strings.HasSuffix(s.Name, "/"+stack) {
			exists = true
		}
	}
	if exists {
		w.must("", false, "pulumi", "stack", "select", stack)
	} else {
		w.must("", false, "pulumi", "stack", "init", stack, "--secrets-provider", "passphrase")
	}
	w.step(4, "Domain and secrets")
	domain := w.ask("Domain", defaults.Domain)
	zone := w.ask("Cloudflare Zone ID (auto detects the domain)", envDefault("CLOUDFLARE_ZONE_ID", defaults.CloudflareZoneID))

	stop(os.Setenv("CLOUDFLARE_API_TOKEN", w.secret("CLOUDFLARE_API_TOKEN", "Cloudflare token (DNS Edit and Zone Read, scoped to this zone)")))
	if strings.EqualFold(zone, "auto") {
		var err error
		zone, err = lookupZone(&http.Client{Timeout: 30 * time.Second}, "https://api.cloudflare.com/client/v4", domain, os.Getenv("CLOUDFLARE_API_TOKEN"))
		stop(err)
		fmt.Fprintln(w.output, "Cloudflare Zone ID:", zone)
	}

	for _, pair := range [][2]string{{"aws:region", region}, {"domain", domain}, {"cloudflareZoneId", zone}} {
		w.must("", false, "pulumi", "config", "set", pair[0], pair[1])
	}
	var configs map[string]json.RawMessage
	stop(decodeCommandJSON("pulumi config --json", w.must("", true, "pulumi", "config", "--json"), &configs))
	for _, name := range []string{"adminKey", "importNamespace"} {
		if _, ok := configs["birthday-invitation:"+name]; ok {
			continue
		}
		if _, ok := configs[name]; ok {
			continue
		}
		b := make([]byte, 40)
		_, e := rand.Read(b)
		stop(e)
		w.must(base64.RawURLEncoding.EncodeToString(b)+"\n", true, "pulumi", "config", "set", name, "--secret")
	}
	fmt.Fprintln(w.output, "Secrets encrypted. Values are not printed to logs.")
	w.step(5, "Build and preview")
	stop(Build(root))
	r := w.command(root, "", false, "go", "build", "-o", filepath.Join(root, "build", "infra-check"), "./infra")
	stop(r.err)
	w.must("", false, "pulumi", "preview", "--diff")
	w.step(6, "Deploy")
	if w.ask("Review the preview. Enter DEPLOY or CANCEL", defaults.Deploy) != "DEPLOY" {
		fmt.Fprintln(w.output, "No application changes applied. The state bucket may already have been prepared.")
		return nil
	}
	w.must("", false, "pulumi", "up", "--yes", "--skip-preview")
	fmt.Fprintln(w.output, "\n✓ Done. Open https://"+domain+"/admin.html")
	fmt.Fprintln(w.output, "From infra: pulumi config get adminKey (privately). For imports: pulumi config get importNamespace and pulumi stack output tableName.")
	return nil
}

// Do not include raw responses: config JSON can contain sensitive information.
func decodeCommandJSON(command, output string, target any) error {
	if err := json.Unmarshal([]byte(output), target); err != nil {
		return fmt.Errorf("%s did not return valid JSON. Check the CLI installation and any command wrappers: %w", command, err)
	}
	return nil
}

func envDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

// Resolve the exact domain only; never silently select another accessible zone.
func lookupZone(client *http.Client, baseURL, domain, token string) (string, error) {
	domain = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), "."))
	req, err := http.NewRequest(http.MethodGet, baseURL+"/zones?name="+url.QueryEscape(domain), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Cloudflare zone lookup failed; check your connection or enter the Zone ID manually")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Cloudflare zone lookup returned HTTP %d; check token Zone Read permission or enter the Zone ID manually", resp.StatusCode)
	}
	var body struct {
		Success bool `json:"success"`
		Result  []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"result"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return "", errors.New("Cloudflare zone lookup returned invalid JSON")
	}
	if !body.Success || len(body.Result) != 1 || body.Result[0].ID == "" || !strings.EqualFold(body.Result[0].Name, domain) {
		return "", errors.New("Cloudflare did not return exactly one matching zone; check the domain and token scope or enter the Zone ID manually")
	}
	return body.Result[0].ID, nil
}
