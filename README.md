# AI Coding 指标看板 (Efficiency Dashboard)

实时追踪 AI 辅助开发效率、量化提效价值的全栈指标看板。采集开发者的 AI 任务数据与 Git 提交记录，计算**含硅量**（AI 生成代码占比）与**综合提效比**，帮助团队直观了解 AI Coding 工具的实际效能。

---

## 技术栈

| 层级 | 技术 |
|------|------|
| 后端 API | Go 1.26 + Gin + GORM（端口 **9990**） |
| 前端界面 | **React 19** + Vite + Tailwind + ECharts（dev 端口 **8881**） |
| 数据处理 CLI | Go 1.25 + Cobra（`kbcli`） |
| 数据库 | PostgreSQL 15（库名 **`costrict_stat`**） |
| 部署 | Docker Compose / Helm |

---

## 项目结构

```
.
├── backend/          # Go REST API 服务（:9990，*_handler_v2.go 各模块处理器）
├── frontend-react/   # React 19 前端看板（Vite，:8881，/api 代理到 :9990）
├── core/             # 共享 Go 库（models GORM 模型 / config / utils）
├── kbcli/            # 数据导入与计算 CLI（含 internal/ 子包：efficiencyv2 / appconfig / util / ...）
├── configs/          # 配置模板（kbcli-config.yaml / server-config.yaml）
├── compose/          # Docker Compose 部署（postgres / server / portal / kbcli）
└── helm/             # Kubernetes Helm 部署
```

> 历史的 `frontend/`（Vue）已下线，前端唯一实现是 `frontend-react/`。

---

## 核心功能

- **首页总览**：省人天 & ROI、提效趋势、Top 提效榜（需求/人）、规模概览
- **需求 / 任务 / 用户 / 仓库 / 组织 / 项目 / 提交** 七大维度看板
- **提交详情**：Commit 含硅量、关联 Task、AI 贡献分析
- **组织视图**：基于 dept-sync 部门树的层级效能汇总

### 关键指标

- **含硅量 (Silica)**：Commit 中由 AI 对话生成的代码行占比（kbcli import-repo 阶段按指纹匹配计算）
- **综合提效比**：传统古法预估耗时 / 实际耗时；多基线（algo / kNN / LLM）融合
- **代码采纳率 / Token 消耗 / 费用**

---

## 快速开始（本地开发）

### 环境要求

- Go 1.25+（backend 需 1.26）、Node.js 18+、PostgreSQL 15、Docker

### 一键启动（推荐）

```bash
make dev                 # 起 postgres(:5432) + backend(:9990) + frontend(:8881)，Ctrl-C 退出
make pipeline ARGS="-f"  # 灌数据：import-anchor(kNN) → import；ARGS 透传给 import
make db-stop             # 停掉 dev 的 postgres 容器
```

> 现成数据可从备份恢复（`.local/*.backup.sql.gz`）；`make pipeline` 需 config 里的上游数据源（mnt）可达。

以下为分步手动启动（即 `make dev` 的等价拆解）：

### 1. 起数据库（PostgreSQL，库 `costrict_stat`）

```bash
# 方式 A：用 compose 起 postgres（推荐）
cd compose && docker compose up -d postgres

# 方式 B：自备 docker
docker run -d --name kanban-postgres -p 5432:5432 \
  -e POSTGRES_PASSWORD=1 -e POSTGRES_DB=costrict_stat postgres:15
```

> 表结构由 backend / kbcli 启动时 GORM AutoMigrate 自动建。需要现成数据可从备份恢复（见 `.local/*.backup.sql.gz`，gitignored）。

### 2. 起后端 API（:9990）

```bash
cd backend
go run .   # 自动读 ./config.yaml 或 ../configs/server-config.yaml
# Swagger: http://localhost:9990/swagger/index.html
```

> 个人覆盖配置放 `.local/`（gitignored）；改 DB 连接时复制一份模板再改，不要动 committed 模板。

### 3. 起前端（:8881）

```bash
cd frontend-react
npm install
npm run dev   # http://localhost:8881，/api 自动代理到 :9990
```

### 4. 灌数据（见下方「数据管线」）

---

## 数据管线（kbcli）

> 调用形态：本地 `cd kbcli && go run . <cmd> --config ../configs/kbcli-config.yaml [flags]`；
> 内网 compose `docker compose exec kbcli /app/bin/kbcli <cmd> --config /app/config.yaml [flags]`。
> 日期 flag 通用：`--date YYYYMMDD`（单天） / `--start-date YYYYMMDD --end-date YYYYMMDD`（区间），不带=全量。

### 命令一览

| 命令 | 说明 |
|------|------|
| `import` | **编排链**：import-conv → import-repo → import-org → import-dept → efficiency-v2 |
| `import-conv` | 导入 AI 任务/对话原始数据（来自 mnt 上游）|
| `import-repo` | 导入仓库/Commit，并按指纹计算**含硅量** |
| `import-org` | 导入用户↔组织映射（非破坏占位，不覆盖真实 org）|
| `import-dept` | 从 **dept-sync** 拉部门树+人员，投影回填 user_org（**组织树**来源）|
| `import-anchor` | 灌 **kNN 锚点母表**（⚠️ **不在 import 链里，需手动**）|
| `efficiency-v2` | 计算/重算 V2 效能（提效比、基线融合、用户/组织聚合）|
| `fix-task` / `fix-commit` | LLM 估算补齐新增 task/commit 耗时 |
| `clean` | 清洗早于某日的过期数据（先 `--dry-run`）|
| `check` / `serve` | 自检 / 异步任务调度服务（:8080，自带 cron）|

