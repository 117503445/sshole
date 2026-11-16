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

## 2026-11-16 在线判定与本机 SSH 就绪可观测

### 背景

- 现场事实：一台办公机（192.168.1.2，Agent 名 `desktop-8nkqc4t-318ddebf`）18:16 之后所有 SSH 都失败，
  但 Hub `ListAgents` 一直报 `online:true`，Entry 只能给出 `ssh: handshake failed: EOF`，
  排查方向被带到"内嵌 sshd 死了"，实际是机器被人关机了。
- 根因：`handleAgentWS` 的读循环只在收到帧时才感知断开；机器直接断电 TCP 不发 FIN，
  Hub 永远读不到 EOF，`state.Control` 一直非空，`Online: state.Control != nil` 就是假的在线。
  控制连接建立后 Agent 侧不发任何数据帧，Hub 也从不主动探测。
- 用户要求：「你先开始 sshole 和 luoxiagent 修复吧」，前一轮已明确要求加强可观测性、要能看到失败原因。

### 实现

1. `pkg/proto/control.go`：新增 `AgentStatus`（Agent → Hub 的 `STATUS` 帧）：版本、提交、构建时间、
   主机名、本机 SSH 端口、`ssh_ready`、`ssh_error`。
2. `pkg/agent/agent.go`：`keepaliveControl` 改为 `reportControlStatus`，连接建立后立即并每 20 秒发一帧
   状态（取代裸 ping，仍承担保活）；写失败即关闭连接触发重连。
3. `pkg/agent/sshd.go`：新增 `watchSSHD`/`restartSSHD`/`sshProbe`/`setSSHCleanup`/`closeSSHCleanup`，
   每 15 秒巡检内嵌 sshd 监听，消失就重启并把失败原因留在状态帧里；`SkipSSHD` 模式只探测不接管，
   避免误杀现场自己的 sshd。
4. `pkg/hub/config.go`、`cmd/hub/main.go`：新增 `AgentPingInterval`（`SSHOLE_HUB_AGENT_PING_INTERVAL`，默认 20s）
   与 `AgentPingTimeout`（`SSHOLE_HUB_AGENT_PING_TIMEOUT`，默认 10s）。
5. `pkg/hub/agent_ws.go`：控制连接建立后启动 `keepaliveAgent` 主动探测，pong 超时立即
   `markAgentOffline` 再关连接（coder/websocket 的本地 `Close` 不会立刻打断阻塞中的 `Read`，
   只依赖读循环收尾会继续假在线）；读循环解析 `STATUS` 帧并记录 `LastSeen`，断开原因写进 Hub 日志。
6. `pkg/hub/agents_api.go`：新增 `GET /api/agents`（Bearer 同 RPC），输出 online、last_seen_ms、
   status_reported、hostname、version、commit、ssh_port、ssh_ready、ssh_error。
7. `pkg/hub/tunnel_ws.go`：Agent 无控制连接时直接 `503` 并附最近状态与最后活跃时间，不再让 Entry 等满 pending 超时。
8. `pkg/entry/entry.go`：新增 `describeAgent`，启动、SSH 测试失败、隧道拨号失败三处都打印 `hub_view`；
   离线时明确写成"最后已知状态"，避免把过期状态读成现在可用。
9. `README.md`：补 Hub 新参数与「在线判定与状态查询」小节。

### 验证

- `gofmt -l` 对改动文件无输出；`CGO_ENABLED=0 go vet ./...` 通过；`go test ./... -count=1` 全绿
  （本机无可用 cgo，`-race` 由 GitHub CI 执行）。
- 新增测试：`TestAgentStatusAndLiveness`（状态帧进入 `/api/agents`；客户端不再回 pong 的半开连接必须被判离线）、
  `TestTunnelRejectsOfflineAgent`（离线 Agent 的 `/tunnel` 必须 503）、
  `TestSSHProbeReportsNotListening`、`TestRestartSSHDRestoresListener`（内嵌 sshd 监听消失后能重新拉起）。
- 本地真链路复现（`projects/luoxi/data/20261116-1845-sshole-observability/`，hub/agent/entry 均为本地构建，
  探测周期压到 1s）：
  1. 正常在线：`{"online":true,...,"hostname":"dev-nixos","ssh_port":22922,"ssh_ready":true,"status_reported":true}`。
  2. `kill -STOP` 冻结 Agent 模拟掉电半开：4 秒后 `online:false` 且 `last_seen_ms` 停住，
     Hub 日志出现 `agent liveness probe failed, marking offline`。
  3. 此时启动 Entry：`agent state ... state="online=false（Hub 已判定离线，机器多半已关机或断网）最后已知状态 hostname=dev-nixos ssh=22922 就绪 last_seen=2026-11-16T18:46:30+08:00"`，
     隧道拨号失败也带同一句 `hub_view`，不再是裸 EOF。
  4. `kill -CONT` 恢复：Agent 自动重连，`online:true` 且 `last_seen_ms` 重新推进。

### CI 往返修正

- 首次 CI（run 43852284769）`test` 失败：`TestRestartSSHDRestoresListener` 触发 `-race` 报数据竞争，
  竞争双方都在依赖 `sshdev/internal/server` 内部——`Start` 在端口已可接受连接之后仍会继续写 listener 字段，
  此时调用 `Stop` 就撞上。本地无可用 cgo（缺 crt），只能靠 CI 发现。
- 修正：`ensureSSHD` 在确认端口可见后再等 `sshdStartSettle`（250ms）才返回，使
  `closeSSHCleanup`/`restartSSHD` 不会在依赖未收敛时调用 `Stop`；这同时消除了生产路径上
  "安装后立刻停服务" 的同一窗口，而不是只把测试调松。

### CI 往返修正（第二次，根因更正）

- `551484c` 的 250ms 收敛窗口没有消除竞争，CI 仍报 `-race`。看依赖源码才确认根因：
  `sshdev/internal/server` 的 `Start` 写 `s.listener`（server.go:139）、`Stop` 读 `s.listener`
  （server.go:275），两者之间没有任何同步原语。竞争检测看的是 happens-before，不是时间间隔，
  所以"多等一会儿"原理上就不可能修好，跨协程调用 `Stop` 一定被判竞争。
- 正确做法：看护协程重启时**不再对已死的旧服务调 `Stop`**，只在新建成功后替换引用；
  监听消失本身说明旧服务 accept 循环已退出。进程退出路径仍调用一次 `Stop`（既有行为，
  不被测试覆盖），依赖侧的同步缺失作为已知问题记录，建议后续在 sshdev 内加锁修根。
- 相应删除 `sshdStartSettle`，测试改为不触发 `Stop`：`TestRestartSSHDRestoresListener`
  只验证能在同端口重建监听，`TestSSHProbeSurfacesRecordedFailure` 验证失败原因会进状态帧。
