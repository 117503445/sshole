# 工作记录

## 任务：多台税务电脑 Agent 名称冲突（2026-11-15）

### 用户交互记录

- 用户输入：「不对，如果我有多台税务电脑就冲突了。我想保证每个sshdev进程如果没有名字，就随机生成一个防止冲突」
- 背景：上一轮把 `--name` 默认值写死成 `temp-device`，两台税务电脑同时跑会抢同一个 Agent 名和同一个 Hub 端口。

### 需求

1. 不带 `--name`（也没有 `SSHOLE_AGENT_NAME`）时，名称必须唯一，不能多台机器撞名。
2. 显式传 `--name` 时行为不变。

### 实现步骤

1. `pkg/agent/naming.go`（新增）：`ResolveAgentName()` 生成 `<主机名>-<8位随机hex>`。
   保留主机名前缀是为了让管理员在 Hub 上认得出是哪台机器，随机后缀才承担防冲突；
   结果写入 `$HOME/.sshole/agent-name` 复用，否则每次重启都会在 Hub 上多留一个永久条目（端口映射是持久化的）。
   `sanitizeName()` 把名称收敛成 `X-Agent` 头能安全承载的字符集（小写字母、数字、`-`）。
2. `pkg/agent/lock.go`：改用共用的 `ssholeDir()`，去掉重复的 `$HOME/.sshole` 拼路径。
3. `cmd/agent/main.go`：`--name` 默认值从 `temp-device` 改为空；为空时调用 `ResolveAgentName()` 并打日志。
   原来回落到裸主机名，克隆镜像会重复，正好是用户要避免的情况。
4. Entry 的 `--agent-name` 默认值保持 `temp-device` 不动（不在本次需求内），但随机名生效后管理员要显式指定，已写进 README。
5. 同步 `README.md`、`docs/configuration.md`。

### 验证结果

- `gofmt -l` 对改动文件无输出；`CGO_ENABLED=0 go vet ./...` 通过；`go test ./... -count=1` 全部通过。
- `pkg/agent/naming_test.go`：稳定复用、20 次生成互不重复、名称形状与字符集、名字文件损坏时重新生成、`sanitizeName` 用例。
- 本地真起 Hub + 两个 Agent（同一台机器、两个不同 `HOME`、都不带 `--name`）：
  分别注册成 `dev-nixos-a46dad30`（端口 10000）和 `dev-nixos-52740bf8`（端口 10001），
  `port_mapping.json` 里两条独立映射；用同一个 `HOME` 重启 Agent 复用原名字；显式 `--name tax-pc-02` 时不生成、也不落名字文件。
- Windows 真机（WIN-7700K，Windows 11 专业版 26200，`nt authority\system`）用生产下载入口的 CI 产物实测：
  下载落地 14,083,584 字节、sha256 `C8096A8E…` 与 release 资产一致；进程自报 `GitCommit=00ca1799d7ca GitDirty=false`；
  不带 `--name` 启动生成 `win-7700k-1043f6c8` 并写入 `%USERPROFILE%\.sshole\agent-name`（SYSTEM 下即 `C:\WINDOWS\system32\config\systemprofile`）；
  重启进程复用同名（Hub 上仍是同一个 `win-7700k-1043f6c8`，端口 10014），内置 sshdev 监听 `127.0.0.1:22222`，`control connected` 后在线 7 分钟无断链。
  再换一个 `USERPROFILE` 起第二个实例（模拟第二台税务电脑），生成 `win-7700k-eecdf6b2` 并拿到独立端口 10015，
  两个名字在 Hub 上同时 `online:true`——这就是多台机器不互相顶掉的直接证据。
  控制链：dev 机用同 release 的 `sshole_entry-linux-amd64 --agent-name win-7700k-1043f6c8` 起 Entry，
  `ssh -p 22231 root@127.0.0.1` 经 Hub 隧道返回 `nt authority\system` / `WIN-7700K`。
  验证后已停进程并删除 `C:\Windows\Temp\sshole-verify` 与本次生成的 `systemprofile\.sshole`。

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

## 2026-11-16 启动横幅：版本、提交与本节点信息

### 背景

- 用户要求：「sshole luoxiagent 下载的文件名带有版本号，启动后也有版本号 commit 标识。sshole 和 luoxiagent 也要在醒目位置显示本节点的信息。」
- 现场问题：员工机上的 `sshole.exe` 是一个黑框，出问题时没人说得清「这是哪台机器、哪个构建」；JSON 日志里的 build info 也不是现场人员能读的形式。

### 实现

1. `pkg/common/banner.go`：新增 `CollectNodeInfo`、`PrintAgentBanner`、`PrintEntryBanner`，用纯文本块打印版本、提交（前 12 位）、构建时间、计算机名、本机地址、系统版本、Agent 名称、Hub 与本地 SSH 端口。
2. `pkg/common/osinfo_windows.go`、`pkg/common/osinfo_other.go`：Windows 用 `windows.RtlGetVersion()` 打印真实内部版本与构建号（不受清单伪装影响），其他平台只打印系统名与架构。
3. `cmd/agent/main.go`、`cmd/entry/main.go`：在配置解析完成后、进入连接循环前打印横幅，保证名称已解析、端口已确定。
4. `README.md`：新增「启动横幅（版本与本节点信息）」。

### 验证

- `gofmt -l` 无输出；`CGO_ENABLED=0 go vet ./...` 通过；`go test ./... -count=1` 全绿（`-race` 交给 CI）。
- `GOOS=windows go build ./...` 通过，横幅代码在 Windows 目标下编译。
- `go run ./cmd/agent` 实跑打印出横幅；本地未注入 ldflags 时版本、提交、构建时间显示 `unknown`，与本节点、Hub、端口一起仍可完整定位设备。
