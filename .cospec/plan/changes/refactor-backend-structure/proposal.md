# 变更：后端架构优化 - 拆分大文件、提取常量、消除重复

## 原因
`analysis_handler.go` 有 1377 行，混合了 AI 调用、计算逻辑、多个 handler、工具函数；硬编码魔法值散布 7+ 处；DB 字段列表重复 4-5 次；process_time 计算算法有两套实现。

## 变更内容
- 提取常量和工具函数到 `constants.go` 和 `utils.go`
- 拆分 `analysis_handler.go` 为 `efficiency_handler.go`（提效分析）和 `attribution_handler.go`（代码归因）
- 提取 AI 调用逻辑到 `ai_client.go`
- 统一 DB 字段列表定义（提取 scan 辅助函数消除重复）
- 合并 `git_handler.go` 中重复的 `getLatestRepoMetrics` 到 `db.go`
- 统一 process_time 计算为一个实现

## 影响
- **受影响的代码**：
  - `backend/constants.go`: 新增，集中所有常量
  - `backend/utils.go`: 新增，通用工具函数
  - `backend/ai_client.go`: 新增，AI 调用封装
  - `backend/efficiency_handler.go`: 新增，从 analysis_handler.go 拆出
  - `backend/attribution_handler.go`: 新增，从 analysis_handler.go 拆出
  - `backend/analysis_handler.go`: 删除（拆分到上述文件）
  - `backend/db.go`: 提取 scan 辅助函数消除字段列表重复
  - `backend/git_handler.go`: 删除重复的 getLatestRepoMetrics，改用 db.go 统一函数
  - `backend/aggregate_handler.go`: 使用公共 process_time 计算函数
  - `backend/main.go`: 路由注册指向新文件中的 handler
