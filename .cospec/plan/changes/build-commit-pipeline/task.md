## 实施

- [x] 3.1 后端 db.go 新增 Commit 分页查询和计数函数
     【目标对象】`backend/db.go`
     【修改目的】现有 `ListCostrictCommits()` 无分页参数，且缺少 `CountCostrictCommits()` 计数函数，无法支持查询 API 的分页需求
     【修改方式】修改 `ListCostrictCommits` 函数签名添加分页参数，新增 `CountCostrictCommits` 函数
     【相关依赖】已有的 `CostrictCommit` 结构体、`scanCostrictCommit()` 辅助函数、`costrictCommitSelectColumns` 变量
     【修改内容】
        - 修改 `ListCostrictCommits` 函数：在现有参数 `(db, repoID, userID, startTime, endTime)` 之后追加 `page, pageSize int` 参数，在 SQL 末尾追加 `LIMIT $N OFFSET $M`（参考 `ListCostrictTasks` 的分页实现方式）
        - 新增 `CountCostrictCommits(db *sql.DB, repoID, userID, startTime, endTime string) (int, error)` 函数：参考 `CountCostrictTasks` 实现，构建动态 WHERE 条件，返回 count(*)
        - 注意：`ListCostrictCommits` 签名变更后，需确认当前无其他调用方（当前代码中无调用）

- [x] 3.2 后端新增 Commit 写入和查询 handler
     【目标对象】`backend/commit_handler_v2.go`（新建）+ `backend/main.go` 的 v2 路由组
     【修改目的】提供 Commit 数据的写入（供 kbcli 调用）和查询（供前端调用）HTTP 接口
     【修改方式】新建 `commit_handler_v2.go` 文件，定义 4 个 handler 函数；在 `main.go` 第 155-161 行的 v2 路由组中追加 commit 路由
     【相关依赖】`backend/db.go` 的 `UpsertCostrictCommit()`、`GetCostrictCommit()`、`ListCostrictCommits()`（3.1 修改后）、`CountCostrictCommits()`（3.1 新增）、`CostrictCommit` 结构体；`backend/utils.go` 的 `parseDateParam()`、`backend/constants.go` 的 `DefaultPageSize`
     【修改内容】
        - 新建 `backend/commit_handler_v2.go`，参照 `task_handler_v2.go` 的代码风格，定义以下 handler：
        - `upsertCommitV2(c *gin.Context)`：POST `/api/v2/commits`，用 `c.ShouldBindJSON(&commit)` 接收 JSON body 映射为 CostrictCommit，调用 `UpsertCostrictCommit(db, &commit)` 写入 PG；错误返回 400/500；成功返回 `{"status":"ok"}`（参照 `upsertTaskV2` 的实现）
        - `batchUpsertCommitsV2(c *gin.Context)`：POST `/api/v2/commits/batch`，接收 `[]CostrictCommit` 数组，空数组直接返回 `{"status":"ok","count":0}`；在 db.go 中新增 `BatchUpsertCostrictCommits(db, commits)` 函数（参照 `BatchInsertCostrictTaskConversations` 的事务模式，逐条 `UpsertCostrictCommit` 在事务中执行）；成功返回 `{"status":"ok","count":N}`
        - `listCommitsV2(c *gin.Context)`：GET `/api/v2/commits`，接收 query 参数 repoId/userId/startDate/endDate（YYYYMMDD 格式，通过 `parseDateParam` 解析后转 RFC3339）+ page/pageSize（默认 DefaultPageSize）；先调用 `CountCostrictCommits` 获取总数，再调用 `ListCostrictCommits`（含分页）获取列表；返回 `{"total":N,"page":P,"pageSize":S,"data":[...]}`（参照 `listTasksV2` 的实现）
        - `getCommitDetailV2(c *gin.Context)`：GET `/api/v2/commits/:commitId`，从 path 取 commitId，从 query 取 repoId（commit_id+repo_id 构成唯一键）；repoId 为空返回 400；调用 `GetCostrictCommit(db, commitID, repoID)` 查询；不存在返回 404 `{"error":"commit not found"}`；存在返回 `{"commit":{...}}`
        - 在 `main.go` v2 路由组（第 155-161 行之间）追加 4 条路由：`v2.POST("/commits", upsertCommitV2)`、`v2.POST("/commits/batch", batchUpsertCommitsV2)`、`v2.GET("/commits", listCommitsV2)`、`v2.GET("/commits/:commitId", getCommitDetailV2)`

