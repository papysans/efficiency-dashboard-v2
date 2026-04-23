FROM golang:1.26.0 AS builder
WORKDIR /app

COPY backend/go.mod backend/go.sum /app/backend/
COPY core /app/core

RUN go env -w CGO_ENABLED=0 && \
    go env -w GO111MODULE=on && \
    go env -w GOPROXY=https://goproxy.cn,https://mirrors.aliyun.com/goproxy,direct
RUN cd /app/backend; go mod download

COPY backend /app/backend

ARG VERSION=v1.0.1
RUN cd /app/backend ; go build -ldflags="-s -w -X 'main.SoftwareVer=$VERSION'" -o /app/efficiency-dashboard-backend  *.go
RUN chmod 755 /app/efficiency-dashboard-backend

FROM alpine:3.21 AS runtime

ENV env prod
ENV TZ Asia/Shanghai
WORKDIR /app
COPY --from=builder /app/efficiency-dashboard-backend /app/efficiency-dashboard-backend

ENTRYPOINT ["/app/efficiency-dashboard-backend"]
