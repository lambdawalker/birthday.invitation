package deploytools

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
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
