## 实施

- [x] 1.1 在 init_db.sql 中新增 4 张核心表的 DDL
     【目标对象】`init_db.sql`
     【修改目的】建立设计文档要求的 task→commit→project 三层数据模型，为后续数据管道、关联引擎、UI 提供存储基础
     【修改方式】在现有表定义（`favorites` 表和最后的 `DO $$ ... END $$` 块）之后追加新表 DDL，使用 `CREATE TABLE IF NOT EXISTS`（与现有表风格一致），不修改已有表
     【相关依赖】无（新增独立表，不依赖现有旧表）
     【修改内容】
        - 新增 `costrict_tasks` 表：id SERIAL PRIMARY KEY, task_id VARCHAR UNIQUE NOT NULL, user_id, user_name, client_id, ide, version, os, os_version, caller, repo_addr, repo_branch, repo_id, project_path, project_id, start_time TIMESTAMP, end_time TIMESTAMP, upstream_tokens BIGINT, downstream_tokens BIGINT, cost DECIMAL(10,2), diff_lines BIGINT, ai_estimated_ancient_days DECIMAL(10,2), ai_estimated_ancient_reason TEXT, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        - 新增 `costrict_task_conversations` 表：id SERIAL PRIMARY KEY, task_id VARCHAR NOT NULL, request_id VARCHAR NOT NULL, sender, prompt_mode, mode, model, start_time TIMESTAMP, end_time TIMESTAMP, process_time BIGINT, process_ttft BIGINT, upstream_tokens BIGINT, downstream_tokens BIGINT, cost DECIMAL(10,2), request_content TEXT, response_content TEXT, user_input TEXT, diff TEXT, diff_lines BIGINT, error_code VARCHAR, error_reason TEXT, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP；添加 UNIQUE(task_id, request_id) 约束
        - 新增 `costrict_commits` 表：id SERIAL PRIMARY KEY, commit_id VARCHAR NOT NULL, commit_time TIMESTAMP, repo_addr, repo_branch, repo_id, git_user_name, git_user_email, user_id, user_name, client_id, project_path, diff_lines BIGINT, ai_estimated_ancient_days DECIMAL(10,2), ai_estimated_ancient_reason TEXT, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP；添加 UNIQUE(commit_id, repo_id) 约束
        - 新增 `costrict_projects` 表：id SERIAL PRIMARY KEY, repo_id VARCHAR UNIQUE NOT NULL, commit_ids JSONB, task_ids JSONB, task_ids_silica JSONB, commit_ids_exclude_manual JSONB, task_ids_manual JSONB, task_ids_silica_manual JSONB, start_time TIMESTAMP, end_time TIMESTAMP, start_time_manual TIMESTAMP, end_time_manual TIMESTAMP, upstream_tokens BIGINT, downstream_tokens BIGINT, cost DECIMAL(10,2), ai_estimated_ancient_days DECIMAL(10,2), ai_estimated_ancient_reason TEXT, ai_estimated_ancient_days_manual DECIMAL(10,2), ai_estimated_ancient_reason_manual TEXT, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        - 为高频查询字段创建索引（使用 `CREATE INDEX IF NOT EXISTS`）：costrict_tasks(user_id), costrict_tasks(repo_id), costrict_tasks(project_id), costrict_tasks(start_time)；costrict_task_conversations(task_id), costrict_task_conversations(start_time)；costrict_commits(repo_id), costrict_commits(user_id), costrict_commits(commit_time)；costrict_projects(repo_id)（已有 UNIQUE 约束自动创建，可省略）

- [x] 1.2 在 backend/db.go 中新增 Go struct 定义和 scan 辅助
     【目标对象】`backend/db.go`
     【修改目的】为 4 张新表定义对应的 Go 数据结构和 scan 辅助函数，支撑后续 CRUD 函数
     【修改方式】在现有 `CodeAttributionRow` struct（第 139 行附近）之后追加新结构体；在现有 `scanRepoMetrics` 函数之后追加新的 selectColumns 变量和 scan 函数
     【相关依赖】现有的 `rowScanner` 接口（第 11 行）；需新增 `encoding/json` 到 import（用于 `json.RawMessage`）
     【修改内容】
        - 定义 `CostrictTask` struct：字段与 costrict_tasks 表一一对应，可空字段使用指针类型（如 `*string`, `*int64`, `*float64`, `*time.Time`），与现有 `ProjectMetrics` 结构体的指针风格保持一致
        - 定义 `CostrictTaskConversation` struct：字段与 costrict_task_conversations 表一一对应，TEXT 类型字段（request_content, response_content, user_input, diff）使用 `*string`
        - 定义 `CostrictCommit` struct：字段与 costrict_commits 表一一对应
        - 定义 `CostrictProject` struct：字段与 costrict_projects 表一一对应，JSONB 字段使用 `json.RawMessage` 类型
        - 为每个 struct 定义 `xxxSelectColumns` 字符串变量（如 `costrictTaskSelectColumns`），列出所有列名，与现有 `projectMetricsSelectColumns` 格式一致
        - 为每个 struct 定义 `scanXxx(s rowScanner, m *Xxx) error` 辅助函数，与现有 `scanProjectMetrics` 模式一致

