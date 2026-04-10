## 实施

- [x] 1.1 在 config.yaml 新增传统开发人天代码量基准配置
     【目标对象】`config.yaml`（根目录）
     【修改目的】新增全局配置项，供后端读取并通过 API 暴露给前端
     【修改方式】在文件末尾追加配置项（新增一行）
     【相关依赖】无
     【修改内容】
        - 新增 `traditional_dev_lines_per_day: 100` 配置项（默认100行/人天）

- [x] 1.2 在 backend/constants.go 新增传统开发人天代码量默认常量
     【目标对象】`backend/constants.go` 的 `const` 块
     【修改目的】集中管理默认值常量，避免魔法数字散落在代码中
     【修改方式】在现有 `const` 块末尾追加新常量
     【相关依赖】无
     【修改内容】
        - 新增常量 `DefaultTraditionalDevLinesPerDay = 100`（传统开发人天代码量基准值，行/人天）

- [x] 1.3 在 backend/main.go 的 Config 结构体中新增配置字段，并注册 /api/v2/config 路由
     【目标对象】`backend/main.go` 的 `Config` 结构体 + `loadConfig` 函数 + `v2` 路由组
     【修改目的】让后端能读取新配置项，并通过路由暴露给前端
     【修改方式】修改 Config 结构体追加字段；修改 loadConfig 追加默认值；在 v2 路由组追加一条 GET 路由
     【相关依赖】`config.yaml`（task 1.1）；`backend/constants.go` 的 `DefaultTraditionalDevLinesPerDay`（task 1.2）；handler 实现在 `backend/config_handler.go`（task 1.4）
     【修改内容】
        - `Config` 结构体新增字段：`TraditionalDevLinesPerDay int \`yaml:"traditional_dev_lines_per_day"\``（与其他顶层字段平级）
        - `loadConfig` 函数中在现有默认值设置末尾追加：`cfg.TraditionalDevLinesPerDay = DefaultTraditionalDevLinesPerDay`
        - `v2` 路由组末尾追加：`v2.GET("/config", getConfigV2)`

- [x] 1.4 新建 backend/config_handler.go，实现 getConfigV2 handler
     【目标对象】新建文件 `backend/config_handler.go`
     【修改目的】将 config 查询 handler 独立成文件，避免 main.go 膨胀，与现有 handler 文件拆分风格一致
     【修改方式】新建文件，声明 `package main`，实现 handler 函数
     【相关依赖】`appConfig`（全局变量，定义在 `backend/main.go`）；`Config.TraditionalDevLinesPerDay`（task 1.3）
     【修改内容】
        - 文件头：`package main`
        - 新增函数 `getConfigV2(c *gin.Context)`：
          - 直接读取全局 `appConfig.TraditionalDevLinesPerDay`
          - 返回 `c.JSON(200, gin.H{"traditional_dev_lines_per_day": appConfig.TraditionalDevLinesPerDay})`
          - 无需错误处理（配置必然存在）

- [x] 1.5 前端新增 getGlobalConfig API 函数
     【目标对象】`frontend/src/api/es.js` 文件末尾
     【修改目的】封装调用后端 /api/v2/config 的方法，供前端组件使用
     【修改方式】在文件末尾追加一行导出函数（与文件中现有箭头函数风格一致）
     【相关依赖】后端 `GET /api/v2/config` 端点（task 1.3/1.4）
     【修改内容】
        - 新增：`export const getGlobalConfig = () => request({ url: '/v2/config', method: 'get' })`

- [x] 2.1 修改 ProjectDetailV2.vue：移除创建时间、更新时间
     【目标对象】`frontend/src/views/ProjectDetailV2.vue` 基础信息卡片的 `el-descriptions`（第 22-51 行）
     【修改目的】基础信息卡片去掉不必要字段，减少冗余信息
     【修改方式】删除两个 `el-descriptions-item`（直接删除对应行，无需其他改动）
     【相关依赖】无（`formatLocalTime` 函数在其他字段仍有使用，不需删除 import）
     【修改内容】
        - 删除 `<el-descriptions-item label="创建时间">{{ formatLocalTime(project.created_at) }}</el-descriptions-item>`
        - 删除 `<el-descriptions-item label="更新时间">{{ formatLocalTime(project.updated_at) }}</el-descriptions-item>`