- [x] 3.3 kbcli pg_writer.go 新增 Commit 映射和写入方法
     【目标对象】`kbcli/pg_writer.go`
     【修改目的】实现 CommitDetail → PGCommitData 的数据映射 + 通过 backend API 批量写入 PG
     【修改方式】在 `pg_writer.go` 文件末尾（`SaveConversationsToPG` 函数之后）追加新结构体和函数
     【相关依赖】`kbcli/git_analyzer.go` 的 `CommitDetail` 结构体（Hash/AuthorName/AuthorEmail/Timestamp/LinesAdded 字段），`kbcli/org_provider.go` 的 `OrgProvider.LookupByGitAuthor()` 返回值 `(userID string, found bool)` 和 `OrgProvider.GetOrgInfo()` 返回 `OrgInfo{UserName}`，`kbcli/db_writer.go` 的 `BackendClient` 结构体
     【修改内容】
        - 定义 `PGCommitData` 结构体，JSON tag 使用 PascalCase（与已有 PGTaskData 风格一致）：CommitID(string)、CommitTime(*time.Time)、RepoAddr(*string)、RepoBranch(*string)、RepoID(*string)、GitUserName(*string)、GitUserEmail(*string)、UserID(*string)、UserName(*string)、ClientID(*string)、ProjectPath(*string)、DiffLines(*int64)、AIEstimatedAncientDays(*float64)、AIEstimatedAncientReason(*string)
        - 实现 `MapCommitDetailsToPG(commits []CommitDetail, repoID string, orgProvider *OrgProvider) []PGCommitData`：
          - 对每个 CommitDetail：CommitID=Hash, CommitTime=ptrTime(Timestamp), GitUserName=ptrString(AuthorName), GitUserEmail=ptrString(AuthorEmail)
          - RepoAddr/RepoBranch：从 repoID 按 "#" 分割（复用 MapTaskDocToPG 中已有的分割逻辑）
          - RepoID=ptrString(repoID)
          - user_id 映射：调用 `orgProvider.LookupByGitAuthor(AuthorName, AuthorEmail)` 获取 `(userID, found)`；若 found 为 true，设置 UserID=ptrString(userID)，再调用 `orgProvider.GetOrgInfo(userID, "")` 获取 OrgInfo.UserName 设置 UserName；若 found 为 false，UserID 和 UserName 均为 nil
          - 边界处理：orgProvider 为 nil 时跳过身份映射，UserID/UserName 保持 nil
          - DiffLines=ptrInt64(LinesAdded)（只计新增行）
          - AIEstimatedAncientDays/AIEstimatedAncientReason 初始为 nil（后续 AI 预估填充）
        - 实现 `(c *BackendClient) SaveCommitsToPG(commits []PGCommitData) error`：参照 `SaveConversationsToPG` 的实现方式，将 commits 序列化为 JSON 后 POST 到 `{baseURL}/api/v2/commits/batch`；空 slice 直接返回 nil；非 200 状态码返回错误

- [x] 3.4 kbcli analyze git 命令增加 PG 写入步骤
     【目标对象】`kbcli/cmd_analyze.go` 的 `runAnalyzeGit` 函数
     【修改目的】在 Git 分析完成后，自动将 commit 数据写入 PG
     【修改方式】在 `runAnalyzeGit` 函数中，第 302-313 行的 backend 保存代码块之后、函数末尾之前，追加 PG commit 写入逻辑
     【相关依赖】`kbcli/pg_writer.go` 的 `MapCommitDetailsToPG()` 和 `(BackendClient).SaveCommitsToPG()`；当前函数中已有的 `orgProvider`（第 175 行创建，可能为 nil）和 `config.BackendURL`
     【修改内容】
        - 在现有的 backend 保存代码块（第 302-313 行 `if config.BackendURL != "" { ... }` 之后）追加：
          - 条件：`config.BackendURL != ""` 且 `len(gitResult.Commits) > 0`
          - 复用已有的 `bc`（BackendClient，第 303 行已创建）；注意 bc 在 `if` 块内声明，需要将 bc 提到外层或重新创建
          - 调用 `MapCommitDetailsToPG(gitResult.Commits, repoID, orgProvider)` 映射 commits（orgProvider 可能为 nil，MapCommitDetailsToPG 内部需处理）
          - 调用 `bc.SaveCommitsToPG(pgCommits)` 批量写入 PG
        - 错误处理：PG 写入失败不中断流程，仅 `fmt.Printf("[Analyze] 警告: commit 写入 PG 失败: %v\n", err)`（与同函数中第 309 行的警告风格一致）
        - 成功时打印 `fmt.Printf("[Analyze] %d 条 commit 已写入 PG\n", len(pgCommits))`

- [x] 3.5 编译验证和 API 测试
     【目标对象】`backend/` 和 `kbcli/` 目录
     【修改目的】验证新增代码编译通过且 API 可正常调用
     【修改方式】在各目录执行 `go build ./...` 编译，然后通过 curl 手动测试
     【相关依赖】3.1~3.4 所有任务的产出
     【修改内容】
        - `backend/` 目录 `go build ./...` 编译通过，无报错
        - `kbcli/` 目录 `go build ./...` 编译通过，无报错
        - 启动 backend 服务，通过 curl 测试单条写入：POST /api/v2/commits，body 包含 CommitID 和 RepoID
        - 通过 curl 测试批量写入：POST /api/v2/commits/batch
        - 通过 curl 测试列表查询：GET /api/v2/commits?startDate=20260301&endDate=20260331，验证返回分页格式
        - 通过 curl 测试详情查询：GET /api/v2/commits/:commitId?repoId=xxx
        - SQL 验证：SELECT count(*) FROM costrict_commits 确认数据已入库
