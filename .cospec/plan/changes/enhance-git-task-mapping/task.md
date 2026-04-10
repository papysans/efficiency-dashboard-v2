## 实施

- [x] 3a.1 扩展 CSV 格式和 OrgProvider：增加 git 身份映射
     【目标对象】`org_mapping.csv` + `kbcli/org_provider.go`
     【修改目的】在 CSV 中新增 git_user_name 和 git_user_email 列，在 OrgProvider 中建立 git 身份 → user_id 的反向映射，使后续 Task-Commit 关联引擎可通过 git author 信息定位系统用户
     【修改方式】修改 CSV 文件新增两列；修改 OrgProvider 结构体新增字段和方法；修改 `load()` 方法（第 62 行）扩展解析逻辑；新增 `LookupByGitAuthor` 方法
     【相关依赖】无
     【修改内容】
        - org_mapping.csv 扩展表头为：`user_id,user_name,org1,org2,org3,org4,git_user_name,git_user_email`（遵循 proposal.md 定义，git_user_name 对应 git log 的 Author Name，git_user_email 对应 git log 的 Author Email）
        - OrgProvider 结构体新增两个字段：`gitNameMap map[string]string`（git_user_name → user_id）、`gitEmailMap map[string]string`（git_user_email → user_id）
        - 修改 `load()` 方法：
          * git_user_name / git_user_email 列为可选（兼容旧格式无这两列的 CSV），通过检查 colIdx 中是否存在这两个 key 来判断
          * 有则在遍历行时写入 gitNameMap / gitEmailMap
          * 在方法末尾将新 map 赋值给结构体字段（与现有 userIDMap/userNameMap 赋值风格一致）
        - 修改 `NewOrgProvider` 函数：初始化时额外初始化 `gitNameMap` 和 `gitEmailMap` 两个 map
        - 新增 `(p *OrgProvider) LookupByGitAuthor(gitName, gitEmail string) (userID string, found bool)` 方法：
          * 优先按 gitEmail 查 gitEmailMap（email 更精确）
          * 未命中再按 gitName 查 gitNameMap
          * 返回对应的 user_id；未找到时 found=false
        - 边界处理：同一个 git_user_email 在 CSV 中出现多次时，后面的覆盖前面的（与现有 userNameMap 行为一致）

- [x] 3a.2 增强 Git 分析器：返回详细 commit 列表
     【目标对象】`kbcli/git_analyzer.go`
     【修改目的】AnalyzeCommits 当前仅返回统计摘要（CommitCount/LinesAdded 等），需增强为同时返回每个 commit 的详细信息（hash、author、时间、变更文件列表、行数），供后续关联引擎使用
     【修改方式】新增 `CommitDetail` 结构体；修改 `GitAnalysisResult` 结构体新增字段；修改 `AnalyzeCommits` 方法（第 79 行）扩展解析逻辑
     【相关依赖】无
     【修改内容】
        - 新增 `CommitDetail` 结构体：
          * Hash string, AuthorName string, AuthorEmail string
          * Timestamp time.Time, Message string
          * FilesChanged []string（该 commit 涉及的文件路径列表）
          * LinesAdded int64, LinesDeleted int64
        - GitAnalysisResult 新增字段：`Commits []CommitDetail`
        - 修改 AnalyzeCommits 方法的 commit 解析循环（现有第 106-116 行）：
          * 现有代码已解析 `%H|%an|%ae|%at|%s` 格式，在此基础上增强
          * 对已解析的每个 commit hash，执行 `git -C <localPath> show --name-only --pretty=format:"" <hash>` 获取变更文件列表
          * 执行 `git -C <localPath> show --numstat --pretty=format:"" <hash>` 获取每个文件的增删行数并汇总
          * 将 %at（unix timestamp 字符串）解析为 time.Time
          * 构建 CommitDetail 并追加到 result.Commits
        - 性能注意：commit 数量可能很多（上百个），每个 commit 执行两次 git 命令。可考虑批量获取：用 `git log --name-only --pretty=format:"COMMIT:%H" --since=... --until=...` 一次性获取所有 commit 的文件列表，再按 "COMMIT:" 分隔解析。但初始实现先用逐个方式，后续优化
        - 错误处理：单个 commit 的 show 命令失败时打印警告并跳过，不中断整体分析

