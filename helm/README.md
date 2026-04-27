# Efficiency Dashboard Helm 部署方案

## 目录结构

```
helm/
├── charts/                          # 独立子 Chart（可单独打包部署）
│   ├── postgresql/                  # PostgreSQL 数据库
│   │   ├── Chart.yaml
│   │   ├── values.yaml
│   │   ├── .helmignore
│   │   └── templates/
│   ├── server/                      # 后端 API 服务
│   │   ├── Chart.yaml
│   │   ├── values.yaml
│   │   ├── .helmignore
│   │   └── templates/
│   ├── portal/                      # 前端 Nginx 服务
│   │   ├── Chart.yaml
│   │   ├── values.yaml
│   │   ├── .helmignore
│   │   └── templates/
│   └── kbcli/                       # 数据导入 CLI 服务
│       ├── Chart.yaml
│       ├── values.yaml
│       ├── .helmignore
│       └── templates/
├── efficiency-dashboard/            # 伞形 Chart（组合部署所有服务）
│   ├── Chart.yaml                   # 声明依赖的子 Chart
│   ├── values.yaml                  # 统一配置
│   └── templates/
│       ├── _helpers.tpl
│       └── NOTES.txt
└── README.md
```

## 两种部署方式

### 方式一：独立部署（推荐用于单独升级某服务）

每个子 Chart 可以独立打包和部署，适合需要单独升级某个服务的场景。

```bash
# 单独部署 PostgreSQL
helm install postgresql ./charts/postgresql -n efficiency --create-namespace

# 单独部署 Server
helm install server ./charts/server -n efficiency \
  --set database.host=postgresql \
  --set database.password=your-password

# 单独部署 Portal
helm install portal ./charts/portal -n efficiency \
  --set backend.host=server

# 单独部署 kbcli
helm install kbcli ./charts/kbcli -n efficiency \
  --set database.host=postgresql \
  --set backend.host=server
```

### 方式二：伞形部署（推荐用于初次部署或全量升级）

通过 `efficiency-dashboard` 伞形 Chart 一次部署所有服务。

```bash
# 部署全部服务
helm install efficiency-dashboard ./efficiency-dashboard -n efficiency --create-namespace

# 只部署部分服务（禁用不需要的）
helm install efficiency-dashboard ./efficiency-dashboard -n efficiency \
  --set portal.enabled=false
```

## 打包子 Chart

每个子 Chart 可以独立打包：

```bash
helm package ./charts/postgresql    # → postgresql-0.2.0.tgz
helm package ./charts/server        # → server-0.2.0.tgz
helm package ./charts/portal        # → portal-0.2.0.tgz
helm package ./charts/kbcli         # → kbcli-0.2.0.tgz
```

## 子 Chart 间的服务引用

当独立部署时，各服务通过 values 中的配置项引用其他服务：

| 服务 | 配置项 | 默认值 | 说明 |
|------|--------|--------|------|
| server | `database.host` | `postgresql` | PostgreSQL 服务地址 |
| server | `statDatabase.host` | `postgresql` | 统计库服务地址 |
| portal | `backend.host` | `server` | 后端 API 服务地址 |
| portal | `backend.port` | `9990` | 后端 API 端口 |
| kbcli | `database.host` | `postgresql` | PostgreSQL 服务地址 |
| kbcli | `backend.host` | `server` | 后端 API 服务地址 |

伞形部署时，这些值会自动配置为 `efficiency-dashboard-{service}` 格式的服务名。

## 自定义配置

```bash
# 使用自定义 values 文件
helm install server ./charts/server -f custom-values.yaml

# 命令行覆盖
helm install server ./charts/server \
  --set database.host=my-postgres \
  --set database.password=my-password \
  --set image.tag=1.0.9
```

## 升级和回滚

```bash
# 升级单个服务
helm upgrade server ./charts/server --set image.tag=1.0.9 -n efficiency

# 回滚
helm rollback server 1 -n efficiency

# 伞形升级
helm upgrade efficiency-dashboard ./efficiency-dashboard -n efficiency
```

## 服务组件

| 组件 | 镜像 | 端口 | 说明 |
|------|------|------|------|
| PostgreSQL | postgres:15-alpine | 5432 | 关系型数据库 |
| Server | efficiency-dashboard-backend | 9990 | 后端 API 服务 |
| Portal | nginx:alpine | 80 | 前端 Web 服务 |
| kbcli | efficiency-dashboard-kbcli | 8080 | 数据导入 CLI |
