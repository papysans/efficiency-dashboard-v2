#!/bin/bash

# 检查是否提供了版本参数
if [ -z "$1" ]; then
    echo "错误：请提供版本号作为参数"
    echo "用法: $0 <version>"
    exit 1
fi

VERSION=$1

echo "=========================================="
echo "开始编译镜像，版本号: $VERSION"
echo "=========================================="

# 1. 编译 backend 的镜像
echo ""
echo "[1/7] 编译 backend 镜像..."
docker build -f backend/Dockerfile.qianliu -t zgsm/efficiency-dashboard-backend:${VERSION} .
if [ $? -ne 0 ]; then
    echo "错误：backend 镜像编译失败"
    exit 1
fi
echo "✓ backend 镜像编译成功: zgsm/efficiency-dashboard-backend:${VERSION}"

# 2. 编译 kbcli 的镜像
echo ""
echo "[2/7] 编译 kbcli 镜像..."
docker build -f kbcli/Dockerfile.qianliu -t zgsm/efficiency-dashboard-kbcli:${VERSION} .
if [ $? -ne 0 ]; then
    echo "错误：kbcli 镜像编译失败"
    exit 1
fi
echo "✓ kbcli 镜像编译成功: zgsm/efficiency-dashboard-kbcli:${VERSION}"

# 3. 修改 compose/.env 文件中的镜像版本
echo ""
echo "[3/7] 更新 compose/.env 文件..."

# 备份原始 .env 文件
cp compose/.env compose/.env.backup

# 修改 IMAGE_SERVER
sed -i.bak "s|^IMAGE_SERVER=zgsm/efficiency-dashboard-backend:.*|IMAGE_SERVER=zgsm/efficiency-dashboard-backend:${VERSION}|" compose/.env

# 修改 IMAGE_KBCLI
sed -i.bak "s|^IMAGE_KBCLI=zgsm/efficiency-dashboard-kbcli:.*|IMAGE_KBCLI=zgsm/efficiency-dashboard-kbcli:${VERSION}|" compose/.env

# 删除 sed 创建的备份文件
rm -f compose/.env.bak

echo "✓ .env 文件已更新:"
echo "  IMAGE_SERVER=zgsm/efficiency-dashboard-backend:${VERSION}"
echo "  IMAGE_KBCLI=zgsm/efficiency-dashboard-kbcli:${VERSION}"

# 4. 停止现有的 docker compose 服务
echo ""
echo "[4/7] 停止现有的 docker compose 服务..."
cd compose
docker compose down
cd ..
echo "✓ docker compose 服务已停止"

# 5. 启动 docker compose 服务
echo ""
echo "[5/7] 启动 docker compose 服务..."
cd compose
docker compose up -d
cd ..
echo "✓ docker compose 服务已启动"

# 6. 进入 kbcli 目录，运行 go build 构建
echo ""
echo "[6/7] 构建 kbcli 可执行文件..."
cd kbcli
go build
if [ $? -ne 0 ]; then
    echo "错误：kbcli 构建失败"
    cd ..
    exit 1
fi
echo "✓ kbcli 可执行文件构建成功"
cd ..

# 7. 通过 remote 方式连接第5步启动的 kbcli 服务，执行 import 命令
echo ""
echo "[7/7] 通过 remote 方式运行 kbcli import --force..."
KBCLI_REMOTE_URL="http://127.0.0.1:8080"
MAX_RETRIES=5
RETRY_INTERVAL=3
cd kbcli

RETRY_COUNT=0
CONNECTED=false
while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    echo "尝试连接 kbcli 服务 ($((RETRY_COUNT+1))/$MAX_RETRIES): $KBCLI_REMOTE_URL/health"
    if curl -sf --connect-timeout 3 "$KBCLI_REMOTE_URL/health" > /dev/null 2>&1; then
        CONNECTED=true
        echo "✓ kbcli 服务已就绪"
        break
    fi
    RETRY_COUNT=$((RETRY_COUNT+1))
    if [ $RETRY_COUNT -lt $MAX_RETRIES ]; then
        echo "  连接失败，${RETRY_INTERVAL}秒后重试..."
        sleep $RETRY_INTERVAL
    fi
done

if [ "$CONNECTED" = false ]; then
    echo "错误：无法连接到 kbcli 服务 (已重试 $MAX_RETRIES 次)"
    cd ..
    exit 1
fi

./kbcli import --force --remote "$KBCLI_REMOTE_URL"
if [ $? -ne 0 ]; then
    echo "错误：kbcli import --force --remote 执行失败"
    cd ..
    exit 1
fi
echo "✓ kbcli import --force --remote 执行完成"
cd ..

echo ""
echo "=========================================="
echo "所有操作完成！版本: $VERSION"
echo "=========================================="
