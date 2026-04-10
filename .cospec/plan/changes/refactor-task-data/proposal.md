# 变更：Task 数据体系重构

## 原因
现有 task 数据存储在 report 数据库的 costrict_tasks 表中，需求要求迁移到新的 costrict_stat 数据库，同时新增 work_dir_id 字段、重命名 project_path→work_dir，并且数据来源改为 kbcli 扫描本地文件（task_summary.json + task_conversation.jsonl），后端重构为双数据库连接模式。前端 Task 详情页需适配新字段和跳转逻辑。需要生成模拟测试数据用于端到端 UI 测试。

## 变更内容
- 创建 costrict_stat 数据库，新建 tasks 表和 task_conversations 表（新字段结构）
- 后端新增 costrict_stat 数据库连接，Task 相关 API 改为从新数据库读写
- 后端 config.yaml 新增 costrict_stat 数据库配置
- 新增 kbcli 子命令 `import-tasks`：扫描 task/summary/ 和 task/conversation/ 目录，解析后写入 costrict_stat 数据库
- work_dir_id 生成算法：client_id 前6位 + "-" + work_dir 路径安全化
- task_real_minutes 算法已有实现，保持不变（已与需求一致）
- efficiency_ratio 计算保持不变（已与需求一致）
- 前端 ProjectDetailV2.vue 改名为 WorkDirDetailV2.vue
- 前端 TaskDetailV2 适配新字段：work_dir_id 跳转到 WorkDirDetail、repo_addr 跳转到 repo 详情、user_id 跳转到用户详情
- 生成模拟 task_summary.json 和 task_conversation.jsonl 文件（10~20个task），并通过导入脚本写入数据库

## 影响
- **受影响的代码**：
  - `backend/config.yaml`: 新增 costrict_stat 数据库配置段
  - `backend/main.go`: 新增 costrict_stat 数据库连接初始化，路由调整
  - `backend/db.go`: 新增 costrict_stat 数据库操作函数（CostrictStatTask/CostrictStatTaskConversation 模型，Upsert/List/Get/Count 函数）
  - `backend/task_handler_v2.go`: Task API handler 改为使用 costrict_stat 数据库
  - `backend/id_utils.go`: 新增 generateWorkDirID() 函数
  - `init_db_stat.sql`: 新建 costrict_stat 数据库的 DDL
  - `kbcli/cmd_import_tasks.go`: 新增 import-tasks 子命令
  - `frontend/src/views/TaskDetailV2.vue`: 适配新字段和跳转
  - `frontend/src/views/TaskViewV2.vue`: 列表页适配
  - `frontend/src/views/ProjectDetailV2.vue` → `WorkDirDetailV2.vue`: 重命名
  - `frontend/src/router/index.js`: 路由更新（project→workdir）
  - `frontend/src/api/es.js`: API 函数适配
  - `frontend/src/App.vue`: 导航菜单适配
  - `tools/gen_test_data.go` (新增): 生成模拟测试数据
