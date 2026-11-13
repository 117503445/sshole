# 工作记录

## 任务：调试 Agent 掉线后不再自愈（2026-11-13）

### 用户交互记录

- 用户输入：税务系统电脑跑 `sshole.exe` 连生产 Hub，管理员侧连不上；要求“修好，预期他从 luoxiadmin 下载的 sshole 能解决这个问题。luoxiadmin 要发版”。
- 现场证据：Hub（ali02 `sshole-hub-sshole-1`）日志里 `temp-device` 只在 2026-11-13 10:39:23 上线，10:39:53 掉线，只活了 30 秒，之后 6 小时再未重连。

### 需求

1. 控制连接空闲被中间网络设备掐断后，Agent 必须能长期保持在线。
2. 一次短暂断网不能让调试 Agent 永久退出，重连不能只试 3 次。
3. Windows 员工机双击即用的默认值（Hub 地址、Token、`temp-device` 名称、`powershell.exe` shell）要进主干，并由 CI 出 Windows Agent 包，供 luoxi-admin 分发。

### 实现步骤

1. `pkg/agent/agent.go`：`readControl` 期间每 20 秒向 Hub 发一次 WS ping（`controlKeepaliveInterval`），10 秒收不到 pong 就关闭连接；重连改为不限次数，退避按 `AgentReconnectBackoff` 递增、上限 30 秒，且退避可被 ctx 取消。
2. `pkg/agent/config.go`、`pkg/common/timeouts.go`、`pkg/common/timeouts_test.go`：删除已无用的 `AgentReconnectMaxRetries`。
3. `cmd/agent/main.go`、`cmd/entry/main.go`：内置生产 Hub 地址、Token 和 `temp-device` 默认名，不带参数即可运行。
4. `pkg/agent/sshd.go`：Windows 下内置 SSHD 默认 shell 改为 `powershell.exe`，否则 sshlib 校验 `/bin/sh` 直接启动失败。
5. `scripts/script/release.go`：Agent 增加 `windows/amd64` 与 `windows/arm64` 出包，CI release 产出 `sshole_agent-windows-amd64.exe`。
6. 同步 `README.md`、`docs/configuration.md`、`docs/design.detailed.md`。

### 验证结果

- `go vet ./...` 通过；`go test ./... -count=1` 全部通过（本机无 cgo，`-race` 由 CI 执行）。
- `GOOS=windows GOARCH=amd64 go build ./cmd/agent`、`./cmd/entry` 均编译通过。
- `go run . release` 本地跑通，产出 `data/release/sshole_agent-windows-amd64.exe` 等全部平台产物。
- 真机验证：用 CI 产出的 Linux Agent 连生产 Hub 观察控制连接是否跨过 30 秒；并把 Hub 地址指向不可达端口，确认进程持续重连不退出。
