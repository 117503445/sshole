# 任务描述中文化

## 主要内容和目的

将 `scripts/tasks` 目录下所有 Taskfile 中的任务描述（desc）改为中文，并补充缺失的描述。

## 更改内容描述

| 文件 | 任务 | 描述（修改后） |
|------|------|---------------|
| `base/Taskfile.yml` | clear | 清屏（新增） |
| | default | 默认任务（从英文翻译） |
| | dev | 开发模式（新增） |
| `build/Taskfile.yml` | bin | 编译二进制文件（新增） |
| | docker | 构建 Docker 镜像（新增） |
| | release | 构建发布版本（新增） |
| `format/Taskfile.yml` | format | 格式化代码（新增） |
| `gen/Taskfile.yml` | gen | 生成 RPC 代码（新增） |
| `test/Taskfile.yml` | e2e | 端到端测试（新增） |

## 验证方法和结果

运行 `task --list-all` 可查看所有任务及其描述，确认中文描述正确显示。