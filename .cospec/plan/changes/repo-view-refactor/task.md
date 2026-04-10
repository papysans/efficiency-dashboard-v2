# repo-view-refactor: 项目视图一级关联维度从 project_id 改为 repo（repo_addr）

## 阶段一：后端 API + 前端基础设施

### 1.1 后端：新增 Repo 列表 API 和 Repo 详情 API + 路由注册
- [x] 1.1.1 在 `backend/project_handler_v2.go` 中新增 `listReposV2` — GET `/api/v2/repos`，从 costrict_projects 表聚合 repo 列表数据（repo_id, repo_addr, repo_branch, task_count, commit_count, total_tokens, total_cost, ai_estimated_days, start_time, end_time）
- [x] 1.1.2 在 `backend/project_handler_v2.go` 中新增 `getRepoDetailV2` — GET `/api/v2/repos/detail?repoId=xxx`，返回 repo 完整关联信息（summary + commits 列表含 silica 和关联 tasks + tasks 列表 + silica_entries + 参与者）
- [x] 1.1.3 在 `backend/main.go` 注册路由 `v2.GET("/repos", listReposV2)` 和 `v2.GET("/repos/detail", getRepoDetailV2)`

### 1.2 前端：API 函数 + 路由 + 导航 + 首页
- [x] 1.2.1 `frontend/src/api/es.js` 新增 `getReposV2(params)` 和 `getRepoDetailV2(repoId)` 函数
- [x] 1.2.2 `frontend/src/router/index.js` 新增 `/repo/:repoId` 路由（ProjectDetailV2），保留原 `/project/:projectId`
- [x] 1.2.3 `frontend/src/App.vue` 导航菜单"项目"改为"仓库"，路由指向 `/project-v2`（内容改为 repo 列表）
- [x] 1.2.4 `frontend/src/views/Home.vue` "总项目数"卡片改为"总仓库数"，导航卡片"项目视图"改为"仓库视图"

## 阶段二：前端视图改造

### 2.1 改造 ProjectViewV2.vue 为 Repo 列表页
- [x] 2.1.1 标题和搜索从"项目ID"改为"仓库地址"搜索
- [x] 2.1.2 API 调用从 `getProjectsV2` 改为 `getReposV2`
- [x] 2.1.3 表格列改为：仓库地址 / 分支 / Task数 / Commit数 / 总费用 / AI预估人天 / 时间范围
- [x] 2.1.4 行点击跳转改为 `/repo/${encodeURIComponent(row.repo_id)}`
- [x] 2.1.5 图表标题从"按项目"改为"按仓库"

### 2.2 改造 ProjectDetailV2.vue 为 Repo 详情页
- [x] 2.2.1 路由参数从 `projectId` 改为 `repoId`，API 调用改为 `getRepoDetailV2`
- [x] 2.2.2 标题改为"仓库详情: {repo_addr}"，概览区域展示仓库元信息
- [x] 2.2.3 新增以 commit 为主要展示维度的 el-table（含可展开行：硅含量 AI 分析理由 + 关联 Tasks 子表格）
- [x] 2.2.4 硅比例用 el-progress 展示（>80%绿色 / 50-80%蓝色 / <50%橙色）
- [x] 2.2.5 参与者列表改为从 tasks 和 commits 聚合用户名（el-link→/user/xxx）

### 2.3 更新跨页面跳转
- [x] 2.3.1 `UserDetailV2.vue` 中参与项目列表 → 改为参与仓库列表，跳转 `/repo/xxx`
- [x] 2.3.2 `OrgDetailV2.vue` 中项目列表 → 改为仓库列表，跳转 `/repo/xxx`

## 验证
- [x] go build ./... 通过
- [x] npm run build 通过
