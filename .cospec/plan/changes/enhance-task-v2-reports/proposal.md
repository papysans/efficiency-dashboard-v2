# 变更：增强 task-v2 页面 - 组织列、筛选保持、用户报表

## 原因
task-v2 页面缺少组织维度信息，导航返回时筛选条件丢失影响用户体验，且缺少按用户维度的报表分析能力。

## 变更内容

### 1. 增加组织列 + 级联筛选
- 后端 `listTasksV2` 关联 `orgMappings` 在返回数据中补充 org1-org4 字段
- 前端在 columns 中"用户"列后增加"组织"列，显示 `org1/org2/.../orgN` 拼接
- 在 KbFilterTable 中新增 `cascade-org` 筛选类型，支持4级级联下拉的宽 popover（400px），复用 `getOrgV2` API 加载选项
- 组织筛选在客户端执行：匹配所选的最深层级 org 值

### 2. 筛选条件保持（导航返回不丢失）
- 在 App.vue 的 `<router-view>` 外层包裹 `<keep-alive :include="['TaskViewV2']">`
- 为 TaskViewV2 组件添加 `defineOptions({ name: 'TaskViewV2' })` 以支持 include 匹配

### 3. 增加报表按钮
- 在"添加到 Project"按钮左侧添加"组织报表"、"用户报表"、"时间报表"3个按钮
- "用户报表"跳转到新页面并携带当前筛选条件（日期范围、组织筛选）
- "组织报表"和"时间报表"按钮点击后 ElMessage.info 提示"敬请期待"

### 4. 用户报表页面（新页面 `/task-v2/report/user`）
- 顶部筛选区：返回按钮 + 日期范围 + 4级组织级联筛选 + 查询按钮（复用 FilterBar + 级联组件模式）
- 从 URL query 参数接收筛选条件（startDate, endDate, org1~org4）
- 调用 `getUsersV2` API（已支持 org1-4 和日期参数），获取按用户维度的聚合数据
- 展示 dashboard：
  - **汇总指标卡**（1行6列）：总Task数、总代码行数、总传统耗时、总实际耗时、总费用、平均提效比
  - **6个图表**（3行2列网格）：
    a) Task数（按用户，横向柱状图）
    b) 代码行数（按用户，横向柱状图）
    c) 传统耗时 vs 实际耗时（按用户，双柱对比图，橙色=传统、蓝色=实际）
    d) 费用（按用户，横向柱状图）
    e) Token消耗（按用户，横向柱状图）
    f) 提效比（按用户，横向柱状图）

## 影响
- **受影响的代码**：
    - `backend/task_handler_v2.go`: `listTasksV2` 关联 orgMappings 补充 org1-4 字段
    - `frontend/src/views/TaskViewV2.vue`: 增加组织列定义、报表按钮
    - `frontend/src/components/KbFilterTable.vue`: 新增 `cascade-org` 筛选类型支持
    - `frontend/src/App.vue`: 添加 keep-alive 包裹
    - `frontend/src/router/index.js`: 新增用户报表路由
    - `frontend/src/views/TaskUserReport.vue`: 新建用户报表页面