- [x] 1.3 在 backend/db.go 中新增 costrict_tasks 的 CRUD 函数
     【目标对象】`backend/db.go`
     【修改目的】提供 task 数据的增删改查能力
     【修改方式】在 struct/scan 定义之后追加函数，遵循现有 `UpsertProjectMetrics`/`GetProjectMetrics`/`ListProjectMetrics` 的函数签名和实现模式
     【相关依赖】任务 1.2 中定义的 `CostrictTask` struct、`costrictTaskSelectColumns`、`scanCostrictTask` 函数
     【修改内容】
        - `UpsertCostrictTask(db *sql.DB, t *CostrictTask) error`：INSERT ... ON CONFLICT(task_id) DO UPDATE SET ...，SQL 占位符使用 `$1`, `$2`（不用 `?`），错误返回 `fmt.Errorf("upsert costrict_tasks 失败: %w", err)`
        - `GetCostrictTask(db *sql.DB, taskID string) (*CostrictTask, error)`：按 task_id 查询单条，使用 `scanCostrictTask` + `costrictTaskSelectColumns`；`sql.ErrNoRows` 时返回 `nil, nil`（与现有 GetProjectMetrics 一致）
        - `ListCostrictTasks(db *sql.DB, userID, repoID, projectID, startTime, endTime string) ([]CostrictTask, error)`：支持按 user_id/repo_id/project_id/时间范围过滤；使用动态拼接 WHERE 条件（空字符串参数表示不过滤）；按 start_time DESC 排序；使用 `defer rows.Close()` 和逐行 scan 模式（与现有 ListProjectMetrics 一致）
        - `DeleteCostrictTask(db *sql.DB, taskID string) error`：按 task_id 删除，错误返回 `fmt.Errorf("删除 costrict_tasks 失败: %w", err)`

- [x] 1.4 在 backend/db.go 中新增 costrict_task_conversations 的 CRUD 函数
     【目标对象】`backend/db.go`
     【修改目的】提供 task 对话明细的增删改查能力
     【修改方式】在 costrict_tasks CRUD 之后追加函数
     【相关依赖】任务 1.2 中定义的 `CostrictTaskConversation` struct、`costrictTaskConversationSelectColumns`、`scanCostrictTaskConversation` 函数
     【修改内容】
        - `InsertCostrictTaskConversation(db *sql.DB, c *CostrictTaskConversation) error`：INSERT ... ON CONFLICT(task_id, request_id) DO NOTHING（忽略重复），SQL 占位符使用 `$1`, `$2`，错误返回 `fmt.Errorf("插入 costrict_task_conversations 失败: %w", err)`
        - `ListCostrictTaskConversations(db *sql.DB, taskID string) ([]CostrictTaskConversation, error)`：按 task_id 查询所有对话，按 start_time ASC 排序（时间正序展示对话流程）；使用 `defer rows.Close()` 和逐行 scan 模式
        - `BatchInsertCostrictTaskConversations(db *sql.DB, convs []CostrictTaskConversation) error`：在单个事务（`db.Begin()`/`tx.Commit()`）中循环调用 INSERT ... ON CONFLICT DO NOTHING；任一失败则 `tx.Rollback()`；用于数据批量导入场景

