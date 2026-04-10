## 实施

- [x] 4.1 后端新增 Project 关联引擎核心逻辑
     【目标对象】`backend/project_associator.go`（新建）
     【修改目的】实现 commit↔task 的关联匹配和 project 聚合记录生成，复用 `kbcli/git_task_matcher.go` 的时间窗口匹配思路
     【修改方式】新建文件，实现关联算法函数
     【相关依赖】`backend/db.go` 的 `ListCostrictTasks`/`CountCostrictTasks`/`ListCostrictCommits`/`CountCostrictCommits`/`UpsertCostrictProject`/`GetCostrictProject` 函数，以及 `CostrictTask`/`CostrictCommit`/`CostrictProject` 结构体
     【修改内容】
        - 新增 `AssociateProjectByRepo(db *sql.DB, repoID string) (*CostrictProject, error)` 函数：
          - 1) 从 PG 全量查询该 repo_id 下所有 commits：由于 `ListCostrictCommits` 带分页参数，先调用 `CountCostrictCommits` 获取总数，再用足够大的 pageSize 一次性查询（或循环分页查询），保证获取全量数据
          - 2) 从 PG 全量查询该 repo_id 下所有 tasks：同理用 `CountCostrictTasks` + `ListCostrictTasks` 全量获取（注意 task 的 RepoID 字段是 `*string` 指针类型，查询时传 repoID 字符串即可，DB 层已处理）
          - 3) 参考 `kbcli/git_task_matcher.go` 的 `MatchTasksToCommits` 时间窗口匹配逻辑：对每个 commit，找 user_id 相同且 commit_time 落在 task 时间窗口（task.start_time - 1h 到 task.end_time + 24h）内的 tasks。注意 task/commit 的时间字段都是 `*time.Time` 指针，需判空
          - 4) 生成 commit_ids 数组（`json.RawMessage`，JSON 格式 `["commitID1","commitID2",...]`）
          - 5) 生成 task_ids 数组（去重后的关联 task 列表，同格式 JSON 数组）
          - 6) 生成 task_ids_silica（`json.RawMessage`，JSON 格式 `[{"task_id":"xxx","silica":0.8},...]`）。硅比例计算规则：task 和 commit 都有非 nil 的 DiffLines 时，silica = min(task.DiffLines, commit.DiffLines) / max(task.DiffLines, commit.DiffLines)；任一方 DiffLines 为 nil 则默认 0.5；max 值为 0 时默认 0.5
          - 7) 计算加权统计：遍历关联的 tasks，按 silica 加权累加 upstream_tokens/downstream_tokens/cost（均为指针类型，需判空，nil 视为 0）
          - 8) start_time = 所有关联 task 中最早的 start_time，end_time = 最晚的 end_time（均为 `*time.Time`，需判空）
          - 9) 构造 project 结构体（manual 类字段留空），调用 `UpsertCostrictProject` 写入 PG
          - 10) 边界处理：无 commits 或无 tasks 时仍生成空的 project 记录（commit_ids/task_ids 为空 JSON 数组 `[]`）；关联过程中任何单条 task/commit 处理失败不中断整体，log 告警后继续
        - 新增 `listDistinctRepoIDs(db *sql.DB) ([]string, error)` 函数：
          - 执行 SQL `SELECT DISTINCT repo_id FROM costrict_commits WHERE repo_id IS NOT NULL UNION SELECT DISTINCT repo_id FROM costrict_tasks WHERE repo_id IS NOT NULL`，返回去重的 repo_id 列表
        - 新增 `AssociateAllProjects(db *sql.DB) (int, error)` 函数：
          - 调用 `listDistinctRepoIDs` 获取所有 repo_id，逐个调用 `AssociateProjectByRepo`
          - 单个 repo 关联失败时 log.Printf 告警后继续下一个（不中断全量处理）
          - 返回成功处理的 project 数量
        - 错误处理统一使用 `fmt.Errorf("xxx 失败: %w", err)` 风格，与 `db.go` 保持一致

- [x] 4.2 后端新增 AI 硅比例分析（可选增强）
     【目标对象】`backend/project_associator.go`
     【修改目的】通过 AI 分析 task 和 commit 的 diff 信息相似度来计算更精确的硅比例，替代简单的 diff_lines 比例规则
     【修改方式】在 `project_associator.go` 中新增 `estimateSilicaWithAI` 函数，在 `AssociateProjectByRepo` 的硅比例计算步骤中调用
     【相关依赖】`backend/ai_client.go` 中的 Anthropic API 调用模式（`x-api-key` header + `/v1/messages` endpoint + `extractJSON` 解析），`backend/main.go` 的 `appConfig.AIEstimation` 配置，`backend/db.go` 的 `ListCostrictTaskConversations` 函数
     【修改内容】
        - 新增 `estimateSilicaWithAI(taskDiffLines int64, commitDiffLines int64, taskConvSummary string) (float64, error)` 函数：
          - 复用 `ai_client.go` 中的 HTTP 请求模式：构造 Anthropic Messages API 请求（使用 `appConfig.AIEstimation` 的 APIKey/BaseURL/Model/TimeoutMS/HTTPProxy 配置）
          - 构造 prompt：包含 task 的对话摘要（user_input 列表拼接）和 diff_lines 信息，要求 AI 返回 0-1.0 的相似度评分
          - 解析 AI 响应：复用 `utils.go` 的 `extractJSON` 函数提取 JSON 结果
          - 返回值范围校验：结果 < 0 取 0，> 1 取 1
          - 如果 AI 未启用、服务不可用或超时，fallback 到简单 diff_lines 比例规则（即任务 4.1 中的计算方式）
        - 在 `AssociateProjectByRepo` 中修改硅比例计算步骤：
          - 先检查 `appConfig.AIEstimation.Enabled`，若开启则对每个 task-commit 关联对调用 `estimateSilicaWithAI`
          - 调用前需通过 `ListCostrictTaskConversations(db, taskID)` 获取对话列表，拼接 user_input 作为摘要
          - 批量处理时在每次 AI 调用间加 `time.Sleep(500*time.Millisecond)` 速率控制，避免 API 过载

