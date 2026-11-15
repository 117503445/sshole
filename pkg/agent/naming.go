package agent

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

// nameSuffixLen 随机后缀的字节数，4 字节渲染成 8 位十六进制。
const nameSuffixLen = 4

// nameHostMax 主机名部分保留的最大长度，避免名称过长。
const nameHostMax = 32

var invalidNameChars = regexp.MustCompile(`[^a-z0-9-]+`)

// ResolveAgentName 在未显式指定 --name 时生成一个不会和其他机器冲突的 Agent 名称。
//
// 名称形如 <hostname>-<8位随机hex>：主机名让管理员在 Hub 上认得出是哪台机器，
// 随机后缀让克隆镜像里重复的主机名也不会互相顶掉。
// 结果写入 $HOME/.sshole/agent-name 并复用，同一台机器重启后仍是同一个 Agent，
// 不会每次启动都在 Hub 上留下一个新条目。
func ResolveAgentName() (string, error) {
	dir, err := ssholeDir()
	if err != nil {
		return "", err
	}

	if data, err := os.ReadFile(filepath.Join(dir, "agent-name")); err == nil {
		if name := sanitizeName(string(data)); name != "" {
			return name, nil
		}
	}

	host, err := os.Hostname()
	if err != nil {
		return "", err
	}
	base := sanitizeName(host)
	if base == "" {
		base = "agent"
	}
	if len(base) > nameHostMax {
		base = base[:nameHostMax]
	}

	buf := make([]byte, nameSuffixLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	name := base + "-" + hex.EncodeToString(buf)

	path := filepath.Join(dir, "agent-name")
	if err := os.WriteFile(path, []byte(name), 0o600); err != nil {
		// 写失败只降级为“下次启动换个名字”，不阻止本次连接。
		log.Warn().Err(err).Str("path", path).Msg("persist agent name failed, name will change on next start")
	}

	return name, nil
}

// sanitizeName 把输入收敛成 Hub 的 X-Agent 头能安全承载的字符集。
func sanitizeName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.NewReplacer("_", "-", ".", "-").Replace(s)
	s = invalidNameChars.ReplaceAllString(s, "")
	return strings.Trim(s, "-")
}

// ssholeDir 返回并创建 $HOME/.sshole 目录，存放单实例锁和 Agent 名称。
func ssholeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".sshole")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}
