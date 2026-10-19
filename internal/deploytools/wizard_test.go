package deploytools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// This child process simulates AWS CLI output selection without credentials.
func TestAWSCLIHelper(t *testing.T) {
	if os.Getenv("ABI_AWS_TEST_HELPER") != "1" {
		return
	}
	args := os.Args
	format := os.Getenv("AWS_DEFAULT_OUTPUT")
	for i, arg := range args {
		if arg == "--output" && i+1 < len(args) {
			format = args[i+1]
		}
	}
	switch format {
	case "json":
		fmt.Print(`{"Account":"123456789012","Arn":"arn:aws:iam::123456789012:user/test","UserId":"test"}`)
	case "table":
		fmt.Print("+--------------------+\n| GetCallerIdentity |\n+--------------------+")
	case "yaml":
		fmt.Print("Account: '123456789012'\n")
	default:
		fmt.Print("ACCOUNT\t123456789012\n")
	}
	os.Exit(0)
}

func TestAWSIdentityIgnoresUserOutputFormat(t *testing.T) {
	t.Setenv("ABI_AWS_TEST_HELPER", "1")
	for _, format := range []string{"text", "table", "yaml", "json"} {
		t.Run(format, func(t *testing.T) {
			t.Setenv("AWS_DEFAULT_OUTPUT", format)
			w := &wizard{output: io.Discard, newCommand: func(name string, args ...string) *exec.Cmd {
				if name != "aws" {
					t.Fatalf("unexpected command: %s", name)
				}
				return exec.Command(os.Args[0], append([]string{"-test.run=^TestAWSCLIHelper$", "--"}, args...)...)
			}}
			result := w.command(t.TempDir(), "", true, "aws", "sts", "get-caller-identity")
			if result.err != nil {
				t.Fatal(result.err, result.stderr)
			}
			var identity struct{ Account string }
			if err := json.Unmarshal([]byte(result.stdout), &identity); err != nil {
				t.Fatalf("profile format %s broke identity parsing: %v", format, err)
			}
			if identity.Account != "123456789012" {
				t.Fatal("account not parsed")
			}
			if os.Getenv("AWS_DEFAULT_OUTPUT") != format {
				t.Fatal("user output preference was changed")
			}
		})
	}
}

func TestJSONErrorNamesCommandWithoutLeakingOutput(t *testing.T) {
	var result map[string]any
	err := decodeCommandJSON("aws sts get-caller-identity", "ACCOUNT private-account-data", &result)
	if err == nil || !strings.Contains(err.Error(), "aws sts get-caller-identity") {
		t.Fatalf("missing command context: %v", err)
	}
	if strings.Contains(err.Error(), "private-account-data") {
		t.Fatal("raw response leaked")
	}
}

func TestPromptDefaultsAndOverride(t *testing.T) {
	w := &wizard{input: strings.NewReader("\r\nCANCEL\r\n"), output: io.Discard}
	if got := w.ask("Create bucket", "CREATE"); got != "CREATE" {
		t.Fatal(got)
	}
	if got := w.ask("Deploy", "DEPLOY"); got != "CANCEL" {
		t.Fatal(got)
	}
}

func TestLookupZone(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		ok         bool
	}{
		{"matching", `{"success":true,"result":[{"id":"zone123","name":"xitlaliabigail.com"}]}`, 200, true},
		{"missing", `{"success":true,"result":[]}`, 200, false},
		{"wrong domain", `{"success":true,"result":[{"id":"zone123","name":"other.com"}]}`, 200, false},
		{"denied", `secret response`, 403, false},
		{"invalid", `secret response`, 200, false},
		{"api failure", `{"success":false,"result":[]}`, 200, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/zones" || r.URL.Query().Get("name") != "xitlaliabigail.com" || r.Header.Get("Authorization") != "Bearer test-token" {
					t.Error("incorrect lookup request")
				}
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			id, err := lookupZone(server.Client(), server.URL, "xitlaliabigail.com", "test-token")
			if tc.ok {
				if err != nil || id != "zone123" {
					t.Fatalf("%s %v", id, err)
				}
			} else if err == nil || strings.Contains(err.Error(), "secret response") || strings.Contains(err.Error(), "test-token") {
				t.Fatalf("unsafe or missing error: %v", err)
			}
		})
	}
}

// Simulate first-login onboarding when no stacks exist. Without the global
// flag this helper fails rather than hanging waiting for project creation.
func TestPulumiCLIHelper(t *testing.T) {
	if os.Getenv("ABI_PULUMI_TEST_HELPER") != "1" {
		return
	}
	found := false
	for _, arg := range os.Args {
		if arg == "--non-interactive" {
			found = true
		}
	}
	if !found {
		fmt.Fprintln(os.Stderr, "You don't have any stacks yet. Create a new Pulumi project")
		os.Exit(2)
	}
	dir, err := os.Getwd()
	if err != nil || filepath.Base(dir) != "infra" {
		os.Exit(3)
	}
	input, err := io.ReadAll(os.Stdin)
	if err != nil || string(input) != os.Getenv("ABI_EXPECTED_INPUT") {
		os.Exit(4)
	}
	fmt.Print("ok")
	os.Exit(0)
}

func TestPulumiCommandsSkipOnboarding(t *testing.T) {
	t.Setenv("ABI_PULUMI_TEST_HELPER", "1")
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "infra"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, input string
		capture     bool
		args        []string
	}{
		{"login", "", false, []string{"login", "s3://test-state/birthday-invitation"}},
		{"list", "", true, []string{"stack", "ls", "--json"}},
		{"init", "", false, []string{"stack", "init", "production", "--secrets-provider", "passphrase"}},
		{"secret", "test-secret\n", true, []string{"config", "set", "adminKey", "--secret"}},
		{"preview", "", false, []string{"preview", "--diff"}},
		{"up", "", false, []string{"up", "--yes", "--skip-preview"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ABI_EXPECTED_INPUT", tc.input)
			w := &wizard{root: root, output: io.Discard, newCommand: func(name string, args ...string) *exec.Cmd {
				if name != "pulumi" {
					t.Fatalf("unexpected command %s", name)
				}
				return exec.Command(os.Args[0], append([]string{"-test.run=^TestPulumiCLIHelper$", "--"}, args...)...)
			}}
			w.must(tc.input, tc.capture, append([]string{"pulumi"}, tc.args...)...)
		})
	}
}
