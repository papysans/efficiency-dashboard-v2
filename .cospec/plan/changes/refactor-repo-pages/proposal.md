# 变更：重构 Repo 列表页和详情页

## 原因
当前 Repo 列表页使用 FilterBar + 手写 el-table，与项目其他 V2 页面（如 Task）使用的 KbFilterTable 模式不一致。需要统一使用 KbFilterTable 实现表头筛选过滤，并完善详情页的效率评估展示（增加 reason 说明字段）。

## 变更内容
- **Repo 列表页重写**：`ProjectViewV2.vue` 从 FilterBar + el-table 改为 KbFilterTable，支持表头列筛选（搜索/枚举/数字/日期类型），去掉图表区域
- **Repo 列表聚合增加字段**：后端 `ListRepoAggregates` 增加 `task_count`（从 commits.task_ids 聚合），增加 `efficiency_ratio` 计算
- **Repo 详情页完善效率评估**：在效率评估卡片中增加 reason 说明字段（repo_ancient_minutes_reason、repo_real_minutes_reason），从各 commit 的 reason 汇总生成
- **统一路由路径**：保持现有路由 `/repo-v2`（列表）、`/repo/:repoAddr/:repoBranch?`（详情）

## 影响
- **受影响的代码**：
    - `frontend/src/views/ProjectViewV2.vue`: 整体重写为 KbFilterTable 模式，去掉 FilterBar/useChart/图表依赖
    - `frontend/src/views/ProjectDetailV2.vue`: 效率评估卡片增加 reason 展示
    - `backend/project_handler_v2.go`: listReposV2 返回数据增加 task_count 和 efficiency_ratio 字段
    - `backend/db.go`: `ListRepoAggregates` SQL 增加 task_count 聚合字段（通过 jsonb_array_length 统计 task_ids）
