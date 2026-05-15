# kbcli 脚本说明

## 文件结构

```
kbcli/
├── scripts/
│   ├── init.sh              # 初始化数据导入脚本（force模式）
│   ├── hourly-cron.sh       # 每小时执行一次的定时任务脚本（非force模式）
│   └── README.md            # 本文档
└── Dockerfile               # 镜像构建文件
```

## 脚本说明

### init.sh

容器首次启动或需要强制重新导入数据时执行：

```bash
/app/scripts/init.sh
```

该脚本会以 **force 模式** 执行以下命令：
- `kbcli import-conv -f`
- `kbcli import-repo -f`
- `kbcli import-org`
- `kbcli efficiency`

### hourly-cron.sh

每小时执行一次的定时任务脚本，以 **非 force 模式** 执行：

```bash
/app/scripts/hourly-cron.sh
```

执行内容：
- `kbcli import-conv`
- `kbcli import-repo`
- `kbcli efficiency`

此脚本可用于手动触发或作为外部调度（如 Kubernetes CronJob）的备用方案。

## 部署说明

### 镜像启动命令

镜像默认启动命令为 `kbcli serve`，内置 HTTP 服务器和定时任务调度器。

```bash
# 启动 kbcli serve（会自动读取 config.yaml 中的 crontab 配置执行定时任务）
docker run -d \
  --name kbcli \
  -v /host/path/config.yaml:/app/config.yaml:ro \
  -v /host/path/tasks:/app/task \
  -v /host/path/repo:/app/repo \
  -v /host/path/analysed:/app/analysed \
  kbcli:latest
```

### 手动执行初始化

```bash
docker exec kbcli /app/scripts/init.sh
```

### 查看日志

```bash
# 查看容器日志
docker logs kbcli -f
```

## Kubernetes 部署

在 Kubernetes 中部署时，可以通过 initContainer 执行初始化脚本：

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kbcli
spec:
  replicas: 1
  template:
    spec:
      initContainers:
      - name: kbcli-init
        image: kbcli:latest
        command: ["/app/scripts/init.sh"]
        volumeMounts:
        - name: config
          mountPath: /app/config.yaml
          subPath: config.yaml
      containers:
      - name: kbcli
        image: kbcli:latest
        ports:
        - containerPort: 8080
        volumeMounts:
        - name: config
          mountPath: /app/config.yaml
          subPath: config.yaml
      volumes:
      - name: config
        configMap:
          name: kbcli-config
```

## 定时任务配置

定时任务通过 `config.yaml` 中的 `serve.crontab` 配置，由 `kbcli serve` 内置调度器执行。

示例配置：

```yaml
serve:
  port: 8080
  crontab:
    - schedule: "0 0 * * * *"
      command: import-conv
      params:
        force: false
    - schedule: "0 5 * * * *"
      command: import-repo
      params:
        force: false
    - schedule: "0 15 * * * *"
      command: efficiency
```

schedule 格式为 6 字段 cron 表达式（秒 分 时 日 月 周）。

## 注意事项

1. **目录权限**：确保挂载的目录有正确的读写权限
2. **时区设置**：容器时区已设置为 `Asia/Shanghai`
3. **配置管理**：`config.yaml` 中的 `serve.crontab` 定义了定时任务的调度规则
