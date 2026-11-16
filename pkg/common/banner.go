package common

import (
	"fmt"
	"net"
	"os"
	"sort"
	"strings"

	"github.com/117503445/sshole/internal/buildinfo"
)

// maxBannerAddresses caps the local addresses printed in the banner so a
// multi-NIC host does not push the useful lines off screen.
const maxBannerAddresses = 8

// NodeInfo is the local machine identity. It answers "which machine is this
// window from" without opening the Hub, and carries no secrets.
type NodeInfo struct {
	Hostname  string
	Addresses []string
	OS        string
}

// CollectNodeInfo reads the local machine identity. Individual failures leave
// that field empty instead of stopping the binary from starting.
func CollectNodeInfo() NodeInfo {
	hostname, _ := os.Hostname()
	return NodeInfo{Hostname: strings.TrimSpace(hostname), Addresses: unicastAddresses(), OS: osDescription()}
}

// PrintAgentBanner prints the agent version, commit and node identity into the
// foreground console, which is the only place someone standing at the machine
// can read them.
func PrintAgentBanner(name, hub string, localPort int, skipSSHD bool) {
	node := CollectNodeInfo()
	sshd := "内置 sshd"
	if skipSSHD {
		sshd = "外部 sshd"
	}
	printBanner("SSHole Agent", [][2]string{
		{"版本", orUnknown(buildinfo.GitVersion)},
		{"提交", shortHash(buildinfo.GitCommit)},
		{"构建时间", orUnknown(buildinfo.BuildTime)},
		{"本节点", orUnknown(node.Hostname)},
		{"本机地址", orUnknown(strings.Join(node.Addresses, " "))},
		{"系统", orUnknown(node.OS)},
		{"Agent 名称", orUnknown(name)},
		{"Hub", orUnknown(hub)},
		{"本地 SSH", fmt.Sprintf("127.0.0.1:%d（%s）", localPort, sshd)},
	})
}

// PrintEntryBanner prints the entry version, commit and target agent, so the
// admin can see which node the tunnel is about to open.
func PrintEntryBanner(hub, agentName string, localPort int) {
	node := CollectNodeInfo()
	printBanner("SSHole Entry", [][2]string{
		{"版本", orUnknown(buildinfo.GitVersion)},
		{"提交", shortHash(buildinfo.GitCommit)},
		{"构建时间", orUnknown(buildinfo.BuildTime)},
		{"本节点", orUnknown(node.Hostname)},
		{"本机地址", orUnknown(strings.Join(node.Addresses, " "))},
		{"目标 Agent", orUnknown(agentName)},
		{"Hub", orUnknown(hub)},
		{"本地 SSH", fmt.Sprintf("127.0.0.1:%d", localPort)},
	})
}

// printBanner writes a plain-text block that stays readable in cmd.exe and
// PowerShell consoles, where zerolog JSON is not.
func printBanner(title string, fields [][2]string) {
	line := strings.Repeat("=", 64)
	fmt.Println(line)
	fmt.Println(title)
	for _, field := range fields {
		fmt.Printf("%s：%s\n", field[0], field[1])
	}
	fmt.Println(line)
}

// unicastAddresses returns global unicast addresses on enabled non-loopback
// interfaces, sorted and truncated for display.
func unicastAddresses() []string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	addresses := []string{}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		list, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, item := range list {
			ip, _, err := net.ParseCIDR(item.String())
			if err != nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
				continue
			}
			if value := ip.String(); !seen[value] {
				seen[value] = true
				addresses = append(addresses, value)
			}
		}
	}
	sort.Strings(addresses)
	if len(addresses) > maxBannerAddresses {
		addresses = addresses[:maxBannerAddresses]
	}
	return addresses
}

// shortHash keeps a commit readable in a banner without dropping the identity.
func shortHash(commit string) string {
	commit = strings.TrimSpace(commit)
	if commit == "" {
		return "unknown"
	}
	if len(commit) <= 12 {
		return commit
	}
	return commit[:12]
}

// orUnknown renders a placeholder so no banner line can be silently empty.
func orUnknown(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unknown"
	}
	return value
}
