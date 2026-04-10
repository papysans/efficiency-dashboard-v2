## 实施

### 阶段一：数据库层（DDL + 连接 + 模型）

- [x] 1.1 创建 costrict_stat 数据库的 DDL 文件
     【目标对象】`init_db_stat.sql`（新建）
     【修改目的】定义 costrict_stat 数据库的 tasks 表和 task_conversations 表，作为 Task 数据新的存储位置
     【修改方式】新建 SQL DDL 文件，参考现有 `init_db.sql` 的风格（IF NOT EXISTS、注释风格、类型风格）
     【相关依赖】参考现有 `init_db.sql` 的表结构风格
     【修改内容】
        - CREATE DATABASE costrict_stat（带 IF NOT EXISTS 检查）
        - CREATE TABLE tasks：
          - task_id VARCHAR(500) PRIMARY KEY UNIQUE
          - user_id VARCHAR(255), user_name VARCHAR(255)
          - client_id VARCHAR(255), client_ide VARCHAR(100), client_version VARCHAR(100), client_os VARCHAR(100), client_os_version VARCHAR(100)
          - caller VARCHAR(100)
          - repo_addr TEXT, repo_branch VARCHAR(500)
          - work_dir TEXT（原 project_path）, work_dir_id VARCHAR(500)（计算字段）
          - diff TEXT, diff_lines INT
          - start_time TIMESTAMPTZ, end_time TIMESTAMPTZ
          - upstream_tokens BIGINT, downstream_tokens BIGINT, cost FLOAT8
          - task_real_minutes FLOAT8, task_real_minutes_reason TEXT, task_real_minutes_manual FLOAT8, task_real_minutes_reason_manual TEXT
          - task_ancient_minutes FLOAT8, task_ancient_minutes_reason TEXT, task_ancient_minutes_manual FLOAT8, task_ancient_minutes_reason_manual TEXT
          - efficiency_ratio FLOAT8
          - created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
        - CREATE TABLE task_conversations：
          - (task_id, request_id) UNIQUE 联合唯一约束
          - sender VARCHAR(50), prompt_mode VARCHAR(50), mode VARCHAR(100), model VARCHAR(200)
          - start_time TIMESTAMPTZ, end_time TIMESTAMPTZ
          - process_time BIGINT, process_ttft BIGINT
          - upstream_tokens BIGINT, downstream_tokens BIGINT, cost FLOAT8
          - request_content TEXT, response_content TEXT, user_input TEXT
          - diff TEXT, diff_lines BIGINT
          - error_code VARCHAR(100), error_reason TEXT
          - created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
        - 注意：字段名用 client_ide 而非 ide，用 work_dir 而非 project_path，用 work_dir_id 而非 project_id，这是与现有 costrict_tasks 表的关键区别

- [x] 1.2 后端新增 costrict_stat 数据库连接配置和初始化
     【目标对象】`backend/config.yaml`、`backend/main.go`
     【修改目的】支持双数据库连接：report（现有）+ costrict_stat（新增），Task 相关读写切换到新数据库
     【修改方式】在 config.yaml 新增配置段；在 main.go 的 Config 结构体新增字段，新增 initStatDB() 函数和 `var statDB *sql.DB` 全局变量
     【相关依赖】现有 `DatabaseConfig` 结构体、`InitDB()` 函数（`backend/db.go`）、`loadConfig()` 函数（`backend/main.go`）
     【修改内容】
        - config.yaml：在 database 段之后新增 stat_database 段，包含 host/port/user/password=1/dbname=costrict_stat/sslmode=disable
        - main.go Config 结构体：新增 `StatDatabase DatabaseConfig \`yaml:"stat_database"\`` 字段
        - main.go loadConfig()：新增 StatDatabase 默认值（host=localhost, port=5432, user=postgres, password=1, dbname=costrict_stat, sslmode=disable）
        - main.go：新增 `var statDB *sql.DB` 全局变量
        - main.go main()：在 initDB() 之后调用 `statDB, err = InitDB(appConfig.StatDatabase)` 初始化 statDB 连接，失败时 log.Fatalf
        - 复用现有 InitDB() 函数，无需新增 initStatDB()