- [x] 2.2 修改 ProjectDetailV2.vue：优化传统开发预估和实际处理耗时的 tooltip
     【目标对象】`frontend/src/views/ProjectDetailV2.vue` 度量信息卡片中"传统开发预估"（第 58-77 行）和"实际处理耗时"（第 80-99 行）两个 `el-descriptions-item`
     【修改目的】让用户理解数据来源，tooltip 改为说明计算来源（汇聚了哪些数据）
     【修改方式】修改两个字段中所有 `el-tooltip` 的 `:content` 属性，从仅显示 reason 文本改为固定说明文字 + reason 文本的拼接表达式；两个字段各有 manual 分支（橙色图标）和非 manual 分支（灰色图标）共两处 tooltip，均需修改
     【相关依赖】无（不新增 computed，直接在模板内联表达式中拼接）
     【修改内容】
        - "传统开发预估"的所有 `el-tooltip :content`：
          - manual 分支（`project_ancient_minutes_reason_manual`）改为：`` `汇聚项目内所有 Task 和 Commit 的传统开发预估时间之和${project.project_ancient_minutes_reason_manual ? '：' + project.project_ancient_minutes_reason_manual : ''}` ``
          - 非 manual 分支（`project_ancient_minutes_reason`）改为：`` `汇聚项目内所有 Task 和 Commit 的传统开发预估时间之和${project.project_ancient_minutes_reason ? '：' + project.project_ancient_minutes_reason : ''}` ``
          - 两处 `v-if` 条件均去掉（固定显示图标，不再依赖 reason 是否存在），或保留 `v-if` 但始终显示固定说明
        - "实际处理耗时"的所有 `el-tooltip :content` 同理：
          - 固定前缀改为：`汇聚项目内所有 Task 和 Commit 的实际 AI 处理耗时之和（不含等待时间）`
          - reason 有值时追加 `：<reason内容>`

- [x] 2.3 修改 ProjectDetailV2.vue：将总上行Tokens+总下行Tokens 替换为总Tokens
     【目标对象】`frontend/src/views/ProjectDetailV2.vue` 度量信息卡片（第 123-124 行）+ `<script setup>` 中的 computed 区域
     【修改目的】合并两个 Token 字段为一个，减少冗余，tooltip 说明各自数值
     【修改方式】删除两个原字段的 `el-descriptions-item`，新增一个"总Tokens"字段；新增 `totalTokens` computed
     【相关依赖】`project.upstream_tokens`、`project.downstream_tokens`（已在 `project` ref 中）
     【修改内容】
        - 在 `<script setup>` 中新增 computed：
          ```
          const totalTokens = computed(() => {
            return (project.value.upstream_tokens || 0) + (project.value.downstream_tokens || 0)
          })
          ```
        - 删除模板中 `<el-descriptions-item label="总上行Tokens">` 和 `<el-descriptions-item label="总下行Tokens">` 两行
        - 新增 `<el-descriptions-item label="总Tokens">` 带 `el-tooltip`：
          - 显示值：`totalTokens > 0 ? totalTokens.toLocaleString() : '-'`
          - tooltip content（内联模板字符串）：`` `上行 Tokens（用户输入）：${(project.upstream_tokens || 0).toLocaleString()}，下行 Tokens（AI 输出）：${(project.downstream_tokens || 0).toLocaleString()}` ``
          - tooltip 使用与现有字段相同的属性：`placement="top" :show-after="200" popper-class="reason-tooltip"`

