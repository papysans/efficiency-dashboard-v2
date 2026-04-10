## 实施

- [x] 2.1 创建 PostgreSQL 建表脚本
     【目标对象】`init_db.sql`（项目根目录新增）
     【修改目的】创建提效分析元信息表，支持存储分析结果和用户纠错
     【修改方式】新增 SQL 文件，按 a.md 6.5 节表结构定义创建三张表
     【相关依赖】a.md 6.5 节表结构定义
     【修改内容】
        - 创建 `project_metrics` 表（对应 a.md 6.5.1）：
          * id SERIAL PRIMARY KEY
          * project_id VARCHAR(500) NOT NULL, analysis_date DATE NOT NULL
          * query_start_date DATE NOT NULL, query_end_date DATE NOT NULL
          * AI 预估原始值（不可修改）：raw_ai_estimated_days DECIMAL(10,2), raw_total_cost DECIMAL(10,2), raw_total_code_lines BIGINT, raw_task_count INTEGER
          * 用户纠正值（可修改）：corrected_ai_estimated_days DECIMAL(10,2), correction_reason TEXT, corrected_by VARCHAR(100), corrected_at TIMESTAMP
          * 实际耗时：actual_start_time TIMESTAMP, actual_end_time TIMESTAMP, total_lead_time_ms BIGINT, total_process_time_ms BIGINT, user_count INTEGER
          * 提效比例：efficiency_ratio_lead DECIMAL(10,2), efficiency_ratio_process DECIMAL(10,2)
          * 成本：api_cost DECIMAL(10,2), daily_rate DECIMAL(10,2) DEFAULT 400.00, cost_saving DECIMAL(10,2), roi DECIMAL(10,2)
          * 溯源：analysis_file_path VARCHAR(500)
          * 元信息：created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
          * UNIQUE(project_id, analysis_date, query_start_date, query_end_date)
        - 创建 `repo_metrics` 表（对应 a.md 6.5.1）：
          * id SERIAL PRIMARY KEY
          * repo_id VARCHAR(500) NOT NULL, analysis_date DATE NOT NULL
          * query_start_date DATE NOT NULL, query_end_date DATE NOT NULL
          * Git 分析数据：git_commit_count INTEGER, git_contributor_count INTEGER, git_lines_added BIGINT, git_lines_deleted BIGINT, git_files_changed INTEGER
          * 双重 AI 预估：raw_ai_estimated_days_from_task DECIMAL(10,2), raw_ai_estimated_days_from_git DECIMAL(10,2), raw_ai_estimated_days_final DECIMAL(10,2)
          * 用户纠正值：corrected_ai_estimated_days DECIMAL(10,2), correction_reason TEXT, corrected_by VARCHAR(100), corrected_at TIMESTAMP
          * 实际耗时：actual_start_time TIMESTAMP, actual_end_time TIMESTAMP, total_lead_time_ms BIGINT, total_process_time_ms BIGINT
          * 提效比例：efficiency_ratio_lead DECIMAL(10,2), efficiency_ratio_process DECIMAL(10,2)
          * 成本：api_cost DECIMAL(10,2), daily_rate DECIMAL(10,2) DEFAULT 400.00, cost_saving DECIMAL(10,2), roi DECIMAL(10,2)
          * 溯源：analysis_file_path VARCHAR(500), git_analysis_file_path VARCHAR(500)
          * 元信息：created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
          * UNIQUE(repo_id, analysis_date, query_start_date, query_end_date)
        - 创建 `correction_history` 表（审计用）：
          * id SERIAL PRIMARY KEY
          * dimension VARCHAR(20) NOT NULL（'project' 或 'repo'）
          * dimension_id VARCHAR(500) NOT NULL, analysis_date DATE NOT NULL
          * field_name VARCHAR(100) NOT NULL, old_value TEXT, new_value TEXT
          * reason TEXT, corrected_by VARCHAR(100), corrected_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

