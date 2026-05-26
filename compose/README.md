# efficiency-dashboard 内网部署（docker-compose）

四个服务：`postgres`（库）、`server`（Go 后端 :9990）、`portal`（nginx + 前端 :80）、`kbcli`（取数/定时，serve :8080）。
`portal` 反代 `/api/` → `server:9990`，前端 dist 与 nginx.conf 已打进镜像。

## 一、有网机器：构建 + 导出

```bash
cd compose
./build.sh        # 按 .env 的 tag 构建 server/kbcli/portal，并 pull postgres
./save.sh         # 导出 efficiency-dashboard-images-<VERSION>.tar
```

## 二、内网机器：导入 + 启动

```bash
docker load -i efficiency-dashboard-images-v1.0.1.tar
# 把整个 compose/ 目录拷过来，按需改 .env（端口/密码）和各 config.yaml
cd compose
docker compose up -d
docker compose ps          # 等 postgres/server healthy
```

访问：`http://<内网IP>:${PORT_PORTAL}`（默认 8088）。

## 三、部署前必改

- **`.env`**：`PORT_*` 端口、`PASSWORD_POSTGRES`、镜像 tag。
- **原始数据挂载**（`server.yml` / `kbcli.yml`）：把 `/mnt/prod_env/user-indicator/raw/{task,repo}` 改成内网实际数据路径。
- **`server/config.yaml` + `kbcli/config.yaml`**：
  - `ai_estimation`：LLM 端点/key（内网可达性自测；不想跑 LLM 设 `enabled: false`）。
  - kbcli `org_dsn`：内网 auth 库连接串（组织映射来源）。

## 四、首次取数（容器内）

```bash
# 进 kbcli 容器按管道顺序跑（首次 import 用 -f 强制；analysed 目录有 silica 缓存会跳过）
docker compose exec kbcli /app/bin/kbcli import-conv  --config /app/config.yaml -f
docker compose exec kbcli /app/bin/kbcli import-repo  --config /app/config.yaml -f
docker compose exec kbcli /app/bin/kbcli import-org   --config /app/config.yaml
docker compose exec kbcli /app/bin/kbcli efficiency-v2 --config /app/config.yaml
```

## 备注

- postgres：`POSTGRES_DB=postgres`（默认库），`initdb.d/10-create-db.sql` 建 `costrict_stat`、`20-*.sql` 装 pgcrypto。**仅首次**（数据卷为空）执行 initdb。
- 库表由后端 GORM AutoMigrate 自动建。
- server healthcheck 走 `/healthz`（镜像已装 curl）。
