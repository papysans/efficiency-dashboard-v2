## 实施

- [x] 1.1 导航栏改为 2 个分组菜单
     【目标对象】`frontend/src/App.vue`
     【修改目的】将平铺的 6 个 el-menu-item 改为 2 个 el-sub-menu 分组，提升导航结构清晰度
     【修改方式】修改第 16-21 行的 6 个 el-menu-item（不含第 15 行「首页」），替换为 2 个 el-sub-menu
     【相关依赖】无
     【修改内容】
        - 保留第 15 行「首页」el-menu-item（index="/"）不变
        - 删除第 16-21 行的 6 个 el-menu-item（仓库、用户、组织、任务、提交、项目）
        - 新增 el-sub-menu「组织看板」（需设置唯一 index，如 index="group-org"），使用 `<template #title>` 插槽设置标题文本，包含子菜单：
          - 组织（index="/org-v2"）
          - 用户（index="/user-v2"）
        - 新增 el-sub-menu「项目看板」（需设置唯一 index，如 index="group-project"），使用 `<template #title>` 插槽设置标题文本，包含子菜单：
          - 项目（index="/project-v2"）
          - 仓库（index="/repo-v2"）
          - 提交（index="/commit-v2"）
          - 任务（index="/task-v2"）
        - 每个子菜单项用 el-menu-item 包裹，保持 index 路由值与原 el-menu-item 一致

- [x] 1.2 修复添加 Task 时 silica 权重传参错误
     【目标对象】`frontend/src/views/ProjectDetailV2.vue`
     【修改目的】前端发送 `task_ids_silica` 数组而非 `silica` 单值，与后端接口对齐，解决 silica 权重存储为 0 的问题
     【修改方式】修改 `submitTask()` 函数（第 717 行）中第 725-728 行的 data 构造逻辑
     【相关依赖】后端 `project_handler_v2.go` 接收 `task_ids_silica []float64`（数组长度与 task_ids 一一对应）
     【修改内容】
        - 在 data 对象中，将原有的 `silica` 单值字段改为 `task_ids_silica` 数组字段
        - `task_ids_silica` 数组的长度与 `task_ids` 数组相同，每个元素均为 `taskForm.value.silica` 的值（即所有选中的 task 共用同一个 silica 权重）
        - 删除原 `silica` 字段，不再发送单值

- [x] 1.3 Task 搜索支持 title 字段过滤并在下拉选项中展示
     【目标对象】`frontend/src/views/ProjectDetailV2.vue`
     【修改目的】搜索 task 时支持按 title 字段过滤，选项列表同时展示 title 信息，方便用户识别
     【修改方式】修改 `searchTasks()` 函数（第 683 行）的过滤条件，以及第 367-377 行的 Task 下拉选项模板
     【相关依赖】无
     【修改内容】
        - `searchTasks()` 的过滤条件中，在现有的 task_id、user_name、work_dir 三个字段之后，追加对 `title` 字段的 toLowerCase + includes 过滤（与其他字段用 `||` 连接，空值处理方式与其他字段保持一致）
        - el-option 的 `:label` 属性改为包含 task_id 前 8 位和 title 的组合字符串（格式参考现有 user_name | work_dir 风格，如 `task_id前8位 | title`）
        - el-option 内部展示 div 中，在现有的 task_id 截断显示和 `user_name | work_dir` 之间或之后，加入 title 字段的展示（title 可能为空，需做空值兜底显示，如 title 为空则不展示或展示占位符）

- [x] 1.4 添加 Repo 对话框时间范围默认填入项目时间
     【目标对象】`frontend/src/views/ProjectDetailV2.vue`
     【修改目的】打开添加 Repo 对话框时，时间范围自动填入项目的起始/结束时间，减少手动输入；结束时间为 null 时用 "now" 文本占位展示，并切换为文本输入模式
     【修改方式】修改 `openAddRepoDialog()` 函数（第 606 行）的表单初始化逻辑，修改第 333-335 行时间范围 el-form-item 的 UI，修改 `submitRepo()` 函数（第 634 行）的 end_time 取值逻辑
     【相关依赖】`project.value.start_time_manual`、`project.value.start_time`、`project.value.end_time_manual`、`project.value.end_time`
     【修改内容】
        - 在 `repoForm` 的响应式对象中增加 `end_time_is_now` 布尔字段（默认 false），用于标记结束时间是否使用 "now" 占位
        - `openAddRepoDialog()` 中，重置 repoForm 时计算并填入默认时间：
          - 项目有效起始时间：优先取 `project.value.start_time_manual`，否则取 `project.value.start_time`
          - 项目有效结束时间：优先取 `project.value.end_time_manual`，否则取 `project.value.end_time`
          - 若有效结束时间为 null：设 `end_time_is_now = true`，`date_range` 仅包含开始时间（单元素数组）
          - 若有效结束时间有值：设 `end_time_is_now = false`，`date_range = [startTime, endTime]`
        - 时间范围 el-form-item 的 UI 根据 `end_time_is_now` 切换：
          - `end_time_is_now = true` 时：展示 el-date-picker（type="date"，绑定 `repoForm.date_range[0]`）+ 固定文本 "至 now"（文本展示，不可编辑）
          - `end_time_is_now = false` 时：保持原有 el-date-picker（type="daterange"，绑定 `repoForm.date_range`）不变
        - `submitRepo()` 中 end_time 取值逻辑：
          - 若 `repoForm.end_time_is_now = true`，end_time 传 null（后端不限制结束时间，"now" 仅为前端展示占位）
          - 否则取 `repoForm.date_range[1]`（与现有逻辑保持一致）