- [x] 2.2 更新后端配置：新增 PostgreSQL 和 rawdata 配置
     【目标对象】`backend/main.go` 的 `Config` 结构体 + `backend/config.yaml`
     【修改目的】扩展现有配置结构体以支持 PG 连接和 rawdata 路径配置
     【修改方式】修改 `backend/main.go` 中已有的 `Config` 结构体，新增字段；修改 `backend/config.yaml` 新增配置块
     【相关依赖】`backend/main.go` 中现有 `Config` 结构体（第14-22行）；`backend/config.yaml` 现有配置
     【修改内容】
        - 在 `backend/main.go` 的 `Config` 结构体中新增：
          * `DatabaseConfig` 子结构体：Host string, Port int, User string, Password string, DBName string, SSLMode string
          * `Config.Database DatabaseConfig` yaml:"database" 字段
          * `Config.RawDataDir string` yaml:"rawdata_dir" 字段
        - 在 `backend/config.yaml` 中新增：
          * database 块：host: localhost, port: 5432, user: postgres, password: "1", dbname: report, sslmode: disable
          * rawdata_dir: "../rawdata"
        - 注意：保持现有 `Config` 中 Server/Elasticsearch/CORS 字段不变

- [x] 2.3 新增 PostgreSQL 数据库连接模块
     【目标对象】`backend/db.go`（新增文件）
     【修改目的】提供 PG 数据库连接和 project_metrics/repo_metrics/correction_history 三张表的 CRUD 操作
     【修改方式】新增文件，package main
     【相关依赖】`backend/main.go` 的 `DatabaseConfig` 结构体（任务 2.2 新增）；`database/sql` + `github.com/lib/pq`
     【修改内容】
        - 实现 `InitDB(cfg DatabaseConfig) (*sql.DB, error)` 函数：
          * 构建 DSN 字符串：`fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s", ...)`
          * 调用 `sql.Open("postgres", dsn)` 并 `db.Ping()` 验证连接
          * 连接失败时返回明确错误信息
        - 实现 project_metrics CRUD（使用 $1/$2 占位符，不用 ?）：
          * `UpsertProjectMetrics(db *sql.DB, m *ProjectMetrics) error`：INSERT ON CONFLICT(project_id, analysis_date, query_start_date, query_end_date) DO UPDATE
          * `GetProjectMetrics(db *sql.DB, projectID string, analysisDate string, startDate string, endDate string) (*ProjectMetrics, error)`：按主键查询，不存在时返回 nil, nil
          * `ListProjectMetrics(db *sql.DB, startDate string, endDate string) ([]ProjectMetrics, error)`：按日期范围查询列表
        - 实现 repo_metrics CRUD（同上模式）：
          * `UpsertRepoMetrics(db *sql.DB, m *RepoMetrics) error`
          * `GetRepoMetrics(db *sql.DB, repoID string, analysisDate string, startDate string, endDate string) (*RepoMetrics, error)`
          * `ListRepoMetrics(db *sql.DB, startDate string, endDate string) ([]RepoMetrics, error)`
        - 实现 correction_history 操作：
          * `InsertCorrectionHistory(db *sql.DB, h *CorrectionHistory) error`：INSERT 一条记录
          * `ListCorrectionHistory(db *sql.DB, dimension string, dimensionID string) ([]CorrectionHistory, error)`：按 dimension+dimension_id 查询，按 corrected_at DESC 排序
        - 定义对应的 Go struct：ProjectMetrics、RepoMetrics、CorrectionHistory，字段与 init_db.sql 一一对应
        - 在 go.mod 中添加 `github.com/lib/pq` 依赖
        - 错误处理：所有数据库操作错误用 `fmt.Errorf("操作描述: %w", err)` 包装返回

