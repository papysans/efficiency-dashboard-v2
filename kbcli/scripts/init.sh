#!/bin/sh

echo "=== kbcli init started ==="

echo "Starting data import..."
/app/bin/kbcli import-conv -f
/app/bin/kbcli import-repo -f
/app/bin/kbcli import-org
/app/bin/kbcli silica -f
/app/bin/kbcli efficiency
echo "Data import completed"

echo "=== kbcli init completed ==="