- [x] 1.3 后端新增 costrict_stat 数据模型和 CRUD 函数
     【目标对象】`backend/db.go`
     【修改目的】为 costrict_stat 数据库的 tasks 和 task_conversations 表提供数据操作函数
     【修改方式】在 db.go 中新增结构体和函数，所有函数使用 statDB 全局变量（与现有 CostrictTask 系列函数风格一致，均使用 *sql.DB 参数）
     【相关依赖】现有 `CostrictTask`/`CostrictTaskConversation` 结构体及其 CRUD 函数作为参考模板
     【修改内容】
        - 新增 StatTask 结构体（对应新 tasks 表），与 CostrictTask 的区别：
          - 字段 IDE→ClientIDE, Version→ClientVersion, OS→ClientOS, OSVersion→ClientOSVersion
          - 字段 ProjectPath→WorkDir, ProjectID→WorkDirID
          - 新增 Diff *string, EfficiencyRatio *float64
          - 去掉 RepoID（新表中不需要）
          - json tag 对应修改（如 `json:"client_ide"`, `json:"work_dir"`, `json:"work_dir_id"` 等）
        - 新增 StatTaskConversation 结构体（与 CostrictTaskConversation 字段基本一致，可直接复用类型）
        - 新增 selectColumns 变量和 scan 辅助函数（仿照现有 costrictTaskSelectColumns/scanCostrictTask 模式）
        - UpsertStatTask(db *sql.DB, t *StatTask) error：INSERT ... ON CONFLICT (task_id) DO UPDATE，SQL 占位符用 $1, $2...
        - GetStatTask(db *sql.DB, taskID string) (*StatTask, error)：不存在返回 nil, nil
        - ListStatTasks(db *sql.DB, userID, workDirID, startTime, endTime string, page, pageSize int) ([]StatTask, error)：动态构建 WHERE 条件
        - CountStatTasks(db *sql.DB, userID, workDirID, startTime, endTime string) (int, error)
        - UpdateStatTaskManual(db *sql.DB, taskID string, realManual *float64, realReasonManual *string, ancientManual *float64, ancientReasonManual *string) error
        - BatchInsertStatTaskConversations(db *sql.DB, convs []StatTaskConversation) error：使用事务，ON CONFLICT DO NOTHING
        - ListStatTaskConversations(db *sql.DB, taskID string) ([]StatTaskConversation, error)
        - 所有错误信息用中文 fmt.Errorf("xxx失败: %w", err) 风格，与现有代码一致

- [x] 1.4 新增 work_dir_id 生成函数
     【目标对象】`backend/id_utils.go`
     【修改目的】实现 work_dir_id 的生成算法，用于将 client_id + work_dir 路径生成唯一标识
     【修改方式】新增 generateWorkDirID 函数
     【相关依赖】现有 `toPathSafeID()` 函数（`backend/id_utils.go`，内部调用 `core.ToPathSafeID`）
     【修改内容】
        - 新增 `func generateWorkDirID(clientID, workDir string) string`
        - 算法：取 clientID 前6位 + "-" + workDir 路径安全化（只保留字母数字，其他字符替换为"-"，多个连续"-"合并为一个，移除首尾"-"）
        - 边界处理：clientID 为空时前缀为空字符串；clientID 不足6位时取全部；workDir 为空时后缀为空字符串
        - 可考虑复用 toPathSafeID() 对 workDir 做路径安全化，如果其逻辑匹配需求的话

### 阶段二：后端 Handler 层

- [x] 2.1 后端 Task handler 改为使用 costrict_stat 数据库
     【目标对象】`backend/task_handler_v2.go`
     【修改目的】Task 相关的 API handler 改为从 costrict_stat 数据库（statDB）读写，使用新的 StatTask 模型
     【修改方式】修改现有 handler 函数中的 db 参数替换为 statDB，将 CostrictTask 替换为 StatTask
     【相关依赖】`backend/db.go` 中的新 StatTask CRUD 函数，`backend/main.go` 中的 statDB 变量
     【修改内容】
        - upsertTaskV2()：ShouldBindJSON 绑定改为 StatTask；调用 UpsertStatTask(statDB, ...)
        - batchUpsertConversationsV2()：绑定改为 []StatTaskConversation；调用 BatchInsertStatTaskConversations(statDB, ...)
        - listTasksV2()：
          - 参数 projectId 改为 workDirId（c.Query("workDirId")）
          - 调用 CountStatTasks/ListStatTasks(statDB, ...)
          - 返回 JSON 中 project_path→work_dir, project_id→work_dir_id
          - efficiency_ratio 计算逻辑保持不变（优先使用 _manual 字段）
        - getTaskDetailV2()：
          - 调用 GetStatTask/ListStatTaskConversations(statDB, ...)
          - calculateTaskRealMinutes 函数签名中的 conversations 参数类型需适配 StatTaskConversation（如果字段兼容则无需修改）
          - 异步更新 task_real_minutes 的 SQL 改为操作 statDB 和新表 tasks
        - updateTaskManualV2()：调用 UpdateStatTaskManual(statDB, ...)
        - fixAncientMinutes()：SQL 中的表名 costrict_tasks 改为 tasks，db 改为 statDB