- [x] 2.4 重构路由和 Request 查询 API
     【目标对象】`backend/main.go` 的 `main()` 函数 + `backend/es_handler.go` 的 `getIndices()`/`getStatData()`/`getStatSummary()` 函数
     【修改目的】路由从 /api/es/ 改为 /api/，移除 Stat 相关 handler，保留 Request 查询，初始化 PG 连接
     【修改方式】修改 `main()` 中路由注册和 PG 初始化；删除 es_handler.go 中的 `getStatData()`/`getStatSummary()` 两个函数；修改 `getIndices()` 的分类逻辑
     【相关依赖】`backend/db.go` 的 `InitDB()` 函数（任务 2.3）；`backend/main.go` 的 `Config.Database` 字段（任务 2.2）
     【修改内容】
        - `backend/main.go` 的 `main()` 函数：
          * 在 ES 客户端初始化之后，新增 PG 初始化：调用 `InitDB(appConfig.Database)`，赋值给包级变量 `var db *sql.DB`
          * PG 初始化失败时 `log.Fatalf` 退出
          * 路由注册改为（删除 /api/es/ 前缀组）：
            - `api.GET("/indices", getIndices)` — 保留
            - `api.GET("/requests", getRawData)` — 路径从 `/es/raw-data` 改为 `/requests`
            - 删除 `/es/stat-data` 和 `/es/stat-summary` 路由
            - 新增路由在后续任务（2.5/2.6/2.9）中注册
        - `backend/es_handler.go`：
          * 删除 `getStatData()` 函数（第119-172行）：Stat 层已移除，该函数查询 `costrict_chat_stat_*` 索引已无效
          * 删除 `getStatSummary()` 函数（第174-285行）：Stat 层已移除，改由实时 Task 聚合替代
          * 修改 `getIndices()` 函数（第54-75行）：将索引分类从 `_raw_`/`_stat_` 改为 `_request_`/`_task_`
            - 将变量名从 rawIndices/statIndices 改为 requestIndices/taskIndices
            - 匹配条件：`strings.Contains(idx.Name, "_request_")` 和 `strings.Contains(idx.Name, "_task_")`
            - 返回 JSON key 从 `"raw"`/`"stat"` 改为 `"request"`/`"task"`
          * 保留辅助函数：`generateIndexNames()`、`getDefaultInt()`、`getDefaultString()` 不变
          * 保留 `getRawData()` 函数不变（仅路由路径调整，handler 逻辑不变）

- [x] 2.5 更新 ES 客户端：支持排序和过滤查询
     【目标对象】`backend/es_client.go` 的 `Search()` 方法
     【修改目的】支持 Task 查询和 Request 查询的排序和过滤条件
     【修改方式】修改现有 `Search()` 方法签名，新增 `SearchWithFilter()` 方法
     【相关依赖】`backend/es_handler.go` 的 `getRawData()` 中 sortField/sortOrder 当前被忽略（第101-103行）
     【修改内容】
        - 修改 `Search()` 方法：
          * 新增 sortField string 和 sortOrder string 参数
          * 在 body map 中添加 `"sort": []map[string]interface{}{{sortField: map[string]interface{}{"order": sortOrder}}}`
          * sortField 为空时不添加 sort 子句
          * 更新 `getRawData()` 中的调用，传入 sortField 和 sortOrder（去掉 `_ = sortField` / `_ = sortOrder`）
        - 新增 `SearchWithFilter(indexNames []string, filters map[string]interface{}, from, size int, sortField, sortOrder string) (*SearchResult, error)` 方法：
          * 构建 bool query：将 filters 中的非空字段作为 must/filter 条件
          * 支持的 filter key：taskId→term(task_id), projectId→term(project_id), userId→term(user_id)
          * 空 filters 时退化为 match_all
          * 复用 Search 的 ignore_unavailable=true, allow_no_indices=true 设置
        - 新增 `AggregateWithQuery(indexNames []string, query map[string]interface{}, aggsQuery map[string]interface{}) (map[string]interface{}, error)` 方法：
          * 在现有 `Aggregate()` 基础上增加 query 参数支持，用于按维度 ID 过滤后再聚合
          * body 结构：`{"size": 0, "query": query, "aggs": aggsQuery}`

