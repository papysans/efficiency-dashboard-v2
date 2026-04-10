## 实施

- [x] 1.1 新建 DateRangePicker.vue 公共组件
     【目标对象】`frontend/src/components/DateRangePicker.vue`
     【修改目的】封装带左侧快捷按钮的时间范围选择器，trigger 弹出 popover 面板
     【修改方式】新建文件，参考 1.png 设计稿
     【相关依赖】Element Plus `el-popover`、`el-date-picker type="daterange"`
     【修改内容】
        - 触发器：一个输入框样式的 div，显示当前日期范围（格式 `YYYY-MM-DD  To  YYYY-MM-DD`），右侧有日历图标和清除按钮（clearable 为 true 时显示）
        - popover 面板布局：左侧快捷按钮列表 + 右侧 el-date-picker type="daterange" 双月历（inline 模式）
        - 左侧快捷按钮：Today / 1 day ago / 3 days ago / 1 week ago / 1 month ago / 3 months ago，点击后高亮选中状态，同时更新日期范围
        - 右侧日历：使用 `el-date-picker type="daterange"` 的 inline 模式（`:inline="true"`），双向绑定日期值
        - Props：`modelValue: Array`（[startDate, endDate] YYYY-MM-DD），`clearable: Boolean`（默认 false），`placeholder: String`，`size: String`（default/small/large）
        - Emits：`update:modelValue`，`change`
        - 快捷按钮计算逻辑：Today=今天0点到现在；1 day ago=昨天到今天；3 days ago=3天前到今天；1 week ago=7天前到今天；1 month ago=30天前到今天；3 months ago=90天前到今天
        - 选中快捷按钮时高亮（active 状态），手动选日历时取消快捷高亮
        - 日历选完后自动关闭 popover 并 emit change

- [x] 1.2 替换 FilterBar.vue 中的 el-date-picker 为 DateRangePicker
     【目标对象】`frontend/src/components/FilterBar.vue`
     【修改目的】统一使用公共时间范围选择组件
     【修改方式】将 `el-date-picker type="daterange"` 替换为 `<DateRangePicker>`
     【相关依赖】`DateRangePicker.vue`
     【修改内容】
        - import DateRangePicker
        - 替换 el-date-picker 为 DateRangePicker，保持 v-model:dateRange 绑定不变
        - 删除 el-date-picker 相关属性（range-separator、start-placeholder 等）

- [x] 1.3 替换 Home.vue 中的 el-date-picker daterange 为 DateRangePicker
     【目标对象】`frontend/src/views/Home.vue`
     【修改目的】统一使用公共时间范围选择组件
     【修改方式】将内联的 `el-date-picker type="daterange"` 替换为 `<DateRangePicker>`
     【相关依赖】`DateRangePicker.vue`
     【修改内容】
        - import DateRangePicker
        - 替换 el-date-picker 为 DateRangePicker，v-model 绑定 dateRange
        - 删除原 el-date-picker 相关属性

- [x] 1.4 替换 EfficiencyPanel.vue 中的 el-date-picker daterange 为 DateRangePicker
     【目标对象】`frontend/src/views/EfficiencyPanel.vue`
     【修改目的】统一使用公共时间范围选择组件
     【修改方式】将 `el-date-picker type="daterange"` 替换为 `<DateRangePicker>`
     【相关依赖】`DateRangePicker.vue`
     【修改内容】
        - import DateRangePicker
        - 替换 el-date-picker 为 DateRangePicker，v-model 绑定 dateRange
        - 保持 margin-left: 16px 等样式

- [x] 1.5 替换 RepoDetailV2.vue 中的快捷按钮+日历为 DateRangePicker
     【目标对象】`frontend/src/views/RepoDetailV2.vue`
     【修改目的】删除冗余的快捷按钮实现，统一用公共组件
     【修改方式】移除 dateShortcuts、activeDateLabel、customDateRange、applyDateShortcut、applyCustomDate、fmtDate 等局部实现，替换为 DateRangePicker
     【相关依赖】`DateRangePicker.vue`
     【修改内容】
        - import DateRangePicker
        - 删除 template 中的快捷按钮组（el-button v-for dateShortcuts）和 el-date-picker customDateRange
        - 添加 `<DateRangePicker v-model="dateRange" @change="loadData" size="small" />`
        - 删除 script 中：dateShortcuts 数组、activeDateLabel ref、customDateRange ref、applyDateShortcut 函数、applyCustomDate 函数、fmtDate 函数
        - 保留 dateRange ref，初始值改为 getDefaultDateRangeWide()（与原 3 months 快捷默认一致）

- [x] 1.6 替换 KbFilterTable.vue 中 date 类型 filter 面板为 DateRangePicker
     【目标对象】`frontend/src/components/KbFilterTable.vue`
     【修改目的】统一 KbFilterTable 内部日期筛选 UI 体验
     【修改方式】将两处 date 类型 filter 面板（表头 popover 和 tagEdit popover）的双 el-date-picker 替换为 DateRangePicker
     【相关依赖】`DateRangePicker.vue`
     【修改内容】
        - import DateRangePicker
        - 表头 popover 中 date 类型面板：删除 `.kb-filter-date-pair` 中的两个 el-date-picker，替换为单个 `<DateRangePicker v-model="dateRangeTemp[col.prop]" />`
        - tagEdit popover 中 date 类型面板：同样替换
        - 调整 date 类型 filter 的临时值管理：tempFilters 中 date 类型改用单个数组 `tempFilters[col.prop]`（而非 `_start`/`_end` 分离），对应更新 onPopoverShow、applyFilter、resetFilter 中的 date 处理逻辑
        - 删除 `.kb-filter-date-pair` 相关 CSS（如果只用于 date 类型）
        - getPopoverWidth 中 date 类型宽度适当调整（DateRangePicker inline 面板更宽，约 560px）