- [x] 1.5 在 backend/db.go 中新增 costrict_commits 的 CRUD 函数
     【目标对象】`backend/db.go`
     【修改目的】提供 commit 数据的增删改查能力
     【修改方式】在 costrict_task_conversations CRUD 之后追加函数
     【相关依赖】任务 1.2 中定义的 `CostrictCommit` struct、`costrictCommitSelectColumns`、`scanCostrictCommit` 函数
     【修改内容】
        - `UpsertCostrictCommit(db *sql.DB, c *CostrictCommit) error`：INSERT ... ON CONFLICT(commit_id, repo_id) DO UPDATE SET ...，错误返回 `fmt.Errorf("upsert costrict_commits 失败: %w", err)`
        - `GetCostrictCommit(db *sql.DB, commitID, repoID string) (*CostrictCommit, error)`：按 commit_id + repo_id 查询单条；`sql.ErrNoRows` 时返回 `nil, nil`
        - `ListCostrictCommits(db *sql.DB, repoID, userID, startTime, endTime string) ([]CostrictCommit, error)`：支持按 repo_id/user_id/时间范围过滤；使用动态拼接 WHERE 条件（空字符串参数表示不过滤）；按 commit_time DESC 排序；使用 `defer rows.Close()`
        - `DeleteCostrictCommit(db *sql.DB, commitID, repoID string) error`：按 commit_id + repo_id 删除

- [x] 1.6 在 backend/db.go 中新增 costrict_projects 的 CRUD 函数
     【目标对象】`backend/db.go`
     【修改目的】提供 project 关联数据的增删改查能力，包括 JSONB 字段和人工调整字段
     【修改方式】在 costrict_commits CRUD 之后追加函数
     【相关依赖】任务 1.2 中定义的 `CostrictProject` struct、`costrictProjectSelectColumns`、`scanCostrictProject` 函数
     【修改内容】
        - `UpsertCostrictProject(db *sql.DB, p *CostrictProject) error`：INSERT ... ON CONFLICT(repo_id) DO UPDATE SET ...（JSONB 字段直接整体覆盖），错误返回 `fmt.Errorf("upsert costrict_projects 失败: %w", err)`
        - `GetCostrictProject(db *sql.DB, repoID string) (*CostrictProject, error)`：按 repo_id 查询单条；`sql.ErrNoRows` 时返回 `nil, nil`
        - `ListCostrictProjects(db *sql.DB, startTime, endTime string) ([]CostrictProject, error)`：支持按时间范围过滤（start_time/end_time）；按 repo_id ASC 排序；使用 `defer rows.Close()`
        - `UpdateCostrictProjectManual(db *sql.DB, repoID string, manualFields *CostrictProject) error`：仅更新人工调整字段（commit_ids_exclude_manual, task_ids_manual, task_ids_silica_manual, start_time_manual, end_time_manual, ai_estimated_ancient_days_manual, ai_estimated_ancient_reason_manual, updated_at），使用 UPDATE ... WHERE repo_id = $1，不存在时返回错误

- [x] 1.7 新建测试数据种子脚本并验证
     【目标对象】`seed_data.sql`（项目根目录新建）
     【修改目的】填充测试数据，验证表结构和 CRUD 函数的正确性，为后续 API 和 UI 开发提供数据基础
     【修改方式】新建 SQL 脚本，按表依赖顺序插入数据（先 tasks → 再 conversations → 再 commits → 最后 projects），使用 INSERT ... ON CONFLICT DO NOTHING 保证幂等可重复执行
     【相关依赖】任务 1.1 中创建的 4 张表必须已存在
     【修改内容】
        - 构造 5 个用户（不同 user_id/user_name，模拟手机号和 GitHub 用户名格式）
        - 构造 3 个仓库（不同 repo_addr/repo_branch，使用 costrict、kanban 等真实项目名）
        - 构造 15 个 task（分布在 3 个仓库、5 个用户中，包含不同的 caller/model/client_ide 组合，start_time/end_time 覆盖近 30 天）
        - 每个 task 构造 2-5 条 conversation 记录（包含 user_input、diff、tokens、cost 等真实格式数据）
        - 构造 12 个 commit（分布在 3 个仓库中，包含 diff_lines、git_user_name/email 等信息）
        - 构造 4 个 project 关联记录（commit_ids/task_ids/task_ids_silica 使用 JSONB 数组格式如 '["id1","id2"]'）
        - 脚本顶部添加注释说明用途和执行方式（`psql -U postgres -d report -f seed_data.sql`）