- [x] 2.6 新增 Task 查询 API
     【目标对象】`backend/task_handler.go`（新增文件）+ `backend/main.go` 的 `main()` 函数路由注册
     【修改目的】提供 Task 数据列表查询和 Task 汇总统计（对应 a.md 8.3）
     【修改方式】新增 `backend/task_handler.go` 文件（package main），在 `main.go` 的 `main()` 中注册路由
     【相关依赖】`backend/es_client.go` 的 `SearchWithFilter()` / `Aggregate()` 方法（任务 2.5）；`backend/es_handler.go` 的 `generateIndexNames()` / `getDefaultInt()` / `getDefaultString()` 辅助函数
     【修改内容】
        - `GET /api/tasks` handler — `getTasks(c *gin.Context)` 函数：
          * 必填参数：startDate, endDate（YYYYMMDD 格式，缺失返回 400）
          * 可选参数：page(默认1), pageSize(默认50), taskId, projectId, userId
          * 索引名生成：`generateIndexNames("costrict_chat_task_", startDate, endDate)`
          * 调用 `esClient.SearchWithFilter(indexNames, filters, from, size, "@timestamp", "desc")`
          * filters 构建：非空的 taskId/projectId/userId 作为 term 条件
          * 返回格式：`{"total": N, "page": P, "pageSize": S, "hits": [...]}`，与现有 `getRawData()` 返回格式保持一致
        - `GET /api/tasks/summary` handler — `getTasksSummary(c *gin.Context)` 函数：
          * 必填参数：startDate, endDate
          * 索引名：`generateIndexNames("costrict_chat_task_", startDate, endDate)`
          * 调用 `esClient.Aggregate()` 构建聚合：
            - sum: api_count, api_cost, ai_estimated_days
            - value_count: task_id → task_count
          * 返回格式：`{"task_count": N, "total_api_count": N, "total_api_cost": N, "total_ai_estimated_days": N}`
        - 在 `main.go` 注册：`api.GET("/tasks", getTasks)` 和 `api.GET("/tasks/summary", getTasksSummary)`

- [x] 2.7 新增维度聚合 API
     【目标对象】`backend/aggregate_handler.go`（新增文件）+ `backend/main.go` 的 `main()` 函数路由注册
     【修改目的】提供实时从 Task 表 ES Aggregation 的维度聚合查询（对应 a.md 8.4），替代已删除的 Stat 表查询
     【修改方式】新增 `backend/aggregate_handler.go` 文件（package main），在 `main.go` 的 `main()` 中注册路由
     【相关依赖】`backend/es_client.go` 的 `Aggregate()` 方法；`backend/es_handler.go` 的 `generateIndexNames()` / `getDefaultInt()` / `getDefaultString()` 辅助函数；a.md 5.3 节 process_time 计算逻辑
     【修改内容】
        - `GET /api/aggregate` handler — `getAggregate(c *gin.Context)` 函数：
          * 必填参数：startDate, endDate, dimension
          * 可选参数：page(默认1), pageSize(默认20)
          * dimension 有效值校验：project/repo/user/org1/org2/org3/org4，无效返回 400
          * 维度到 ES 字段映射：
            - project→"project_id", repo→"repo_id", user→"user_id", org1→"org1"
            - org2：使用 script 拼接 `doc['org1'].value + '_' + doc['org2'].value`
            - org3：使用 script 拼接 `doc['org1'].value + '_' + doc['org2'].value + '_' + doc['org3'].value`
            - org4：使用 script 拼接 `doc['org1'].value + '_' + doc['org2'].value + '_' + doc['org3'].value + '_' + doc['org4'].value`
          * 索引名：`generateIndexNames("costrict_chat_task_", startDate, endDate)`
          * 构建 ES terms aggregation（按 dimension 对应字段/script 分桶，size=pageSize）：
            - 子聚合 sum：user_in_chars, assistant_out_code_lines, api_count, api_cost, api_in_tokens, api_out_tokens, ai_estimated_days
            - 子聚合 value_count：task_id → task_count
            - 子聚合 min：api_request_time → start_time
            - 子聚合 max：api_end_time → end_time
          * 解析聚合 buckets 结果，计算派生字段：
            - lead_time = end_time - start_time（毫秒）
            - process_time 实时计算（见下方逻辑）：对每个 bucket 的 key 值，二次查询该维度下所有 task 的 api_request_time 和 api_end_time，按 a.md 5.3 节合并算法计算
          * process_time 合并算法（a.md 5.3 节）：
            - 查询该 bucket key 下所有 task 的 api_request_time 和 api_end_time
            - 按 api_request_time 升序排序
            - 遍历：gap = 当前.api_request_time - segEnd，gap ≤ 10分钟则合并段（segEnd = max(segEnd, 当前.api_end_time)），gap > 10分钟则结算当前段累加到 totalProcessTime 并开新段
            - 最终 totalProcessTime 即 process_time（毫秒）
          * 注意：process_time 计算需要二次查询 ES 获取每个 bucket 下的 task 时间列表，对性能有影响；可使用 top_hits 子聚合或者分页后对当前页 buckets 做二次查询
          * 返回格式（与现有 getStatSummary 的 ProjectSummary 风格对齐）：
            ```
            {"total": bucket数, "page": P, "pageSize": S, "items": [
              {"key": "xxx", "user_in_chars": N, "code_lines": N, "api_count": N, "api_cost": N, "api_in_tokens": N, "api_out_tokens": N, "task_count": N, "ai_estimated_days": N, "start_time": "...", "end_time": "...", "lead_time": N, "process_time": N}
            ]}
            ```
        - `GET /api/aggregate/summary` handler — `getAggregateSummary(c *gin.Context)` 函数（对应 a.md 8.4）：
          * 必填参数：startDate, endDate, dimension
          * 构建与 getAggregate 相同维度映射的 terms aggregation，但 size 设大（如 10000）以获取全量
          * 返回所有 bucket 的汇总值：总 task_count、总 api_count、总 api_cost、总 ai_estimated_days、unique bucket 数量
          * 返回格式：`{"dimension": "project", "bucket_count": N, "total_task_count": N, "total_api_count": N, "total_api_cost": N, "total_ai_estimated_days": N}`
        - 在 `main.go` 注册：`api.GET("/aggregate", getAggregate)` 和 `api.GET("/aggregate/summary", getAggregateSummary)`

