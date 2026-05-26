FROM --platform=$BUILDPLATFORM golang:1.26.0 AS builder
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=v1.0.1
WORKDIR /app

COPY ./kbcli/go.mod ./kbcli/go.sum /app/kbcli/
COPY core /app/core

RUN go env -w CGO_ENABLED=0 && \
    go env -w GO111MODULE=on && \
    go env -w GOPROXY=https://goproxy.cn,https://mirrors.aliyun.com/goproxy,direct
RUN cd ./kbcli && go mod download

COPY ./kbcli /app/kbcli

RUN cd /app/kbcli && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w -X 'main.SoftwareVer=${VERSION}'" -o /app/bin/kbcli *.go && \
    chmod 755 /app/bin/kbcli

RUN chmod +x /app/kbcli/scripts/*.sh

FROM alpine:3.21 AS runtime

ENV env=prod
ENV TZ=Asia/Shanghai
WORKDIR /app

RUN apk add --no-cache tzdata
COPY --from=builder /app/bin/kbcli /app/bin/kbcli
COPY --from=builder /app/kbcli/scripts /app/scripts

CMD ["/app/bin/kbcli", "serve"]
