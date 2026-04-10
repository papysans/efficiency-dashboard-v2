# 变更：重构 Task 详情页面信息分类布局

## 原因
当前 Task 详情页面所有字段平铺在一个 el-descriptions 中（18 项），加上 3 张提效摘要大卡片，信息层次不清晰。需要按"基础信息 → 度量信息 → 对话历史"三个区域归类，突出关键度量指标，并新增文件链接功能方便查看原始数据。

## 变更内容

### 1. 页面布局重构（前端）

**去掉现有的 3 张提效摘要大卡片**（el-row + el-col × 3），将核心度量全部整合到度量信息区，但提效比需要突出展示。

**三区域划分**：

#### 区域一：基础信息（el-card + el-descriptions）
- Task ID、用户（el-link）、仓库（el-link）、工作目录（el-link）
- 开始时间、结束时间
- 系统 = `client_os` + `client_os_version`（合并为一个字段，如 "Windows 10.0"）
- 客户端 = `client_ide` + `client_version`（合并为一个字段，如 "vscode 2.5.3"）
- 模式 = `caller`

#### 区域二：度量信息（el-card + el-descriptions）
- **生成代码量**（原 Diff 行数改名，单位"行"）：值旁边附带 el-link，点击在新标签页打开对应的 task/summary JSON 文件
- **实际耗时**（task_real_minutes）：加 `?` 图标 tooltip 展示 reason；若存在 `_manual` 版本则优先展示 manual 值，自动预估版本用删除线标注
- **传统编程耗时**（task_ancient_minutes，原"古法预估"）：展示方式同实际耗时
- **API 请求次数**：conversations.length
- **总 Tokens**：展示 `upstream_tokens + downstream_tokens` 的和，tooltip 分别展示 "上行: xxx / 下行: xxx"
- **费用**：备注单位"元"
- **提效比例**（efficiency_ratio）：使用颜色高亮突出展示（≥300% 绿色，≥150% 蓝色，其他灰色）

#### 区域三：对话历史（el-card + 列表/时间线）
每个 request 展示以下信息，去掉 response_content：
- a) `start_time` `end_time` `process_time`(ms) `process_ttft`(ms)
- b) `prompt_mode` `mode` `model`；如有错误显示 `error_code` + `error_reason`（红色标注）
- c) `upstream_tokens` `downstream_tokens` `cost` `diff_lines`
- d) `user_input`（保留展开/收起）
- e) 链接，指向 `task/conversation/YYYY/MM/DD/{taskID}.jsonl` 文件，新标签页打开

对话历史顶部展示一个整体 conversation 文件链接。

### 2. 后端新增文件读取 API

新增 `GET /api/v2/tasks/file` 接口，接受 `type`（summary/conversation）和 `taskId` + 日期参数，从 `task/` 目录读取对应文件并返回内容。

- summary：`task/summary/YYYY/MM/DD/{taskID}.json`（已被分析的在 `task/analysed/` 下）
- conversation：`task/conversation/YYYY/MM/DD/{taskID}.jsonl`

后端 config 新增 `task_dir` 配置项（默认 `"../task"`）。

## 影响
- **受影响的代码**：
    - `frontend/src/views/TaskDetailV2.vue`: 重构整个页面模板和部分 computed 逻辑
    - `backend/task_handler_v2.go`: 新增文件读取 handler
    - `backend/main.go`: 新增路由 + config 字段
    - `backend/config.yaml`: 新增 `task_dir` 配置
