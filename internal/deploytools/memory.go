package deploytools

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/crypto/scrypt"
	"gopkg.in/yaml.v3"
)

const memoryFile = ".deploy-memory.enc"
const memoryHeader = "ABI-DEPLOY-1\n"

type deploymentMemory struct {
	Account string
	Values  map[string]string
}

// Fixed KDF parameters keep malformed files from selecting excessive resource costs.
func memoryCipher(password string, salt []byte) (cipher.AEAD, error) {
	key, err := scrypt.Key([]byte(password), salt, 32768, 8, 1, 32)
	if err != nil {
		return nil, err
	}
	defer clear(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
func readMemory(path, password string) (deploymentMemory, error) {
	var m deploymentMemory
	data, err := os.ReadFile(path)
	if err != nil {
		return m, err
	}
	h := len(memoryHeader)
	if len(data) < h+16+12+16 || string(data[:h]) != memoryHeader {
		return m, errors.New("invalid deployment memory file; restore a backup rather than overwriting it")
	}
	aead, err := memoryCipher(password, data[h:h+16])
	if err != nil {
		return m, err
	}
	start := h + 16
	plain, err := aead.Open(nil, data[start:start+aead.NonceSize()], data[start+aead.NonceSize():], []byte(memoryHeader))
	if err != nil {
		return m, errors.New("cannot unlock deployment memory: incorrect passphrase or damaged file; saved data was not changed")
	}
	defer clear(plain)
	if json.Unmarshal(plain, &m) != nil || m.Values == nil {
		return m, errors.New("invalid deployment memory contents")
	}
	return m, nil
}
func writeMemory(path, password string, m deploymentMemory) error {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	aead, err := memoryCipher(password, salt)
	if err != nil {
		return err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return err
	}
	plain, err := json.Marshal(m)
	if err != nil {
		return err
	}
	defer clear(plain)
	data := append([]byte(memoryHeader), salt...)
	data = append(data, nonce...)
	data = aead.Seal(data, nonce, plain, []byte(memoryHeader))
	return replacePrivateFile(path, data)
}

// Rename a synced sibling file: a failed write leaves the previous vault intact.
func replacePrivateFile(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".deploy-memory-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
func (w *wizard) remember(name, prompt, fallback string) string {
	old := w.memory.Values[name]
	if old != "" && !w.reconfigure {
		return old
	}
	if old != "" {
		fallback = old
	}
	value := w.ask(prompt, fallback)
	w.memory.Values[name] = value
	return value
}
func (w *wizard) rememberSecret(name, env, label string) string {
	value := os.Getenv(env)
	if value == "" {
		value = w.memory.Values[name]
		if value == "" {
			value = w.secret(env, label)
		} else if w.reconfigure {
			value = w.hiddenValue(label+" (Enter keeps saved value)", value)
		}
	}
	w.memory.Values[name] = value
	return value
}

// Remove the legacy plaintext phone only after the encrypted copy is durable.
func migratePrivateDefault(root string) error {
	path := filepath.Join(root, "deploy.defaults.yaml")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var doc yaml.Node
	if yaml.Unmarshal(data, &doc) != nil {
		return errors.New("cannot migrate contact_phone in deploy.defaults.yaml")
	}
	if len(doc.Content) == 0 {
		return nil
	}
	node := doc.Content[0]
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == "contact_phone" && node.Content[i+1].Value != "" {
			node.Content[i+1].Value = ""
			node.Content[i+1].Tag = "!!str"
			data, err = yaml.Marshal(&doc)
			if err != nil {
				return err
			}
			return replacePrivateFile(path, data)
		}
	}
	return nil
}