- [x] 3a.3 实现 Task-Commit 关联引擎
     【目标对象】`kbcli/git_task_matcher.go`（新增文件）
     【修改目的】建立 task ↔ commit 的多对多关联关系，并对 commit 进行代码来源分类（ai_current / human / ai_other / unknown 四种类型）
     【修改方式】新增文件，包含关联匹配函数和代码来源分类函数
     【相关依赖】`kbcli/org_provider.go` 的 `LookupByGitAuthor` 方法；`kbcli/git_analyzer.go` 的 `CommitDetail` 结构体；`kbcli/task_content.go` 的 `TaskContentFile` 结构体（注意 StartTime/EndTime 是 string 类型，需用 time.Parse(time.RFC3339, ...) 解析）
     【修改内容】
        - 定义 `TaskCommitMatch` 结构体：
          * TaskID string, CommitHash string
          * UserID string（通过 LookupByGitAuthor 映射得到的系统用户 ID）
          * MatchScore float64（0-1 之间，1 表示完全匹配）
          * MatchReason string（匹配原因描述，如 "时间窗口+文件路径匹配"）
        - 定义 `CodeSourceType` 字符串常量：`CodeSourceAICurrent = "ai_current"`、`CodeSourceHuman = "human"`、`CodeSourceAIOther = "ai_other"`、`CodeSourceUnknown = "unknown"`
        - 定义 `CommitClassification` 结构体：
          * CommitHash string, CodeSource string（四种类型之一）
          * MatchedTaskIDs []string, UserID string
          * LinesAdded int64, LinesDeleted int64
        - 实现 `MatchTasksToCommits(commits []CommitDetail, tasks []TaskContentFile, orgProvider *OrgProvider) ([]TaskCommitMatch, []CommitClassification)`：
          1. 对每个 commit，通过 orgProvider.LookupByGitAuthor(commit.AuthorName, commit.AuthorEmail) 尝试映射 user_id
          2. 映射成功的 commit：在同一 user_id 下查找时间窗口匹配的 task——commit.Timestamp 在 [task.StartTime - 1h, task.EndTime + 24h] 范围内（task 的时间字段需从 string 解析为 time.Time）
          3. 对时间匹配的 (commit, task) 对，计算文件路径交集得分：commit.FilesChanged ∩ task 所有 Conversations[].CodeOutputs[].Path，得分 = 匹配文件数 / max(commit 文件数, task 文件数)
          4. 得分 > 0 的记录为有效关联
          5. 时间窗口内有同用户 task 但文件路径无交集的，仍保留关联（得分设为 0.3，MatchReason 标记"仅时间窗口匹配"）
          6. task 无 code_outputs 时（如纯 chat 类 task），按纯时间窗口匹配，得分 0.5
        - 代码来源分类逻辑（在同一函数中完成，遍历所有 commit）：
          * 有 TaskCommitMatch 关联的 commit → `ai_current`
          * 无关联但 user_id 映射成功（说明是项目成员但没用我们 AI）→ `human`
          * 无关联且 user_id 映射失败，但 commit message 包含 "AI"/"Copilot"/"copilot"/"GPT"/"gpt" 等关键词（大小写不敏感）→ `ai_other`
          * 以上都不满足 → `unknown`
        - 边界处理：tasks 为空时，所有能映射 user_id 的 commit 标记为 human，不能映射的按 ai_other/unknown 逻辑处理

- [x] 3a.4 实现代码来源分析（行级文本比对）
     【目标对象】`kbcli/code_attribution.go`（新增文件）
     【修改目的】对标记为 ai_current 的 commit，通过行级文本比对，精确区分"AI 写的代码行数"和"人写/修改的代码行数"
     【修改方式】新增文件
     【相关依赖】`kbcli/git_task_matcher.go` 的 `TaskCommitMatch` 和 `CommitClassification`；`kbcli/task_content.go` 的 `TaskContentFile`
     【修改内容】
        - 定义 `CodeAttribution` 结构体：
          * CommitHash string, TaskID string
          * OurAICodeLines int64（匹配到 task code_outputs 的行数）
          * HumanCodeLines int64（commit 中未匹配到的新增行数）
          * TotalAddedLines int64（commit 总新增行数）
        - 定义 `AttributionSummary` 结构体：
          * TotalOurAILines int64, TotalHumanLines int64
          * CommitCount int
          * Details []CodeAttribution
        - 实现 `AnalyzeCodeAttribution(commitHash string, matchedTasks []TaskContentFile, gitLocalPath string) (*CodeAttribution, error)`：
          * 获取 commit diff：执行 `git -C <gitLocalPath> show --no-color --unified=0 <hash>`，只关注 `+` 开头的新增行（排除 `+++` 文件头行）
          * 提取 commit 中每个文件的新增代码行
          * 对每个新增代码行，与 matchedTasks 中所有 Conversations[].CodeOutputs[].Code 做行级比对
          * 比对策略：去除首尾空白（strings.TrimSpace）后完全匹配 → 标记为 AI 代码行；否则标记为人写代码行
          * 空行和纯注释行也参与比对
          * 统计各类行数，返回 CodeAttribution
        - 实现 `SummarizeAttributions(attributions []CodeAttribution) *AttributionSummary`：遍历 attributions 汇总统计
        - 错误处理：git show 执行失败时返回 error，由调用方决定是跳过还是中断

