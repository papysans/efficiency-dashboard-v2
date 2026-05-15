#!/bin/sh

# 每小时执行一次的定时任务脚本（非force模式）
# 用于 kbcli serve 外部手动调度或备用场景

echo "=== kbcli hourly cron started at $(date) ==="

/app/bin/kbcli import-conv
/app/bin/kbcli import-repo
/app/bin/kbcli efficiency

echo "=== kbcli hourly cron completed at $(date) ==="
