# 环境变量添加组件前缀

## 主要内容和目的

为三个组件（Agent、Hub、Entry）的环境变量添加统一前缀，避免环境变量冲突，提高可维护性。

## 更改内容描述

### 代码变更

1. **cmd/agent/main.go**: Agent 环境变量添加 `SSHOLE_AGENT_` 前缀
2. **cmd/hub/main.go**: Hub 环境变量添加 `SSHOLE_HUB_` 前缀
3. **cmd/entry/main.go**: Entry 环境变量添加 `SSHOLE_ENTRY_` 前缀

### 文档变更

1. **README.md**: 更新环境变量表格
2. **docs/configuration.md**: 更新配置说明

### 环境变量映射

| 组件 | 前缀 |
|------|------|
| Agent | `SSHOLE_AGENT_` |
| Hub | `SSHOLE_HUB_` |
| Entry | `SSHOLE_ENTRY_` |

## 验证方法和结果

执行 `go build ./...` 编译通过，无错误。