- [x] 3a.5 新建 PG 关联表和扩展现有表
     【目标对象】`init_db.sql` + `backend/db.go`
     【修改目的】持久化存储 Task-Commit 关联关系、代码归因结果，扩展 metrics 表支持代码来源统计和用户自定义人天
     【修改方式】在 init_db.sql 末尾新增两个 CREATE TABLE 语句、新增 ALTER TABLE 语句扩展现有表；在 db.go 中新增结构体和 CRUD 函数（遵循现有 Upsert*/Get*/List* 命名风格）
     【相关依赖】`init_db.sql` 现有的 project_metrics（第 4 行）、repo_metrics（第 50 行）表结构
     【修改内容】
        - init_db.sql 新增 `task_commit_mapping` 表（遵循现有 `CREATE TABLE IF NOT EXISTS` 风格）：
          * id SERIAL PRIMARY KEY
          * repo_id VARCHAR(500) NOT NULL
          * task_id VARCHAR(100) NOT NULL
          * commit_hash VARCHAR(40) NOT NULL
          * user_id VARCHAR(100)（通过 git 身份映射得到的系统用户 ID）
          * match_score DECIMAL(5,2)（关联得分 0-1）
          * match_reason TEXT（关联原因描述）
          * code_source VARCHAR(20) NOT NULL（ai_current / human / ai_other / unknown）
          * analysis_date DATE NOT NULL
          * created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
          * UNIQUE(repo_id, task_id, commit_hash)
        - init_db.sql 新增 `code_attribution` 表：
          * id SERIAL PRIMARY KEY
          * repo_id VARCHAR(500) NOT NULL
          * commit_hash VARCHAR(40) NOT NULL
          * task_id VARCHAR(100)（可为空，human/unknown 类型的 commit 无关联 task）
          * our_ai_code_lines BIGINT DEFAULT 0
          * human_code_lines BIGINT DEFAULT 0
          * total_added_lines BIGINT DEFAULT 0
          * analysis_date DATE NOT NULL
          * created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
          * UNIQUE(repo_id, commit_hash, task_id)
        - init_db.sql 使用 ALTER TABLE 扩展 project_metrics 表新增字段（用 `DO $$ ... END $$` 包裹 IF NOT EXISTS 检查，避免重复执行报错）：
          * `our_ai_code_lines BIGINT DEFAULT 0`
          * `human_code_lines BIGINT DEFAULT 0`
          * `user_manual_days DECIMAL(10,2)`（用户自定义人天，完全独立于 corrected_ai_estimated_days，前者是用户自己估的，后者是对 AI 估值的纠正）
          * `user_manual_days_reason TEXT`
          * `user_manual_days_by VARCHAR(100)`
          * `user_manual_days_at TIMESTAMP`
        - init_db.sql 使用 ALTER TABLE 扩展 repo_metrics 表新增字段（同上模式）：
          * `our_ai_code_lines BIGINT DEFAULT 0`（ai_current 分类的代码行数）
          * `human_code_lines BIGINT DEFAULT 0`（human 分类的代码行数）
          * `ai_other_code_lines BIGINT DEFAULT 0`（ai_other 分类的代码行数）
          * `unknown_code_lines BIGINT DEFAULT 0`（unknown 分类的代码行数）
          * `mapped_task_count INTEGER DEFAULT 0`（成功关联到 task 的 commit 数）
          * `user_manual_days DECIMAL(10,2)`
          * `user_manual_days_reason TEXT`
          * `user_manual_days_by VARCHAR(100)`
          * `user_manual_days_at TIMESTAMP`
        - user_manual_days 与 corrected_ai_estimated_days 的关系说明：
          * corrected_ai_estimated_days：对 AI 自动估算值的人工纠正，用于提效计算
          * user_manual_days：用户根据实际经验自行填入的"如果没有 AI 要花多少人天"，独立于 AI 估算流程，仅作参考展示
          * 提效计算优先级：使用 corrected_ai_estimated_days（如有）> raw_ai_estimated_days
          * user_manual_days 不参与自动提效比例计算，但前端可展示为参考对比值
        - db.go 新增结构体和函数（遵循现有代码风格，使用 `$1, $2` 占位符）：
          * `TaskCommitMapping` 结构体（字段对应 task_commit_mapping 表）
          * `UpsertTaskCommitMapping(db *sql.DB, m *TaskCommitMapping) error` —— ON CONFLICT(repo_id, task_id, commit_hash) DO UPDATE
          * `ListTaskCommitMappings(db *sql.DB, repoID string, startDate, endDate string) ([]TaskCommitMapping, error)` —— 按 analysis_date 范围查询
          * `CodeAttributionRow` 结构体（字段对应 code_attribution 表，避免与 kbcli 的 CodeAttribution 混淆）
          * `UpsertCodeAttribution(db *sql.DB, a *CodeAttributionRow) error`
          * `ListCodeAttributions(db *sql.DB, repoID string, startDate, endDate string) ([]CodeAttributionRow, error)`
          * `UpdateUserManualDays(db *sql.DB, dimension, id string, analysisDate, startDate, endDate string, days float64, reason, by string) error` —— 更新 user_manual_days 相关四个字段，同时写入 correction_history 审计记录
          * 修改现有 `RepoMetrics` 结构体（第 57 行）：新增 OurAICodeLines、HumanCodeLines、AIOtherCodeLines、UnknownCodeLines、MappedTaskCount、UserManualDays、UserManualDaysReason、UserManualDaysBy、UserManualDaysAt 字段
          * 修改现有 `ProjectMetrics` 结构体（第 26 行）：新增 OurAICodeLines、HumanCodeLines、UserManualDays、UserManualDaysReason、UserManualDaysBy、UserManualDaysAt 字段
          * 修改现有 `UpsertRepoMetrics`（第 227 行）和 `UpsertProjectMetrics`（第 108 行）：INSERT 和 UPDATE 语句中加入新字段
          * 修改现有 `GetRepoMetrics`（第 278 行）、`ListRepoMetrics`（第 313 行）、`GetProjectMetrics`（第 153 行）、`ListProjectMetrics`（第 186 行）：SELECT 和 Scan 中加入新字段

