## 实施

### Phase 1: 清理旧 project 残留

- [x] 1.1 后端 V2 handler 中 project_count 改名为 work_dir_count
     【目标对象】`backend/user_handler_v2.go`, `backend/org_handler_v2.go`
     【修改目的】消除旧 project 概念（实为 work_dir 计数）的语义混乱
     【修改方式】SQL 别名和 Go 结构体字段名 `project_count` → `work_dir_count`
     【修改内容】
        - `user_handler_v2.go`: SQL 中 `COUNT(DISTINCT work_dir_id) as project_count` → `as work_dir_count`，Go struct 字段同步
        - `org_handler_v2.go`: 同上
        - `dashboard_handler_v2.go`: 如有 `total_projects` 改为 `total_work_dirs`（检查并修改）

- [x] 1.2 前端 V2 页面中 project_count 改名
     【目标对象】`frontend/src/views/UserViewV2.vue`, `frontend/src/views/OrgViewV2.vue`
     【修改目的】前端字段名和列标题与后端同步
     【修改方式】将表格列定义中 `project_count` 改为 `work_dir_count`，列标题改为"工作目录数"
     【修改内容】
        - `UserViewV2.vue`: 列 prop 和 label 修改
        - `OrgViewV2.vue`: 列 prop 和 label 修改

- [x] 1.3 清理 V1 体系中的 project 维度
     【目标对象】`backend/aggregate_handler.go`, `backend/efficiency_handler.go`, `frontend/src/views/EfficiencyPanel.vue`
     【修改目的】将 V1 中 "project" 维度改为 "work_dir" 避免与新 project 概念冲突
     【修改方式】替换维度映射键名和前端选项
     【修改内容】
        - `aggregate_handler.go`: `dimensionFieldMap` 中 `"project": "project_id"` → `"work_dir": "project_id"`
        - `efficiency_handler.go`: dimension 参数校验和处理中 `"project"` → `"work_dir"`
        - `EfficiencyPanel.vue`: 维度下拉选项 "project" → "work_dir"

- [x] 1.4 删除不在路由中的旧面板文件
     【目标对象】`frontend/src/views/ProjectPanel.vue`
     【修改目的】删除已废弃的旧面板文件，避免与新 project 页面混淆
     【修改方式】删除文件
     【修改内容】
        - 删除 `ProjectPanel.vue`
        - 搜索确认无其他文件引用此组件

### Phase 2: 数据库 + 后端 Project CRUD

- [x] 2.1 创建 projects 表 DDL
     【目标对象】`init_db_stat.sql`
     【修改目的】在 costrict_stat 数据库中新增 projects 表存储虚拟项目数据
     【修改方式】在 init_db_stat.sql 末尾追加 CREATE TABLE IF NOT EXISTS projects
     【修改内容】
        - project_id UUID DEFAULT gen_random_uuid() PRIMARY KEY
        - name VARCHAR(500) NOT NULL
        - description TEXT
        - repos JSONB DEFAULT '[]' (repo 过滤条件数组)
        - task_ids JSONB DEFAULT '[]' (task ID 数组)
        - task_ids_silica JSONB DEFAULT '[]' (权重数组，与 task_ids 一一对应)
        - start_time TIMESTAMPTZ (自动计算：所有关联数据中最早时间)
        - end_time TIMESTAMPTZ (自动计算：所有关联数据中最晚时间)
        - start_time_manual TIMESTAMPTZ (用户手动配置的开始时间)
        - end_time_manual TIMESTAMPTZ (用户手动配置的结束时间)
        - upstream_tokens BIGINT DEFAULT 0
        - downstream_tokens BIGINT DEFAULT 0
        - cost FLOAT8 DEFAULT 0
        - project_ancient_minutes FLOAT8 (传统开发时长预估总和)
        - project_ancient_minutes_reason TEXT
        - project_ancient_minutes_manual FLOAT8
        - project_ancient_minutes_reason_manual TEXT
        - project_real_process_minutes FLOAT8 (实际 process_time 累加)
        - project_real_process_minutes_reason TEXT
        - project_real_process_minutes_manual FLOAT8
        - project_real_process_minutes_reason_manual TEXT
        - project_real_lead_minutes FLOAT8 (end_time - start_time)
        - project_real_lead_minutes_reason TEXT
        - project_real_lead_minutes_manual FLOAT8
        - project_real_lead_minutes_reason_manual TEXT
        - created_at TIMESTAMPTZ DEFAULT NOW()
        - updated_at TIMESTAMPTZ DEFAULT NOW()

