#!/bin/sh

# 创建日志目录
mkdir -p /var/log

# 安装crontab配置 - dcron从/etc/crontabs/读取配置
# 先删除旧文件以避免K8s环境中的文件锁定或权限问题
rm -f /etc/crontabs/root
cp /app/scripts/crontab /etc/crontabs/root
chmod 644 /etc/crontabs/root

echo "import data..."
/app/bin/kbcli import-task -f
/app/bin/kbcli import-repo -f
/app/bin/kbcli import-org
/app/bin/kbcli silica -f
/app/bin/kbcli efficiency

echo "Starting kbcli cron job..."
# 启动cron服务
# Alpine Linux使用dcron，确保使用正确的路径和权限
if [ -x /usr/sbin/crond ]; then
    /usr/sbin/crond -l 2 -f
elif [ -x /usr/bin/crond ]; then
    /usr/bin/crond -l 2 -f
elif command -v crond >/dev/null 2>&1; then
    crond -l 2 -f
else
    echo "Error: crond not found or not executable"
    exit 1
fi
