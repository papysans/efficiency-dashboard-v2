# 变更：Task 详情页增强 —— 时间本地化、字段修正、实际耗时计算与提效比展示

## 原因
Task 详情页存在多个展示问题（时间为原始 RFC3339 格式、仓库/项目字段混淆、前后端字段名不匹配），且缺少关键业务指标（实际编码耗时、古法预估时间、提效比例），需要全面增强以支撑效能分析场景。

## 变更内容

### 1. 时间本地化
- 所有展示给用户的时间统一转换为浏览器本地时间格式（`YYYY-MM-DD HH:mm:ss`）
- 涉及 Task 元信息（start_time, end_time）和对话历史的 timestamp

### 2. 仓库/Project 字段分离展示
- 当前"仓库"字段实际显示的是 project_id 格式的数据（`clientID[:10]:projectPath`）
- 修正为：「仓库(Repo)」展示 repo_addr + repo_branch（为空时显示 repo_id），可跳转 repo 页面
- 新增：「Project」展示 project_id，可跳转 project 页面
- 数据导入层修正：从 rawdata JSON 中提取 git repo 信息生成正确的 repo_id 和 repo_addr

### 3. task_real_minutes —— 任务实际耗时（分钟）
- 算法：对 conversation 的 start_time 排序，相邻间隔≤30分钟（可配置）视为连续编码，超过则断开（加5分钟延长值，可配置），累加得到实际耗时
- 全局配置：`task_real_minutes.gap_threshold_minutes`（默认30）、`task_real_minutes.extension_minutes`（默认5）
- 存储：task_real_minutes、task_real_minutes_reason 存入 DB，支持 _manual 后缀人工修正
- 计算策略：API 获取详情时实时计算 + 写入 DB 缓存
- 显示：根据大小自适应（分钟/小时分钟/人天）

### 4. task_ancient_minutes —— AI 预估古法编程时间（分钟）
- 将现有 `ai_estimated_ancient_days`（人天）字段**重命名**为 `task_ancient_minutes`（分钟）
- 已有数据清零（后续重新通过 AI 生成）
- 同样支持 _manual 后缀人工修正字段

### 5. efficiency_ratio —— 提效比
- 计算公式：`(task_ancient_minutes / task_real_minutes) * 100`
- 优先使用 _manual 值（如果有）
- 前端展示为百分比格式

### 6. 对话历史时间片段可视化
- el-timeline-item 用不同颜色区分：连续时间片内为绿色节点，断开后的第一个对话为橙色节点
- 断开处显示「间隔 XX 分钟（不计入）」的提示标记

### 7. 人工修正入口
- Task 详情页添加「编辑」按钮，弹出对话框编辑 task_real_minutes_manual、task_ancient_minutes_manual 及其 reason

### 8. 修复现有前后端字段名不匹配 Bug
- `task.total_cost` → `task.cost`
- `task.ai_estimated_days` → 新的 `task_ancient_minutes` 字段
- `conv.model_name` → `conv.model`
- `conv.total_tokens` → 计算 `upstream_tokens + downstream_tokens`

### 9. 输出设计文档v2.md
- 根据本次变更更新 task_summary.json 数据结构定义

## 影响
- **受影响的代码**：
  - `frontend/src/views/TaskDetailV2.vue`：时间格式化、字段名修正、仓库/Project 分离、耗时指标展示、时间片段可视化、人工修正对话框
  - `backend/task_handler_v2.go`：task_real_minutes 实时计算逻辑、manual 更新 API
  - `backend/db.go`：CostrictTask struct 字段重命名、新增字段、SQL 更新（UpsertCostrictTask/GetCostrictTask/scanCostrictTask 等）
  - `backend/config.yaml` + `backend/main.go`：新增 task_real_minutes 配置项
  - `init_db.sql`：ALTER TABLE 添加新字段、重命名旧字段
  - `kbcli/raw_parser.go`：修正 repo_id 的生成逻辑，从 rawdata 中提取 git repo 信息
  - `kbcli/pg_writer.go`：字段映射更新
  - `设计文档v2.md`：更新 task_summary.json 定义
