## 实施

- [x] 7.1 后端新增 Org 聚合查询 API
     【目标对象】`backend/org_handler_v2.go`（新建）+ `backend/main.go`
     【修改目的】基于 org_mapping.csv + PG 数据实现组织层级聚合查询
     【修改方式】新建 handler 文件；在启动时加载 org_mapping.csv 到内存（或复用现有 org 映射机制），在 API 中关联 PG 数据
     【相关依赖】`org_mapping.csv`（项目根目录），`backend/db.go` 的 ListCostrictTasks/ListCostrictCommits
     【修改内容】
        - 启动时加载 org_mapping.csv 到内存 map：user_id → {user_name, org1, org2, org3, org4}
        - `listOrgV2(c *gin.Context)`：GET `/api/v2/orgs`
          - 参数：level(org1/org2/org3/org4), parent（上级组织路径，如 "研发中心/平台部"），startDate, endDate
          - 逻辑：
            1) 从 org_mapping 中筛选符合 parent 条件的用户列表
            2) 从 costrict_tasks 中按这些 user_id 聚合：task_count, total_cost, total_tokens
            3) 从 costrict_commits 中按这些 user_id 聚合：commit_count, total_diff_lines
            4) 按当前 level 的 org 值分组，返回子组织列表
          - 返回 `{"data":[{org_name, user_count, task_count, commit_count, project_count, total_cost, total_diff_lines}]}`
        - `getOrgDetailV2(c *gin.Context)`：GET `/api/v2/orgs/detail`
          - 参数：org_path（完整组织路径），startDate, endDate
          - 返回该组织下的用户列表（含各自的 task/commit 统计）和项目列表
          - 返回 `{"org_path":"...", "users":[{user_id, user_name, task_count, commit_count, total_cost}], "projects":[{repo_id, task_count, commit_count}]}`
        - 在 main.go 的 v2 路由组注册

- [x] 7.2 前端新增 Org v2 API + OrgViewV2.vue 页面
     【目标对象】`frontend/src/api/es.js` + `frontend/src/views/OrgViewV2.vue`（新建）
     【修改目的】实现组织层级下钻 + 展开查看组织内用户/项目
     【修改方式】新增 API 函数，新建 Vue 页面
     【相关依赖】composables: useChart；utils: formatters, chart, date
     【修改内容】
        - api/es.js：新增 `getOrgV2(params)` 和 `getOrgDetailV2(params)`
        - OrgViewV2.vue 页面布局：
          ```
          kb-panel
            ├── kb-filter-card（日期 + 查询）
            ├── 面包屑导航（全部 > org1选中 > org2选中 > ...）
            ├── kb-table-card（组织列表表格）
            │     └── el-table: 组织名 / 用户数 / task数 / commit数 / 项目数 / 总费用 / diff行数
            ├── 详情区（v-if="selectedOrg"）
            │     ├── 4指标卡片：用户数 / Task数 / Commit数 / 总费用
            │     ├── 该组织下的用户列表 — 行可点击跳转到 /user-v2
            │     └── 该组织下的项目列表 — 行可点击跳转到 /project-v2
            └── kb-charts-area
                  ├── 费用分布(按子组织) — 柱状图
                  └── 代码产出(按子组织) — 柱状图
          ```
        - 交互：表格行双击 → 下钻到下一级；单击 → 展开详情区
        - 面包屑点击 → 返回上级
        - 最深到 org4 级别不可再下钻

- [x] 7.3 路由导航更新 + 构建验证
     【目标对象】`frontend/src/router/index.js` + `App.vue` + frontend/ + backend/
     【修改内容】
        - router 新增 `/org-v2` 路由
        - App.vue 菜单新增"组织(v2)"
        - go build + npm run build 通过
