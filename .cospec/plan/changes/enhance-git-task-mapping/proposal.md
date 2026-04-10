# 变更：增强 Git 分析 - Task-Commit 关联 + 代码来源分析 + 提效自动计算

## 原因
当前 Git 分析仅统计 commit 数量和代码行数，缺少与 AI 对话 Task 的关联。需要建立 git commit ↔ task 的对应关系，区分"我们 AI 写的代码"和"人写的代码"，从而准确计算 AI 提效比例。用户需要看到：一个 project/repo/人/组织，在一段周期内，AI 贡献了多少代码、节省了多少人天、提效倍率是多少。

## 变更内容
- **CSV 格式扩展**：在 org_csv_file 中增加 `git_user_name` 和 `git_user_email` 两列，建立 AI 对话用户 ↔ git 提交人的身份映射
- **OrgProvider 扩展**：新增 gitNameMap / gitEmailMap，支持通过 git author 反查 user_id
- **Task-Commit 关联引擎**：基于「同一用户 + 时间窗口 + 文件路径交集」的最佳努力匹配，建立多对多关联关系
- **代码来源分析**：将 commit diff 新增代码与 task 中 AI 生成的 code_outputs 做文本比对，区分"AI 写的行数"和"人写的行数"
- **PG 新建关联表**：`task_commit_mapping`（Task-Commit 多对多关联）和 `code_attribution`（代码来源分析结果）
- **提效指标扩展**：在 project_metrics / repo_metrics 中新增 `our_ai_code_lines`、`human_code_lines`、`user_manual_days`（用户自定义人天，不覆盖 AI 预估值）等字段
- **后端 API 扩展**：新增关联查询和代码归因查询接口
- **kbcli 增强**：`analyze git` 命令增加关联和归因步骤

## 影响
- **受影响的代码**：
    - `org_mapping.csv`: 新增 git_user_name, git_user_email 列
    - `kbcli/org_provider.go`: 扩展 OrgProvider，新增 gitNameMap/gitEmailMap + LookupByGit 方法
    - `kbcli/git_analyzer.go`: 增强 AnalyzeCommits 返回详细 commit 列表（含文件路径），新增 Task-Commit 关联和代码归因函数
    - `kbcli/cmd_analyze.go`: 增强 analyze git 命令流程
    - `init_db.sql`: 新增 task_commit_mapping 和 code_attribution 表，扩展 project_metrics/repo_metrics
    - `backend/db.go`: 新增关联表和归因表的 CRUD
    - `backend/analysis_handler.go`: 新增关联查询和代码归因接口
    - `backend/main.go`: 注册新路由
