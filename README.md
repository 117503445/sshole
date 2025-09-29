# sshole

基于 Go 开发的 SSH 连接代理系统，通过 WebSocket 实现内网穿透，允许从开发机 SSH 连接到内网容器。

## 项目概述

本系统由三个核心组件组成：

- **Agent**: 运行在容器内，建立与 Hub 的 WebSocket 连接，转发容器内的 SSH 数据
- **Hub**: 运行在公网服务器，作为中转服务器，处理 Agent 和 Entry 之间的数据转发
- **Entry**: 运行在开发机，监听本地端口，转发 SSH 连接到 Hub

## fc

目标函数内有 curl

## TODO

- [x] 认证，防止非授权用户使用
- [] agent 追加 SSH 公钥
- [] 单实例多 conn，防止 SSHD 冲突
- [] sshd 临时文件夹
- [] 多版本 cli 的 fc 函数冲突
- [] fc3 instance