- [x] restructure-pg-schema | task: 1.2-fix-1 为 4 个 Costrict struct 添加 JSON tags
     【目标对象】`backend/db.go`
     【修改目的】修复 API 返回 JSON 字段名为 PascalCase 的问题，前端期望 snake_case
     【修改方式】为 CostrictTask、CostrictTaskConversation、CostrictCommit、CostrictProject 4 个 struct 的所有 exported 字段添加 `json:"snake_case_name"` tag
     【修改内容】
        - `CostrictTask`（约第 154-180 行）：ID→`json:"id"`, TaskID→`json:"task_id"`, UserID→`json:"user_id"`, UserName→`json:"user_name"`, ClientID→`json:"client_id"`, IDE→`json:"ide"`, Version→`json:"version"`, OS→`json:"os"`, OSVersion→`json:"os_version"`, Caller→`json:"caller"`, RepoAddr→`json:"repo_addr"`, RepoBranch→`json:"repo_branch"`, RepoID→`json:"repo_id"`, ProjectPath→`json:"project_path"`, ProjectID→`json:"project_id"`, StartTime→`json:"start_time"`, EndTime→`json:"end_time"`, UpstreamTokens→`json:"upstream_tokens"`, DownstreamTokens→`json:"downstream_tokens"`, Cost→`json:"cost"`, DiffLines→`json:"diff_lines"`, AIEstimatedAncientDays→`json:"ai_estimated_ancient_days"`, AIEstimatedAncientReason→`json:"ai_estimated_ancient_reason"`, CreatedAt→`json:"created_at"`, UpdatedAt→`json:"updated_at"`
        - `CostrictTaskConversation`（约第 183-206 行）：ID→`json:"id"`, TaskID→`json:"task_id"`, RequestID→`json:"request_id"`, Sender→`json:"sender"`, PromptMode→`json:"prompt_mode"`, Mode→`json:"mode"`, Model→`json:"model"`, StartTime→`json:"start_time"`, EndTime→`json:"end_time"`, ProcessTime→`json:"process_time"`, ProcessTTFT→`json:"process_ttft"`, UpstreamTokens→`json:"upstream_tokens"`, DownstreamTokens→`json:"downstream_tokens"`, Cost→`json:"cost"`, RequestContent→`json:"request_content"`, ResponseContent→`json:"response_content"`, UserInput→`json:"user_input"`, Diff→`json:"diff"`, DiffLines→`json:"diff_lines"`, ErrorCode→`json:"error_code"`, ErrorReason→`json:"error_reason"`, CreatedAt→`json:"created_at"`
        - `CostrictCommit`（约第 209-227 行）：ID→`json:"id"`, CommitID→`json:"commit_id"`, CommitTime→`json:"commit_time"`, RepoAddr→`json:"repo_addr"`, RepoBranch→`json:"repo_branch"`, RepoID→`json:"repo_id"`, GitUserName→`json:"git_user_name"`, GitUserEmail→`json:"git_user_email"`, UserID→`json:"user_id"`, UserName→`json:"user_name"`, ClientID→`json:"client_id"`, ProjectPath→`json:"project_path"`, DiffLines→`json:"diff_lines"`, AIEstimatedAncientDays→`json:"ai_estimated_ancient_days"`, AIEstimatedAncientReason→`json:"ai_estimated_ancient_reason"`, CreatedAt→`json:"created_at"`, UpdatedAt→`json:"updated_at"`
        - `CostrictProject`（约第 230-252 行）：ID→`json:"id"`, RepoID→`json:"repo_id"`, CommitIDs→`json:"commit_ids"`, TaskIDs→`json:"task_ids"`, TaskIDsSilica→`json:"task_ids_silica"`, CommitIDsExcludeManual→`json:"commit_ids_exclude_manual"`, TaskIDsManual→`json:"task_ids_manual"`, TaskIDsSilicaManual→`json:"task_ids_silica_manual"`, StartTime→`json:"start_time"`, EndTime→`json:"end_time"`, StartTimeManual→`json:"start_time_manual"`, EndTimeManual→`json:"end_time_manual"`, UpstreamTokens→`json:"upstream_tokens"`, DownstreamTokens→`json:"downstream_tokens"`, Cost→`json:"cost"`, AIEstimatedAncientDays→`json:"ai_estimated_ancient_days"`, AIEstimatedAncientReason→`json:"ai_estimated_ancient_reason"`, AIEstimatedAncientDaysManual→`json:"ai_estimated_ancient_days_manual"`, AIEstimatedAncientReasonManual→`json:"ai_estimated_ancient_reason_manual"`, CreatedAt→`json:"created_at"`, UpdatedAt→`json:"updated_at"`
