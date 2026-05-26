#!/usr/bin/env bash
# 把构建好的镜像导出为单个 tar，供内网离线 docker load。
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
set -a; source "${SCRIPT_DIR}/.env"; set +a

OUT="${SCRIPT_DIR}/efficiency-dashboard-images-${VERSION}.tar"
echo "==> docker save -> ${OUT}"
docker save -o "${OUT}" \
  "${IMAGE_SERVER}" "${IMAGE_KBCLI}" "${IMAGE_NGINX}" "${IMAGE_POSTGRES}"
echo "OK. 把 ${OUT} 和整个 compose/ 目录拷到内网后："
echo "  docker load -i $(basename "${OUT}")"
echo "  cd compose && docker compose up -d"
