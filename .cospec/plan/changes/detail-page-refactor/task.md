## 实施：Project/User 详情独立页面改造

### 阶段 1：ProjectViewV2 改造 + ProjectDetailV2 新建

- [x] 1.1 改造 ProjectViewV2.vue：删除详情展开相关代码（selectedProject、detailData、showManualDialog、manualForm、manualSubmitting、participants、getTaskSilica/parseSilicaArray、silicaChart、handleRowClick中的详情加载逻辑、整个详情区域模板、人工调整对话框模板），行点击改为 router.push({ path: '/project/' + encodeURIComponent(row.project_id) })，保留筛选区、搜索框、列表表格、分页、总览图表
- [x] 1.2 新建 ProjectDetailV2.vue：路由 /project/:projectId，包含返回按钮+标题、项目元信息（el-descriptions）、参与者列表（el-table，用户名el-link→/user/userId）、Task列表（task_id el-link→/task/taskId）、Commit列表、硅比例图表、人工调整按钮+对话框；从 ProjectViewV2 迁移详情相关逻辑

### 阶段 2：UserViewV2 改造 + UserDetailV2 新建

- [x] 2.1 改造 UserViewV2.vue：删除详情展开代码（selectedUser、detailData、projectStats、贡献趋势图、详情区模板），行点击改为 router.push({ path: '/user/' + row.user_id })，保留筛选区、搜索框、列表表格、分页、总览图表
- [x] 2.2 新建 UserDetailV2.vue：路由 /user/:userId，包含返回按钮+标题、用户指标4卡片、参与项目列表（项目ID el-link→/project/projectId）、Task列表（task_id el-link→/task/taskId）、Commit列表、贡献趋势图（双折线图）

### 阶段 3：首页 + OrgViewV2 + 路由

- [x] 3.1 Home.vue：所有8个指标卡片都能点击跳转（总项目数→/project-v2、总用户数→/user-v2、总Task数→/project-v2、总Commit数→/project-v2、总费用→/project-v2、总Token数→/user-v2、总代码行数→/project-v2、AI预估人天→/project-v2），每个卡片加 cursor:pointer 和 @click
- [x] 3.2 OrgViewV2.vue：用户行点击改为 /user/${userId}，项目行点击改为 /project/${encodeURIComponent(projectId)}
- [x] 3.3 router/index.js：新增 /project/:projectId 和 /user/:userId 路由

### 阶段 4：编译验证

- [x] 4.1 npm run build 通过

### 阶段 5：E2E 测试适配

- [x] 5.1 更新 v2-full-test.spec.js：测试 3.2/3.3/4.2/4.3/9.1/9.2/9.3/11.1 从展开改为跳转详情页验证；7.1/12.1 增加详情页 JS 错误检查；新增 13.1 首页指标卡片可点击、13.2 项目详情页独立加载
- [x] 5.2 更新 ui-audit.spec.js：审查 3/5/8 从展开改为跳转详情页审查
