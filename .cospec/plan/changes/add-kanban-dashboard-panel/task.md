## 实施

- [x] 1.1 搭建后端 Go(Gin) 项目框架
     【目标对象】`backend/`
     【修改目的】创建后端服务基础结构
     【修改方式】参考 `D:\My\PubCode\comdigger\backend\main.go` 的结构，但数据源改为 Elasticsearch
     【相关依赖】`kbcli/es_client.go` 的 ES 客户端逻辑
     【修改内容】
        - 创建 `backend/go.mod`，模块名 `kanban/backend`，依赖 gin、go-elasticsearch/v8、gopkg.in/yaml.v3
        - 创建 `backend/config.yaml`，配置 server.port=9990、elasticsearch 连接信息（复用根目录 config.yaml 的 ES 配置）、cors.allow_origins=["http://localhost:8880"]
        - 创建 `backend/main.go`，参考 comdigger 的 main.go 结构：加载配置 → 初始化 ES 客户端 → 创建 Gin 路由 → 注册 API → 启动服务
        - 创建 `backend/es_client.go`，从 `kbcli/es_client.go` 复制并改造为 backend 包可用（CreateIndexIfNotExists 不需要，只需 Search 方法）
        - 新增 `Search(indexName string, query map[string]interface{}, from, size int)` 方法，封装 ES _search API，返回 hits 和 total
        - 新增 `SearchIndices(pattern string)` 方法，获取匹配 pattern 的索引列表
        - 新增 `Aggregate(indexName string, aggsQuery map[string]interface{})` 方法，封装 ES 聚合查询

- [x] 1.2 实现后端 ES 查询 API
     【目标对象】`backend/es_handler.go`
     【修改目的】提供前端所需的 ES 数据查询接口
     【修改方式】新建 handler 文件，注册路由到 main.go
     【相关依赖】`backend/es_client.go`、`kbcli/es_mappings.go`（了解字段结构）
     【修改内容】
        - `GET /api/es/indices` — 查询 ES 中 costrict_chat_raw_* 和 costrict_chat_stat_* 索引列表，返回 `[{name, docCount, indexType(raw/stat)}]`
        - `GET /api/es/raw-data` — 查询 raw 层数据，参数：startDate(YYYYMMDD)、endDate(YYYYMMDD)、page(默认1)、pageSize(默认50)、sortField(默认@timestamp)、sortOrder(默认desc)
          - 根据 startDate/endDate 计算涉及的索引名列表（costrict_chat_raw_20260330 到 costrict_chat_raw_20260331）
          - 构建 ES bool query，支持多索引查询
          - 返回 `{total, page, pageSize, hits: [{_source字段}]}` 
        - `GET /api/es/stat-data` — 查询 stat 层数据，参数同 raw-data，额外支持 dimension(project/user) 过滤
          - dimension=project 时只返回有 project_id 字段的文档
          - dimension=user 时只返回有 user_uuid 字段的文档
          - 返回格式同 raw-data
        - `GET /api/es/stat-summary` — 聚合统计，参数：startDate、endDate
          - 对 stat 索引按 project_id 做 terms aggregation
          - 聚合 sum(project_aic_user_in_chars)、sum(project_aic_assistant_out_code_lines)、sum(project_aic_api_count)、sum(project_aic_api_cost)、sum(project_aic_api_in_tokens)、sum(project_aic_api_out_tokens)、sum(project_aic_process_time)
          - 返回 `[{project_id, metrics: {user_in_chars, code_lines, api_count, api_cost, ...}}]`
        - 在 `backend/main.go` 的 api group 中注册以上 4 个路由