### 从零 / 大改的全流程顺序

```
import-anchor（kNN 锚点，单独灌一次）
  → import（= conv → repo → org → dept → efficiency-v2 一条串起来）
```

### 常用 ops

```bash
# ① 改了 scope/阈值 → 只重算口径（最常用，不重读 mnt，快）
kbcli efficiency-v2 --config <cfg>                                  # 全量重算（配置改动全部生效）
kbcli efficiency-v2 --config <cfg> --start-date 20260525 --end-date 20260604   # 区间
kbcli efficiency-v2 --config <cfg> --date 20260526                 # 单天
#   ⚠️ 带日期只重算那个窗口的 need；改了 scope/阈值想全部生效，跑不带日期的全量那条。

# ② 源数据变了 / 要重扫 mnt → 重新取数+重算（含 efficiency-v2，重）
kbcli import --config <cfg> -f --start-date 20260525 --end-date 20260527   # 区间强制重扫
kbcli import --config <cfg> -f                                            # 全量强制
#   ⚠️ -f 必加，否则已导入的 session 会被跳过（analysed-dir 有缓存）。

# ③ 换了 kNN 锚点母表 CSV → 重灌 anchor_set
kbcli import-anchor --config <cfg>                                  # 用 config 里的 anchor_set_csv
kbcli import-anchor --config <cfg> --csv /path/to/anchor_set.csv

# ④ 修复 / 清洗（偶尔）
kbcli fix-task   --config <cfg> --start-date 20260525 --end-date 20260527
kbcli fix-commit --config <cfg> --date 20260526
kbcli clean      --config <cfg> --before 2026-05-18 --dry-run       # 先预览再去掉 --dry-run
```

### 自动化（cron）

`kbcli serve`（:8080）内置定时任务：每 4h 增量 `import`、每日 04:30 `fix-task` / 05:00 `fix-commit`、每周日 03:00 `force` 全量 `import` 纠偏。
**⚠️ `import-anchor`(kNN) 不在 cron 里**，换锚点时需手动跑。

### 升级后要不要全量重导？

- **纯代码更新（重构 / 删死码 / bug 修，不改 schema/口径）** → **不必重导**，重建 `kbcli`/`backend` 二进制（或拉新镜像）即可，存量数据有效。
- **改了 efficiency_v2 的 scope/阈值/权重** → 跑 ①「全量 `efficiency-v2`」即可（不重读 mnt）。
- **改了取数/含硅量算法（如裸代码指纹适配）** → 跑 ②「`import -f`」重扫重算。

---

## 数据模型（主要表，V2 口径）

- `needs` — 需求边界聚合（提效比、基线融合 baseline_fused_work_min 等 V2 核心）
- `commits` / `tasks` / `task_conversations` / `sessions` — 提交 / 任务 / 对话 / 会话明细
- `user_org` / `dept` / `dept_user` — 用户组织映射 / 部门树
- `user_productivity_v2` — 按周聚合的用户生产力（V2）
- `baseline_fusion_weights` — 基线融合权重
- `projects` / `user_groups` — 虚拟项目 / 用户分组

> 旧 `user_productivity`（V1 按日预聚合表）**已下线**，相关读取改为从 tasks/commits 基表实时聚合。

---

## 部署

### Docker Compose（内网）

```bash
cd compose && docker compose up -d         # postgres + server(:9990) + portal(nginx) + kbcli
```

镜像打包：`make package-all VER=1.0.x`（后端+前端）/ `make package-portal VER=1.0.x`（仅前端）。

### Helm（Kubernetes）

```bash
helm install efficiency-dashboard ./helm/efficiency-dashboard -n efficiency --create-namespace
```

详见 [helm/README.md](helm/README.md)、[compose/README.md](compose/README.md)。

---

## 测试

```bash
cd backend && go test ./...           # 后端单元测试
cd kbcli   && go test ./...           # kbcli 单元测试（含 internal/ 子包）
cd frontend-react && npm run test     # 前端 vitest
```

> backend 的 `*_integration_test.go` 白盒集成套件已废弃移除；如需端到端覆盖建议另起黑盒方案。

---

## 配置说明

| 文件 | 用途 |
|------|------|
| `configs/server-config.yaml` / `backend/config.yaml` | 后端（端口、DB DSN、CORS）|
| `configs/kbcli-config.yaml` | kbcli（DB、模型价格、algo/efficiency_v2 调参、dept_sync、cron）|
| `compose/*/config.yaml` | compose 各服务配置 |
| `.local/*`（gitignored）| 个人本地覆盖，复制模板后修改 |

### 存储后端（disk / s3）

`task_dir` / `repo_dir` / `analysed_dir` 等目录路径支持两种存储后端，按路径前缀自动识别，可混搭：

- 本地磁盘：普通路径，如 `/mnt/prod_env/user-indicator/raw/task`（默认，行为不变）
- S3 兼容对象存储（MinIO 等）：`s3://bucket/prefix` 形式，需同时配置 `storage.s3` 连接参数

```yaml
storage:
  s3:
    endpoint: "minio.intranet:9000"   # 不含 scheme
    access_key: "xxx"
    secret_key: "xxx"
    use_ssl: false

task_dir: "s3://kanban-raw/task"
repo_dir: "s3://kanban-raw/repo"
analysed_dir: "./analysed"            # 可与 s3 混搭
```

配置了 `s3://` 路径但 `storage.s3` 缺失或凭证无效时，kbcli / backend 启动即报错退出（fail-fast）。

---

## License

Apache 2.0
