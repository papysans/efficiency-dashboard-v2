#!/usr/bin/env bash
# 在【有网】机器上构建全部镜像并按 .env 的 tag 打标。
# Dockerfile 的 COPY 路径都基于仓库根目录，故 build context 必须是仓库根。
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
set -a; source "${SCRIPT_DIR}/.env"; set +a

cd "${REPO_ROOT}"
echo "==> build server  -> ${IMAGE_SERVER}"
docker build -f backend/Dockerfile        -t "${IMAGE_SERVER}" --build-arg VERSION="${VERSION}" .
echo "==> build kbcli   -> ${IMAGE_KBCLI}"
docker build -f kbcli/Dockerfile          -t "${IMAGE_KBCLI}"  --build-arg VERSION="${VERSION}" .
echo "==> build portal  -> ${IMAGE_NGINX}"
docker build -f compose/portal/Dockerfile -t "${IMAGE_NGINX}"  .
echo "==> pull postgres -> ${IMAGE_POSTGRES}"
docker pull "${IMAGE_POSTGRES}"
echo "OK. 构建完成。离线交付请运行 ./save.sh 导出 tar。"
