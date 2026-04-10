# 变更：实现指标看板的 Dashboard 和工程面板（MVP）

## 原因
kanban项目已有 kbcli 将 rawdata 写入 ES，但缺少后端服务和前端页面。需要搭建完整的 Web 应用，实现原始数据查看（Dashboard）和工程面板（Panel）两个核心页面，让用户能够可视化查看 AI Coding 的各项指标数据。

## 变更内容

### 1. 后端服务搭建（Go + Gin，端口 9990）
- 参考 comdigger 的 `backend/main.go` 搭建 Go(Gin) 后端框架
- **数据源为 Elasticsearch**（非 PostgreSQL），复用 kbcli 中已有的 ES 客户端逻辑
- 提供 ES 查询 API：
  - `GET /api/es/indices` — 获取可用的 ES 索引列表（costrict_chat_raw_* 和 costrict_chat_stat_*）
  - `GET /api/es/raw-data` — 查询 raw 层数据（支持日期范围、分页、排序）
  - `GET /api/es/stat-data` — 查询 stat 层数据（支持日期范围、维度过滤 project/user）
  - `GET /api/es/stat-summary` — 聚合统计（按 project_id 汇总各项指标，用于图表展示）

### 2. 前端应用搭建（Vue3 + Element Plus + ECharts，端口 8880）
- 参考 comdigger 的前端架构，创建 Vue3 + Vite 项目
- 复用 comdigger 的组件模式（PageLayout、表格、图表等），适配 ES 数据源

### 3. Dashboard 页面（原始数据查看）
- 参考 comdigger 的 Dashboard 页面设计
- **筛选条件**：日期范围（起止日期）、索引类型（raw/stat 切换 Tab）
- **数据展示**：ES 查询结果以表格形式展示，支持分页和排序
- raw 表展示字段：timestamp、username、project_id、model、user_in_chars、assistant_out_code_lines、api_cost、api_process_time 等
- stat 表展示字段：project_id、project_aic_user_in_chars、project_aic_assistant_out_code_lines、project_aic_api_count、project_aic_api_cost 等

### 4. 工程面板页面（Project 维度图表）
- 参考 comdigger 的 Panel 页面设计
- **筛选条件**：日期范围
- **图表展示**（按 project_id 维度）：
  - 代码行数柱状图（assistant_out_code_lines）
  - API 费用柱状图（api_cost）
  - Token 使用量柱状图（api_in_tokens + api_out_tokens）
  - API 调用次数柱状图（api_count）
  - 处理时长柱状图（process_time）
- 使用 ECharts 渲染，el-row/el-col 网格布局

## 影响
- **受影响的规范**：新增后端服务和前端应用
- **受影响的代码**：
  - `backend/main.go`: 新建，Go(Gin) 后端入口，参考 comdigger 的 backend/main.go
  - `backend/es_handler.go`: 新建，ES 查询 handler，提供 raw/stat 数据查询 API
  - `backend/es_client.go`: 新建，从 kbcli/es_client.go 提取 ES 客户端为可复用包
  - `backend/config.yaml`: 新建，后端配置（端口 9990、ES 连接、CORS）
  - `backend/go.mod`: 新建，Go 模块定义
  - `frontend/`: 新建 Vue3 前端项目
  - `frontend/src/views/Dashboard.vue`: 新建，原始数据查看页面
  - `frontend/src/views/ProjectPanel.vue`: 新建，工程面板页面
  - `frontend/src/views/Home.vue`: 新建，首页/导航
  - `frontend/src/components/RawDataTable.vue`: 新建，raw 数据表格组件
  - `frontend/src/components/StatDataTable.vue`: 新建，stat 数据表格组件
  - `frontend/src/router/index.js`: 新建，路由配置
  - `frontend/src/api/es.js`: 新建，ES 数据 API 调用
  - `frontend/vite.config.js`: 新建，Vite 配置（端口 8880，代理到 9990）
