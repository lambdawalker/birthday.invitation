package deploytools

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// FindRoot walks upward so compiled commands work from any repository subfolder.
func FindRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil && strings.Contains(strings.ReplaceAll(string(data), "\r\n", "\n"), "module github.com/lambdawalker/birthday.invitation\n") {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("run this command from inside the birthday.invitation repository")
		}
		dir = parent
	}
}

func Build(root string) error {
	out := filepath.Join(root, "build")
	if err := os.MkdirAll(out, 0755); err != nil {
		return err
	}
	cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w", "-o", filepath.Join(out, "bootstrap"), "./cmd/lambda")
	cmd.Dir = root
	env := os.Environ()
	for _, pair := range []string{"GOOS=linux", "GOARCH=arm64", "CGO_ENABLED=0"} {
		key := strings.SplitN(pair, "=", 2)[0] + "="
		clean := []string{}
		for _, item := range env {
			if !strings.HasPrefix(item, key) {
				clean = append(clean, item)
			}
		}
		env = append(clean, pair)
	}
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build Lambda: %w", err)
	}
	return Archive(filepath.Join(out, "bootstrap"), filepath.Join(out, "lambda.zip"))
}

// Archive records Unix executable permissions even when built on Windows.
func Archive(binary, destination string) error {
	source, err := os.Open(binary)
	if err != nil {
		return err
	}
	defer source.Close()
	tmp, err := os.CreateTemp(filepath.Dir(destination), "lambda-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	z := zip.NewWriter(tmp)
	h := &zip.FileHeader{Name: "bootstrap", Method: zip.Deflate}
	h.SetMode(0755)
	h.SetModTime(time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC))
	w, err := z.CreateHeader(h)
	if err == nil {
		_, err = io.Copy(w, source)
	}
	closeZip := z.Close()
	closeFile := tmp.Close()
	if err != nil {
		return err
	}
	if closeZip != nil {
		return closeZip
	}
	if closeFile != nil {
		return closeFile
	}
	// Windows cannot rename over an existing file.
	if err = os.Remove(destination); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(tmp.Name(), destination)
}
