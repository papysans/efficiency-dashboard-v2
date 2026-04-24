FROM golang:1.26.0 AS builder
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=v1.0.1
WORKDIR /app

COPY ./core /app/core
COPY ./kbcli/go.mod ./kbcli/go.sum /app/kbcli/

RUN go env -w CGO_ENABLED=0 && \
    go env -w GO111MODULE=on && \
    go env -w GOPROXY=https://goproxy.cn,https://mirrors.aliyun.com/goproxy,direct
RUN cd ./kbcli && go mod download

COPY ./kbcli /app/kbcli

RUN cd /app/kbcli && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w -X 'main.SoftwareVer=${VERSION}'" -o /app/kbcli *.go && \
    chmod 755 /app/kbcli

FROM alpine:3.21 AS runtime

ENV env prod
ENV TZ Asia/Shanghai
WORKDIR /app

# 安装cron和必要的工具
RUN apk add --no-cache dcron tzdata

COPY --from=builder /app/kbcli /app/kbcli

# 复制crontab相关脚本
COPY ./kbcli/scripts /app/scripts

# 设置脚本可执行权限
RUN chmod +x /app/kbcli
RUN chmod +x /app/scripts/*.sh

ENTRYPOINT ["/app/scripts/docker-entrypoint.sh"]
