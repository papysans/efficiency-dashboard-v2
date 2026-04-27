FROM golang:1.26.0 AS builder
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=v1.0.1
WORKDIR /app

COPY backend/go.mod backend/go.sum /app/backend/

RUN go env -w CGO_ENABLED=0 && \
    go env -w GO111MODULE=on && \
    go env -w GOPROXY=https://goproxy.cn,https://mirrors.aliyun.com/goproxy,direct
RUN cd /app/backend && go mod download

COPY backend /app/backend

RUN cd /app/backend && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w -X 'main.SoftwareVer=${VERSION}'" -o /app/efficiency-dashboard-backend *.go && \
    chmod 755 /app/efficiency-dashboard-backend

FROM alpine:3.21 AS runtime

ENV env=prod
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=builder /app/efficiency-dashboard-backend /app/efficiency-dashboard-backend

ENTRYPOINT ["/app/efficiency-dashboard-backend"]
