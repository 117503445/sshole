# 优化日志路径格式显示

## 主要内容和目的

在执行 `t test:e2e` 测试时，日志中显示的文件路径过长，例如：
```
../root/go/pkg/mod/github.com/117503445/sshdev@v0.0.0-20260305035020-6ae97e49d913/internal/server/server.go:97
```

期望显示为更简洁的格式：
```
github.com/117503445/sshdev@v0.0.0-20260305035020-6ae97e49d913/internal/server/server.go:97
```

## 更改内容描述

### 1. `pkg/common/logger.go`
- 添加 `SetCallerMarshalFunc()` 函数，设置全局的 `zerolog.CallerMarshalFunc`
- 使用正则表达式匹配 Go module cache 路径，提取模块路径格式
- 使用 `strings.Cut` 作为备用方案处理 `pkg/mod/` 路径

### 2. `cmd/agent/main.go`, `cmd/hub/main.go`, `cmd/entry/main.go`
- 在 `init()` 函数中，于 `glog.InitZeroLog()` 之后调用 `common.SetCallerMarshalFunc()`
- 确保 caller 路径格式化函数不被外部库覆盖

## 验证方法和结果

执行 `t test:e2e`，观察日志输出。日志路径现在正确显示为模块路径格式：
```
github.com/117503445/sshdev@v0.0.0-20260305035020-6ae97e49d913/internal/server/server.go:97
```