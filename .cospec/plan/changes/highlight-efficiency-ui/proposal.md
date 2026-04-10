# 变更：全站提效信息突出展示

## 原因
项目核心目标是展示 AI Coding 的提效成果，但当前 UI 中提效信息（古法预估、实际耗时、提效比、节省时间）不醒目，淹没在非核心指标（Tokens、请求数）中。需要全站改进，让提效数据成为视觉焦点。

## 变更内容

### 1. 首页仪表盘 (Home.vue)
- 第二行指标：用"总实际耗时"替换"总Token数"，用"总节省时间"替换"总代码行数"
- 新增第三行横幅：**平均提效比**大字展示（绿色醒目卡片）
- 快速导航新增"提交视图"卡片
- 后端 dashboard API 需返回：total_real_minutes、avg_efficiency_ratio

### 2. Task 列表页 (TaskViewV2.vue)
- 表格新增"实际耗时"+"提效比"列（提效比用颜色徽章：≥300%绿、≥150%蓝、<150%灰）
- 图表改为："古法预估 vs 实际耗时"对比图 + "提效比分布"图

### 3. Task 详情页 (TaskDetailV2.vue)
- 统计摘要 3 卡片从"总请求数/总Tokens/总费用"改为**"古法预估/实际耗时/提效比"**
- 提效比卡片颜色动态：≥300%绿、≥150%蓝、<150%灰

### 4. Commit 列表页 (CommitViewV2.vue)
- 表格新增"提效比"列（颜色徽章同 Task 列表）
- 顶部新增 4 个汇总指标卡片：总Commit数 | 总古法预估 | 总实际耗时 | 平均提效比
- 图表改为："古法预估 vs 实际耗时"对比图 + "提效比分布"图

### 5. Commit 详情页 (CommitDetailV2.vue)
- 元信息下方新增 3 个提效指标卡片（古法预估/实际耗时/提效比），替代"暂无关联Task"的空白区域

### 6. 后端 API 支持
- dashboard API 返回 total_real_minutes、avg_efficiency_ratio
- task 列表 API 已返回 task_real_minutes，但后端需要在列表 API 中也计算 efficiency_ratio

## 影响
- **受影响的代码**：
  - `frontend/src/views/Home.vue`
  - `frontend/src/views/TaskViewV2.vue`
  - `frontend/src/views/TaskDetailV2.vue`
  - `frontend/src/views/CommitViewV2.vue`
  - `frontend/src/views/CommitDetailV2.vue`
  - `backend/dashboard_handler_v2.go`
