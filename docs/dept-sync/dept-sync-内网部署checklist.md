# efficiency-dashboard 内网部署 Checklist（含 dept-sync 对接）

> 适用：换新环境 / 全新部署。当前版本 **v1.2.1**（整站挂 `/kanban`）。
> 原则：镜像走 CI ghcr→内网 mirror；**配置是挂载的（不在镜像里），换环境必须重填**。

---

## 0. 前置：dept-sync 服务（外部依赖）
组织树 + import-dept 依赖 dept-sync。新环境要：
1. 起 dept-sync 服务（它自己那套，宿主进程或容器，默认监听 8080，路由前缀 `/costrict-dept-info`）。
2. 建一个 **query_key**（数据接口鉴权 `X-Query-Key`）：
   - 它的 DB 是 PostgreSQL（库 `dept_sync`），往 `query_key` 表插一条：
     `INSERT INTO query_key(key,remark,status,create_time,update_time) VALUES('<生成一个>','kanban',1,now(),now());`
   - 或用 admin_key（默认 123456，头 `X-Admin-Key`）调 `POST /costrict-dept-info/api/api/admin/keys`。
3. 记下 dept-sync 的**可达地址**（kbcli/server 容器要能连）：同机宿主进程用宿主 IP，如 `http://<宿主IP>:8080`；同 compose 网络则用服务名。
4. 确认 HR 同步过、有全量数据：`curl -H "X-Query-Key:<key>" http://<addr>/costrict-dept-info/api/v1/department/tree` 返回非空。

---

## 1. 改 `compose/.env`
```ini
VERSION=beta-v1.2.1
IMAGE_SERVER=<mirror>/.../server:beta-v1.2.1
IMAGE_KBCLI=<mirror>/.../kbcli:beta-v1.2.1
IMAGE_NGINX=<mirror>/.../portal:beta-v1.2.1
IMAGE_POSTGRES=postgres:17-alpine        # 与现网一致
PORT_PORTAL=8088 / PORT_BACKEND=9990 / PORT_POSTGRES=5432 / PORT_KBCLI=8080  # 按新环境端口
DATA_TASK_DIR=<新环境 mnt task 原始数据目录>
DATA_REPO_DIR=<新环境 mnt repo 原始数据目录>
POSTGRES_DB=postgres / POSTGRES_USER=postgres / PASSWORD_POSTGRES=<密码>
```

## 2. 改 `compose/kbcli/config.yaml`（挂载进 kbcli 的 /app/config.yaml）
```yaml
stat_database: { host, port, user, password, dbname: costrict_stat }   # 指向本栈 postgres
analysis_start_date: "20260525"          # 分析起始日下界：不处理此日期之前的数据
backend_url: "http://server:9990"        # import-dept 跑完自动 orgs/refresh（compose 服务名）
efficiency_mode: new                     # 用 efficiency-v2 + kNN
dept_sync:
  base_url: "http://<dept-sync可达地址>:8080"
  query_key: "<第0步建的 query_key>"
  fallback_org_name: "深信服科技股份有限公司"   # 桥接不到部门的看板用户兜底归这
  fallback_dept_name: "未知部门"
efficiency_v2:
  anchor_set_csv: /app/docs/data/efficiency_v2_anchor_set.csv   # 镜像自带，import-anchor 用
serve:
  init: { command: import, params: { force: false } }
  crontab:
    - { schedule: "0 0 */4 * * *", command: import, params: { force: false } }
    - { schedule: "0 30 4 * * *", command: fix-task }
    - { schedule: "0 0 5 * * *",  command: fix-commit }
    - { schedule: "0 0 3 * * 0",  command: import, params: { force: true } }
```

## 3. 改 `compose/server/config.yaml`（挂载进 server 的 /app/config.yaml）
```yaml
stat_database: { ... costrict_stat }     # 同上
dept_sync:
  base_url: "http://<dept-sync可达地址>:8080"
  query_key: "<同一个 query_key>"
  root_dept_name: "深信服科技股份有限公司"   # 组织树单根（排除脏数据孤儿部门）
```

---

## 4. 拉镜像 + 起栈
```bash
cd compose
docker compose pull
docker compose up -d
```
- kbcli 的 `init.command: import` 会**自动跑全流程**：import-conv→import-repo→import-org→import-dept→efficiency(-v2)，空库自动灌数据（含 dept-sync 真名/部门回填 user_org）。
- 盯日志：`docker compose logs -f kbcli`，等出现 `[import] 全部步骤完成`。

## 5. kNN 锚点（efficiency_mode=new 必做，且要在算 efficiency-v2 之后补一次）
init import 的 efficiency-v2 在空 anchor_set 上算过一遍（kNN 无锚点），所以补：
```bash
docker compose exec kbcli /app/bin/kbcli import-anchor --config /app/config.yaml      # 灌 kNN 锚点
docker compose exec kbcli /app/bin/kbcli efficiency-v2 --config /app/config.yaml      # 锚点就绪后重算
docker compose restart server                                                          # 重载 org 映射（保险）
```
（legacy 模式不用 kNN，跳过本步）

## 6. 验证
```bash
# 数据进库
docker exec -e PGPASSWORD=<pw> <pg容器> psql -U postgres -d costrict_stat -c \
"SELECT (SELECT count(*) FROM commits) commits,(SELECT count(*) FROM dept) dept,
        (SELECT count(*) FROM dept_user) dept_user,(SELECT count(*) FROM user_org) user_org,
        (SELECT count(*) FROM anchor_set) anchors;"
# 入口重定向到 /kanban
curl -s http://127.0.0.1:<PORT_PORTAL>/ -o /dev/null -w '%{http_code} -> %{redirect_url}\n'   # 期望 302 -> /kanban/
# 组织树接口（server 代理 dept-sync）
curl -s "http://127.0.0.1:<PORT_BACKEND>/api/v2/dept-tree" -o /dev/null -w 'dept-tree:%{http_code}\n'
```
浏览器开 `http://<host>:<PORT_PORTAL>/kanban/` → 导航有「组织」→ 点进去是组织树（单根深信服、点部门出成员、无活动沉底）。

---

## 常见坑（来自实战）
- **配置不在镜像里**：改 config 必须改宿主挂载文件 + 重起对应容器；改 .env 镜像 tag 后要 `docker compose pull`。
- **dept_sync 两份都要配**：kbcli（import-dept 写库用）+ server（组织树代理用），query_key 同一个。
- **kbcli 容器连 dept-sync 用宿主 IP**（dept-sync 是宿主进程时），不能用 127.0.0.1。
- **import ≠ efficiency-v2**：import 是全流程；单独重算效能用 efficiency-v2；kNN 锚点单独 import-anchor。
- **orgs/refresh**：import-dept 跑完调它（backend_url 配对才自动）；否则 `docker compose restart server`。
- **定时 import 不会冲掉部门**：import-org 兜底已非破坏 + import-dept 在流程里自愈（v1.1.19+）。
- **CI 查状态**：`gh` 默认指错仓库，必须 `-R papysans/efficiency-dashboard-v2`。