- [x] 2.2 后端路由确认
     【目标对象】`backend/main.go`
     【修改目的】确保 /api/v2/tasks 相关路由正确连接到更新后的 handler（handler 函数名不变，内部改用 statDB）
     【修改方式】检查并确认现有路由注册无需修改（因为 handler 函数名不变）
     【相关依赖】`backend/task_handler_v2.go` 中的 handler 函数
     【修改内容】
        - 确认 v2.POST("/tasks", upsertTaskV2) 等6条路由的 handler 名称不变，无需修改路由注册代码
        - 注意：如果 fixAncientMinutes 也指向 statDB，需在 handler 内部改（已在2.1中处理）

### 阶段三：前端适配

- [x] 3.1 前端 ProjectDetailV2.vue 改名为 WorkDirDetailV2.vue
     【目标对象】`frontend/src/views/ProjectDetailV2.vue` → `frontend/src/views/WorkDirDetailV2.vue`
     【修改目的】将 Project 维度改为 WorkDir 维度，适配需求中的 work_dir 概念
     【修改方式】重命名文件，修改组件内部文字和路由参数引用
     【相关依赖】`frontend/src/router/index.js`、`frontend/src/App.vue`
     【修改内容】
        - 重命名文件 ProjectDetailV2.vue → WorkDirDetailV2.vue
        - 修改页面标题 "仓库详情" → "工作目录详情"
        - 修改路由参数引用：route.params.repoId/projectId → route.params.workDirId
        - API 调用适配（如 getProjectDetailV2 → 后续看是否需要新建 API 或复用）

- [x] 3.2 前端路由和导航更新
     【目标对象】`frontend/src/router/index.js`
     【修改目的】新增 /workdir/:workDirId 路由，指向 WorkDirDetailV2.vue
     【修改方式】修改路由配置，新增路由条目
     【相关依赖】`frontend/src/views/WorkDirDetailV2.vue`
     【修改内容】
        - 新增路由：`{ path: '/workdir/:workDirId', name: 'WorkDirDetail', component: () => import('@/views/WorkDirDetailV2.vue') }`
        - 保留现有 `/project/:projectId` 路由（向后兼容）和 `/repo/:repoId` 路由
        - App.vue 导航菜单无需修改（现有导航中没有 "项目" 菜单项，无需改名）

- [x] 3.3 前端 TaskDetailV2.vue 适配新字段和跳转
     【目标对象】`frontend/src/views/TaskDetailV2.vue`
     【修改目的】展示新字段（work_dir, work_dir_id, client_ide 等），实现跳转链接
     【修改方式】修改 el-descriptions 中的字段展示和跳转逻辑
     【相关依赖】router 配置（/workdir/:workDirId 路由）、API 返回的新字段
     【修改内容】
        - "Project" 描述项改为 "工作目录"：
          - 原：`<el-link ... @click="router.push('/project/' + task.project_id)">{{ task.project_id }}</el-link>`
          - 改为：展示 work_dir，可点击 work_dir_id 跳转到 `/workdir/${task.work_dir_id}`
        - "仓库" 描述项：repo_addr 字段可点击跳转到 `/repo/${encodeURIComponent(task.repo_id)}`（保持现有逻辑不变，因为repo_id仍存在于返回中是通过后端计算的）
        - 新增字段展示（在合适位置插入 el-descriptions-item）：
          - "IDE"：展示 client_ide 字段
          - "版本"：展示 client_version 字段
          - "操作系统"：展示 client_os + client_os_version
          - "模式"：展示 caller 字段（如果现有未展示）
        - user_id 跳转保持不变（已有）

