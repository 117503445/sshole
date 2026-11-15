package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestResolveAgentNameStable 验证同一台机器（同一个 HOME）重复调用拿到同一个名字，
// 这样 agent 重启不会在 Hub 上留下越来越多的一次性条目。
func TestResolveAgentNameStable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	first, err := ResolveAgentName()
	if err != nil {
		t.Fatalf("ResolveAgentName: %v", err)
	}
	second, err := ResolveAgentName()
	if err != nil {
		t.Fatalf("ResolveAgentName second: %v", err)
	}
	if first != second {
		t.Fatalf("name not stable: %q != %q", first, second)
	}
}

// TestResolveAgentNameUniquePerMachine 验证主机名相同时（克隆镜像）不同机器仍拿到不同名字。
func TestResolveAgentNameUniquePerMachine(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 20; i++ {
		t.Setenv("HOME", t.TempDir())
		name, err := ResolveAgentName()
		if err != nil {
			t.Fatalf("ResolveAgentName: %v", err)
		}
		if seen[name] {
			t.Fatalf("duplicated agent name %q", name)
		}
		seen[name] = true
	}
}

// TestResolveAgentNameShape 验证名字带主机名前缀和随机后缀，且不含非法字符。
func TestResolveAgentNameShape(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	name, err := ResolveAgentName()
	if err != nil {
		t.Fatalf("ResolveAgentName: %v", err)
	}
	host, err := os.Hostname()
	if err != nil {
		t.Fatalf("hostname: %v", err)
	}
	base := sanitizeName(host)
	if base == "" {
		base = "agent"
	}
	if !strings.HasPrefix(name, base+"-") {
		t.Fatalf("name %q does not start with sanitized hostname %q", name, base)
	}
	suffix := strings.TrimPrefix(name, base+"-")
	if len(suffix) != nameSuffixLen*2 {
		t.Fatalf("random suffix %q length = %d, want %d", suffix, len(suffix), nameSuffixLen*2)
	}
	if got := sanitizeName(name); got != name {
		t.Fatalf("name %q is not sanitized, got %q", name, got)
	}
}

// TestResolveAgentNameRegenerated 验证名字文件为空或含非法字符时重新生成并覆盖。
func TestResolveAgentNameRegenerated(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".sshole")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, "agent-name")

	for _, bad := range []string{"  \n", "---", "主机"} {
		if err := os.WriteFile(path, []byte(bad), 0o600); err != nil {
			t.Fatalf("write %q: %v", bad, err)
		}
		name, err := ResolveAgentName()
		if err != nil {
			t.Fatalf("ResolveAgentName(%q): %v", bad, err)
		}
		if name == "" {
			t.Fatalf("name file %q did not trigger regeneration", bad)
		}
		stored, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read back name: %v", err)
		}
		if string(stored) != name {
			t.Fatalf("stored %q, want %q", stored, name)
		}
	}
}

// TestSanitizeName 验证主机名被收敛成 X-Agent 头可安全承载的字符集。
func TestSanitizeName(t *testing.T) {
	cases := map[string]string{
		"WIN-7700K":       "win-7700k",
		"  DESKTOP_ABC  ": "desktop-abc",
		"Tax.PC.01":       "tax-pc-01",
		"---":             "",
		"主机":              "",
		"Host!@#$%Name":   "hostname",
	}
	for in, want := range cases {
		if got := sanitizeName(in); got != want {
			t.Errorf("sanitizeName(%q) = %q, want %q", in, got, want)
		}
	}
}
