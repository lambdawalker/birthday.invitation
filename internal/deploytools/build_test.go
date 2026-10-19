package deploytools

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestArchivePermissionsAndReproducibility(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "bootstrap")
	dst := filepath.Join(dir, "lambda.zip")
	if err := os.WriteFile(src, []byte("test binary"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Archive(src, dst); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(dst)
	if err := Archive(src, dst); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(dst)
	if !bytes.Equal(first, second) {
		t.Fatal("archive is not reproducible")
	}
	r, err := zip.OpenReader(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if len(r.File) != 1 || r.File[0].Name != "bootstrap" || r.File[0].Mode().Perm() != 0755 {
		t.Fatal("Lambda bootstrap must be executable at ZIP root")
	}
}
func TestFindRoot(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module github.com/lambdawalker/birthday.invitation\r\n"), 0600)
	nested := filepath.Join(dir, "some", "folder")
	os.MkdirAll(nested, 0700)
	got, err := FindRoot(nested)
	if err != nil || got != dir {
		t.Fatalf("%q %v", got, err)
	}
}
func TestBucketValidation(t *testing.T) {
	for _, name := range []string{"abi-pulumi-123456789012-us-east-1", "abc", "valid.bucket"} {
		if !ValidBucket(name) {
			t.Errorf("valid: %s", name)
		}
	}
	for _, name := range []string{"ab", "UPPER-case", "a..b", "192.168.1.1", "xn--bucket", "bucket--x-s3", "bad.-name", "bucket.mrap", "bad_name"} {
		if ValidBucket(name) {
			t.Errorf("invalid: %s", name)
		}
	}
}