- [x] 3.4 前端 TaskViewV2.vue 列表页适配新字段
     【目标对象】`frontend/src/views/TaskViewV2.vue`
     【修改目的】列表页展示新字段，将 project_id 列替换为 work_dir 相关列
     【修改方式】修改 el-table-column 列配置
     【相关依赖】API 返回的新字段名（work_dir, work_dir_id 替代 project_path, project_id）
     【修改内容】
        - 将 `prop="project_id" label="项目"` 列改为 `prop="work_dir_id" label="工作目录"` 或展示 work_dir
        - 其他列保持不变

- [x] 3.5 前端 API 函数适配（如需要）
     【目标对象】`frontend/src/api/es.js`
     【修改目的】确认现有 API 函数是否需要调整参数
     【修改方式】检查并按需修改
     【相关依赖】后端 API 参数变更（projectId → workDirId）
     【修改内容】
        - getTasksV2：调用参数中如有 projectId，改为 workDirId
        - 其他 Task 相关 API 函数（getTaskDetailV2, updateTaskManualV2）无需修改（URL 和方法不变）

### 阶段四：kbcli 导入命令 + 测试数据

- [x] 4.1 新增 kbcli import-tasks 子命令
     【目标对象】`kbcli/cmd_import_tasks.go`（新建）、`kbcli/cmd_root.go`
     【修改目的】扫描本地 task/summary/ 和 task/conversation/ 目录，解析后直接写入 costrict_stat 数据库
     【修改方式】新建 cmd_import_tasks.go 文件实现导入逻辑；在 cmd_root.go 的 switch-case 中注册 "import-tasks" 子命令
     【相关依赖】kbcli 现有命令架构（`cmd_root.go` RunCLI switch-case 模式）、`kbcli/pg_writer.go` 中的 PG 连接方式、`kbcli/config.go` 配置
     【修改内容】
        - cmd_import_tasks.go：
          - 新增 `func runImportTasks(config *Config, args []string)` 函数
          - 参数：--task-dir（默认 ./task）、--stat-db-dsn（或从 config 读取 costrict_stat 数据库连接信息）
          - 直连 costrict_stat 数据库（使用 sql.Open("postgres", dsn)），不走 HTTP API（与 kbcli 现有直连 PG 的风格一致）
          - 扫描 task/summary/ 下所有 .json 文件（支持 YYYY/MM/DD/$task_id.json 目录结构）
          - 对每个 task_summary.json：
            1. 解析 JSON 为 task 结构体
            2. 查找对应的 task/conversation/YYYY/MM/DD/$task_id.jsonl 文件
            3. 解析 .jsonl 文件（每行一个 JSON）为 conversation 列表
            4. 从 conversation 累加计算 start_time（最小值）、end_time（最大值）、upstream_tokens、downstream_tokens、cost 总和
            5. 调用 generateWorkDirID(clientID, workDir) 生成 work_dir_id
            6. 用 INSERT ... ON CONFLICT (task_id) DO UPDATE 写入 tasks 表
            7. 用事务批量写入 task_conversations 表（ON CONFLICT DO NOTHING）
          - 成功导入单个 task 后，将 task/summary/ 中对应文件 mv 到 task/analysed/YYYY/MM/DD/$task_id.json（保持原目录结构）
          - 如果 task/analysed/ 目录不存在则自动创建
          - 错误处理：单个文件导入失败时打印错误并 continue（不中断整体流程），最终汇总成功/失败数
        - cmd_root.go：
          - 在 switch-case 中新增 `case "import-tasks": runImportTasks(config, subArgs)`
          - 在 printUsage() 中新增 import-tasks 子命令说明
        - 注意：kbcli 的 config.go 可能需要新增 costrict_stat 数据库连接配置字段

