#!/usr/bin/env bash
# 数据管线一键：import-anchor(kNN 锚点，手动前置) → import(conv→repo→org→dept→efficiency-v2)。
# 额外参数透传给 import，例如：
#   scripts/pipeline.sh -f                                  # 全量强制重扫
#   scripts/pipeline.sh -f --start-date 20260525 --end-date 20260527
#   CONFIG=.local/kbcli-config.yaml scripts/pipeline.sh --date 20260526
# 注：import 读 config 里的 task_dir/repo_dir(上游 mnt 数据)，需该数据源可达。
set -euo pipefail
cd "$(dirname "$0")/.."

CONFIG="${CONFIG:-configs/kbcli-config.yaml}"
BIN="temp/kbcli-pipeline"

echo "▶ 构建 kbcli（config: $CONFIG）"
mkdir -p temp
( cd kbcli && go build -o "../$BIN" . )

echo "▶ [1/2] import-anchor —— 灌 kNN 锚点母表"
"$BIN" import-anchor --config "$CONFIG"

echo "▶ [2/2] import —— conv→repo→org→dept→efficiency-v2  ${*:-（全量增量）}"
"$BIN" import --config "$CONFIG" "$@"

echo "✓ 数据管线完成"
