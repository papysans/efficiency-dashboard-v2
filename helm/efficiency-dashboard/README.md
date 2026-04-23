# Efficiency Dashboard Helm Chart

Efficiency Dashboard 的 Kubernetes Helm 部署方案。

## 架构概述

该 Helm Chart 部署以下组件：

- **Elasticsearch**: 搜索和分析引擎，用于存储和检索数据
- **PostgreSQL**: 关系型数据库，用于存储业务数据
- **Server**: 后端服务，提供 API 接口
- **Portal (Nginx)**: 前端服务，提供 Web 界面

## 前置要求

- Kubernetes 集群（1.19+）
- Helm 3.x
- 足够的存储空间用于持久化数据

## 安装

### 1. 克隆仓库

```bash
git clone <repository-url>
cd efficiency-dashboard/helm
```

### 2. 配置 values.yaml

根据您的环境需求修改 `values.yaml` 文件中的配置项：

```yaml
# 修改镜像仓库地址
global:
  imageRegistry: "your-registry.example.com"

# 修改密码等敏感信息
postgresql:
  auth:
    password: "your-secure-password"

elasticsearch:
  password: "your-secure-password"
```

### 3. 安装 Chart

```bash
# 安装到 default 命名空间
helm install efficiency-dashboard ./efficiency-dashboard

# 安装到指定命名空间
helm install efficiency-dashboard ./efficiency-dashboard --namespace efficiency --create-namespace
```

### 4. 查看安装状态

```bash
helm status efficiency-dashboard
kubectl get pods -n efficiency
```

## 卸载

```bash
helm uninstall efficiency-dashboard -n efficiency
```

## 配置说明

### Elasticsearch 配置

```yaml
elasticsearch:
  enabled: true
  image:
    repository: docker.elastic.co/elasticsearch/elasticsearch
    tag: 8.9.0
  persistence:
    enabled: true
    size: 10Gi
  resources:
    limits:
      memory: 1Gi
    requests:
      memory: 512Mi
```

### PostgreSQL 配置

```yaml
postgresql:
  enabled: true
  image:
    repository: postgres
    tag: 14
  persistence:
    enabled: true
    size: 20Gi
  auth:
    user: postgres
    password: "your-password"
    database: report
    statDatabase: costrict_stat
```

### Server 配置

```yaml
server:
  enabled: true
  image:
    repository: efficiency-dashboard-backend
    tag: "1.0.4"
  volumes:
    taskDir:
      enabled: true
      hostPath: "/path/to/task/directory"
    analysedDir:
      enabled: true
  aiEstimation:
    enabled: true
    apiKey: "your-api-key"
    baseUrl: "https://open.bigmodel.cn/api/anthropic"
```

### Portal 配置

```yaml
portal:
  enabled: true
  image:
    repository: nginx
    tag: alpine
```

### Ingress 配置

```yaml
ingress:
  enabled: true
  className: "nginx"
  hosts:
    - host: efficiency-dashboard.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: efficiency-dashboard-tls
      hosts:
        - efficiency-dashboard.example.com
```

## 持久化存储

### 使用 PVC

对于生产环境，建议使用 PVC 来持久化数据：

```yaml
elasticsearch:
  persistence:
    enabled: true
    storageClass: "standard"
    size: 10Gi

postgresql:
  persistence:
    enabled: true
    storageClass: "standard"
    size: 20Gi
```

### 使用 HostPath

对于测试环境，可以使用 HostPath：

```yaml
server:
  volumes:
    taskDir:
      enabled: true
      pvcEnabled: false
      hostPath: "/mnt/prod_env/user-indicator/raw/task"
```

## 升级

```bash
helm upgrade efficiency-dashboard ./efficiency-dashboard --namespace efficiency
```

## 回滚

```bash
# 查看历史版本
helm history efficiency-dashboard -n efficiency

# 回滚到上一个版本
helm rollback efficiency-dashboard -n efficiency

# 回滚到指定版本
helm rollback efficiency-dashboard 2 -n efficiency
```

## 故障排查

### 查看 Pod 日志

```bash
# Server 日志
kubectl logs -n efficiency -l app.kubernetes.io/name=efficiency-dashboard-server --tail=100 -f

# Portal 日志
kubectl logs -n efficiency -l app.kubernetes.io/name=efficiency-dashboard-portal --tail=100 -f

# PostgreSQL 日志
kubectl logs -n efficiency -l app.kubernetes.io/name=efficiency-dashboard-postgresql --tail=100 -f

# Elasticsearch 日志
kubectl logs -n efficiency -l app.kubernetes.io/name=efficiency-dashboard-elasticsearch --tail=100 -f
```

### 连接到数据库

```bash
kubectl run -it --rm postgres-client \
  --image=postgres:14 --restart=Never \
  --env="PGPASSWORD=your-password" \
  --command -- psql -h efficiency-dashboard-postgresql -U postgres -d report
```

### 连接到 Elasticsearch

```bash
kubectl run -it --rm es-client \
  --image=nicolaka/netshoot --restart=Never \
  --command -- curl http://efficiency-dashboard-elasticsearch:9200/_cluster/health?pretty
```

### 进入 Pod

```bash
# 进入 Server Pod
kubectl exec -it -n efficiency deployment/efficiency-dashboard-server -- sh

# 进入 Portal Pod
kubectl exec -it -n efficiency deployment/efficiency-dashboard-portal -- sh
```

## 生产环境建议

1. **使用 Secret 管理敏感信息**

```bash
# 创建 Secret
kubectl create secret generic efficiency-dashboard-secrets \
  --from-literal=postgresql-password=your-password \
  --from-literal=elasticsearch-password=your-password \
  --from-literal=ai-api-key=your-api-key \
  -n efficiency

# 在 values.yaml 中引用
postgresql:
  auth:
    password: ${POSTGRESQL_PASSWORD}
```

2. **配置资源限制**

根据实际负载调整资源限制：

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

3. **启用自动扩缩容**

```yaml
autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 5
  targetCPUUtilizationPercentage: 80
```

4. **配置备份策略**

定期备份 PostgreSQL 数据和 Elasticsearch 数据。

5. **监控和告警**

配置 Prometheus 和 Grafana 进行监控，并设置告警规则。

## 常见问题

### 1. Pod 无法启动

检查资源是否充足：
```bash
kubectl describe pod <pod-name> -n efficiency
```

### 2. 数据库连接失败

检查密码是否正确，PostgreSQL 是否就绪：
```bash
kubectl get pods -n efficiency -l app.kubernetes.io/name=efficiency-dashboard-postgresql
```

### 3. Elasticsearch 无法访问

检查 Elasticsearch 是否正常运行：
```bash
kubectl get pods -n efficiency -l app.kubernetes.io/name=efficiency-dashboard-elasticsearch
kubectl logs -n efficiency -l app.kubernetes.io/name=efficiency-dashboard-elasticsearch
```

## 许可证

请参考项目根目录的 LICENSE 文件。

## 支持

如有问题，请提交 Issue 或联系维护团队。
