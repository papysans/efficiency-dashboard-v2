#!/usr/bin/env bash
# 本地开发一键栈：postgres(:5432) + backend(:9990) + frontend(:8881)。
# 用 stock postgres:15，端口/库对齐 backend/config.yaml 与 configs/kbcli-config.yaml。
# Ctrl-C 退出时自动停 backend/frontend；DB 容器保留（数据持久 + 下次更快，停用 `make db-stop`）。
set -uo pipefail
cd "$(dirname "$0")/.."

PG_CONTAINER="efficiency-dev-pg"
PG_PORT=5432
PG_DB=costrict_stat

echo "▶ [1/3] PostgreSQL ($PG_CONTAINER :$PG_PORT/$PG_DB)"
if ! docker info >/dev/null 2>&1; then
  echo "✗ Docker 守护进程不可用，请先启动 Docker Desktop（或 dockerd）再重试。" >&2
  exit 1
fi
if docker ps -a --format '{{.Names}}' | grep -qx "$PG_CONTAINER"; then
  docker start "$PG_CONTAINER" >/dev/null
else
  docker run -d --name "$PG_CONTAINER" -p "$PG_PORT:5432" \
    -e POSTGRES_PASSWORD=1 -e POSTGRES_DB="$PG_DB" postgres:15 >/dev/null
fi
ready=0
for _ in $(seq 1 30); do
  if docker exec "$PG_CONTAINER" pg_isready -U postgres -d "$PG_DB" >/dev/null 2>&1; then ready=1; break; fi
  sleep 1
done
if [ "$ready" != 1 ]; then
  echo "✗ PostgreSQL 30s 内未就绪，已放弃。排查：docker logs $PG_CONTAINER" >&2
  exit 1
fi
echo "  ✓ DB ready"

cleanup() {
  echo ""
  echo "▶ 停止 backend/frontend（DB 容器保留，停用 make db-stop）..."
  lsof -ti:9990 | xargs -r kill 2>/dev/null || true
  lsof -ti:8881 | xargs -r kill 2>/dev/null || true
  exit 0
}
trap cleanup INT TERM

echo "▶ [2/3] backend (:9990) —— 读 backend/config.yaml"
( cd backend && go run . ) &

echo "▶ [3/3] frontend (:8881)"
if [ ! -d frontend-react/node_modules ]; then
  echo "  首次运行，安装前端依赖（npm install）..."
  ( cd frontend-react && npm install )
fi
( cd frontend-react && npm run dev ) &

sleep 3
cat <<'BANNER'

════════════════════════════════════════════════════
  本地栈已拉起：
    前端看板   http://localhost:8881/kanban/
    后端 API   http://localhost:9990  (swagger: /swagger/index.html)
    数据库     localhost:5432 / costrict_stat
  灌数据见 `make pipeline` 或 README「数据管线」。
  Ctrl-C 退出（DB 容器保留）。
════════════════════════════════════════════════════
BANNER
wait