- [x] 1.3 搭建前端 Vue3 项目框架
     【目标对象】`frontend/`
     【修改目的】创建前端应用基础结构
     【修改方式】参考 `D:\My\PubCode\comdigger\frontend` 的结构，但简化（只保留看板需要的部分）
     【相关依赖】comdigger 前端的组件模式
     【修改内容】
        - 创建 `frontend/package.json`，依赖：vue3、vue-router4、element-plus、@element-plus/icons-vue、axios、echarts、vite、@vitejs/plugin-vue
        - 创建 `frontend/vite.config.js`，端口 8880，proxy /api 到 http://localhost:9990
        - 创建 `frontend/index.html`
        - 创建 `frontend/src/main.js`，挂载 Vue app、Element Plus、Router
        - 创建 `frontend/src/App.vue`，简单布局（顶部导航栏 + router-view）
        - 创建 `frontend/src/style.css`，基础样式
        - 创建 `frontend/src/router/index.js`，路由：`/` → Home、`/dashboard` → Dashboard、`/project-panel` → ProjectPanel
        - 创建 `frontend/src/api/index.js`，axios 实例（baseURL=/api）
        - 创建 `frontend/src/api/es.js`，封装 4 个 API 调用函数：getIndices、getRawData、getStatData、getStatSummary

- [x] 1.4 实现 Dashboard 页面（原始数据查看）
     【目标对象】`frontend/src/views/Dashboard.vue`
     【修改目的】展示 ES 中 raw 和 stat 索引的原始数据
     【修改方式】参考 `D:\My\PubCode\comdigger\frontend\src\views\Dashboard.vue` 的页面布局和筛选模式
     【相关依赖】`frontend/src/api/es.js`
     【修改内容】
        - 页面顶部筛选区（参考 comdigger 的 PageLayout 模式）：
          - 日期范围选择器（el-date-picker type=daterange）
          - 索引类型切换（el-tabs：raw / stat 两个 tab）
          - stat tab 下显示维度切换（project / user）
        - 数据表格区（el-table）：
          - raw 模式：展示 @timestamp、username、project_id、model、sender、user_in_chars、assistant_out_code_lines、api_process_time、api_ttft、api_in_tokens、api_out_tokens、api_cost 等列
          - stat/project 模式：展示 project_id、project_aic_user_in_chars、project_aic_assistant_out_code_lines、project_aic_api_count、project_aic_api_cost、project_aic_process_time、project_aic_lead_time 等列
          - stat/user 模式：展示 user_uuid、user_aic_user_in_chars、user_aic_assistant_out_code_lines、user_aic_api_count、user_aic_api_cost 等列
        - 分页组件（el-pagination），支持切换每页条数和翻页
        - 筛选条件变化时自动重新查询（防抖 300ms）
        - URL 参数同步（startDate、endDate、tab、dimension）

- [x] 1.5 实现工程面板页面（Project 维度图表）
     【目标对象】`frontend/src/views/ProjectPanel.vue`
     【修改目的】以图表形式展示 project 维度的聚合统计数据
     【修改方式】参考 `D:\My\PubCode\comdigger\frontend\src\views\PanelView.vue` 的 el-row/el-col 网格布局 + ChartCard 模式
     【相关依赖】`frontend/src/api/es.js`、ECharts
     【修改内容】
        - 页面顶部筛选区：
          - 日期范围选择器（el-date-picker type=daterange）
        - 图表区域（el-row + el-col，24栅格）：
          - 每个图表卡片使用 el-card 包裹，包含标题和 ECharts 图表
          - 图表1（colSpan=12）：代码行数柱状图 — 按 project_id 展示 assistant_out_code_lines，横向柱状图，按值降序
          - 图表2（colSpan=12）：API 费用柱状图 — 按 project_id 展示 api_cost，横向柱状图
          - 图表3（colSpan=12）：Token 使用量柱状图 — 按 project_id 展示 api_in_tokens 和 api_out_tokens（双柱对比）
          - 图表4（colSpan=12）：API 调用次数柱状图 — 按 project_id 展示 api_count
          - 图表5（colSpan=24）：处理时长柱状图 — 按 project_id 展示 process_time（毫秒转分钟显示）
        - 使用 ECharts 的 bar chart（横向），数据从 getStatSummary API 获取
        - 日期范围变化时自动重新查询
        - URL 参数同步（startDate、endDate）

- [x] 1.6 实现首页导航
     【目标对象】`frontend/src/views/Home.vue`
     【修改目的】提供页面导航入口
     【修改方式】简单的导航卡片布局
     【修改内容】
        - 展示两个导航卡片：Dashboard（原始数据查看）和 ProjectPanel（工程面板）
        - 点击跳转到对应路由
        - 使用 el-card + el-row/el-col 布局
