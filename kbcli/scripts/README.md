# kbcli Crontab 定时任务说明

## 概述

本项目在kbcli中配置了crontab定时任务，每小时自动执行一次任务导入操作。

## 文件结构

```
kbcli/
├── scripts/
│   ├── crontab              # crontab配置文件
│   ├── docker-entrypoint.sh # Docker容器启动脚本
│   └── README.md            # 本文档
└── Dockerfile               # 已更新以支持cron
```

## 工作原理

1. **Docker容器启动**：执行 `docker-entrypoint.sh`
2. **环境检查**：验证必需的环境变量（TASK_DIR、ANALYSED_DIR）
3. **安装crontab**：将crontab配置安装到系统中
4. **启动cron服务**：运行 `crond -l 2 -f`
5. **定时执行**：每小时0分执行 `kbcli import`

## 环境变量

部署Docker容器时需要设置以下环境变量：

| 变量名 | 说明 | 示例 |
|--------|------|------|
| `TASK_DIR` | 任务数据目录 | `/data/tasks` |
| `ANALYSED_DIR` | 分析结果目录 | `/data/analysed` |

## 部署说明

### 构建镜像

```bash
cd kbcli
docker build -t kbcli:latest .
```

### 运行容器

```bash
docker run -d \
  --name kbcli-cron \
  -e TASK_DIR=/data/tasks \
  -e ANALYSED_DIR=/data/analysed \
  -v /host/path/tasks:/data/tasks \
  -v /host/path/analysed:/data/analysed \
  kbcli:latest
```

### 查看日志

```bash
# 查看cron执行日志
docker exec kbcli-cron cat /var/log/kbcli-cron.log

# 查看容器日志
docker logs kbcli-cron -f
```

## Crontab配置

当前配置：每小时执行一次（每小时的第0分钟）

```
0 * * * * /app/scripts/import-task-cron.sh >> /var/log/kbcli-cron.log 2>&1
```

如需调整执行频率，可以修改 `scripts/crontab` 文件。

## 调试

### 手动执行任务

```bash
docker exec -it kbcli-cron /app/scripts/import-task-cron.sh
```

### 进入容器调试

```bash
docker exec -it kbcli-cron sh
```

### 检查cron服务状态

```bash
docker exec kbcli-cron ps aux | grep crond
```

## 注意事项

1. **目录权限**：确保挂载的目录有正确的读写权限
2. **时区设置**：容器时区已设置为 `Asia/Shanghai`
3. **日志管理**：定期清理 `/var/log/kbcli-cron.log` 避免日志文件过大
4. **环境变量**：启动容器前必须设置 TASK_DIR 和 ANALYSED_DIR

## Kubernetes部署

如果需要在Kubernetes中部署，可以参考以下ConfigMap和环境变量配置：

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kbcli-cron-config
data:
  TASK_DIR: "/data/tasks"
  ANALYSED_DIR: "/data/analysed"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kbcli-cron
spec:
  replicas: 1
  template:
    spec:
      containers:
      - name: kbcli-cron
        image: kbcli:latest
        envFrom:
        - configMapRef:
            name: kbcli-cron-config
        volumeMounts:
        - name: tasks-data
          mountPath: /data/tasks
        - name: analysed-data
          mountPath: /data/analysed
      volumes:
      - name: tasks-data
        persistentVolumeClaim:
          claimName: tasks-pvc
      - name: analysed-data
        persistentVolumeClaim:
          claimName: analysed-pvc
```
