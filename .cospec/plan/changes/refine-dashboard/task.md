## 实施

- [x] 8.1 后端新增 Dashboard 汇总 API
     【目标对象】`backend/dashboard_handler_v2.go`（新建）+ `backend/main.go`
     【修改目的】提供全局汇总指标，供首页 Dashboard 展示
     【修改方式】新建 handler 文件，SQL 聚合多表统计
     【相关依赖】全局变量 db，costrict_tasks/commits/projects 三张表
     【修改内容】
        - `getDashboardSummary(c *gin.Context)`：GET `/api/v2/dashboard/summary`，可选 startDate/endDate
          - SQL 聚合统计：
            - 从 costrict_tasks: COUNT(task_id) as total_tasks, COUNT(DISTINCT user_id) as total_users, COUNT(DISTINCT repo_id) as total_repos, SUM(cost) as total_cost, SUM(upstream_tokens+downstream_tokens) as total_tokens, SUM(ai_estimated_ancient_days) as total_ai_days
            - 从 costrict_commits: COUNT(*) as total_commits, SUM(diff_lines) as total_diff_lines
            - 从 costrict_projects: COUNT(*) as total_projects
          - 返回 `{"total_tasks":N, "total_users":N, "total_repos":N, "total_commits":N, "total_projects":N, "total_cost":F, "total_tokens":N, "total_diff_lines":N, "total_ai_estimated_days":F}`
        - 在 main.go v2 路由注册

- [x] 8.2 改造首页为 Dashboard 概览 + 整理导航菜单
     【目标对象】`frontend/src/views/Home.vue` + `frontend/src/api/es.js` + `frontend/src/App.vue`
     【修改目的】首页展示全局汇总指标 + 快速导航到各视图 + 整理 v2 页面作为主导航
     【修改方式】改造 Home.vue，新增 API 函数，调整 App.vue 导航菜单
     【修改内容】
        - api/es.js：新增 `getDashboardSummary(params)` — GET `/v2/dashboard/summary`
        - Home.vue 改造：
          ```
          kb-panel
            ├── 标题区: "AI Coding 指标看板"
            ├── 日期筛选: el-date-picker(daterange) + 查询按钮
            ├── 指标卡片区（el-row + el-col，2行×4列）
            │     ├── 总项目数(project icon, 点击跳转/project-v2)
            │     ├── 总用户数(user icon, 点击跳转/user-v2)
            │     ├── 总Task数
            │     ├── 总Commit数
            │     ├── 总费用(元)
            │     ├── 总Token数
            │     ├── 总代码行数
            │     └── AI预估人天
            └── 快速导航区: 3张卡片(项目视图/用户视图/组织视图)，点击跳转
          ```
        - App.vue 导航菜单整理：
          - 保留: 首页(/) 
          - 新v2视图作为主要导航: 项目(/project-v2) / 用户(/user-v2) / 组织(/org-v2)
          - 旧视图归入"更多"子菜单（el-sub-menu）: Dashboard / 提效分析 / 项目(旧) / 仓库 / 用户(旧) / 组织(旧)

- [x] 8.3 构建验证
     【目标对象】backend/ + frontend/
     【修改内容】
        - go build ./... 通过
        - npm run build 通过
