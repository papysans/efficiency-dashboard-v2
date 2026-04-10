## 实施

### 阶段一：后端 - 新增文件读取 API

- [x] 1.1 后端新增 `task_dir` 配置项
     【目标对象】`backend/main.go`、`backend/config.yaml`
     【修改目的】添加 task 数据目录路径配置，供文件读取 API 使用
     【修改方式】在 `Config` 结构体中新增字段，在 `loadConfig` 函数中设置默认值，在 `config.yaml` 中新增配置行
     【相关依赖】无
     【修改内容】
        - `backend/main.go`：在 `Config` 结构体（第25行）中新增 `TaskDir string \`yaml:"task_dir"\`` 字段，放在 `RawDataDir` 字段附近
        - `backend/main.go`：在 `loadConfig` 函数中 `cfg.RawDataDir = "../rawdata"` 之后新增 `cfg.TaskDir = "../task"` 默认值
        - `backend/config.yaml`：在 `rawdata_dir` 配置项后新增 `task_dir: "../task"`

- [x] 1.2 后端新增文件读取 API handler
     【目标对象】`backend/task_handler_v2.go`
     【修改目的】提供读取 task/summary 和 task/conversation 文件内容的 API，供前端链接打开原始数据
     【修改方式】在文件末尾新增 `getTaskFile` handler 函数
     【相关依赖】`backend/main.go` 中的 `appConfig.TaskDir` 配置项；需要导入 `os`、`path/filepath`
     【修改内容】
        - 新增 `getTaskFile(c *gin.Context)` 函数，接受 query 参数 `type`（summary/conversation）、`taskId`、`date`（YYYY-MM-DD 格式）
        - 参数校验：`type` 只允许 "summary" 或 "conversation"；`taskId` 和 `date` 不能为空；`date` 必须符合 YYYY-MM-DD 格式（用 `time.Parse("2006-01-02", date)` 校验）
        - 路径安全校验：`taskId` 和 `date` 中禁止包含 `..` 或 `/` 或 `\`，防止路径穿越
        - 从 `date` 中拆分出 YYYY、MM、DD 构建子路径
        - summary 类型：先查 `{TaskDir}/summary/YYYY/MM/DD/{taskID}.json`，若不存在则查 `{TaskDir}/analysed/YYYY/MM/DD/{taskID}.json`；返回 Content-Type `application/json`
        - conversation 类型：查 `{TaskDir}/conversation/YYYY/MM/DD/{taskID}.jsonl`；返回 Content-Type `text/plain; charset=utf-8`
        - 文件不存在时返回 404 + `{"error": "文件不存在"}`
        - 使用 `os.ReadFile` 读取文件内容，读取失败返回 500 + 错误信息
        - 遵循仓库既有的错误处理风格：`c.JSON(http.StatusXxx, gin.H{"error": ...})`

- [x] 1.3 后端注册文件读取路由
     【目标对象】`backend/main.go`
     【修改目的】将文件读取 API 注册到 v2 路由组
     【修改方式】在 v2 路由组中 `v2.GET("/tasks/:taskId", getTaskDetailV2)` 之前插入新路由
     【相关依赖】任务 1.2 中新增的 `getTaskFile` handler
     【修改内容】
        - 在 `v2.GET("/tasks", listTasksV2)`（第189行）之后、`v2.GET("/tasks/:taskId", getTaskDetailV2)`（第190行）之前，插入 `v2.GET("/tasks/file", getTaskFile)`
        - 注意：必须在 `/tasks/:taskId` 之前注册，否则 Gin 会将 "file" 匹配为 `:taskId` 参数

### 阶段二：前端 - 新增 API 函数

- [x] 2.1 前端新增文件读取 API 函数
     【目标对象】`frontend/src/api/es.js`
     【修改目的】封装后端文件读取接口供 TaskDetailV2.vue 调用
     【修改方式】在文件末尾新增 `getTaskFileV2` 导出函数
     【相关依赖】`frontend/src/api/index.js` 中的 `request` 实例
     【修改内容】
        - 新增 `export function getTaskFileV2(params)` 函数，调用 `request({ url: '/v2/tasks/file', method: 'get', params })`
        - params 包含 `type`、`taskId`、`date` 三个字段
        - 函数命名遵循仓库已有风格（如 `getTaskDetailV2`、`getRepoDetailV2New` 等 V2 后缀命名）

### 阶段三：前端 - 页面布局重构

- [x] 3.1 前端删除提效摘要大卡片
     【目标对象】`frontend/src/views/TaskDetailV2.vue`
     【修改目的】去掉 3 张 el-row/el-col 提效摘要大卡片，核心度量全部整合到度量信息区
     【修改方式】删除模板中 `<!-- 提效统计摘要 -->` 注释到 `</el-row>` 之间的代码（第83-155行）
     【相关依赖】无（`efficiencyColor` computed 属性保留，在度量信息区复用）
     【修改内容】
        - 删除 `<el-row :gutter="12">` 及其包含的 3 个 `<el-col>` 和内部的 `el-card`（第84-155行的全部代码）
        - 保留 `efficiencyColor` computed 属性，后续在度量信息区使用
        - 保留 `kb-metric-card`、`kb-metric-label`、`kb-metric-value` 样式定义（如有），后续不再需要可在最后清理

- [x] 3.2 前端重构基础信息区
     【目标对象】`frontend/src/views/TaskDetailV2.vue`
     【修改目的】将原有 el-descriptions 拆分为"基础信息"区域，只包含基础字段，合并系统和客户端字段
     【修改方式】修改 `<!-- Task 元信息卡片 -->` 注释下的 `el-card` 和 `el-descriptions` 内容（第12-81行）
     【相关依赖】`repoDisplay` computed 属性，`formatLocalTime` 函数
     【修改内容】
        - el-card 添加 header="基础信息"
        - el-descriptions 保留 9 个字段：
          (1) Task ID：`task.task_id`
          (2) 用户：`el-link`，点击跳转 `/user/{user_id}`，显示 `task.user_name || task.user_id`（复用现有逻辑）
          (3) 仓库：`el-link`，点击跳转 `/repo/{repo_id}`，显示 `repoDisplay`（复用现有逻辑）
          (4) 工作目录：`el-link`，点击跳转 `/workdir/{work_dir_id}`（复用现有逻辑）
          (5) 开始时间：`formatLocalTime(task.start_time)`
          (6) 结束时间：`formatLocalTime(task.end_time)`
          (7) 系统：合并 `client_os` + 空格 + `client_os_version`（如 "Windows 10.0"），复用现有第34行的合并逻辑
          (8) 客户端：合并 `client_ide` + 空格 + `client_version`（如 "vscode 2.5.3"），原来是两个独立字段（IDE 和版本），现在合并
          (9) 模式：`task.caller`
        - 删除原来在此区域的费用、Diff行数、古法预估、实际耗时、提效比、总请求数、总Tokens、总费用字段（这些移到度量信息区）

- [x] 3.3 前端新增度量信息区
     【目标对象】`frontend/src/views/TaskDetailV2.vue`
     【修改目的】新增独立的"度量信息"卡片区域，集中展示所有度量字段
     【修改方式】在基础信息 el-card 之后、对话历史 el-card 之前，新增第二个 `el-card` + `el-descriptions`
     【相关依赖】`efficiencyColor` computed、`fmtCostVal` 函数、`formatDuration` 函数、`totalTokens` computed、后端返回的 `task` 和 `conversations` 数据、任务 3.5 中的 `getTaskFileUrl` 函数
     【修改内容】
        - 新增 `<el-card shadow="never" header="度量信息">`，包含 `<el-descriptions :column="3" border>`
        - 7 个字段：
          (1) **生成代码量**（label="生成代码量"）：显示 `task.diff_lines` + " 行"；值旁边附带 `el-link`（文字"查看详情"，`target="_blank"`，href 为 `getTaskFileUrl('summary')`），点击在新标签页打开 summary JSON
          (2) **实际耗时**（label="实际耗时"）：复用现有模板中的 manual 优先展示逻辑——若 `task.task_real_minutes_manual != null` 则优先展示 manual 值 + `?` 图标 tooltip 展示 `task_real_minutes_reason_manual`，自动预估版本用删除线标注；否则展示 `formatDuration(task.task_real_minutes)` + `?` 图标 tooltip 展示 `task_real_minutes_reason`（即复用原第56-75行的模板逻辑）
          (3) **传统编程耗时**（label="传统编程耗时"）：展示方式同实际耗时，使用 `task_ancient_minutes` 系列字段（即复用原第36-55行的模板逻辑）
          (4) **API 请求次数**（label="API请求次数"）：显示 `conversations.length`，无数据时显示 "-"
          (5) **总 Tokens**（label="总Tokens"）：显示 `totalTokens.toLocaleString()`；外包 `el-tooltip`，tooltip 内容为 "上行: {upstream总和} / 下行: {downstream总和}"；需要新增 `totalUpstreamTokens` 和 `totalDownstreamTokens` 两个 computed（分别对 conversations 求和）
          (6) **费用**（label="费用"）：显示 `fmtCostVal(task.cost)` + " 元"；复用现有的 `fmtCostVal` 函数和 `totalCostSum` 回退逻辑
          (7) **提效比例**（label="提效比例"）：显示 `Math.round(task.efficiency_ratio) + '%'`，使用 `efficiencyColor` computed 控制字体颜色，字体加大加粗（`font-size: 20px; font-weight: bold`）；颜色规则：≥300% 绿色 `#67C23A`，≥150% 蓝色 `#409EFF`，其他灰色 `#909399`（已由 `efficiencyColor` computed 实现）

