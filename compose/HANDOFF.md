# efficiency-dashboard 部署交付说明（运维转 Helm 用）

> 目标读者：运维。本文件把这套 docker-compose 包**转成 Helm / 上 K8s** 所需的全部信息一次说清：
> **① 服务路由 ② 磁盘/卷 ③ 启动命令 ④ 配置文件位置**，外加转 Helm 的几个关键坑。
> 唯一真源是本目录（`compose/`）。仓库根原有的 `helm/` 已**弃用删除**，请勿参考。
> compose 自身的构建/离线/升级流程见同目录 [`README.md`](./README.md)；本文件只讲交付给 K8s 的契约。

> **⚠️ 如何取这个包（重要）**：开发机的工作目录里混着本地运行产物（镜像 tar / `kbcli/analysed/` 缓存 / `.env`、`*.local.*` 本地配置 / `postgres/data/`），这些都已 gitignore，**不属于交付内容、且含本地密钥，切勿直接物理拷整个目录**。请用下面任一方式取**只含版本库跟踪文件**的干净包：
>
> ```bash
> # 方式一：导出干净 tar（只含 git 跟踪文件，自动剔除所有本地产物）
> git archive --format=tar.gz -o efficiency-dashboard-compose.tar.gz HEAD compose/
>
> # 方式二：直接 clone 仓库后只用 compose/ 目录
> git clone <repo> && cd <repo>/compose
> ```

---

## 0. 一句话架构

5 个服务（第 5 个可选）：

```
                    ┌─────────── 外部唯一入口 ───────────┐
   用户浏览器 ──────▶ portal (nginx :80)   应用挂在 /kanban/ 子路径
                          │  /kanban/api/ →  server :9990 (strip /kanban → /api/)
                          ▼
                    server (Go :9990) ──┬── postgres :5432  (库 costrict_stat)
                          │             └── chat-stats :8080 (可选, /api/v2/chat/* 反代)
   kbcli (serve :8080) ───┘  自动 import + 内置 crontab，写库 + 读上游 mnt
```

**全新部署可行、且无需迁移旧库**：postgres 首次起来由 initdb 建 `costrict_stat` 库 + 装 pgcrypto；
后端 GORM AutoMigrate 自动建表；kbcli `serve` 启动即自动 `import`（从只读 mnt 拉原始数据）→ 算提效。
整个 DB 可从上游 mnt 完全重建，所以丢掉旧库不影响结果。**唯一代价**：首次全量 import + LLM 估算要跑一段时间。

