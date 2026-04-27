#!/bin/sh

# 创建日志目录
mkdir -p /var/log

# 安装crontab配置 - dcron从/etc/crontabs/读取配置
# 先删除旧文件以避免K8s环境中的文件锁定或权限问题
rm -f /etc/crontabs/root
cp /app/scripts/crontab /etc/crontabs/root
chmod 644 /etc/crontabs/root

echo "import data..."
/app/bin/kbcli import

echo "Starting kbcli cron job..."
# 启动cron服务
# Alpine Linux使用dcron
crond -l 2 -f