- [x] 2.2 后端 Project 数据库 CRUD 函数
     【目标对象】`backend/db.go`
     【修改目的】新增 Project 相关的数据库操作函数
     【相关依赖】2.1 的 projects 表结构
     【修改方式】在 db.go 末尾追加函数，参数统一使用 `db *sql.DB`（调用时传 statDB）
     【修改内容】
        - `CreateProject(db, project) (projectID, error)`: INSERT 新建项目，返回生成的 UUID
        - `GetProject(db, projectID) (*Project, error)`: SELECT WHERE project_id
        - `ListProjects(db) ([]Project, error)`: SELECT 全量列表，按 updated_at DESC 排序
        - `UpdateProject(db, project) error`: UPDATE 更新 repos/task_ids/task_ids_silica + 聚合字段
        - `DeleteProject(db, projectID) error`: DELETE WHERE project_id
        - `UpdateProjectManual(db, projectID, manualFields) error`: UPDATE 手动修正字段
        - `UpdateProjectAggregates(db, projectID, aggregates) error`: UPDATE 聚合计算字段
        - Go struct `Project` 定义，对应 projects 表所有列

- [x] 2.3 后端 Project 聚合计算逻辑
     【目标对象】新建 `backend/project_handler_v2.go`
     【修改目的】实现 project 聚合计算：从 repos 过滤 commits + 直接 task_ids → 去重 → 累加 tokens/cost/minutes
     【相关依赖】`backend/db.go` 中的 `ListStatCommits`, `GetStatTask`, `ListStatTasks` 函数
     【修改方式】新建文件，实现 `recalculateProjectAggregates(projectID)` 核心函数
     【修改内容】
        - 解析 project.repos JSONB：对每个 repo 条件，根据 repo_addr + repo_branch + start_time/end_time 查询 commits 表
        - 应用白名单/黑名单：如 include_only_commits 非空则只取白名单；否则取时间范围内 commits 减去 exclude_commits
        - 从所有匹配的 commits 提取 task_ids（去重）
        - 合并 project.task_ids（直接指定的 task）与 commits 提取的 task_ids，去重
        - 逐个获取 task 数据，累加 upstream_tokens/downstream_tokens/cost
        - 累加 project_ancient_minutes（commits 和 tasks 的 ancient_minutes 之和）
        - 累加 project_real_process_minutes（commits 和 tasks 的 real_minutes 之和）
        - 计算 start_time/end_time（所有数据中最早/最晚时间）
        - 计算 project_real_lead_minutes = end_time - start_time
        - 调用 UpdateProjectAggregates 写回数据库

- [x] 2.4 后端 Project API Handler
     【目标对象】`backend/project_handler_v2.go`, `backend/main.go`
     【修改目的】实现 Project 的 RESTful API 端点
     【相关依赖】2.2 的 CRUD 函数，2.3 的聚合计算函数
     【修改方式】在 project_handler_v2.go 中实现 handler 函数，在 main.go 中注册路由
     【修改内容】
        - `createProjectV2`: POST /api/v2/projects → 创建 project，触发聚合计算
        - `listProjectsV2`: GET /api/v2/projects → 列表查询
        - `getProjectDetailV2`: GET /api/v2/projects/:projectId → 详情（含关联 commits/tasks 列表）
        - `updateProjectV2`: PUT /api/v2/projects/:projectId → 更新配置，触发重算
        - `deleteProjectV2`: DELETE /api/v2/projects/:projectId → 删除
        - `updateProjectManualV2`: PUT /api/v2/projects/:projectId/manual → 手动修正
        - `addTasksToProjectV2`: POST /api/v2/projects/:projectId/tasks → 添加 task_ids + silica，触发重算
        - `addRepoToProjectV2`: POST /api/v2/projects/:projectId/repos → 添加 repo 过滤规则，触发重算
        - `removeRepoFromProjectV2`: DELETE /api/v2/projects/:projectId/repos/:index → 删除 repo 规则，触发重算
        - `checkProjectConflictsV2`: POST /api/v2/projects/check-conflicts → 检查 commits 是否已属于其他 project
        - 在 main.go v2 路由组中注册所有端点

### Phase 3: 前端 Project 页面

