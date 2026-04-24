# Efficiency Dashboard Helm 部署方案

本目录包含 Efficiency Dashboard 应用的 Kubernetes Helm Chart 部署方案。

## 目录结构

```
helm/
└── efficiency-dashboard/
    ├── Chart.yaml              # Chart 元数据
    ├── values.yaml              # 默认配置值
    ├── README.md                # Chart 使用文档
    ├── .helmignore              # Helm 打包忽略文件
    └── templates/               # Kubernetes 资源模板
        ├── _helpers.tpl         # 可重用模板函数
        ├── NOTES.txt            # 安装后提示信息
        ├── serviceaccount.yaml  # ServiceAccount
        ├── postgresql-*.yaml    # PostgreSQL 相关配置
        ├── server-*.yaml        # Server 相关配置
        ├── portal-*.yaml        # Portal (Nginx) 相关配置
        ├── ingress.yaml         # Ingress 配置
        └── files/
            └── nginx.conf       # Nginx 配置文件
```

## 快速开始

### 1. 安装 Chart

```bash
# 进入 helm 目录
cd helm

# 安装到 default 命名空间
helm install efficiency-dashboard ./efficiency-dashboard

# 安装到指定命名空间
helm install efficiency-dashboard ./efficiency-dashboard --namespace efficiency --create-namespace
```

### 2. 查看安装状态

```bash
helm status efficiency-dashboard
kubectl get pods -n efficiency
kubectl get svc -n efficiency
```

### 3. 卸载 Chart

```bash
helm uninstall efficiency-dashboard -n efficiency
```

## 自定义配置

### 使用自定义 values.yaml

创建自定义配置文件 `custom-values.yaml`：

```yaml
# 修改镜像仓库
global:
  imageRegistry: "your-registry.example.com"

# 修改密码
postgresql:
  auth:
    password: "your-secure-password"

server:
  aiEstimation:
    apiKey: "your-api-key"

# 启用 Ingress
ingress:
  enabled: true
  hosts:
    - host: efficiency-dashboard.example.com
      paths:
        - path: /
          pathType: Prefix
```

使用自定义配置安装：

```bash
helm install efficiency-dashboard ./efficiency-dashboard -f custom-values.yaml
```

### 命令行参数覆盖

```bash
helm install efficiency-dashboard ./efficiency-dashboard \
  --set postgresql.auth.password=my-password \
  --set server.replicaCount=3
```

## 部署架构

该 Helm Chart 部署以下组件：

### 1. PostgreSQL (StatefulSet)
- **镜像**: `postgres:14`
- **端口**: 5432
- **存储**: 20Gi PVC
- **数据库**: report, costrict_stat
- **用途**: 关系型数据库

### 2. Server (Deployment)
- **镜像**: `efficiency-dashboard-backend:1.0.4`
- **端口**: 9990
- **挂载**:
  - 任务目录 (HostPath 或 PVC)
  - 分析目录 (EmptyDir 或 PVC)
- **用途**: 后端 API 服务

### 3. Portal (Deployment)
- **镜像**: `nginx:alpine`
- **端口**: 80
- **配置**: 自定义 nginx.conf
- **用途**: 前端 Web 服务

## 持久化存储

### 使用 StorageClass

```yaml
postgresql:
  persistence:
    enabled: true
    storageClass: "fast-ssd"
    size: 20Gi
```

### 使用 HostPath (仅用于测试)

```yaml
server:
  volumes:
    taskDir:
      enabled: true
      pvcEnabled: false
      hostPath: "/mnt/data/task"
```

## 高级功能

### 1. 自动扩缩容 (HPA)

```yaml
autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 5
  targetCPUUtilizationPercentage: 80
  targetMemoryUtilizationPercentage: 80
```

### 2. 节点选择

```yaml
nodeSelector:
  kubernetes.io/hostname: node-1

affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          labelSelector:
            matchExpressions:
              - key: app.kubernetes.io/name
                operator: In
                values:
                  - efficiency-dashboard-server
          topologyKey: kubernetes.io/hostname
```

### 3. 资源限制

```yaml
server:
  resources:
    limits:
      memory: 2Gi
      cpu: 2000m
    requests:
      memory: 1Gi
      cpu: 1000m
```

## 升级和回滚

### 升级

```bash
helm upgrade efficiency-dashboard ./efficiency-dashboard -f custom-values.yaml
```

### 回滚

```bash
# 查看历史版本
helm history efficiency-dashboard

# 回滚到上一个版本
helm rollback efficiency-dashboard

# 回滚到指定版本
helm rollback efficiency-dashboard 2
```

## 监控和日志

### 查看日志

```bash
# Server 日志
kubectl logs -n efficiency -l app.kubernetes.io/name=efficiency-dashboard-server --tail=100 -f

# Portal 日志
kubectl logs -n efficiency -l app.kubernetes.io/name=efficiency-dashboard-portal --tail=100 -f

# PostgreSQL 日志
kubectl logs -n efficiency -l app.kubernetes.io/name=efficiency-dashboard-postgresql --tail=100 -f
```

### 查看资源使用

```bash
kubectl top pods -n efficiency
kubectl top nodes
```

## 故障排查

### Pod 无法启动

```bash
kubectl describe pod <pod-name> -n efficiency
kubectl logs <pod-name> -n efficiency
```

### 服务无法访问

```bash
kubectl get svc -n efficiency
kubectl get endpoints -n efficiency
```

### 数据库连接问题

```bash
kubectl run -it --rm postgres-client \
  --image=postgres:14 --restart=Never \
  --env="PGPASSWORD=your-password" \
  --command -- psql -h efficiency-dashboard-postgresql -U postgres -d report
```

## 生产环境建议

1. **使用 Secret 管理敏感信息**
2. **配置资源限制和请求**
3. **启用自动扩缩容**
4. **配置备份策略**
5. **设置监控和告警**
6. **使用持久化存储**
7. **配置 Ingress 和 TLS**
8. **定期更新和升级**

## 注意事项

1. 默认配置仅适用于开发和测试环境
2. 生产环境请修改所有默认密码和密钥
3. 确保有足够的存储空间和资源
4. 建议使用私有镜像仓库
5. 定期备份 PostgreSQL 数据

## 更多信息

详细配置说明请参考 [`efficiency-dashboard/README.md`](./efficiency-dashboard/README.md)