- [x] 2.4 修改 ProjectDetailV2.vue：新增"生成代码量"字段
     【目标对象】`frontend/src/views/ProjectDetailV2.vue` 度量信息卡片 + `<script setup>` computed 区域
     【修改目的】展示项目内所有 commit 的代码行数汇总
     【修改方式】新增 computed 属性 `totalCodeLines`；在度量信息 `el-descriptions` 中新增 `el-descriptions-item`
     【相关依赖】`commits` ref（已在 `loadData` 中赋值，每条 commit 有 `diff_lines` 字段，见 Commits 列表第 217 行）
     【修改内容】
        - 新增 computed：
          ```
          const totalCodeLines = computed(() => {
            return commits.value.reduce((sum, row) => sum + (row.diff_lines || 0), 0)
          })
          ```
        - 在度量信息卡片新增 `<el-descriptions-item label="生成代码量">`，带 `el-tooltip`：
          - tooltip content：`项目内所有 Commit 的代码变更行数（diff_lines）之和`
          - 显示值：`totalCodeLines > 0 ? totalCodeLines.toLocaleString() + ' 行' : '-'`
          - tooltip 图标使用与现有字段相同的 `QuestionFilled` 图标样式（灰色，`color: #909399`）

- [x] 2.5 修改 ProjectDetailV2.vue：新增"实际耗时"字段（单位人天）
     【目标对象】`frontend/src/views/ProjectDetailV2.vue` 度量信息卡片 + `<script setup>` computed 区域
     【修改目的】以人天为单位展示实际处理耗时，便于与传统预估对比；1人天 = 480分钟（8小时），与 `formatDuration` 函数中的换算逻辑一致
     【修改方式】新增 computed 属性 `actualWorkDays`；在度量信息 `el-descriptions` 中新增 `el-descriptions-item`
     【相关依赖】`project.project_real_process_minutes_manual`、`project.project_real_process_minutes`（已在 `project` ref 中）；1人天=480分钟（与 `formatters.js` 中 `formatDuration` 的换算一致）
     【修改内容】
        - 新增 computed：
          ```
          const actualWorkDays = computed(() => {
            const minutes = project.value.project_real_process_minutes_manual ?? project.value.project_real_process_minutes
            if (minutes == null || minutes <= 0) return null
            return minutes / 480
          })
          ```
          - 边界处理：minutes 为 null 或 ≤0 时返回 null
        - 在度量信息卡片新增 `<el-descriptions-item label="实际耗时">`，带 `el-tooltip`：
          - tooltip content：`等同于实际处理耗时，以人天（8小时/天）为单位展示，来源：汇聚项目内所有 Task 和 Commit 的实际 AI 处理耗时之和`
          - 显示值：`actualWorkDays != null ? actualWorkDays.toFixed(2) + ' 人天' : '-'`