- [x] 3a.6 增强 kbcli analyze git 命令流程
     【目标对象】`kbcli/cmd_analyze.go`
     【修改目的】在 Git 分析流程中集成 Task-Commit 关联和代码归因步骤，将分析结果写入 PG
     【修改方式】修改 `runAnalyzeGit` 函数（第 38 行），在现有 EnsureRepo + AnalyzeCommits 之后增加关联和归因步骤；新增 `--project-id` 命令行参数
     【相关依赖】3a.1 OrgProvider.LookupByGitAuthor；3a.2 CommitDetail；3a.3 MatchTasksToCommits；3a.4 AnalyzeCodeAttribution；3a.5 数据库 CRUD 函数
     【修改内容】
        - 新增 `--project-id` 参数（可选），用于按 project_id 从 ES 过滤 task
        - runAnalyzeGit 流程扩展（在现有 AnalyzeCommits 之后、AI 估时之前插入）：
          1. （已有）EnsureRepo
          2. （已有）AnalyzeCommits 获取详细 commit 列表（增强后包含 Commits 字段）
          3. 加载 OrgProvider（通过 config.OrgCSVFile 路径）
          4. 从 ES task 索引查询该 repo 对应的 task 列表：
             * 构建 ES 查询：bool filter 中 term 匹配 repo_id 或 project_id，时间范围匹配 startDate-endDate
             * 使用现有 kbcli/es_client.go 的 ES 客户端执行查询
             * 从返回的每条 task 中取 source_file 字段，拼接 rawDataDir 前缀读取 task 内容文件
             * 将读取到的内容反序列化为 []TaskContentFile
          5. 调用 MatchTasksToCommits 建立关联，获得 matches 和 classifications
          6. 对每个 classification 为 ai_current 的 commit，调用 AnalyzeCodeAttribution
          7. 汇总统计：our_ai_code_lines, human_code_lines, ai_other_code_lines, unknown_code_lines
          8. （已有）AI 预估人天
          9. 初始化 PG 数据库连接（参考 backend 的 InitDB，或复用 kbcli 已有的 PG 连接逻辑）
          10. 写入 PG：
              * 遍历 matches 调用 UpsertTaskCommitMapping
              * 遍历 attributions 调用 UpsertCodeAttribution
              * 更新 repo_metrics 的代码来源相关字段
          11. 保存完整分析结果到 JSON 文件（现有逻辑，增加关联和归因数据）
        - 错误处理：ES 查询失败或 task 文件读取失败时打印警告继续执行（关联步骤非阻断性的，不影响基础 Git 分析结果）