- [x] 2.8 新增提效分析 API
     【目标对象】`backend/analysis_handler.go`（新增文件）+ `backend/main.go` 的 `main()` 函数路由注册
     【修改目的】提供提效分析数据查询、触发计算、纠错功能、纠错历史查询和分析文件下载（对应 a.md 8.5）
     【修改方式】新增 `backend/analysis_handler.go` 文件（package main），在 `main.go` 的 `main()` 中注册路由
     【相关依赖】`backend/db.go` 的 PG CRUD 函数（任务 2.3）；`backend/es_client.go` 的 ES 查询方法（任务 2.5）；a.md 第六章提效分析逻辑；`backend/main.go` 的全局变量 `db *sql.DB` 和 `appConfig.RawDataDir`
     【修改内容】
        - `GET /api/analysis/efficiency` handler — `getEfficiency(c *gin.Context)` 函数：
          * 必填参数：dimension(project/repo), id, startDate, endDate
          * dimension 不是 project/repo 时返回 400
          * 查询逻辑：
            1. 先查 PG（dimension=project → GetProjectMetrics，dimension=repo → GetRepoMetrics）
            2. 如果 PG 有缓存结果，直接返回（标注 is_corrected=true/false 基于 corrected_ai_estimated_days 是否为 NULL）
            3. 如果 PG 无结果，实时从 Task 表计算：
               - 查询该维度下所有 task（如 project_id=id 的所有 task）
               - 按 user_id 分组，每用户计算 lead_time 和 process_time（a.md 6.2.2）
               - process_time 按 a.md 5.3 合并算法计算（相邻 task gap ≤10分钟合并）
               - 汇总所有用户的 lead_time/process_time
               - 计算提效比例（a.md 6.2.3）：efficiency_ratio = ai_estimated_days / (time_ms / 28800000) * 100
               - 计算成本指标（a.md 6.2.4）：cost_saving = ai_estimated_days × daily_rate - api_cost, roi = cost_saving / api_cost × 100
          * 返回格式与 a.md 8.5 响应示例一致：包含 ai_estimated（raw_days/corrected_days/is_corrected）、actual_time（users 列表）、efficiency、cost、analysis_file 字段
        - `POST /api/analysis/efficiency/calculate` handler — `calculateEfficiency(c *gin.Context)` 函数：
          * 参数（JSON body）：dimension, id, startDate, endDate
          * 强制重新计算（不读缓存）：
            1. 从 ES Task 表查询该维度的所有 task
            2. 执行完整提效计算流程（同上）
            3. 生成分析过程文件：写入 `{rawdata_dir}/YYYY-MM/analysis/{dimension}_{id}_{YYYYMMDD}.json`（格式见 a.md 6.4.2）
            4. 写入 PG：Upsert project_metrics 或 repo_metrics
          * 返回计算结果 + analysis_file_path
          * 错误处理：ES 查询失败、PG 写入失败、文件写入失败均返回 500 + 错误信息
        - `PUT /api/analysis/efficiency/correct` handler — `correctEfficiency(c *gin.Context)` 函数：
          * 参数（JSON body）：dimension, id, field(当前仅支持 "ai_estimated_days"), value(float64), reason(必填), by(必填)
          * field 不是 "ai_estimated_days" 时返回 400
          * reason 为空时返回 400
          * 操作步骤：
            1. 查询 PG 当前记录（不存在返回 404）
            2. 记录 old_value，更新 corrected_ai_estimated_days = value
            3. 写入 correction_history（记录 old_value、new_value、reason、corrected_by）
            4. 重新计算 efficiency_ratio_lead 和 efficiency_ratio_process（用 corrected 值替代 raw 值）
            5. 更新 PG 记录（efficiency_ratio + corrected 字段 + updated_at）
          * 返回更新后的完整记录
        - `GET /api/analysis/efficiency/history` handler — `getEfficiencyHistory(c *gin.Context)` 函数：
          * 参数：dimension, id
          * 调用 `ListCorrectionHistory(db, dimension, id)` 查询 PG
          * 返回格式：`{"items": [{"field_name": "...", "old_value": "...", "new_value": "...", "reason": "...", "corrected_by": "...", "corrected_at": "..."}]}`
        - `GET /api/analysis/efficiency/file` handler — `getEfficiencyFile(c *gin.Context)` 函数：
          * 参数：dimension, id, date
          * 拼接文件路径：`{rawdata_dir}/YYYY-MM/analysis/{dimension}_{id}_{date}.json`
          * 文件不存在返回 404
          * 文件存在则读取并返回 JSON 内容（Content-Type: application/json）
          * 注意：id 可能包含特殊字符（如 URL 中的 repo_id），需要对文件名做安全处理（替换非法字符）
        - 在 `main.go` 注册：
          * `api.GET("/analysis/efficiency", getEfficiency)`
          * `api.POST("/analysis/efficiency/calculate", calculateEfficiency)`
          * `api.PUT("/analysis/efficiency/correct", correctEfficiency)`
          * `api.GET("/analysis/efficiency/history", getEfficiencyHistory)`
          * `api.GET("/analysis/efficiency/file", getEfficiencyFile)`

