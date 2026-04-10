## 实施

- [x] 9.1 修复 task_ids_silica 数据格式 + 补全 org_mapping 测试数据
     【目标对象】`seed_data.sql` + `org_mapping.csv`
     【修改目的】统一数据格式，确保前后端一致；补全 org 映射使 Org 视图有完整数据
     【修改内容】
        - `seed_data.sql`：将 costrict_projects 中的 task_ids_silica 改为与 associator 一致的格式 `[{"task_id":"task-001","silica":0.85},{"task_id":"task-003","silica":0.72}]`
        - `seed_data.sql`：同步修正 task_ids_manual/task_ids_silica_manual 格式
        - `org_mapping.csv`：新增 user-001~user-005 的映射行，分布在不同的 org1~org4 层级中（如：示例公司/研发体系/平台部/基础设施组、示例公司/研发体系/后端组/支付小组 等）
        - 重新执行 init_db.sql + seed_data.sql 验证数据正确

- [x] 9.2 前端修复硅比例读取逻辑
     【目标对象】`frontend/src/views/ProjectViewV2.vue` + `frontend/src/views/UserViewV2.vue`
     【修改目的】适配 `task_ids_silica` 的 `[{task_id, silica}]` 对象数组格式
     【修改内容】
        - ProjectViewV2.vue：修改 `getTaskSilica(taskId)` 函数，从对象数组中按 task_id 查找对应 silica 值（而非按索引取值）
        - UserViewV2.vue：修改 `projectStats` 中的硅比例均值计算，从对象数组中提取 silica 数值后求平均
        - 两处都需要安全处理：task_ids_silica 可能是 null/undefined/空数组/格式异常

- [x] 9.3 新建 Task 详情页
     【目标对象】`frontend/src/views/TaskDetailV2.vue`（新建）+ `frontend/src/router/index.js` + `frontend/src/App.vue`
     【修改目的】实现 Task 对话历史查看页面，这是设计文档的核心功能
     【修改内容】
        - 新建 TaskDetailV2.vue：
          - 路由：`/task/:taskId`，从 URL 参数获取 taskId
          - 调用 `getTaskDetailV2(taskId)` 获取 task 详情 + conversations
          - 顶部：task 元信息卡片（task_id, user_name, repo_id, start_time~end_time, cost, diff_lines, AI预估人天）
          - 中部：对话历史列表（核心功能），每条 conversation 显示：
            - 发送者标记（user/agent 用不同颜色/图标区分）
            - 时间戳 + 模型名称
            - user_input 内容（如有）
            - 响应摘要（response_content 截断显示，可展开）
            - process_time / tokens / cost 小标签
            - 样式参考聊天界面（左右对齐或上下排列）
          - 底部：返回按钮 + 跳转到关联 project 链接
        - router/index.js：新增路由 `{ path: '/task/:taskId', name: 'TaskDetail', component: ... }`

- [x] 9.4 所有视图增加跨页面点击跳转
     【目标对象】`ProjectViewV2.vue` + `UserViewV2.vue` + `OrgViewV2.vue`
     【修改目的】实现设计文档要求的"多视角互通"
     【修改内容】
        - ProjectViewV2.vue：
          - 参与者列表的 user_name 列改为 el-link，点击 → `router.push({ path: '/user-v2', query: { userId: row.user_id } })`
          - Task 列表的 task_id 列改为 el-link，点击 → `router.push({ path: '/task/' + row.task_id })`
          - Commit 列表的 commit_id 列改为 el-link（暂跳转到对应 task 或显示 commit 信息）
        - UserViewV2.vue：
          - 参与项目列表的 repo_id 列改为 el-link，点击 → `router.push({ path: '/project-v2', query: { repoId: row.repo_id } })`
          - Task 列表的 task_id 列改为 el-link → `/task/{taskId}`
          - 页面支持从 query 参数 `userId` 自动加载指定用户详情
        - OrgViewV2.vue：
          - 用户列表行点击 → `router.push({ path: '/user-v2', query: { userId: row.user_id } })`
          - 项目列表行点击 → `router.push({ path: '/project-v2', query: { repoId: row.repo_id } })`
        - ProjectViewV2.vue/UserViewV2.vue：支持从 URL query 参数自动选中指定项目/用户（onMounted 时检查 route.query.repoId/userId，有则自动触发详情展开）

- [x] 9.5 Project 详情增加人工调整入口
     【目标对象】`frontend/src/views/ProjectViewV2.vue`
     【修改目的】为 project 的 manual 字段提供 UI 操作入口
     【修改内容】
        - 在详情区标题行增加"人工调整"按钮（el-button, type=warning）
        - 点击弹出 el-dialog 对话框：
          - 显示当前自动关联的 task_ids 和 silica 值
          - 允许编辑：排除 commit（commit_ids_exclude_manual）、调整 task 关联和硅比例（task_ids_manual/task_ids_silica_manual）
          - 调整 AI 预估人天（ai_estimated_ancient_days_manual + reason）
          - 调整时间范围（start_time_manual/end_time_manual）
        - 提交调用 `updateProjectManualV2(repoId, data)` API

- [x] 9.6 Project/User 列表增加搜索过滤
     【目标对象】`ProjectViewV2.vue` + `UserViewV2.vue`
     【修改目的】支持在列表中快速搜索定位
     【修改内容】
        - ProjectViewV2.vue：筛选区增加 el-input 搜索框（placeholder="搜索仓库ID"），对表格数据做前端过滤（repo_id 模糊匹配）
        - UserViewV2.vue：筛选区增加 el-input 搜索框（placeholder="搜索用户名/ID"），对表格数据做前端过滤（user_name 或 user_id 模糊匹配）
        - 过滤使用 computed 属性，不改变原始数据

- [ ] 9.7 编译验证 + Playwright 测试更新
     【目标对象】frontend/ + e2e/v2-full-test.spec.js
     【修改内容】
        - npm run build 通过
        - 更新 Playwright 测试：增加 Task 详情页测试、跨页面导航测试、搜索过滤测试
        - 运行全部测试确保通过
