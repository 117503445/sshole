# FROM alpine:3.22.1
FROM alpine@sha256:eafc1edb577d2e9b458664a15f23ea1c370214193226069eb22921169fc7e43f
# FROM registry.cn-hangzhou.aliyuncs.com/117503445-mirror/sync@sha256:eafc1edb577d2e9b458664a15f23ea1c370214193226069eb22921169fc7e43f
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g' /etc/apk/repositories
RUN apk --update add ca-certificates openssh-client
WORKDIR /workspace

COPY data/entry/sshole_entry /workspace/sshole_entry

ENTRYPOINT [ "/workspace/sshole_entry" ]