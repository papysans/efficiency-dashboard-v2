## 实施

- [x] 10.1 后端 listProjectsV2 改为从 tasks 表聚合
     【目标对象】`backend/project_handler_v2.go`
     【修改目的】让 Project 列表展示所有有 task 数据的项目，不再仅依赖 costrict_projects 表
     【修改方式】重写 listProjectsV2 函数，用 SQL 从 costrict_tasks 按 project_id 分组聚合
     【修改内容】
        - 新 SQL 聚合逻辑（参考 user_handler_v2.go 的模式）：
          ```sql
          SELECT project_id, MAX(user_name) as sample_user, 
            COUNT(*) as task_count, COALESCE(SUM(cost),0) as total_cost,
            COALESCE(SUM(upstream_tokens+downstream_tokens),0) as total_tokens,
            COALESCE(SUM(diff_lines),0) as total_diff_lines,
            COALESCE(SUM(ai_estimated_ancient_days),0) as ai_estimated_days,
            MIN(start_time) as min_time, MAX(end_time) as max_time,
            COUNT(DISTINCT user_id) as user_count
          FROM costrict_tasks [WHERE 时间条件] 
          GROUP BY project_id
          ```
        - 同时从 costrict_commits 按 project_id 匹配聚合 commit_count（如果 commit 也有 project_path 字段可以关联）
        - 返回字段调整：key 从 repo_id 改为 project_id
        - 保留分页和搜索逻辑
        - 过滤空 project_id

- [x] 10.2 后端 projectDetailResponse 改为从 tasks/commits 实时查询
     【目标对象】`backend/project_handler_v2.go`
     【修改目的】Project 详情不再依赖 JSONB 数组，直接从 tasks/commits 表查询
     【修改内容】
        - 新增 `getProjectDetailByProjectIdV2` handler：GET `/api/v2/projects/by-project-id?projectId=xxx`
        - 逻辑：
          1) 从 costrict_tasks WHERE project_id = $1 查所有 tasks
          2) 从查到的 tasks 中提取 repo_id 列表，用 repo_id 反查 costrict_commits
          3) 如果 costrict_projects 表有对应 repo_id 的记录，附带返回 silica 数据
          4) 返回 `{project_id, tasks, commits, silica_data, summary:{task_count, commit_count, total_cost, total_tokens, ...}}`
        - 在 main.go 注册新路由
        - 保留原有 `/api/v2/projects/:repoId` 路由不动（向后兼容）

- [x] 10.3 前端 ProjectViewV2 适配新数据源
     【目标对象】`frontend/src/views/ProjectViewV2.vue` + `frontend/src/api/es.js`
     【修改目的】表格 key 从 repo_id 改为 project_id，详情调用新 API
     【修改内容】
        - api/es.js：新增 `getProjectDetailByProjectId(projectId)` → GET `/v2/projects/by-project-id?projectId=xxx`
        - ProjectViewV2.vue 表格列调整：
          - 第一列从"仓库ID"改为"项目ID"，prop 从 repo_id 改为 project_id（截断显示，tooltip 显示全文）
          - 新增"用户数"列
        - 详情加载逻辑：行点击 → 调用 getProjectDetailByProjectId(row.project_id) 而非 getProjectDetailV2(row.repo_id)
        - 详情区指标卡片：从 API 返回的 summary 读取（不再从 JSONB 解析）
        - 参与者列表：从返回的 tasks 按 user_id 分组（保持不变）
        - 硅比例：如果有 silica_data 则展示，否则隐藏

- [x] 10.4 User 列表时间格式化 + 全局空数据提示
     【目标对象】`UserViewV2.vue` + `ProjectViewV2.vue` + `OrgViewV2.vue` + `Home.vue`
     【修改目的】修复时间显示格式和空数据体验
     【修改内容】
        - UserViewV2.vue 活跃时间列：添加格式化函数，ISO8601 → YYYY-MM-DD
          ```javascript
          function fmtDate(val) { return val ? val.substring(0, 10) : '-' }
          ```
          模板改为 `{{ fmtDate(row.first_active) }} ~ {{ fmtDate(row.last_active) }}`
        - 所有 v2 视图的 el-table 添加 `empty-text="暂无数据"`
        - Home.vue：筛选区从 el-card 中移出，放在指标卡片上方单独一行（用 div.kb-filter-row 包裹）

- [ ] 10.5 编译验证 + 重新运行审查测试
     【目标对象】backend/ + frontend/
     【修改内容】
        - go build ./... 通过
        - npm run build 通过
        - 重启 backend
        - 运行 Playwright ui-audit 测试验证改进效果
