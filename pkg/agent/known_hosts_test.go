package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendAuthorizedKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	key1 := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFakeKeyOne user1@host"
	key2 := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFakeKeyTwo user2@host"

	if err := appendAuthorizedKey(key1); err != nil {
		t.Fatalf("append key1: %v", err)
	}
	path := filepath.Join(home, ".sshole", "authorized_keys")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat authorized_keys: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("authorized_keys perm = %o, want 600", info.Mode().Perm())
	}

	// Idempotent: appending the same key must not duplicate it.
	if err := appendAuthorizedKey(key1); err != nil {
		t.Fatalf("re-append key1: %v", err)
	}
	if err := appendAuthorizedKey(key2); err != nil {
		t.Fatalf("append key2: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read authorized_keys: %v", err)
	}
	if got := strings.Count(string(data), key1); got != 1 {
		t.Fatalf("key1 appears %d times, want 1", got)
	}
	if !strings.Contains(string(data), key2) {
		t.Fatalf("key2 missing: %q", data)
	}

	// Empty key is a no-op and must not create garbage lines.
	before := string(data)
	if err := appendAuthorizedKey("   "); err != nil {
		t.Fatalf("append empty key: %v", err)
	}
	after, _ := os.ReadFile(path)
	if string(after) != before {
		t.Fatal("empty key changed authorized_keys")
	}
}