- [x] 4.3 后端新增 Project 查询和管理 API
     【目标对象】`backend/project_handler_v2.go`（新建）+ `backend/main.go` 的 v2 路由组
     【修改目的】提供 Project 的查询列表、详情展示、触发关联分析、人工调整 4 个 HTTP 接口
     【修改方式】新建 `project_handler_v2.go` handler 文件，在 `main.go` 的 v2 路由组（第 155-166 行）中新增 4 条路由注册
     【相关依赖】`backend/project_associator.go` 的 `AssociateProjectByRepo`/`AssociateAllProjects`，`backend/db.go` 的 `ListCostrictProjects`/`GetCostrictProject`/`UpdateCostrictProjectManual`/`GetCostrictTask`/`GetCostrictCommit`
     【修改内容】
        - 新增 `triggerProjectAssociation(c *gin.Context)` 函数：
          - POST `/api/v2/projects/associate`
          - 可选 query 参数 `repoId`：有值则调用 `AssociateProjectByRepo(db, repoId)`，无值则调用 `AssociateAllProjects(db)`
          - 成功返回 `gin.H{"status": "ok", "count": N}`（N 为处理的 project 数量，单个 repo 时 N=1）
          - 错误返回 `gin.H{"error": err.Error()}` + 500 状态码
        - 新增 `listProjectsV2(c *gin.Context)` 函数：
          - GET `/api/v2/projects`
          - 可选参数 startDate/endDate（格式 YYYYMMDD，用 `parseDateParam` 解析后转 RFC3339）
          - 调用 `ListCostrictProjects(db, startTime, endTime)` 获取全量列表
          - 分页在 handler 层做内存分页（用 `getDefaultInt(c, "page", 1)` 和 `getDefaultInt(c, "pageSize", DefaultPageSize)` 获取参数，对结果切片），因为 DB 层不支持分页
          - 返回 `gin.H{"total": len(all), "page": page, "pageSize": pageSize, "data": pagedSlice}`，与 `listTasksV2`/`listCommitsV2` 响应格式一致
        - 新增 `getProjectDetailV2(c *gin.Context)` 函数：
          - GET `/api/v2/projects/:repoId`
          - 调用 `GetCostrictProject(db, repoId)` 获取 project 记录，不存在返回 404 + `gin.H{"error": "project not found"}`
          - 解析 project 的 TaskIDs（`json.RawMessage`）为 `[]string`，逐个调用 `GetCostrictTask(db, taskID)` 获取关联 task 详情
          - 解析 project 的 CommitIDs（`json.RawMessage`）为 `[]string`，逐个调用 `GetCostrictCommit(db, commitID, repoId)` 获取关联 commit 详情
          - 返回 `gin.H{"project": project, "tasks": tasks, "commits": commits}`
          - 边界处理：某个 taskID/commitID 查不到时跳过（不报错），仅返回查到的记录
        - 新增 `updateProjectManualV2(c *gin.Context)` 函数：
          - PUT `/api/v2/projects/:repoId/manual`
          - 接收 JSON body 绑定到 `CostrictProject` 结构体（只使用 manual 相关字段）
          - 调用已有的 `UpdateCostrictProjectManual(db, repoId, &project)` 写入
          - 成功返回 `gin.H{"status": "ok"}`，记录不存在返回 404
        - 在 `main.go` 的 v2 路由组中新增路由注册（在第 166 行 `}` 前）：
          - `v2.POST("/projects/associate", triggerProjectAssociation)`
          - `v2.GET("/projects", listProjectsV2)`
          - `v2.GET("/projects/:repoId", getProjectDetailV2)`
          - `v2.PUT("/projects/:repoId/manual", updateProjectManualV2)`

- [x] 4.4 编译验证和关联测试
     【目标对象】`backend/` 目录
     【修改目的】验证新增的关联引擎和 API 代码能正确编译运行
     【修改方式】编译 backend 并通过 curl 调用 API 验证
     【相关依赖】`backend/project_associator.go`、`backend/project_handler_v2.go`、`backend/main.go`
     【修改内容】
        - 在 backend/ 目录执行 `go build ./...` 确保编译通过，修复编译错误
        - 启动 backend 服务
        - 调用 POST `/api/v2/projects/associate` 触发全量关联分析，验证返回 `{"status":"ok","count":N}`
        - SQL 验证：`SELECT repo_id, jsonb_array_length(task_ids) as task_count, jsonb_array_length(commit_ids) as commit_count FROM costrict_projects;` 确认数据已写入
        - 调用 GET `/api/v2/projects` 验证列表查询返回分页格式
        - 调用 GET `/api/v2/projects/{repoId}` 验证详情含 project/tasks/commits 三个字段
