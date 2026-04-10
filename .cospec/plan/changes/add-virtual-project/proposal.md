# 变更：添加虚拟项目 (Virtual Project) 功能

## 原因
项目经理需要一个"虚拟项目"视图，用于统计一个项目周期内的 AI Coding 提效情况、参与人员、成本等。当前系统缺乏跨 repo/task 的聚合维度，且旧的 "project" 概念（实际指 work_dir/repo）残留在代码中造成语义混乱。

## 变更内容

### 一、清理旧 project 残留
- 将 V2 后端 handler 中的 `project_count`（实为 `COUNT(DISTINCT work_dir_id)`）改为 `work_dir_count`
- 将前端 UserViewV2、OrgViewV2 中对应的字段名和列标题同步修改
- 删除不在路由中的旧面板文件 `ProjectPanel.vue`
- 清理 V1 后端 `aggregate_handler.go` 和 `efficiency_handler.go` 中的 `project` 维度映射，改为 `work_dir`
- 清理 `EfficiencyPanel.vue` 中 project 维度选项

### 二、新建 projects 表（costrict_stat 数据库）
- 创建 `projects` 表，存储虚拟项目的配置和聚合数据
- 核心字段：project_id(UUID PK)、name、repos(JSONB)、task_ids(JSONB)、task_ids_silica(JSONB)
- 自动计算字段：start_time、end_time、upstream/downstream_tokens、cost、各种 minutes
- 手动修正字段：start_time_manual、end_time_manual、各种 minutes_manual + reason_manual
- DDL 追加到 init_db_stat.sql

### 三、后端 Project API（V2 体系）
- **CRUD**：创建/列表/详情/更新/删除 project
- **手动修正**：PUT /api/v2/projects/:projectId/manual
- **添加 tasks**：POST /api/v2/projects/:projectId/tasks（从 task 页面多选添加）
- **添加 repo 规则**：POST /api/v2/projects/:projectId/repos（从 repo 详情页添加）
- **删除 repo 规则**：DELETE /api/v2/projects/:projectId/repos/:index
- **冲突检测**：POST /api/v2/projects/check-conflicts（检查 commit 是否已属于其他 project）
- **聚合计算**：配置变更时自动重算 tokens、cost、minutes 等聚合字段

### 四、前端 Project 页面
- **ProjectViewV2.vue** (/project-v2)：复用 KbFilterTable 的列表页，含创建按钮
- **ProjectDetailV2.vue** (/project/:projectId)：复用 TaskDetailV2 布局模式的详情页，展示配置、度量、关联 repos/tasks/commits、手动修正、提效比例
- 路由注册 + 侧边导航添加入口

### 五、现有页面改造
- **TaskViewV2.vue**：添加多选 checkbox + "添加到 Project" 按钮
- **RepoDetailV2.vue**：添加"添加到 Project" 按钮 + 配置对话框（时间范围/白名单/黑名单）
- 冲突提示交互

### 六、API 封装
- `frontend/src/api/es.js` 中新增所有 project 相关 API 调用方法

## 影响
- **受影响的代码**：
  - `backend/user_handler_v2.go`: `project_count` → `work_dir_count`
  - `backend/org_handler_v2.go`: `project_count` → `work_dir_count`
  - `backend/aggregate_handler.go`: project 维度映射改为 work_dir
  - `backend/efficiency_handler.go`: project 维度映射改为 work_dir
  - `backend/db.go`: 新增 Project 相关 CRUD 函数
  - `backend/main.go`: 注册新路由
  - 新增 `backend/project_handler_v2.go`: Project API handler
  - `init_db_stat.sql`: 新增 projects 表 DDL
  - `frontend/src/views/UserViewV2.vue`: project_count → work_dir_count
  - `frontend/src/views/OrgViewV2.vue`: project_count → work_dir_count
  - 删除 `frontend/src/views/ProjectPanel.vue`
  - `frontend/src/views/EfficiencyPanel.vue`: project 维度改为 work_dir
  - 新增 `frontend/src/views/ProjectViewV2.vue`: Project 列表页
  - 新增 `frontend/src/views/ProjectDetailV2.vue`: Project 详情页
  - `frontend/src/views/TaskViewV2.vue`: 添加多选和"添加到 Project" 功能
  - `frontend/src/views/RepoDetailV2.vue`: 添加"添加到 Project" 功能
  - `frontend/src/router/index.js`: 新增路由
  - `frontend/src/api/es.js`: 新增 API 方法
  - `frontend/src/views/Home.vue`: 导航添加 Project 入口
