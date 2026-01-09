package agent

import (
	"os"
	"path/filepath"
	"strings"
)

func appendAuthorizedKey(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	ssholeDir := filepath.Join(home, ".sshole")
	if err := os.MkdirAll(ssholeDir, 0o700); err != nil {
		return err
	}
	authorizedKeys := filepath.Join(ssholeDir, "authorized_keys")
	if data, err := os.ReadFile(authorizedKeys); err == nil {
		if strings.Contains(string(data), key) {
			return nil
		}
	}
	f, err := os.OpenFile(authorizedKeys, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(key + "\n")
	return err
}