- [x] 3.4 前端重构对话历史区
     【目标对象】`frontend/src/views/TaskDetailV2.vue`
     【修改目的】重构对话历史区域：去掉 response_content，重新组织每个对话节点信息，增加文件链接
     【修改方式】修改 `<!-- 对话历史 -->` 注释下的 `el-card` 内容（第157-202行）
     【相关依赖】`timelineItems` computed、`formatLocalTime` 函数、`fmtCostVal` 函数、任务 3.5 中的 `getTaskFileUrl` 函数
     【修改内容】
        - 对话历史 el-card 的 header 插槽中，在"对话历史"文字旁边新增 `el-link`（文字"查看原始数据"，`target="_blank"`，href 为 `getTaskFileUrl('conversation')`），方便直接查看整体 conversation 文件
        - 每个对话节点（`el-timeline-item` 中 `item.type !== 'gap'` 的部分）重新组织内容为 4 行：
          a) 时间行：`formatLocalTime(item.conv.start_time)` ~ `formatLocalTime(item.conv.end_time)` | 耗时 `item.conv.process_time` ms | TTFT `item.conv.process_ttft` ms
          b) 模式行：`item.conv.prompt_mode` | `item.conv.mode` | `item.conv.model`；若 `item.conv.error_code` 存在，追加红色 `el-tag`（type="danger"）显示 `error_code: error_reason`
          c) 指标行：上行 `item.conv.upstream_tokens` | 下行 `item.conv.downstream_tokens` | 费用 `fmtCostVal(item.conv.cost)` | 代码 `item.conv.diff_lines` 行
          d) 输入行：`item.conv.user_input`，保留现有的展开/收起逻辑（复用 `getDisplayText`、`toggleExpand`、`isExpanded` 函数）
        - 删除 response_content 展示部分（原第184-191行的 `<div v-if="item.conv.response_content">` 整块）
        - 删除原有的头部 sender tag 和底部标签区域（原第171-175行、第193-197行），用新的 4 行替代

- [x] 3.5 前端新增文件链接辅助函数和 computed
     【目标对象】`frontend/src/views/TaskDetailV2.vue`
     【修改目的】构建 summary 和 conversation 文件的 API URL，以及 Tokens 分项 computed
     【修改方式】在 `<script setup>` 中新增函数和 computed
     【相关依赖】`task` ref、`conversations` ref、`frontend/src/api/index.js` 的 baseURL（`/api`）
     【修改内容】
        - 新增 `getTaskFileUrl(type)` 函数：从 `task.value.start_time` 中提取日期部分（`new Date(task.value.start_time)` 得到 YYYY-MM-DD），拼接 `/api/v2/tasks/file?type=${type}&taskId=${task.value.task_id}&date=${date}` 返回完整 URL；用于 el-link 的 href（`target="_blank"`）
        - 注意边界：若 `task.value.start_time` 为空则返回空字符串
        - 新增 `totalUpstreamTokens` computed：`conversations.value.reduce((s, c) => s + (c.upstream_tokens || 0), 0)`
        - 新增 `totalDownstreamTokens` computed：`conversations.value.reduce((s, c) => s + (c.downstream_tokens || 0), 0)`
