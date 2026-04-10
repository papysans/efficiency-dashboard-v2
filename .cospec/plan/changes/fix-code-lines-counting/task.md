## 实施

### Part 1: 修复代码行数统计 (kbcli)

- [x] 1.1 修复 `calcOutCodeLines()` 中 apply_diff 的参数读取
     【目标对象】`kbcli/raw_parser.go` 的 `calcOutCodeLines()` 函数
     【修改目的】apply_diff 工具的参数 key 是 `"diff"` 而非 `"content"`，导致所有 apply_diff 代码行数记为 0
     【修改方式】修改 `calcOutCodeLines()` 函数，对 apply_diff 分支改读 `args["diff"]` 并实现 REPLACE 部分行数提取
     【相关依赖】无
     【修改内容】
        - 当 funcName 为 `apply_diff` 时，从 `args["diff"]` 读取内容（而非 `args["content"]`）
        - 实现 REPLACE 部分行数提取：遍历 diff 文本中所有 SEARCH/REPLACE 块，只累加每个 `=======` 到 `>>>>>>> REPLACE` 之间的非空行数
        - 边界处理：diff 为空字符串时返回 0；diff 中不含 `=======` 标记时返回 0（格式不合法，跳过）；一个 diff 中可能包含多个 SEARCH/REPLACE 块，需全部累加
        - 当 funcName 为 `write_to_file` 时，保持现有的 `args["content"]` 逻辑不变

- [x] 1.2 修复 `ExtractTaskContent()` 中 apply_diff 的代码内容提取
     【目标对象】`kbcli/task_content.go` 的 `ExtractTaskContent()` 函数中提取 code_outputs 的循环体
     【修改目的】apply_diff 的代码内容提取同样读取了错误的 key `"content"`，导致 code_outputs 中 code 字段为空
     【修改方式】修改循环中 apply_diff 分支的 key 读取逻辑，从 `args["diff"]` 读取并提取 REPLACE 部分
     【相关依赖】无
     【修改内容】
        - 当 funcName 为 `apply_diff` 时，从 `args["diff"]` 读取 diff 内容
        - 提取所有 REPLACE 部分（每个 `=======` 到 `>>>>>>> REPLACE` 之间的内容），拼接作为 code 字段值
        - 边界处理：diff 为空时 code 字段为空字符串；多个 SEARCH/REPLACE 块时用换行拼接各 REPLACE 部分
        - 保持 `write_to_file` 的 `args["content"]` 逻辑不变

- [x] 1.3 修正测试数据中 apply_diff 的参数格式
     【目标对象】`kbcli/raw_parser_test.go` 的 `TestCalcOutCodeLines_ApplyDiff`（TP-25）和 `TestCalcOutCodeLines_MultipleToolCalls`（TP-28）测试函数
     【修改目的】TP-25 测试用例使用了 `"content"` key，与真实数据不符；TP-28 的 apply_diff 条目也使用了 `"content"` key
     【修改方式】将测试数据改为真实的 `"diff"` key + SEARCH/REPLACE 格式，更新期望值
     【相关依赖】依赖 1.1 的 REPLACE 部分行数提取逻辑
     【修改内容】
        - 修改 TP-25（`TestCalcOutCodeLines_ApplyDiff`）：Arguments 改为 `{"path":"b.go","diff":"<<<<<<< SEARCH\nold1\nold2\n=======\nnew1\nnew2\n>>>>>>> REPLACE"}`，期望行数改为 2（REPLACE 部分 2 行）
        - 修改 TP-28（`TestCalcOutCodeLines_MultipleToolCalls`）：apply_diff 条目的 Arguments 改为 diff 格式，更新总期望行数（write_to_file 2行 + apply_diff REPLACE 部分行数）
        - 确保 `go test ./kbcli/...` 全部通过

### Part 2: 提取前端公共工具函数

- [x] 2.1 创建 `src/utils/formatters.js` — 统一格式化函数
     【目标对象】`frontend/src/utils/formatters.js`（新建）
     【修改目的】消除 fmtCost/fmtDays/fmtMsToMin 在 Dashboard、ProjectPanel、RepoPanel、UserPanel、OrgPanel、EfficiencyPanel 共 6 个组件中的重复定义
     【修改方式】新建文件，提取所有格式化函数为 el-table formatter 兼容签名
     【相关依赖】无
     【修改内容】
        - `fmtCost(row, col, value)`: 格式化费用，保留4位小数，null 返回空字符串（兼容 el-table :formatter 三参数签名）
        - `fmtDays(row, col, value)`: AI预估人天，保留1位小数，null 或 0 返回 '-'
        - `fmtMsToMin(row, col, value)`: 毫秒转分钟，保留1位小数，后缀 ' min'
        - 注意：EfficiencyPanel 中的 `fmtDays`/`fmtCost`/`msToDays`/`fmtRatio`/`ratioColor` 是独立的单参数函数且逻辑不同（如 fmtCost 保留2位小数，0值返回 '-'），不应合并到此公共模块中，EfficiencyPanel 保持本地定义

