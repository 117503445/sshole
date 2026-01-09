# FROM alpine:3.22.1
FROM alpine@sha256:eafc1edb577d2e9b458664a15f23ea1c370214193226069eb22921169fc7e43f
# FROM registry.cn-hangzhou.aliyuncs.com/117503445-mirror/sync@sha256:eafc1edb577d2e9b458664a15f23ea1c370214193226069eb22921169fc7e43f
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g' /etc/apk/repositories
RUN apk --update add ca-certificates openssh
WORKDIR /workspace

# Create SSH host keys directory
RUN mkdir -p /tmp/sshole_agent/opt/openssh/etc && \
    ssh-keygen -t ed25519 -f /tmp/sshole_agent/opt/openssh/etc/ssh_host_ed25519_key -N "" -C "sshole"

COPY data/agent/sshole_agent /workspace/sshole_agent

ENTRYPOINT [ "/workspace/sshole_agent" ]