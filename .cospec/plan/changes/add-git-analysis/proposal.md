# 变更：新增 Git 分析功能（Repo 维度 commit 分析 + 二次 AI 预估）

## 原因
a.md 6.3 节要求 Repo 维度除了基础提效计算外，还需要分析 git commit 记录，并基于 commit 数据进行二次 AI 预估人天。这是提效分析的增强功能，为管理人员提供更准确的提效评估。

## 变更内容
- 新增 Git 分析 CLI 命令：`kbcli analyze git --repo-id=xxx --start-date=20260301 --end-date=20260331`
- 实现 Git 仓库克隆/更新：将远程 repo 克隆到本地缓存目录
- 实现 Git commit 分析：统计 commit 数量、贡献者、代码变更量
- 实现基于 commit 的二次 AI 预估：调用大模型 API 综合 task 数据和 git 数据重新预估人天
- 新增后端 Git 分析 API：`/api/analysis/git`
- 生成分析结果文件保存到 rawdata

## 影响
- **受影响的代码**：
    - `kbcli/git_analyzer.go`: **新增文件**，Git 克隆/pull + commit 分析逻辑
    - `kbcli/cmd_analyze.go`: **新增文件**，analyze 命令（含 git 子命令）
    - `kbcli/cmd_root.go`: 注册 analyze 命令
    - `backend/git_handler.go`: **新增文件**，Git 分析 API handler
    - `backend/main.go`: 注册 Git 分析路由