- [x] 2.2 创建 `src/utils/date.js` — 统一日期工具
     【目标对象】`frontend/src/utils/date.js`（新建）
     【修改目的】消除 `getDefaultDateRange()` 在 Dashboard、ProjectPanel、RepoPanel、UserPanel、OrgPanel、EfficiencyPanel 共 6 个组件中的重复定义
     【修改方式】新建文件，提取日期工具函数
     【相关依赖】无
     【修改内容】
        - `getDefaultDateRange()`: 返回最近7天的日期范围 `[startStr, endStr]`，格式 YYYY-MM-DD
        - `formatDateParam(dateStr)`: 将 'YYYY-MM-DD' 格式转为 'YYYYMMDD'（即 `dateStr.replace(/-/g, '')`）

- [x] 2.3 创建 `src/utils/chart.js` — 统一图表配置
     【目标对象】`frontend/src/utils/chart.js`（新建）
     【修改目的】消除 `createBarOption`/`createDualBarOption`/`truncateName` 在 ProjectPanel、UserPanel、OrgPanel 共 3 个组件中的重复定义
     【修改方式】新建文件，提取图表配置函数
     【相关依赖】无
     【修改内容】
        - `truncateName(name, maxLen=30)`: 截断名称，超长加 '...'
        - `createBarOption(title, data, color, valueFormatter)`: 横向柱状图配置，签名与现有 ProjectPanel 中一致（data 为 [{name, value}] 数组）
        - `createDualBarOption(title, data1, data2, label1, label2, color1, color2)`: 双柱对比图配置，签名与现有 ProjectPanel 中一致
        - `CHART_COLORS`: 导出统一配色常量 `{ blue: '#409EFF', orange: '#E6A23C', green: '#67C23A', red: '#F56C6C', gray: '#909399' }`

### Part 3: 提取前端 Composables

- [x] 3.1 创建 `src/composables/useFavorites.js` — 收藏逻辑
     【目标对象】`frontend/src/composables/useFavorites.js`（新建）
     【修改目的】消除 isFavorited/toggleFavorite/loadFavorites/applyFavoritesFilter 在 ProjectPanel、RepoPanel、UserPanel、OrgPanel 共 4 个面板的重复
     【相关依赖】`frontend/src/api/es.js` 的 `addFavorite`/`removeFavorite`/`getFavorites`/`getVirtualGroupAggregate`
     【修改方式】新建文件，提取为 Vue 3 Composable，接收 dimension 参数
     【修改内容】
        - `useFavorites(dimension)` 返回 `{ favorites, showFavoritesOnly, isFavorited, toggleFavorite, loadFavorites, applyFavoritesFilter }`
        - 内部管理 `favorites` ref 列表和 `showFavoritesOnly` ref 状态
        - `isFavorited(row)`: 判断 row.key 是否在 favorites 中
        - `toggleFavorite(row)`: 调用 addFavorite/removeFavorite 切换收藏状态，包含 ElMessage 反馈和错误处理
        - `loadFavorites()`: 调用 getFavorites 加载收藏列表
        - `applyFavoritesFilter(dateParams, fetchAggregate)`: 接收日期参数和聚合查询函数，过滤普通收藏项并聚合虚拟组收藏项，返回合并后的数据
        - 注意：OrgPanel 的 dimension 是动态的（currentLevel），需支持响应式 dimension 参数（接受 ref 或 getter）

- [x] 3.2 创建 `src/composables/useUrlSync.js` — URL 同步
     【目标对象】`frontend/src/composables/useUrlSync.js`（新建）
     【修改目的】消除 syncUrl/restoreFromUrl 在 Dashboard、ProjectPanel、RepoPanel、UserPanel、OrgPanel 共 5 个面板的重复
     【修改方式】新建文件，提取为 Composable
     【相关依赖】`vue-router` 的 `useRoute`/`useRouter`
     【修改内容】
        - `useUrlSync(paramDefs)` 返回 `{ syncToUrl, restoreFromUrl }`
        - `paramDefs` 为参数定义数组，每项包含 `{ key: 'startDate', ref: dateRangeRef, type: 'dateRange' }` 等
        - `syncToUrl()`: 将各 ref 值写入 URL query
        - `restoreFromUrl()`: 从 URL query 恢复到各 ref
        - 注意：Dashboard 有额外的 tab/dimension 参数，需支持自定义参数类型