- [x] 3a.7 新增后端关联查询、代码归因和用户自定义人天 API
     【目标对象】`backend/analysis_handler.go` + `backend/main.go`
     【修改目的】提供关联关系查询、代码归因统计查询、代码来源统计查询、用户自定义人天设置接口
     【修改方式】在 analysis_handler.go 末尾新增 handler 函数；在 main.go 的 api 路由组（第 108 行）中注册新路由
     【相关依赖】`backend/db.go` 的 ListTaskCommitMappings、ListCodeAttributions、UpdateUserManualDays 函数
     【修改内容】
        - 新增 `getTaskCommitMappings` handler，注册路由 `GET /api/analysis/task-commits`：
          * 参数：repo_id（必填）, startDate, endDate（必填，格式 YYYYMMDD）
          * 调用 ListTaskCommitMappings 查询
          * 返回：`{"items": [{task_id, commit_hash, user_id, match_score, match_reason, code_source}]}`
          * 参数校验：repo_id/startDate/endDate 为空时返回 400
        - 新增 `getCodeAttribution` handler，注册路由 `GET /api/analysis/code-attribution`：
          * 参数：repo_id（必填）, startDate, endDate（必填）
          * 调用 ListCodeAttributions 查询
          * 返回：`{"summary": {total_our_ai_lines, total_human_lines}, "details": [{commit_hash, task_id, our_ai_lines, human_lines}]}`
        - 新增 `getCodeSourceStats` handler，注册路由 `GET /api/analysis/code-source`：
          * 参数：repo_id（必填）, startDate, endDate（必填）
          * 从 repo_metrics 表查询代码来源汇总字段（our_ai_code_lines, human_code_lines, ai_other_code_lines, unknown_code_lines, mapped_task_count）
          * 返回：`{"code_source": {"ai_current": {lines, percentage}, "human": {...}, "ai_other": {...}, "unknown": {...}}}`
          * 百分比根据总行数实时计算
        - 新增 `updateUserManualDays` handler，注册路由 `PUT /api/analysis/efficiency/manual-days`：
          * 参数（JSON body）：dimension（project/repo）, id, startDate, endDate, value（float64，人天数）, reason（必填）, by（必填）
          * 调用 UpdateUserManualDays 更新 PG，该函数内部同时写入 correction_history
          * 不影响 corrected_ai_estimated_days 和 raw_ai_estimated_days 的值
          * 不触发提效比例重算（user_manual_days 仅供参考展示）
          * 返回：`{"message": "ok", "user_manual_days": value}`
        - main.go 注册新路由（在现有 api 路由组第 122 行后追加）：
          * `api.GET("/analysis/task-commits", getTaskCommitMappings)`
          * `api.GET("/analysis/code-attribution", getCodeAttribution)`
          * `api.GET("/analysis/code-source", getCodeSourceStats)`
          * `api.PUT("/analysis/efficiency/manual-days", updateUserManualDays)`

- [x] 3a.8 编译验证和测试
     【目标对象】`kbcli/` + `backend/`
     【修改目的】确保编译通过和核心逻辑正确
     【修改方式】执行编译命令；新增测试文件并运行
     【相关依赖】3a.1-3a.7 所有前序任务
     【修改内容】
        - kbcli 编译：在 kbcli/ 目录执行 `go build ./...`，确保通过
        - backend 编译：在 backend/ 目录执行 `go build ./...`，确保通过
        - 新增 `kbcli/git_task_matcher_test.go`：
          * 测试用例 1：给定 2 个 commit（同一用户、不同时间）和 2 个 task，验证时间窗口匹配正确
          * 测试用例 2：给定 commit 变更文件 [a.go, b.go] 和 task 输出文件 [a.go, c.go]，验证文件交集得分 = 1/max(2,2) = 0.5
          * 测试用例 3：commit 的 author 未在 CSV 映射中，验证分类为 unknown 或 ai_other（根据 commit message）
          * 测试用例 4：tasks 为空时，所有 commit 按 human/ai_other/unknown 分类
        - 新增 `kbcli/code_attribution_test.go`：
          * 测试行级比对：给定已知的 git diff 输出和 task code_outputs，验证 AI 行数和人工行数统计正确
          * 测试空 diff 场景：commit 无新增行时返回全零
        - 修改 `kbcli/org_provider_test.go`：新增测试用例验证 git 身份映射：
          * CSV 包含 git_user_id/git_user_name 时，LookupByGitAuthor 能正确返回 user_id
          * CSV 无 git 列时（旧格式），LookupByGitAuthor 返回 found=false
        - 执行更新后的 init_db.sql：`psql -U postgres -d report -f init_db.sql` 确保新表和新字段创建成功
