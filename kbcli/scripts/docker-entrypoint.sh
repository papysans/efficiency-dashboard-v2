#!/bin/sh
set -e

# 创建日志目录
mkdir -p /var/log

# 安装crontab配置 - dcron从/etc/crontabs/读取配置
cp /app/scripts/crontab /etc/crontabs/root
chmod 644 /etc/crontabs/root

echo "import data..."
/app/bin/kbcli import

echo "Starting kbcli cron job..."
# 启动cron服务
# Alpine Linux使用dcron
crond -l 2 -f