- [x] 3.3 创建 `src/composables/useChart.js` — ECharts 生命周期
     【目标对象】`frontend/src/composables/useChart.js`（新建）
     【修改目的】消除 ProjectPanel、UserPanel、OrgPanel、EfficiencyPanel 共 4 个面板中 ECharts init/dispose/resize 的重复管理代码
     【修改方式】新建文件，提取为 Composable
     【相关依赖】`echarts` 库
     【修改内容】
        - `useChart(containerRef)` 返回 `{ chart, setOption, dispose }`
        - 自动处理 onMounted 时 init / onUnmounted 时 dispose / window resize 时 resize
        - 注意：各面板有多个图表实例，需支持多次调用（每个 containerRef 一个实例）
        - 注意：resize 监听器只需注册一次，多个实例共享

### Part 4: 重构各面板组件引用公共模块

- [x] 4.1 重构 Dashboard.vue — 引用公共模块
     【目标对象】`frontend/src/views/Dashboard.vue`
     【修改目的】删除重复的 getDefaultDateRange/fmtCost/fmtDays/fmtMsToMin，引用公共模块
     【修改方式】import 替换本地函数定义
     【相关依赖】`frontend/src/utils/formatters.js`、`frontend/src/utils/date.js`
     【修改内容】
        - 导入 `utils/formatters.js` 的 `fmtCost`、`fmtDays`、`fmtMsToMin`
        - 导入 `utils/date.js` 的 `getDefaultDateRange`
        - 删除本地 `fmtCost`（第176行）、`fmtDays`（第189行）、`fmtMsToMin`（第182行）、`getDefaultDateRange`（第195行）函数定义
        - 保留 `fmtTimestamp`（此函数仅 Dashboard 使用，不提取）
        - 保留 `syncUrl`/`restoreFromUrl`（Dashboard 有特殊的 tab/dimension 参数）

- [x] 4.2 重构 EfficiencyPanel.vue — 引用公共模块
     【目标对象】`frontend/src/views/EfficiencyPanel.vue`
     【修改目的】删除重复的 getDefaultDateRange，引用公共模块
     【修改方式】import 替换本地 getDefaultDateRange 函数定义
     【相关依赖】`frontend/src/utils/date.js`
     【修改内容】
        - 导入 `utils/date.js` 的 `getDefaultDateRange`
        - 删除本地 `getDefaultDateRange`（第286行）函数定义
        - 保留本地 `fmtDays`/`fmtCost`/`msToDays`/`fmtRatio`/`ratioColor`（这些函数签名和逻辑与其他面板不同，是单参数版本且含特殊逻辑如 msToDays 的分钟显示）

- [x] 4.3 重构 ProjectPanel.vue — 引用公共模块
     【目标对象】`frontend/src/views/ProjectPanel.vue`
     【修改目的】删除大量重复代码，引用公共模块和 composables
     【修改方式】import 公共模块替换本地定义
     【相关依赖】`frontend/src/utils/formatters.js`、`frontend/src/utils/date.js`、`frontend/src/utils/chart.js`、`frontend/src/composables/useFavorites.js`、`frontend/src/composables/useUrlSync.js`、`frontend/src/composables/useChart.js`
     【修改内容】
        - 导入 `utils/formatters.js` 的 `fmtCost`/`fmtDays`/`fmtMsToMin`，删除本地定义（第182/188/194行）
        - 导入 `utils/date.js` 的 `getDefaultDateRange`，删除本地定义（第168行）
        - 导入 `utils/chart.js` 的 `createBarOption`/`createDualBarOption`/`truncateName`，删除本地定义（第201/207/248行）
        - 导入 `composables/useFavorites.js`，替换本地 favorites/showFavoritesOnly/isFavorited/toggleFavorite/loadFavorites/applyFavoritesFilter（第340-453行），dimension 传 'project'
        - 导入 `composables/useUrlSync.js`，替换本地 syncUrl/restoreFromUrl（第456-471行）
        - 导入 `composables/useChart.js`，替换本地 initCharts/disposeCharts/handleResize（第295-337/542-548行），为每个图表 ref 创建 useChart 实例

