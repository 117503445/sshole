FROM registry.cn-hangzhou.aliyuncs.com/117503445-mirror/sync@sha256:c3dedc45bf4e47a3d94eeb6aa65a1f085e3810c6cc54a33633cd050e03ed2a13
# FROM golang:1.25.1-bookworm

RUN go env -w GOPROXY=https://goproxy.cn,direct

RUN go install mvdan.cc/garble@latest

WORKDIR /workspace

COPY go.mod go.sum ./

RUN go mod download

ENTRYPOINT [ "./scripts/build.sh" ]