#!/bin/sh
set -e

# 创建日志目录
mkdir -p /var/log

# 检查必需的环境变量
if [ -z "$TASK_DIR" ]; then
    echo "ERROR: TASK_DIR environment variable is required"
    exit 1
fi

if [ -z "$ANALYSED_DIR" ]; then
    echo "ERROR: ANALYSED_DIR environment variable is required"
    exit 1
fi

echo "Starting kbcli cron job..."
echo "TASK_DIR: $TASK_DIR"
echo "ANALYSED_DIR: $ANALYSED_DIR"

# 安装crontab配置
crontab /app/scripts/crontab

# 启动cron服务
# Alpine Linux使用dcron或crond
crond -l 2 -f
