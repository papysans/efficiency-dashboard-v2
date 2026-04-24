#!/bin/sh
set -e

# 创建日志目录
mkdir -p /var/log

echo "Starting kbcli cron job..."

# 安装crontab配置
crontab /app/scripts/crontab

# 启动cron服务
# Alpine Linux使用dcron或crond
crond -l 2 -f
