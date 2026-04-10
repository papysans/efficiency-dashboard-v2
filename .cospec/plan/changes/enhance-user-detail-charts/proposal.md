# 变更：增强用户详情页（布局优化 + 跳转联动 + 多维图表）

## 原因
UserDetailV2 页面的筛选条件独占一行浪费空间；按天明细中的 Task数/Commit数 无法联动到对应列表页；图表仅有单一提效比折线图，缺乏代码量、耗时对比、费用等多维度展示。GroupView 存在相同的布局问题。

## 变更内容

### 1. 布局优化（UserDetailV2 + GroupView）
- 将独立的 `FilterBar` 第二行改为：标题行右侧直接嵌入 `DateRangePicker`（参考 RepoDetailV2 的做法），节省一行空间

### 2. 按天明细跳转联动（UserDetailV2）
- **Task数列**：数值改为 `el-link`，点击跳转 `/task-v2`，携带 query 参数：`startDate`、`endDate`（当天日期，`yyyyMMdd` 格式）、`userName`（用户名）
- **Commit数列**：同理跳转 `/commit-v2`，携带 `startDate`、`endDate`、`userName`
- **TaskViewV2.vue**：`onMounted` 读取 `route.query` 中的 `startDate/endDate/userName`，初始化 `serverDateRange` 并在数据加载后调用 `setFilter('user_name', userName)`
- **CommitViewV2.vue**：同上，日期字段为 `commit_time`

### 3. 图表重设计（UserDetailV2 + GroupView）
将现有单一「提效比趋势折线图」替换为 5 个独立图表（2列网格布局，最后一个图居中）：
- **图1 — Task数 & Commit数**：分组柱状图（两色并排），x轴日期，y轴数量
- **图2 — 代码行数**：分组柱状图，Task代码行数（蓝）+ Commit代码行数（绿）并排
- **图3 — 耗时对比**：分组柱状图，每天4根柱：Task传统耗时（浅蓝）/ Task实际耗时（深蓝）/ Commit传统耗时（浅绿）/ Commit实际耗时（深绿），视觉上传统>实际体现提效
- **图4 — 费用**：单色柱状图（橙色），x轴日期，y轴费用（元，4位小数）
- **图5 — 提效比趋势**：双折线图（保留现有逻辑），Task提效比（蓝）+ Commit提效比（绿），y轴单位 %

## 影响

- **受影响的规范**：用户详情、组织详情
- **受影响的代码**：
    - `frontend/src/views/UserDetailV2.vue`：布局改造（标题行内嵌 DateRangePicker）、Task数/Commit数列添加跳转、图表区域重写为5图
    - `frontend/src/views/GroupView.vue`：布局改造（标题行内嵌 DateRangePicker）、图表区域重写为5图（成员维度数据聚合方式相同）
    - `frontend/src/views/TaskViewV2.vue`：`onMounted` 读取 URL query 参数初始化日期和用户筛选
    - `frontend/src/views/CommitViewV2.vue`：同上，日期字段为 `commit_time`
