## 实施

- [x] 3.1 实现 Git 仓库管理（克隆/更新）
     【目标对象】`kbcli/git_analyzer.go`（新增文件）
     【修改目的】管理 git 仓库的本地缓存，支持克隆和更新
     【修改方式】新增文件
     【相关依赖】本地 git 命令行工具
     【修改内容】
        - 定义 `GitAnalyzer` 结构体：RepoURL string, LocalPath string, CacheDir string
        - 实现 `NewGitAnalyzer(repoURL string, cacheDir string) *GitAnalyzer`
          * cacheDir 默认为 config.RawDataDir + "/git_cache"
          * LocalPath 为 cacheDir + URL hash（避免特殊字符问题）
        - 实现 `(g *GitAnalyzer) EnsureRepo() error`
          * 如果 LocalPath 不存在：`git clone repoURL localPath`
          * 如果已存在：`git -C localPath pull`
          * 使用 os/exec 调用 git 命令

- [x] 3.2 实现 Git Commit 分析
     【目标对象】`kbcli/git_analyzer.go`
     【修改目的】分析指定时间范围内的 git commit 记录
     【修改方式】在 GitAnalyzer 上新增分析方法
     【修改内容】
        - 定义 `GitAnalysisResult` 结构体：CommitCount, ContributorCount int, LinesAdded, LinesDeleted int64, FilesChanged int, CommitMessages []string
        - 实现 `(g *GitAnalyzer) AnalyzeCommits(startDate, endDate string) (*GitAnalysisResult, error)`
          * 执行 `git log --since=startDate --until=endDate --pretty=format:"%H|%an|%ae|%at|%s"` 获取 commit 列表
          * 执行 `git log --since=startDate --until=endDate --stat --format=""` 获取代码变更统计
          * 解析输出：统计 commit 数量、贡献者去重数、新增/删除行数、变更文件数
          * 收集 commit messages

- [x] 3.3 实现基于 Git 的二次 AI 预估
     【目标对象】`kbcli/git_analyzer.go`
     【修改目的】结合 task 数据和 git 数据，AI 二次预估人天
     【修改方式】新增函数
     【相关依赖】`kbcli/ai_estimator.go` 的 AI API 调用模式
     【修改内容】
        - 实现 `EstimateFromGit(config AIEstimationConfig, gitResult *GitAnalysisResult, taskSummary map[string]interface{}) (float64, string, error)`
          * 构建 prompt：使用 a.md 6.3.2 节的提示词模板
          * 替换变量：{{total_code_lines}}, {{commit_count}}, {{lines_added}}, {{start_time}}, {{end_time}}, {{contributor_count}}
          * 调用 AI API（复用 ai_estimator.go 中的 HTTP 请求模式）
          * 返回 repo_ai_estimated_days 和 repo_ai_estimated_reason

- [x] 3.4 新增 analyze 命令和 Git 分析子命令
     【目标对象】`kbcli/cmd_analyze.go`（新增文件）+ `kbcli/cmd_root.go`
     【修改目的】提供 CLI 命令触发 Git 分析
     【修改方式】新增文件和注册命令
     【修改内容】
        - 新增 cmd_analyze.go：
          * `runAnalyze(config *Config, args []string)` 主函数
          * 子命令 `git`：`kbcli analyze git --repo-id=xxx --start-date=20260301 --end-date=20260331`
          * 流程：EnsureRepo → AnalyzeCommits → EstimateFromGit → 保存结果到 rawdata/analysis/ 目录 → 更新 PG repo_metrics 表
        - cmd_root.go：注册 `analyze` 命令

- [x] 3.5 新增后端 Git 分析 API
     【目标对象】`backend/git_handler.go`（新增文件）+ `backend/main.go`
     【修改目的】提供 Git 分析数据查询接口
     【修改方式】新增 handler 文件和注册路由
     【相关依赖】`backend/db.go` 的 repo_metrics 查询；a.md 8.6 节
     【修改内容】
        - `GET /api/analysis/git` handler：从 PG repo_metrics 表查询 git 分析数据
        - `POST /api/analysis/git/analyze` handler：触发 git 分析（调用 kbcli 或内部函数）
        - `GET /api/analysis/git/commits` handler：从分析文件读取 commit 列表
        - main.go 注册新路由

- [x] 3.6 编译验证和单元测试
     【目标对象】`kbcli/` + `backend/`
     【修改目的】确保代码编译通过和核心逻辑正确
     【修改内容】
        - kbcli: go build ./... 通过
        - backend: go build ./... 通过
        - 新增 git_analyzer_test.go：测试 commit 日志解析、统计计算