**两个硬前提（缺了跑不起来）**：
1. 上游 mnt 两个**只读**数据源（task / repo）必须挂进 server + kbcli。这是数据命脉。
2. **服务名硬编码**（见 [§6 转 Helm 的坑](#6-转-helm-的关键坑)）：K8s Service 必须叫 `server` / `postgres` / `chat-stats`，否则要改挂 nginx.conf 和 config。

---

## 1. 服务路由

| 流向 | 端口 | 性质 | 说明 |
|---|---|---|---|
| **外部 → portal** | **80** | **Ingress（唯一对外）** | 整站挂 `/kanban/` 子路径；访问 `/` 会 302 → `/kanban/`。Ingress 指到 `portal:80` 即可 |
| portal `/kanban/api/` → server | 9990 | ClusterIP | nginx 反代并 strip `/kanban` → `/api/`，集群内 |
| server `/api/v2/chat/*` → chat-stats | 8080 | ClusterIP | 集群内，**可选**服务，默认关 |
| server / kbcli → postgres | 5432 | ClusterIP | 集群内 |
| postgres / kbcli | 5432 / 8080 | ClusterIP（不对外） | kbcli:8080 不在看板请求链路上，仅 serve/手动触发用；都无需 Ingress |

> **只需要一条 Ingress**：外部域名/路径 → `portal:80`。其余全是集群内 ClusterIP。
> 应用根路径是 `/kanban/`，配置 Ingress 时注意（详见 [§6](#6-转-helm-的关键坑)）。

---

## 2. 磁盘 / 卷

真正要你**预留持久卷并挂到具体位置**的只有 2 个（PG 数据 + analysed）；其余是只读数据源或配置。

| 卷 | 服务 | 容器内路径 | 读写 | 来源 / 内容 | 持久化建议 |
|---|---|---|---|---|---|
| **PG 数据** | postgres | `/var/lib/postgresql/data` | RW | 数据库本体，随数据增长 | **PVC，建议 20Gi**（可调；精确值可在内网量现库 `du -sh` 后定） |
| **analysed 缓存** | kbcli | `/app/analysed` | RW | silica 分析缓存 + `org_mapping.csv` | **PVC，建议 10Gi** |
| **task 数据源** | server + kbcli | `/app/task` | **RO** | 上游 mnt：对话原始数据 | 上游既有 PV / hostPath / NFS，**大小由上游定** |
| **repo 数据源** | server + kbcli | `/app/repo` | **RO** | 上游 mnt：commit 原始数据 | 上游既有 PV / hostPath / NFS，**大小由上游定** |
| initdb 脚本 | postgres | `/docker-entrypoint-initdb.d` | RO | 3 个 SQL，**仅数据卷为空的首次**执行 | ConfigMap（见 [§4](#4-配置文件位置)） |
| anchor CSV | kbcli | `/app/docs/data/efficiency_v2_anchor_set.csv` | RO | kNN 锚点母表 | ConfigMap 或打进镜像（缺了 kNN 基线降级，不致命） |

> **task/repo 由上游提供**，我们只读挂载，不负责其容量。compose 里走 `.env` 的 `DATA_TASK_DIR` / `DATA_REPO_DIR`
> （默认 `/mnt/prod_env/user-indicator/raw/task`、`.../repo`）。转 Helm 时映射成对应的只读 PV/hostPath 即可。

---

## 3. 启动命令

> 下表是**容器层命令**（直接进 K8s Deployment 的 `command`/`args`）。
> 注意 `docker compose up -d` / `docker compose exec` 是 compose 编排层命令，**不进 Helm**，对应物分别是「Deployment 的容器命令」和「`kubectl exec`」。

| 服务 | 镜像 | 启动命令 | 备注 |
|---|---|---|---|
| **server** | `…/efficiency-dashboard-v2/server` | `/app/efficiency-dashboard-backend` | ENTRYPOINT，无参，读 `/app/config.yaml`；健康检查 `GET /healthz` |
| **kbcli** | `…/efficiency-dashboard-v2/kbcli` | `/app/bin/kbcli serve` | CMD；启动即 `import` 增量 + **内置 crontab 调度**（见下）+ 监听 :8080 |
| **portal** | `…/efficiency-dashboard-v2/portal` | `nginx -g 'daemon off;'` | nginx 镜像默认；前端 dist + nginx.conf 已打进镜像 |
| **postgres** | `postgres:15-alpine` | 官方 entrypoint | env：`POSTGRES_DB=postgres` / `POSTGRES_USER` / `POSTGRES_PASSWORD`；首次跑 initdb.d |
| **chat-stats**（可选） | `chat-indicator-statistics:local` | 镜像默认 CMD | 读 `/app/config.yaml`，监听 :8080；健康检查 `GET /health` |

**kbcli 内置 crontab**（在 `kbcli/config.yaml` 的 `serve.crontab`，无需外部 CronJob）：
- 每 4 小时 `import`（增量，含 import-org / import-dept / efficiency 重算）
- 每日 04:30 `fix-task`（LLM 估算新增 task）
- 每日 05:00 `fix-commit`（LLM 估算新增 commit）
- 每周日 03:00 `import -f`（全量校正一次）

### ⚠️ kbcli 是 `CMD` 不是 `ENTRYPOINT`（server 才是 ENTRYPOINT）

- 长驻 pod 跑 `/app/bin/kbcli serve` ✓。
- 跑**一次性命令**（import / efficiency-v2 等）时，K8s 里**必须写全路径**：
  `command: ["/app/bin/kbcli", "import", "-f"]`，**不能只在 `args` 写 `import`** ——
  否则 `import` 被当成可执行文件名，报 `executable file not found in $PATH`（镜像无 ENTRYPOINT 可前缀）。

### kbcli 手动运维命令（`kubectl exec` ＝ 原 `docker compose exec`）

| 场景 | 命令 |
|---|---|
| **首次立刻出 v2 看板**（不等整点） | `kubectl exec <kbcli-pod> -- /app/bin/kbcli import -f` 再 `... efficiency-v2` |
| 改了 config 口径/阈值/权重重算（不读 mnt，轻量幂等） | `kubectl exec <kbcli-pod> -- /app/bin/kbcli efficiency-v2` |
| 灌 kNN 锚点母表 | `kubectl exec <kbcli-pod> -- /app/bin/kbcli import-anchor` |

> `import` 必须带 `-f`，否则已导入 session 被 `analysed` 缓存跳过；`efficiency-v2` 只从 DB 重算、不读 mnt。
> 容器 cwd=`/app` 默认读 `/app/config.yaml`，`--config` 可省。

---

## 4. 配置文件位置

| 服务 | 宿主源文件（compose/） | 容器内挂载点 | 模式 | 含敏感信息？ |
|---|---|---|---|---|
| server | `server/config.yaml` | `/app/config.yaml` | ro | **是**：LLM `api_key` |
| kbcli | `kbcli/config.yaml` | `/app/config.yaml` | ro | **是**：LLM `api_key` + `org_dsn`（带库密码） |
| postgres | `postgres/initdb.d/*.sql` | `/docker-entrypoint-initdb.d` | ro | 否（仅首次执行） |
| portal | （nginx.conf 已在镜像内） | `/etc/nginx/nginx.conf` | ro | 否；**仅在改服务名时**才需覆盖挂载（见 [§6](#6-转-helm-的关键坑)） |
| chat-stats（可选） | `chat-stats/config.yaml` | `/app/config.yaml` | ro | 库密码 |

转 Helm 建议：非敏感配置走 **ConfigMap**，敏感字段走 **Secret**。

### 必填 / 脱敏清单（上线前务必替换；committed 的是占位/他人 key）

| 字段 | 所在文件 | 当前值 | 要做什么 |
|---|---|---|---|
| `PASSWORD_POSTGRES` | `.env` → 注入 postgres 与各 config 的 `stat_database.password` | `1` | **改强密码**，进 Secret；三处保持一致 |
| `ai_estimation.api_key` | `server/config.yaml`、`kbcli/config.yaml` | 占位/他人 key | 换自己的 LLM key，进 Secret（走 costrict 网关另需 `x_api_key` 头） |
| `ai_estimation.base_url` / `model` | 同上 | sangfor newapi | 按内网网关确认；不跑 LLM 设 `enabled: false`（融合自动重归一到 algo+knn） |
| `org_dsn` | `kbcli/config.yaml` | `host=10.72.10.64 …` | 内网 auth 库连接串（组织映射来源），进 Secret；连不上则组织维度为空、不影响其它 |
| `dept_sync.base_url` / `query_key` | `server/config.yaml`、`kbcli/config.yaml` | 空 | 填内网 dept-sync 地址 + 管理员下发的 query_key（部门树/真名/部门）；空=不启用 |
| `chat_stats.base_url`（可选） | `server/config.yaml`（默认注释） | 注释 | 启用 chat-stats 时打开为 `http://chat-stats:8080` |

---

## 5. 数据库初始化（首次）

postgres 数据卷为空的首次启动会跑 `initdb.d`：
- `10-create-db.sql`：建主库 `costrict_stat`（应用连这个，不是默认 `postgres` 库）
- `20-init_db_stat.sql`：在 `costrict_stat` 装 `pgcrypto` 扩展（`gen_random_uuid` 需要）
- `30-create-chat-summary-db.sql`：建 `chat_summary` 库（仅 chat-stats 用）

库表本身由后端/各服务 **GORM AutoMigrate** 自动创建，无需手工 DDL。
**注意**：initdb.d 仅在数据卷为空时执行一次。如果用已有数据卷升级，需要补的库要手动建（见各 SQL 注释）。

---

## 6. 转 Helm 的关键坑

1. **服务名硬编码 —— 最重要**。下列引用写死，K8s Service 名必须**严格对齐**，否则改挂配置：
   - `portal/nginx.conf`：`proxy_pass http://server:9990/api/;`（location `/kanban/api/`）→ Service 必须叫 **`server`**
   - `server/config.yaml` 与 `kbcli/config.yaml`：`stat_database.host: postgres` → Service 必须叫 **`postgres`**
   - `server/config.yaml`：`chat_stats.base_url: http://chat-stats:8080` → Service 必须叫 **`chat-stats`**
   - 若 Helm 习惯用 `<release>-server` 之类前缀名，则要把上述 host **参数化覆盖**：config.yaml 用 ConfigMap 模板注入，nginx.conf 覆盖挂载（compose 里默认打进镜像，挂载会覆盖）。

2. **应用挂在 `/kanban/` 子路径**。nginx 已处理 `/` → 302 `/kanban/`。Ingress 不要把路径重写掉 `/kanban`；
   直接把外部入口整段转发到 portal:80 最省事。

3. **initdb 仅首次执行**。用 PVC 全新部署没问题；若复用已有数据卷，新加的库/扩展不会自动补。

4. **首次全量 import 耗时**。kbcli serve 启动即增量 import；要立刻出 v2 看板需手动跑一次 `import -f` + `efficiency-v2`（见 [§3](#3-启动命令)）。给 kbcli 留足 CPU/内存与启动宽限。

5. **健康检查**：server `GET /healthz`、chat-stats `GET /health`、postgres `pg_isready`、kbcli serve `GET :8080/health`。portal 无内置 HTTP 健康端点。

6. **启动依赖顺序**：postgres(healthy) → server / kbcli → portal。Helm 无强依赖编排，靠各服务 restart + 健康检查自愈即可。

7. **chat-stats 是可选第 5 服务**，默认不部署。需要平台客观指标（LLM token/成本/时延）时再启用，并打开 server config 的 `chat_stats.base_url`、建 `chat_summary` 库。其镜像无 CI，需手动构建（见 README）。

---

## 7. 镜像

| 服务 | 镜像（默认 tag 见 `.env` 的 `VERSION`） |
|---|---|
| server | `ghcr.io/zgsm-sangfor/efficiency-dashboard-v2/server:<tag>` |
| kbcli | `ghcr.io/zgsm-sangfor/efficiency-dashboard-v2/kbcli:<tag>` |
| portal | `ghcr.io/zgsm-sangfor/efficiency-dashboard-v2/portal:<tag>` |
| postgres | `postgres:15-alpine` |
| chat-stats（可选） | `chat-indicator-statistics:local`（手动构建） |

构建/离线导出/拉取流程见 [`README.md`](./README.md)。内网拉 ghcr 需把三个 package 设为 public 或用 PAT 登录。
