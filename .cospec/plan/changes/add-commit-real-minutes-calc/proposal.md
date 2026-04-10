# 变更：新增 commit 实际耗时分拆计算（AI/古法）

## 原因
commit 的实际耗时 commit_real_minutes 当前没有自动计算逻辑，需要基于关联 task 的实际耗时和硅比例自动计算，分拆为 AI 耗时和古法耗时两部分。

## 变更内容
- 新增 `commit_real_ai_minutes` 字段：AI 参与部分的实际耗时 = sum(task_ids[i].task_real_minutes * task_ids_silica[i])
- 新增 `commit_real_ancient_minutes` 字段：非 AI 部分的古法预估耗时 = sum(task_ids[i].task_ancient_minutes * (1-task_ids_silica[i]))
- 修改 `commit_real_minutes` 计算算法：commit_real_minutes = commit_real_ai_minutes + commit_real_ancient_minutes
- 当 task_ids 为空时：commit_real_ai_minutes = 0，commit_real_ancient_minutes = commit_ancient_minutes
- 计算时机：在 getCommitDetailV2 中实时计算，异步写回数据库（与 task_real_minutes 模式一致）
- 更新 seed_data.sql，为 commit 添加 task 关联数据，为 task 补充 ancient_minutes 和 real_minutes 数据

## 影响
- **受影响的代码**：
    - `init_db.sql`: 新增 commit_real_ai_minutes、commit_real_ancient_minutes 两个字段的迁移 SQL
    - `backend/db.go:214-241 CostrictCommit struct`: 新增两个字段
    - `backend/db.go:467-500 costrictCommitSelectColumns + scanCostrictCommit`: 新增字段的 select 和 scan
    - `backend/db.go:1519-1542 UpdateCostrictCommitTaskAssoc`: 更新函数支持写入新字段
    - `backend/commit_handler_v2.go:93-171 getCommitDetailV2`: 添加计算逻辑，实时计算两个新字段和 commit_real_minutes，异步写回
    - `seed_data.sql`: 更新 commit 数据添加 task_ids/task_ids_silica，更新 task 数据添加 task_ancient_minutes/task_real_minutes
