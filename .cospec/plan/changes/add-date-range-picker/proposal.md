# 变更：新增通用时间范围选择公共组件并统一替换全站使用

## 原因
项目中存在多处分散的时间范围选择实现（el-date-picker daterange、RepoDetailV2 的自定义快捷按钮+日历、KbFilterTable 内部的双 date picker），UI 体验不一致，且 RepoDetailV2 的快捷按钮逻辑无法复用。需要统一封装为一个带左侧快捷按钮的公共组件。

## 变更内容
- 新建 `DateRangePicker.vue` 公共组件：
  - 触发方式：点击输入框（显示当前日期范围）弹出 popover 面板
  - 面板左侧：快捷按钮列表（Today / 1 day ago / 3 days ago / 1 week ago / 1 month ago / 3 months ago）
  - 面板右侧：Element Plus `el-date-picker type="daterange"` 双月历
  - 底部：显示已选日期范围（格式 YYYY-MM-DD To YYYY-MM-DD）+ 清除按钮
  - Props：`modelValue: Array`（[startDate, endDate] YYYY-MM-DD 格式），`clearable: Boolean`
  - Emits：`update:modelValue`，`change`
- 替换 `FilterBar.vue` 中的 `el-date-picker` 为新组件
- 替换 `Home.vue` 中内联的 `el-date-picker daterange` 为新组件
- 替换 `EfficiencyPanel.vue` 中的 `el-date-picker daterange` 为新组件
- 替换 `RepoDetailV2.vue` 中的快捷按钮组 + `el-date-picker daterange` 为新组件（删除冗余的 `dateShortcuts`、`activeDateLabel`、`customDateRange`、`applyDateShortcut`、`applyCustomDate`、`fmtDate` 等局部实现）
- 替换 `KbFilterTable.vue` 中 date 类型 filter 的双 `el-date-picker type="date"` 为新组件（包括表头 popover 和 tagEdit popover 两处）

## 影响
- **受影响的规范**：时间范围筛选 UI 交互
- **受影响的代码**：
  - `frontend/src/components/DateRangePicker.vue`：新建公共组件
  - `frontend/src/components/FilterBar.vue`：替换内部 el-date-picker 为 DateRangePicker
  - `frontend/src/views/Home.vue`：替换内联 el-date-picker daterange 为 DateRangePicker
  - `frontend/src/views/EfficiencyPanel.vue`：替换内联 el-date-picker daterange 为 DateRangePicker
  - `frontend/src/views/RepoDetailV2.vue`：删除快捷按钮+自定义日历逻辑，统一用 DateRangePicker
  - `frontend/src/components/KbFilterTable.vue`：date 类型 filter 面板改用 DateRangePicker
