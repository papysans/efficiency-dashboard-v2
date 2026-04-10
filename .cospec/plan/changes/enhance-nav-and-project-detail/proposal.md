# 变更：优化导航栏分组与项目详情页交互

## 原因
导航栏当前为平铺菜单，缺乏分组结构；项目详情页添加 Task 时 silica 权重传参错误导致存储为 0，Task 搜索缺少 title 字段支持，添加 Repo 时时间范围无默认值。

## 变更内容
- 导航栏改为 2 个 el-sub-menu 分组：「组织看板」（组织、用户）和「项目看板」（项目、仓库、提交、任务）
- 修复添加 Task 时 silica 权重传参：前端由 `silica` 单值改为 `task_ids_silica` 数组，与后端接口对齐
- Task 搜索对话框支持 title 字段过滤，下拉选项同时展示 title
- 添加 Repo 对话框时间范围默认填入项目的起始/结束时间（结束时间为 null 时填入 "now" 字符串占位，并切换为文本输入模式）

## 影响
- **受影响的规范**：导航栏 UI、项目详情页操作
- **受影响的代码**：
    - `frontend/src/App.vue`：导航栏 el-menu-item 改为 el-sub-menu 分组结构
    - `frontend/src/views/ProjectDetailV2.vue`：
        - `submitTask()`：data 中 `silica` 改为 `task_ids_silica` 数组（每个 task_id 对应同一个 silica 值）
        - `searchTasks()`：过滤条件加入 `title` 字段
        - Task 下拉选项 label 和展示内容加入 `title`
        - `openAddRepoDialog()`：`date_range` 默认填入项目起始/结束时间；结束时间为 null 时用文本输入框填入 "now"
