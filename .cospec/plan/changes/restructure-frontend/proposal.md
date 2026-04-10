# 变更：前端全面重构（Dashboard + 提效分析 + 各维度 Panel + 纠错 UI）

## 原因
当前前端只有 3 个页面（Home、Dashboard、ProjectPanel），且 API 路径仍是旧的 `/api/es/*` 前缀（后端已改为 `/api/*`），后端 18 个路由中前端只使用了 4 个。需要全面重构前端以适配新后端 API，实现提效分析面板、多维度 Panel 和纠错功能 UI。

## 变更内容
- **API 层重写**：适配后端新路由，覆盖所有 18 个 API
- **Dashboard 重构**：改为 Request 数据 + Task 聚合数据双视图
- **提效分析面板**（核心）：指标卡片、用户参与表、成本分析
- **Project Panel 重构**：项目列表 + 提效详情
- **Repo Panel**：仓库维度 + Git 分析展示
- **User Panel**：用户活动和个人提效
- **Org Panel**：组织层级导航和提效对比
- **纠错功能 UI**：修改 AI 预估人天 + 审计历史
- **路由扩展**：新增 6+ 个页面路由
- **导航重构**：顶部导航适配新页面

## 影响
- **受影响的代码**：
    - `frontend/src/api/es.js`: **重写**为覆盖全部 API
    - `frontend/src/router/index.js`: 新增路由
    - `frontend/src/views/Dashboard.vue`: 重构适配新 API
    - `frontend/src/views/ProjectPanel.vue`: 重构为项目列表+提效分析
    - `frontend/src/views/EfficiencyPanel.vue`: **新增**，提效分析面板
    - `frontend/src/views/RepoPanel.vue`: **新增**，仓库维度面板
    - `frontend/src/views/UserPanel.vue`: **新增**，用户维度面板
    - `frontend/src/views/OrgPanel.vue`: **新增**，组织层级面板
    - `frontend/src/App.vue`: 导航菜单扩展
    - `frontend/src/views/Home.vue`: 首页卡片扩展
