# 2025.12.23
FROM registry.cn-hangzhou.aliyuncs.com/117503445-mirror/dev@sha256:fd7e4d56b240dae5fccb8aa6639c6b97d37089498900bde43a9719bacb60e74d

RUN go install github.com/bufbuild/buf/cmd/buf@latest && go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest && go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && go install connectrpc.com/connect/cmd/protoc-gen-connect-go@latest
# RUN pnpm add -g @bufbuild/protoc-gen-es