- [x] 2.6 修改 ProjectDetailV2.vue：替换"提效比"为四个新指标字段
     【目标对象】`frontend/src/views/ProjectDetailV2.vue` 度量信息卡片（第 127-132 行）+ `<script setup>` 中的 computed 区域（第 383-393 行）+ `onMounted`（第 733-735 行）+ `import` 语句（第 366 行）
     【修改目的】用更细化的四个指标替代单一提效比，帮助用户多维度理解效能
     【修改方式】删除原"提效比" `el-descriptions-item`；新增四个 `el-descriptions-item`；新增对应 computed；新增全局配置加载逻辑；删除原 `efficiencyRatio` 和 `efficiencyColor` computed（若无其他地方引用）
     【相关依赖】
        - `getGlobalConfig` API（task 1.5，需在 import 语句中追加）
        - `totalCodeLines` computed（task 2.4）
        - `actualWorkDays` computed（task 2.5）
        - `project.project_ancient_minutes_manual ?? project.project_ancient_minutes`（传统开发预估，分钟）
        - `project.project_real_lead_minutes_manual ?? project.project_real_lead_minutes`（项目周期，分钟）
        - `getEfficiencyColor`（已 import，但注意：该函数接收百分比值，新指标为倍数，需乘以100后传入，见下方说明）
     【修改内容】
        - 在 `import { getProjectDetail, ... }` 语句中追加 `getGlobalConfig`
        - 新增 `const globalConfig = ref({ traditional_dev_lines_per_day: 100 })`
        - 修改 `onMounted`：在 `loadData()` 调用后追加异步加载全局配置：
          ```
          getGlobalConfig().then(res => {
            globalConfig.value = res.data || res
          }).catch(() => {})
          ```
        - 新增 computed `traditionalDevLinesPerDay`：`globalConfig.value.traditional_dev_lines_per_day || 100`
        - 新增 computed `ancientWorkDays`：
          ```
          const ancientWorkDays = computed(() => {
            const minutes = project.value.project_ancient_minutes_manual ?? project.value.project_ancient_minutes
            if (minutes == null || minutes <= 0) return null
            return minutes / 480
          })
          ```
        - 新增 computed `leadWorkDays`：
          ```
          const leadWorkDays = computed(() => {
            const minutes = project.value.project_real_lead_minutes_manual ?? project.value.project_real_lead_minutes
            if (minutes == null || minutes <= 0) return null
            return minutes / 480
          })
          ```
        - 新增 computed `actualLinesPerDay`（实际人天代码量）：
          ```
          const actualLinesPerDay = computed(() => {
            if (!actualWorkDays.value || actualWorkDays.value <= 0) return null
            return totalCodeLines.value / actualWorkDays.value
          })
          ```
        - 新增 computed `traditionalLinesPerDay`（传统开发人天代码量）：
          ```
          const traditionalLinesPerDay = computed(() => {
            if (!ancientWorkDays.value || ancientWorkDays.value <= 0) return null
            return totalCodeLines.value / ancientWorkDays.value
          })
          ```
        - 新增 computed `devEfficiencyRatio`（开发提效比，倍数）：
          ```
          const devEfficiencyRatio = computed(() => {
            if (!actualWorkDays.value || actualWorkDays.value <= 0) return null
            if (!ancientWorkDays.value) return null
            return ancientWorkDays.value / actualWorkDays.value
          })
          ```
        - 新增 computed `e2eEfficiencyRatio`（端到端提效比，倍数）：
          ```
          const e2eEfficiencyRatio = computed(() => {
            if (!leadWorkDays.value || leadWorkDays.value <= 0) return null
            if (!ancientWorkDays.value) return null
            return ancientWorkDays.value / leadWorkDays.value
          })
          ```
        - 删除原 `efficiencyRatio` 和 `efficiencyColor` computed（确认仅在"提效比"字段中使用，删除后不影响其他地方）
        - 删除模板中原"提效比" `<el-descriptions-item>`（第 127-132 行）
        - 在度量信息 `el-descriptions` 中新增四个 `<el-descriptions-item>`：
          1. **实际人天代码量**：显示 `actualLinesPerDay != null ? actualLinesPerDay.toFixed(0) + ' 行/人天' : '-'`；tooltip：`生成代码量 ÷ 实际耗时（人天），反映 AI 辅助下实际的代码产出效率`
          2. **传统开发人天代码量**：显示 `traditionalLinesPerDay != null ? traditionalLinesPerDay.toFixed(0) + ' 行/人天' : '-'`；tooltip（内联模板字符串）：`` `生成代码量 ÷ 传统开发预估（人天），可与企业传统基准（${traditionalDevLinesPerDay} 行/人天）对比验证预估合理性` ``
          3. **开发提效比**：显示 `devEfficiencyRatio != null ? devEfficiencyRatio.toFixed(2) + 'x' : '-'`，用 `getEfficiencyColor` 着色（注意：`getEfficiencyColor` 接收百分比，需传入 `devEfficiencyRatio * 100`，例如3倍传入300）；tooltip：`传统开发预估 ÷ 实际耗时，反映 AI 工具在纯开发环节的提效倍数`
          4. **端到端提效比**：显示 `e2eEfficiencyRatio != null ? e2eEfficiencyRatio.toFixed(2) + 'x' : '-'`，同样用 `getEfficiencyColor(e2eEfficiencyRatio * 100)` 着色；tooltip：`传统开发预估 ÷ 项目周期（含等待、评审等），反映整个项目流程的端到端提效倍数`
          - 着色字段的 `<span>` 使用与原"提效比"相同的样式：`fontSize: '20px', fontWeight: 'bold'`