- [x] 3.1 新增 Project API 封装
     【目标对象】`frontend/src/api/es.js`
     【修改目的】新增 project 相关的所有 API 调用方法
     【修改方式】在 es.js 末尾追加 API 方法
     【修改内容】
        - `createProject(data)`: POST /v2/projects
        - `getProjects()`: GET /v2/projects
        - `getProjectDetail(projectId)`: GET /v2/projects/${projectId}
        - `updateProject(projectId, data)`: PUT /v2/projects/${projectId}
        - `deleteProject(projectId)`: DELETE /v2/projects/${projectId}
        - `updateProjectManual(projectId, data)`: PUT /v2/projects/${projectId}/manual
        - `addTasksToProject(projectId, data)`: POST /v2/projects/${projectId}/tasks
        - `addRepoToProject(projectId, data)`: POST /v2/projects/${projectId}/repos
        - `removeRepoFromProject(projectId, index)`: DELETE /v2/projects/${projectId}/repos/${index}
        - `checkProjectConflicts(data)`: POST /v2/projects/check-conflicts

- [x] 3.2 新建 Project 列表页
     【目标对象】新建 `frontend/src/views/ProjectViewV2.vue`
     【修改目的】Project 列表页，复用 KbFilterTable 标准模式
     【相关依赖】`KbFilterTable.vue` 组件，3.1 的 API 方法
     【修改方式】复用 TaskViewV2.vue 的列表页模式创建新组件
     【修改内容】
        - KbFilterTable 列定义：name(text筛选)、start_time(date)、end_time(date)、repo_count(number)、task_count(number)、cost(number)、project_ancient_minutes(number)、project_real_process_minutes(number)、efficiency_ratio(自定义插槽,彩色tag)
        - 顶部工具栏："+创建项目"按钮，点击弹出创建对话框（输入 name + description）
        - 行点击：router.push 到 /project/${row.project_id}
        - 提效比 = (effective_ancient / effective_real_process) * 100，manual 优先取值

- [x] 3.3 新建 Project 详情页
     【目标对象】新建 `frontend/src/views/ProjectDetailV2.vue`
     【修改目的】Project 详情页，展示配置、度量、关联数据、手动修正、提效比例
     【相关依赖】3.1 的 API 方法，TaskDetailV2.vue 的布局模式
     【修改方式】复用 TaskDetailV2 的布局模式（el-card + el-descriptions + el-table），新建组件
     【修改内容】
        - **标题区**: 返回按钮 + 项目名称(可编辑) + "人工调整"按钮
        - **基础信息 el-descriptions**: 项目名称、描述、起始时间(含manual覆盖逻辑)、结束时间、创建/更新时间
        - **度量信息 el-descriptions**: 
          - 传统开发预估 (project_ancient_minutes, manual优先+tooltip)
          - 实际处理耗时 (project_real_process_minutes, manual优先+tooltip)
          - 项目周期 (project_real_lead_minutes, manual优先+tooltip)
          - 总Tokens、总费用
          - **提效比展示**：传统预估/实际处理 的百分比，大字体彩色显示
        - **Repos 配置区 el-card**: 
          - el-table 展示每个 repo 规则：repo_addr、repo_branch、时间范围、白名单/黑名单 commits 数、操作(删除按钮)
          - "+添加 Repo" 按钮
        - **Tasks 列表 el-table**: task_id(链接)、用户、时长、silica权重、费用，支持删除
        - **Commits 列表 el-table**: commit_id(链接)、用户、时间、代码行、时长
        - **手动修正对话框 el-dialog**: 
          - project_ancient_minutes_manual + reason
          - project_real_process_minutes_manual + reason
          - project_real_lead_minutes_manual + reason
          - start_time_manual、end_time_manual

- [x] 3.4 注册路由和导航入口
     【目标对象】`frontend/src/router/index.js`, `frontend/src/views/Home.vue`
     【修改目的】将 Project 页面注册到路由并在导航中添加入口
     【修改方式】追加路由配置和导航链接
     【修改内容】
        - router/index.js: 添加 `/project-v2` → ProjectViewV2, `/project/:projectId` → ProjectDetailV2
        - Home.vue: 导航栏添加 "Project" 入口（与 Task/Repo/Commit 等并列）

### Phase 4: 现有页面改造（添加到 Project 功能）

