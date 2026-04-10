## 实施

- [x] 1.1 CommitViewV2.vue 新增「仓库」列
     【目标对象】`frontend/src/views/CommitViewV2.vue`
     【修改目的】在 Commit 列表中显示仓库地址/分支信息，支持点击跳转到仓库详情页
     【修改方式】在 columns 数组中新增列定义，添加 slotName 自定义渲染为 el-link
     【相关依赖】`useRouter`（已有）、`serverDateRange`（已有）
     【修改内容】
        - 在 columns 数组中（在「用户」列之后）新增仓库列定义
        - 在模板 `<KbFilterTable>` 中新增 `#cell-repo_addr` slot 渲染 el-link
        - 新增 `handleRepoClick(row)` 函数：构造跳转路径并携带日期参数

- [x] 1.2 RepoDetailV2.vue 支持从 URL query 参数初始化日期范围
     【目标对象】`frontend/src/views/RepoDetailV2.vue`
     【修改目的】从 CommitViewV2 跳转时携带的日期参数能自动初始化仓库详情页的日期筛选
     【修改方式】在 onMounted 中读取 route.query.startDate/endDate
     【修改内容】
        - 在 onMounted 中读取 URL query 参数 startDate 和 endDate
        - 若存在则转换为 yyyy-MM-dd 格式并赋值给 dateRange
        - 否则使用默认日期范围