- [x] 4.4 重构 RepoPanel.vue — 引用公共模块
     【目标对象】`frontend/src/views/RepoPanel.vue`
     【修改目的】删除重复代码，引用公共模块和 composables
     【修改方式】import 公共模块替换本地定义
     【相关依赖】`frontend/src/utils/formatters.js`、`frontend/src/utils/date.js`、`frontend/src/composables/useFavorites.js`、`frontend/src/composables/useUrlSync.js`
     【修改内容】
        - 导入 `utils/formatters.js` 的 `fmtCost`/`fmtDays`/`fmtMsToMin`，删除本地定义（第207/212/217行）
        - 导入 `utils/date.js` 的 `getDefaultDateRange`，删除本地定义（第222行）
        - 导入 `composables/useFavorites.js`，替换本地收藏逻辑（第335-442行），dimension 传 'repo'
        - 导入 `composables/useUrlSync.js`，替换本地 syncUrl/restoreFromUrl（第236-250行）
        - 注意：RepoPanel 无柱状图（仅有饼图且是特有逻辑），不引入 `utils/chart.js` 和 `composables/useChart.js`

- [x] 4.5 重构 UserPanel.vue — 引用公共模块
     【目标对象】`frontend/src/views/UserPanel.vue`
     【修改目的】删除重复代码，引用公共模块和 composables
     【修改方式】import 公共模块替换本地定义
     【相关依赖】`frontend/src/utils/formatters.js`、`frontend/src/utils/date.js`、`frontend/src/utils/chart.js`、`frontend/src/composables/useFavorites.js`、`frontend/src/composables/useUrlSync.js`、`frontend/src/composables/useChart.js`
     【修改内容】
        - 导入 `utils/formatters.js` 的 `fmtCost`/`fmtDays`/`fmtMsToMin`，删除本地定义（第155/160/165行）
        - 导入 `utils/date.js` 的 `getDefaultDateRange`，删除本地定义（第142行）
        - 导入 `utils/chart.js` 的 `createBarOption`/`truncateName`，删除本地定义（第170/175行）
        - 导入 `composables/useFavorites.js`，替换本地收藏逻辑（第267-380行），dimension 传 'user'
        - 导入 `composables/useUrlSync.js`，替换本地 syncUrl/restoreFromUrl（第238-252行）
        - 导入 `composables/useChart.js`，替换本地 initCharts/disposeCharts/handleResize（第215-236/435-438行）

- [x] 4.6 重构 OrgPanel.vue — 引用公共模块
     【目标对象】`frontend/src/views/OrgPanel.vue`
     【修改目的】删除重复代码，引用公共模块和 composables
     【修改方式】import 公共模块替换本地定义
     【相关依赖】`frontend/src/utils/formatters.js`、`frontend/src/utils/date.js`、`frontend/src/utils/chart.js`、`frontend/src/composables/useFavorites.js`、`frontend/src/composables/useUrlSync.js`、`frontend/src/composables/useChart.js`
     【修改内容】
        - 导入 `utils/formatters.js` 的 `fmtCost`/`fmtDays`/`fmtMsToMin`，删除本地定义（第173/178/183行）
        - 导入 `utils/date.js` 的 `getDefaultDateRange`，删除本地定义（第159行）
        - 导入 `utils/chart.js` 的 `createBarOption`/`truncateName`，删除本地定义（第190/196行）
        - 导入 `composables/useFavorites.js`，替换本地收藏逻辑（第358-458行），dimension 传动态值 `currentLevel`（需传 ref 或 getter）
        - 导入 `composables/useUrlSync.js`，替换本地 syncUrl/restoreFromUrl（第262-277行）
        - 导入 `composables/useChart.js`，替换本地 initCharts/disposeCharts/handleResize（第236-259/481-484行）

- [x] 4.7 添加 axios 响应拦截器统一错误处理
     【目标对象】`frontend/src/api/index.js` 的 `request` axios 实例
     【修改目的】消除各组件中重复的 `catch (err) { console.error(...); ElMessage.error(...) }` 模式
     【修改方式】在现有 `request` axios 实例上添加响应拦截器
     【相关依赖】`element-plus` 的 `ElMessage`
     【修改内容】
        - 添加响应拦截器：`request.interceptors.response.use(res => res, error => { ... })`
        - 在 error 回调中统一 `ElMessage.error(错误信息)` 提示
        - 返回 `Promise.reject(error)` 让调用方仍可 catch
        - 注意：添加拦截器后，各面板组件的 catch 块中仍需保留业务逻辑（如清空数据 `tableData.value = []`），只可删除 `ElMessage.error(...)` 调用和 `console.error(...)` 调用
