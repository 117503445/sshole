FROM registry.cn-hangzhou.aliyuncs.com/117503445-mirror/sync@sha256:c3dedc45bf4e47a3d94eeb6aa65a1f085e3810c6cc54a33633cd050e03ed2a13 AS builder
# FROM golang:1.25.1-bookworm

RUN go env -w GOPROXY=https://goproxy.cn,direct

RUN go install mvdan.cc/garble@latest

WORKDIR /workspace

COPY go.mod go.sum ./

RUN go mod download

COPY cmd ./cmd
COPY pkg ./pkg
COPY scripts/build-bin.sh ./scripts/

RUN ./scripts/build-bin.sh

# FROM alpine:3.22.1 AS certs
FROM registry.cn-hangzhou.aliyuncs.com/117503445-mirror/sync@sha256:eafc1edb577d2e9b458664a15f23ea1c370214193226069eb22921169fc7e43f AS certs
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g' /etc/apk/repositories
RUN apk --update add ca-certificates

FROM scratch

COPY --from=builder /workspace/sshole /workspace/sshole
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

WORKDIR /workspace

ENTRYPOINT [ "/workspace/sshole", "fc" ]