- [x] 4.2 生成模拟测试数据文件
     【目标对象】`tools/gen_test_data/main.go`（新建目录和文件）
     【修改目的】生成 15 个模拟 task 的 task_summary.json 和 task_conversation.jsonl 文件，方便端到端测试
     【修改方式】新建 Go 程序，用 go run 执行
     【相关依赖】需求中定义的 JSON 字段结构（与 task 1.1 DDL 中的字段对应）
     【修改内容】
        - 新建 tools/gen_test_data/main.go
        - 生成 15 个 task，分布在 2026-04-01 ~ 2026-04-05 之间
        - 每个 task 生成 task/summary/YYYY/MM/DD/$task_id.json（包含 task 基本信息）
        - 每个 task 生成 task/conversation/YYYY/MM/DD/$task_id.jsonl（每个 task 3~8 条 conversation，每行一个 JSON）
        - 覆盖多种场景：
          - 不同 user_id/user_name（至少3个用户）
          - 不同 repo_addr/repo_branch
          - 不同 client_ide（vscode/jetbrains/cli）
          - 不同 caller（user/agent）
        - 包含一些有 error_code 的 conversation（约10%~20%）
        - diff_lines 值在 50~2000 之间随机
        - 包含 diff 文本内容（简单模拟）

### 阶段五：验证与确认

- [x] 5.1 创建 costrict_stat 数据库并执行 DDL
     【目标对象】数据库（PostgreSQL costrict_stat）
     【修改目的】初始化 costrict_stat 数据库和表
     【修改方式】在 PostgreSQL 中执行 init_db_stat.sql
     【相关依赖】任务 1.1 产出的 `init_db_stat.sql`
     【修改内容】
        - 连接 PostgreSQL 创建 costrict_stat 数据库（如果不存在）
        - 切换到 costrict_stat 数据库执行 DDL 创建 tasks 和 task_conversations 表

- [x] 5.2 运行测试数据生成 + 导入，验证 UI
     【目标对象】整体验证
     【修改目的】端到端验证：生成数据 → 导入数据库 → 前端展示
     【修改方式】依次运行工具和导入命令，手动验证前端页面
     【相关依赖】任务 4.1（kbcli import-tasks）、任务 4.2（gen_test_data）、前端页面
     【修改内容】
        - 运行 `go run tools/gen_test_data/main.go` 生成模拟数据文件
        - 运行 `kbcli import-tasks --task-dir=./task` 导入数据到 costrict_stat
        - 验证导入后 task/summary/ 中的文件已被 mv 到 task/analysed/
        - 验证 /task-v2 列表页正确显示数据（work_dir_id 列替代 project_id）
        - 验证 /task/:taskId 详情页正确显示所有新字段（client_ide, work_dir, work_dir_id, caller 等）
        - 验证跳转链接：work_dir_id → /workdir/:workDirId 详情页、repo_addr → /repo/:repoId 详情页、user_id → /user/:userId 详情页

### 确认项（无需修改）

- [x] task_real_minutes 计算算法：现有 `calculateTaskRealMinutes()` 函数（`backend/task_handler_v2.go`）已实现 gap_threshold + extension 算法，无需修改
- [x] efficiency_ratio 计算：现有 `listTasksV2()` 和 `getTaskDetailV2()` 中已实现优先使用 _manual 字段的逻辑，无需修改计算逻辑（仅需确保新 StatTask 结构体包含相同字段）
- [x] formatDuration 显示规则：前端 `formatDuration()` 函数（`frontend/src/utils/formatters.js`）已有且逻辑正确，无需修改

### 阶段四审查修复

- [x] refactor-task-data | task: 4.2-fix-1 修复 gen_test_data 中 ErrorCode 类型不匹配
     【目标对象】`tools/gen_test_data/main.go`
     【修改目的】gen_test_data 中 ConversationEntry.ErrorCode 为 `*int`，生成的 JSON error_code 是数字（如500），而 kbcli 导入端 taskConversation.ErrorCode 为 `*string`（对应数据库 VARCHAR(100)），JSON unmarshal 数字到 string 会失败
     【修改方式】将 ErrorCode 类型从 `*int` 改为 `*string`，errorCodes 数组从 int 改为 string
     【修改内容】
        - ConversationEntry 结构体中 `ErrorCode *int` → `ErrorCode *string`
        - `var errorCodes = []int{500, 429, ...}` → `var errorCodes = []string{"500", "429", ...}`
        - 生成 error 时 `code := errorCodes[idx]` 赋值逻辑保持指针风格不变
        - 编译验证通过
