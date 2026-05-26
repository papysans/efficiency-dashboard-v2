# AI Coding 指标看板 (Efficiency Dashboard)

实时追踪 AI 辅助开发效率，量化提效价值的全栈指标看板系统。通过采集开发者的 AI 任务数据、Git 提交记录，计算含硅量与综合提效比，帮助团队直观了解 AI Coding 工具的实际效能。

---

## 技术栈

| 层级 | 技术 |
|------|------|
| 后端 API | Go 1.26 + Gin + GORM + PostgreSQL |
| 前端界面 | Vue 3 + Element Plus + Vite + ECharts |
| 数据处理 CLI | Go 1.25 + Cobra |
| 数据库 | PostgreSQL 15 |
| 部署 | Docker Compose / Helm |

---

## 项目结构

```
.
├── backend/          # Go REST API 服务（端口 9990）
│   ├── main.go
│   └── *_handler_v2.go   # 各模块 API 处理器
├── frontend/         # Vue 3 前端看板
│   ├── package.json
│   └── src/
│       ├── views/        # 页面视图
│       ├── components/   # 公共组件
│       └── router/       # 路由配置
├── core/             # 共享 Go 库
│   ├── models/       # 数据库模型（GORM）
│   ├── config/       # 数据库连接配置
│   └── utils/        # 工具函数（效能比计算、文本处理等）
├── kbcli/            # CLI 数据导入与计算工具
│   ├── cmd_import*.go    # 数据导入命令
│   ├── cmd_silica.go     # 含硅量计算
│   ├── cmd_efficiency.go # 效能计算
│   └── cmd_serve.go      # 异步任务 HTTP 服务（端口 8080）
├── compose/          # Docker Compose 部署配置
└── helm/             # Kubernetes Helm 部署方案
```

---

## 核心功能

### 1. 多维度数据看板

- **首页仪表盘**：核心指标汇总（仓库数、用户数、Task 数、Commit 数、总费用、综合提效比）
- **仓库视图**：按仓库维度查看代码提交、含硅量等指标
- **用户视图**：按开发者维度统计生产力数据，支持用户分组
- **组织视图**：按组织架构层级汇总效能指标
- **提交视图**：逐条查看 Commit 详情及 AI 贡献分析
- **任务视图**：查看 AI 交互任务详情、Token 消耗与耗时
- **项目视图**：虚拟项目管理，聚合多个仓库与任务进行提效分析

### 2. 数据采集与处理（kbcli）

| 命令 | 说明 |
|------|------|
| `kbcli import-conv` | 导入 AI 任务原始数据 |
| `kbcli import-repo` | 导入仓库与 Commit 数据 |
| `kbcli import-org`  | 导入用户组织架构映射 |
| `kbcli silica`      | 计算 Commit 含硅量（AI 生成代码占比） |
| `kbcli efficiency`  | 按日计算用户与组织效能数据 |
| `kbcli import`      | 顺序执行完整导入流程 |
| `kbcli serve`       | 启动异步任务调度 HTTP 服务 |

### 3. 关键指标

- **含硅量 (Silica)**：Commit 中由 AI 对话生成的代码行占比
- **综合提效比**：传统预估耗时 / 实际耗时，反映 AI 辅助的效率提升
- **代码采纳率**：Task 产生的代码行被后续 Commit 采纳的比例
- **Token 消耗**：Upstream / Downstream Token 统计及费用估算

---

## 数据模型

主要业务表：

- `tasks` / `task_conversations` — AI 任务及对话明细
- `commits` — Git 提交记录及含硅量分析结果
- `sessions` — 客户端会话信息
- `user_org` — 用户与组织架构映射
- `user_productivity` — 按日聚合的用户生产力数据
- `projects` — 虚拟项目定义（聚合仓库与任务）
- `user_groups` — 用户分组配置

---

## 快速开始

### 环境要求

- Go 1.25+
- Node.js 18+
- PostgreSQL 15+

### 1. 启动数据库

```bash
cd compose
# 使用 Docker Compose 一键启动全部服务
docker-compose up -d
```

或手动创建 PostgreSQL 数据库：
- 数据库名：`report`
- 用户：`postgres`

### 2. 启动后端 API

```bash
cd backend
go mod tidy
go run .
# 默认监听 http://localhost:9990
# Swagger 文档：http://localhost:9990/swagger/index.html
```

### 3. 启动前端

```bash
cd frontend
npm install
npm run dev
# 默认访问 http://localhost:5173
```

### 4. 运行数据导入（示例）

```bash
cd kbcli
go run . import --task-dir /path/to/tasks --repo-dir /path/to/repos
```

---

## 部署方式

### Docker Compose

```bash
cd compose
docker-compose up -d
```

服务组成：
- `postgres` — 数据库
- `server` — 后端 API
- `portal` — 前端 Nginx
- `kbcli` — 数据处理 CLI 服务

前端看板可单独打包为 Nginx 镜像：

```bash
make package-portal VER=1.0.2
```

后端与前端镜像一起打包：

```bash
make package-all VER=1.0.2
```

### Helm（Kubernetes）

支持两种部署模式：

**独立部署**（单独升级某服务）：
```bash
helm install postgresql ./charts/postgresql -n efficiency --create-namespace
helm install server ./charts/server -n efficiency
helm install portal ./charts/portal -n efficiency
```

**伞形部署**（一键全量部署）：
```bash
helm install efficiency-dashboard ./efficiency-dashboard -n efficiency --create-namespace
```

详见 [helm/README.md](helm/README.md)。

---

## API 文档

后端和 kbcli serve 均内置 Swagger UI：

- Backend：`http://localhost:9990/swagger/index.html`
- KBCLI Serve：`http://localhost:8080/swagger/index.html`

---

## 测试

### 后端集成测试

```bash
cd backend
go test -v ./...
```

### 前端测试

```bash
cd frontend
npm run test
```

---

## 配置说明

主要配置文件：

- `backend/config.yaml` — 后端服务配置（端口、数据库 DSN、CORS）
- `kbcli/config.yaml` — CLI 工具配置（数据目录、数据库连接、定时任务）
- `compose/*/config.yaml` — Docker Compose 各服务配置

---

## 贡献与反馈

- 问题反馈：[GitHub Issues](https://github.com/zgsm-ai/costrict-cli/issues)

---

## License

Apache 2.0