- [ ] 2.9 新增 Git 分析 API
     【目标对象】`backend/git_handler.go`（新增文件）+ `backend/main.go` 的 `main()` 函数路由注册
     【修改目的】提供 Repo 维度的 Git commit 分析功能（对应 a.md 8.6 的 3 个接口）
     【修改方式】新增 `backend/git_handler.go` 文件（package main），在 `main.go` 的 `main()` 中注册路由
     【相关依赖】`backend/db.go` 的 `UpsertRepoMetrics()` / `GetRepoMetrics()`（任务 2.3）；`backend/main.go` 的 `appConfig.RawDataDir`；`os/exec` 包执行 git 命令；a.md 6.3 节 Git Commit 分析逻辑
     【修改内容】
        - `GET /api/analysis/git` handler — `getGitAnalysis(c *gin.Context)` 函数：
          * 参数：repo_id, startDate, endDate
          * 查询 PG repo_metrics 获取已有 git 分析数据（git_commit_count, git_contributor_count, git_lines_added, git_lines_deleted, git_files_changed）
          * PG 无数据返回空结构（不自动触发分析），提示用户调用 POST analyze
          * 返回格式：`{"repo_id": "...", "git_analysis": {"commit_count": N, "contributor_count": N, "lines_added": N, "lines_deleted": N, "files_changed": N}, "ai_estimated": {"from_task": N, "from_git": N, "final": N}}`
        - `POST /api/analysis/git/analyze` handler — `analyzeGit(c *gin.Context)` 函数：
          * 参数（JSON body）：repo_id, startDate, endDate
          * 执行步骤（a.md 6.3.1）：
            1. 在临时目录 clone repo（如果已存在则 git pull）
            2. 执行 `git log --since=startDate --until=endDate --pretty=format:"%H|%an|%ae|%at|%s"` 获取 commit 列表
            3. 执行 `git log --since=startDate --until=endDate --numstat --pretty=format:""` 获取 lines_added/deleted/files_changed
            4. 统计 commit_count, contributor_count(distinct author), lines_added, lines_deleted, files_changed
            5. 将结果写入 PG repo_metrics 的 git_* 字段
            6. 生成分析文件：`{rawdata_dir}/YYYY-MM/analysis/repo_{repo_id}_{date}.json`
          * 错误处理：git clone/pull 失败返回 500 + 详细错误；repo_id 格式无效（不是 git URL）返回 400
          * 返回 git 分析结果
        - `GET /api/analysis/git/commits` handler — `getGitCommits(c *gin.Context)` 函数：
          * 参数：repo_id, startDate, endDate
          * 如果本地有已 clone 的 repo，直接执行 git log 获取 commit 列表
          * 如果本地没有，返回 404 并提示先调用 POST analyze
          * 返回格式：`{"commits": [{"hash": "...", "author": "...", "date": "...", "message": "..."}]}`
        - 在 `main.go` 注册：
          * `api.GET("/analysis/git", getGitAnalysis)`
          * `api.POST("/analysis/git/analyze", analyzeGit)`
          * `api.GET("/analysis/git/commits", getGitCommits)`

