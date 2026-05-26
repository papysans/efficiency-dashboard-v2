# builder 固定在构建机架构上原生跑（前端 dist 与架构无关），多架构时不走 QEMU 模拟
FROM --platform=$BUILDPLATFORM node:20-alpine AS builder
WORKDIR /app/frontend

COPY frontend/package*.json ./
RUN npm ci

COPY frontend ./
RUN npm run build

FROM nginx:1.27-alpine AS runtime
ENV TZ=Asia/Shanghai

COPY compose/portal/nginx.conf /etc/nginx/nginx.conf
COPY --from=builder /app/frontend/dist /var/www
