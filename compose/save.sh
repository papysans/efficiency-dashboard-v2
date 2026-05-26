#!/usr/bin/env bash
# 拉取【目标架构】镜像并导出为单个 tar.gz，供内网离线 docker load。
# 默认 linux/amd64（内网服务器是 x86）。在 arm Mac 上构建/导出也能得到 amd64 镜像。
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
set -a; source "${SCRIPT_DIR}/.env"; set +a

PLATFORM="${TARGET_PLATFORM:-linux/amd64}"
OUT="${SCRIPT_DIR}/efficiency-dashboard-images-${VERSION}-${PLATFORM//\//-}.tar.gz"

echo "==> 拉取 ${PLATFORM} 变体（多架构 manifest 会按此选对架构，避免 save 出错误架构）"
for img in "${IMAGE_SERVER}" "${IMAGE_KBCLI}" "${IMAGE_NGINX}" "${IMAGE_POSTGRES}"; do
  echo "    pull --platform ${PLATFORM} ${img}"
  docker pull --platform "${PLATFORM}" "${img}"
done

echo "==> docker save -> ${OUT}"
docker save "${IMAGE_SERVER}" "${IMAGE_KBCLI}" "${IMAGE_NGINX}" "${IMAGE_POSTGRES}" | gzip > "${OUT}"
echo "OK. 把 ${OUT} 和整个 compose/ 目录拷到内网后："
echo "  docker load -i $(basename "${OUT}")"
echo "  cd compose && docker compose up -d        # 注意用 'docker compose'(v2)，不是 'docker-compose'(v1)"