- [x] 4.1 TaskViewV2 添加多选和"添加到 Project"功能
     【目标对象】`frontend/src/views/TaskViewV2.vue`
     【修改目的】允许用户在 task 列表页多选 task 并添加到指定 project
     【相关依赖】3.1 的 `addTasksToProject` 和 `getProjects` API
     【修改方式】在现有 KbFilterTable 基础上添加 selection column 和操作按钮
     【修改内容】
        - KbFilterTable 添加 `selection` 类型列（el-table-column type="selection"）
        - 顶部工具栏添加"添加到 Project"按钮（选中数量 > 0 时可用）
        - 点击弹出 el-dialog：
          - el-select 选择现有 project（从 getProjects 加载列表），或"+ 创建新 Project"选项
          - 创建新 project 时显示名称输入框
          - 展示已选 task 列表（数量、用户、时长摘要）
          - silica 权重设置：统一设置默认 1.0，支持逐个调整
          - 确认按钮 → 调用 addTasksToProject API

- [x] 4.2 RepoDetailV2 添加"添加到 Project"功能
     【目标对象】`frontend/src/views/RepoDetailV2.vue`
     【修改目的】允许用户从 repo 详情页将该 repo（含过滤条件）添加到指定 project
     【相关依赖】3.1 的 `addRepoToProject`, `getProjects`, `checkProjectConflicts` API
     【修改方式】在标题栏添加操作按钮，新增配置对话框
     【修改内容】
        - 标题栏添加"添加到 Project"按钮
        - 点击弹出 el-dialog：
          - el-select 选择目标 project（或创建新 project）
          - 时间范围选择：el-date-picker daterange（支持不选=从最开始到现在）
          - 白名单模式切换：el-switch "仅包含指定 commits"
            - 开启时：显示当前 repo 下 commits 多选表格，用户勾选要包含的 commits
            - 关闭时：显示可选的排除 commits 多选表格
          - 冲突检测：确认前调用 checkProjectConflicts API
            - 有冲突时显示 el-alert 警告：列出冲突的 commit ID 和所属 project 名称
            - 用户可选择继续添加或取消
          - 确认按钮 → 调用 addRepoToProject API

### Phase 1 审查修复

- [x] add-virtual-project | task: 1.1-fix-1 修复 db.go 中 UpdateUserManualDays 维度判断 BUG
     【目标对象】`backend/db.go`
     【修改目的】efficiency_handler.go 已将外部维度从 "project" 改为 "work_dir"，但 db.go 中 UpdateUserManualDays 的内部分支判断未同步，导致 work_dir 维度数据会错误写入 repo_metrics 表
     【修改内容】`dimension == "project"` → `dimension == "work_dir"`

- [x] add-virtual-project | task: 1.2-fix-1 修复 UserDetailV2.vue 中 project_count 残留
     【目标对象】`frontend/src/views/UserDetailV2.vue`
     【修改目的】后端已将字段改为 work_dir_count，但用户详情页未同步修改
     【修改内容】`detailData?.user?.project_count` → `detailData?.user?.work_dir_count`

- [x] add-virtual-project | task: 1.1-fix-2 修复 dashboard_handler_v2.go 错误信息残留
     【目标对象】`backend/dashboard_handler_v2.go`
     【修改内容】错误信息 "查询 projects 聚合失败" → "查询 work_dirs 聚合失败"

- [x] add-virtual-project | task: 1.1-fix-3 统一局部变量名 projectCount → workDirCount
     【目标对象】`backend/org_handler_v2.go`, `backend/user_handler_v2.go`
     【修改内容】Scan 目标局部变量名 `projectCount` → `workDirCount`，与 struct 字段名保持一致

### Phase 4 审查修复

- [x] add-virtual-project | task: 4.1-fix-1 修复 TaskViewV2 和 RepoDetailV2 中 project_id 字段名错误
     【目标对象】`frontend/src/views/TaskViewV2.vue`, `frontend/src/views/RepoDetailV2.vue`
     【修改目的】后端 API 返回的 project 对象使用 `project_id` 字段，但前端错误使用了 `p.id` 和 `data.id`
     【修改内容】
        - TaskViewV2.vue 第57行：el-option 的 `:key="p.id"` `:value="p.id"` → `:key="p.project_id"` `:value="p.project_id"`
        - TaskViewV2.vue 第327行：`projectId = data.id` → `projectId = data.project_id`
        - RepoDetailV2.vue 第150行：el-option 的 `:key="p.id"` `:value="p.id"` → `:key="p.project_id"` `:value="p.project_id"`
        - RepoDetailV2.vue 第449行：`projectId = data.id` → `projectId = data.project_id`
