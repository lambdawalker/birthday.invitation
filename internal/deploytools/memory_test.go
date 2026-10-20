package deploytools

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMemoryEncryptionAndRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), memoryFile)
	original := deploymentMemory{Account: "123456789012", Values: map[string]string{"cloudflareToken": "private-token", "contactPhone": "+14045550101", "region": "us-east-1"}}
	password := "a long testing passphrase"
	if err := writeMemory(path, password, original); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	for _, secret := range []string{"private-token", "+14045550101", "us-east-1"} {
		if bytes.Contains(data, []byte(secret)) {
			t.Fatal("plaintext in vault")
		}
	}
	got, err := readMemory(path, password)
	if err != nil || got.Values["contactPhone"] != original.Values["contactPhone"] || got.Account != original.Account {
		t.Fatal("roundtrip failed", err)
	}
	if _, err = readMemory(path, "wrong password"); err == nil {
		t.Fatal("accepted wrong password")
	}
	unchanged, _ := os.ReadFile(path)
	if !bytes.Equal(data, unchanged) {
		t.Fatal("failed unlock modified vault")
	}
	if err = writeMemory(path, password, original); err != nil {
		t.Fatal(err)
	}
	fresh, _ := os.ReadFile(path)
	if bytes.Equal(data, fresh) {
		t.Fatal("salt/nonce reused")
	}
	fresh[len(fresh)-1] ^= 1
	os.WriteFile(path, fresh, 0600)
	if _, err = readMemory(path, password); err == nil {
		t.Fatal("accepted tampered ciphertext")
	}
	for _, bad := range [][]byte{nil, []byte(memoryHeader), bytes.Repeat([]byte("x"), 80)} {
		os.WriteFile(path, bad, 0600)
		if _, err = readMemory(path, password); err == nil {
			t.Fatal("accepted malformed vault")
		}
	}
}
func TestRememberSkipsPromptsAndReconfigures(t *testing.T) {
	var out bytes.Buffer
	w := &wizard{input: strings.NewReader("new-region\n"), output: &out, memory: deploymentMemory{Values: map[string]string{"region": "saved-region"}}}
	if got := w.remember("region", "Region", "default"); got != "saved-region" || out.Len() != 0 {
		t.Fatal("saved answer prompted")
	}
	w.reconfigure = true
	if got := w.remember("region", "Region", "default"); got != "new-region" || w.memory.Values["region"] != got {
		t.Fatal("reconfigure not remembered")
	}
	t.Setenv("ABI_TEST_SECRET", "")
	w.reconfigure = false
	w.memory.Values["token"] = "saved-secret"
	out.Reset()
	if got := w.rememberSecret("token", "ABI_TEST_SECRET", "Token"); got != "saved-secret" || out.Len() != 0 {
		t.Fatal("secret not reused silently")
	}
	t.Setenv("ABI_TEST_SECRET", "replacement")
	if got := w.rememberSecret("token", "ABI_TEST_SECRET", "Token"); got != "replacement" || w.memory.Values["token"] != got {
		t.Fatal("environment rotation not saved")
	}
}
func TestPrivateDefaultMigration(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "deploy.defaults.yaml")
	os.WriteFile(path, []byte("# retain comment\naws_region: us-east-2\ncontact_phone: '+14045550101'\n"), 0600)
	if err := migratePrivateDefault(root); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if bytes.Contains(data, []byte("+14045550101")) || !bytes.Contains(data, []byte("us-east-2")) || !bytes.Contains(data, []byte("retain comment")) {
		t.Fatal("migration damaged defaults")
	}
	if err := migratePrivateDefault(root); err != nil {
		t.Fatal(err)
	}
}
