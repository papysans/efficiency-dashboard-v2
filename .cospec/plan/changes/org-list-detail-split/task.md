# Org 列表页行点击改为跳转独立详情页

## 阶段 1：改造 OrgViewV2.vue

- [x] 1.1 删除详情展开区域（`v-if="selectedOrg"` 整个块：指标卡片、用户列表、项目列表）及相关变量函数（`selectedOrg`、`detailData`、`detailLoading`、`detailMetrics`、`handleRowClick`中的详情加载、`handleCloseDetail`、`handleUserClick`、`handleProjectClick`）
- [x] 1.2 将"组织名称"列改为模板列，org4以外级别用 el-link 点击下钻（替代双击），org4级别显示纯文本；删除双击事件 `handleRowDblClick`、行样式 `rowClassName`
- [x] 1.3 每行末尾增加"操作"列，放"查看详情" el-link，点击跳转到 `/org/:orgPath`（用 encodeURIComponent 编码完整路径）
- [x] 1.4 在筛选区增加 4 个级联下拉选择器（org1~org4），实现级联逻辑：页面加载获取 org1 选项，选中某级后获取下级选项并清空更下级，查询按钮根据选中的最深级别确定 level 和 parent 参数
- [x] 1.5 删除面包屑导航及相关变量函数（`breadcrumb`、`handleBreadcrumbClick`、`getCurrentParent`）

## 阶段 2：新建 OrgDetailV2.vue + 路由

- [x] 2.1 新建 `frontend/src/views/OrgDetailV2.vue`：路由参数 orgPath 解码后调用 `getOrgDetailV2` 加载数据；页面包含返回按钮+标题、组织路径面包屑（只读）、4指标卡片、用户列表（el-link跳用户详情）、项目列表（el-link跳项目详情）、空数据提示
- [x] 2.2 在 `frontend/src/router/index.js` 添加路由 `{ path: '/org/:orgPath', name: 'OrgDetail', component: () => import('@/views/OrgDetailV2.vue') }`

## 阶段 3：验证

- [x] 3.1 执行 `npm run build` 确保编译通过
