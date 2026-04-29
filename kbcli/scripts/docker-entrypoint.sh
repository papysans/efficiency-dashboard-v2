#!/bin/sh

echo "=== kbcli container entrypoint started ==="

# 确保以root用户运行
if [ "$(id -u)" -ne 0 ]; then
    echo "Error: This container must run as root to start cron daemon"
    exit 1
fi

# 创建日志目录
mkdir -p /var/log

# 安装crontab配置 - dcron从/etc/crontabs/读取配置
# 先删除旧文件以避免K8s环境中的文件锁定或权限问题
rm -f /etc/crontabs/root
if [ -f /app/scripts/crontab ]; then
    cp /app/scripts/crontab /etc/crontabs/root
    chmod 644 /etc/crontabs/root
    echo "Crontab configuration installed"
else
    echo "Warning: /app/scripts/crontab not found"
fi

echo "Starting data import..."
/app/bin/kbcli import-task -f
/app/bin/kbcli import-repo -f
/app/bin/kbcli import-org
/app/bin/kbcli silica -f
/app/bin/kbcli efficiency
echo "Data import completed"

echo "Starting kbcli cron job..."
# 启动cron服务
# Alpine Linux使用dcron，确保使用正确的路径和权限

# 确保crond有执行权限
for crond_path in /usr/sbin/crond /usr/bin/crond; do
    if [ -f "$crond_path" ]; then
        chmod +x "$crond_path" 2>/dev/null
        echo "Made $crond_path executable"
    fi
done

# 尝试启动crond
if [ -x /usr/sbin/crond ]; then
    echo "Using /usr/sbin/crond"
    exec /usr/sbin/crond -l 2 -f
elif [ -x /usr/bin/crond ]; then
    echo "Using /usr/bin/crond"
    exec /usr/bin/crond -l 2 -f
elif command -v crond >/dev/null 2>&1; then
    crond_path=$(command -v crond)
    echo "Using crond from PATH: $crond_path"
    exec crond -l 2 -f
else
    echo "Error: crond not found or not executable"
    echo "Searching for crond..."
    find / -name "crond" 2>/dev/null || true
    exit 1
fi
