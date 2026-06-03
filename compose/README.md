# efficiency-dashboard 内网部署（docker-compose）

四个服务：`postgres`（库）、`server`（Go 后端 :9990）、`portal`（nginx + 前端 :80）、`kbcli`（取数/定时 serve :8080）。
`portal` 反代 `/api/` → `server:9990`；前端 dist + nginx.conf 已打进 portal 镜像。
配置全部**挂载**进容器（不打进镜像），所以改配置/数据路径**不用重建镜像**。

## 一、有网机器：构建 + 导出镜像

```bash
cd compose
./build.sh        # 按 .env 的 tag 构建 server/kbcli/portal + pull postgres
./save.sh         # 导出 efficiency-dashboard-images-<VERSION>.tar（约 156MB）
```
把 `tar` + 整个 `compose/` 目录拷到内网。

## 二、内网机器：导入镜像

```bash
docker load -i efficiency-dashboard-images-v1.0.1.tar
```

## 三、部署前改 `.env` 和两个 config（关键）

**`compose/.env`**：
- `DATA_TASK_DIR` / `DATA_REPO_DIR` → **上游只读映射的两个 mnt 路径**（对话数据=task，commit数据=repo）。位置不固定时只改这两行。
- `PORT_PORTAL`(默认8088)/`PORT_BACKEND`/`PORT_POSTGRES`/`PORT_KBCLI`、`PASSWORD_POSTGRES`、镜像 tag。

**`compose/kbcli/config.yaml`（取数+LLM 主用）和 `compose/server/config.yaml`**：
- `ai_estimation`：填你的 LLM key。如走 costrict 网关：
  ```yaml
  ai_estimation:
    enabled: true
    api_key: "sk-..."                         # 你的 key
    x_api_key: "costrict_team_..."            # 网关需要此头
    base_url: "https://zgsm.sangfor.com/newapi"
    model: "costrict-openrouter-deepseek-v4-flash"
    api_format: "openai"
  ```
  （内网对该网关可达性自测；不想跑 LLM 设 `enabled: false`，融合自动重归一到 algo+knn。）
- kbcli `org_dsn`：内网 auth 库连接串（组织映射来源；连不上则组织维度为空，不影响其它）。

> 注意：committed 的 config 里带的是**别人的占位 key**，务必替换成你自己的；这些文件在宿主机改、不要提交。

## 四、启动

```bash
cd compose
docker compose up -d
docker compose ps          # 等 postgres / server 变 healthy
docker compose logs -f kbcli   # 看自动取数进度
```
访问 `http://<内网IP>:${PORT_PORTAL}`。

## 五、扫描（取数 → 算提效）

kbcli `serve` **启动即自动 `import`**（import-conv→repo→org→v1 efficiency），并按 crontab：
- 每小时 `import`（增量）
- 每小时 :30 `efficiency-v2`（产出 v2 看板：needs / 提效比）

**首次想立刻看到 v2 看板**（不等整点），手动各跑一次：
```bash
docker compose exec kbcli /app/bin/kbcli import        --config /app/config.yaml -f   # 全量重扫
docker compose exec kbcli /app/bin/kbcli efficiency-v2 --config /app/config.yaml
```
> `import` 只到 v1；**v2 看板必须 `efficiency-v2`**。`-f` 强制重扫（否则 analysed 的 silica 缓存会跳过已扫的）。

## 发包与内网拉取（papysans/efficiency-dashboard-v2 → ghcr）

镜像统一为 `ghcr.io/papysans/efficiency-dashboard-v2/<server|kbcli|portal>`，tag = `beta-<git tag>`。

### 一、外网发包（GitHub Actions）
1. 在仓库 papysans/efficiency-dashboard-v2 打 tag 并推送：`git tag v1.1.13 && git push origin v1.1.13`（或 Actions 页手动跑 `build-and-push-images`，version 填 v1.1.13）。
2. CI 构建多架构(amd64+arm64)并推到 `ghcr.io/papysans/efficiency-dashboard-v2/{server,kbcli,portal}:beta-v1.1.13`。
3. ghcr 包默认 private；内网要拉，需在 GitHub 把这三个 package 设为 public（或内网用 PAT `docker login ghcr.io`）。

### 二、内网部署（二选一）
- A 内网可直连 ghcr：`cd compose && docker compose --env-file .env pull && docker compose --env-file .env up -d`（.env 已指向上述镜像）。
- B 离线：外网 `cd compose && bash save.sh` 导出 tar.gz → 把 tar.gz 与整个 compose/ 拷到内网 → `docker load -i efficiency-dashboard-images-*.tar.gz && cd compose && docker compose up -d`。

### 三、升级新版本
改 compose/.env 的 `VERSION` 与 `IMAGE_*` tag（如 beta-v1.1.5）→ 重新 pull/save → `docker compose up -d`。

## 备注

- postgres：`POSTGRES_DB=postgres`（默认库），`initdb.d/10-create-db.sql` 建 `costrict_stat`、`20-*.sql` 装 pgcrypto，**仅数据卷为空的首次**执行。库表由后端 GORM AutoMigrate 自动建。
- server healthcheck 走 `/healthz`（镜像已装 curl）。
- 数据是 mnt 上游报上来的，里面若有 benchmark/探针账号会按原样进库（已确认接受）。
- 本机冒烟测试用 `.env.local` + `docker-compose.local.yml`（避开本机占用端口、挂本地数据、用含真实 key 的 `kbcli/config.local.yaml`），勿带去内网。