- [ ] 2.10 编译验证和集成测试
     【目标对象】`backend/` 整体
     【修改目的】确保所有代码编译通过，各 API 端点可正常访问
     【修改方式】编译、建表、启动验证
     【相关依赖】所有前置任务（2.1-2.9）
     【修改内容】
        - `go build ./...`（在 backend/ 目录下）编译通过，无编译错误
        - 执行 `init_db.sql` 创建 PG 数据库表：`$env:PGPASSWORD='1'; psql -U postgres -d report -f ../init_db.sql`
        - 启动后端服务（`go run .`），验证以下端点可访问：
          * `GET /api/indices` — 返回 200（request/task 分类）
          * `GET /api/requests?startDate=20260301&endDate=20260301` — 返回 200
          * `GET /api/tasks?startDate=20260301&endDate=20260301` — 返回 200
          * `GET /api/tasks/summary?startDate=20260301&endDate=20260301` — 返回 200
          * `GET /api/aggregate?startDate=20260301&endDate=20260301&dimension=project` — 返回 200
          * `GET /api/aggregate/summary?startDate=20260301&endDate=20260301&dimension=project` — 返回 200
          * `GET /api/analysis/efficiency?dimension=project&id=test&startDate=20260301&endDate=20260301` — 返回 200（空数据）
          * `GET /api/analysis/efficiency/history?dimension=project&id=test` — 返回 200（空列表）
          * `GET /api/analysis/git?repo_id=test&startDate=20260301&endDate=20260301` — 返回 200（空数据）
        - 验证旧路由 `/api/es/stat-data` 和 `/api/es/stat-summary` 返回 404（已删除）
