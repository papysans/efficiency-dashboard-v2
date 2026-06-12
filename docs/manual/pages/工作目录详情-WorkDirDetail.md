# 页面说明书 · 工作目录详情（WorkDirDetail）

> 路由 `/kanban/workdir/:workDirId`
> 组件 `frontend-react/src/pages/workdir/WorkDirDetail.tsx`
> 数据接口 复用 `GET /api/v2/repos/detail`（把 `workDirId` 当 `repo_addr` 传）

---

## 0. 这个页面是什么

按工作目录（实际按 `repo_addr` 过滤）查该目录下的提交聚合：① 仓库概览 → ② Commit 列表 → ③ 参与者列表。

> **本页复用《仓库详情》接口**（`WorkDirDetail.tsx:46` 把 workDirId 当 repo_addr、分支传空）。因此多个字段后端结构性不返回、前端硬降级为 `-`（见 §3）。唯一比例是 commit 级 silica（AI 代码占比，小数口径）。

---

## 1. 字段速查表

### ② 仓库概览
仓库地址 / 分支 / **用户数（恒 `-`）** / 关联 Task 数 / 关联 Commit 数 / **总费用（恒 `-`）** / **传统开发时长预估（恒 `-`）**。
> 用户数/总费用/传统耗时三项是接口复用导致的**结构性 `-`**（后端不返回该视图），不是数据缺失。

### ③ Commit 列表
展开箭头 / Commit ID / 提交者（git 名，不经真名映射）/ 提交时间 / Diff 行数 / **AI 代码占比（commit 级 silica，进度条+百分比）** / **关联 Task 数（恒 `-`）**。
> 展开区显示 matched_tasks，但该接口不返回 → 展开恒"暂无关联 Task"。

### ④ 参与者列表（前端聚合）
用户名（真名，可点）/ Task 数 / Commit 数。

---

## 2. 筛选 / 排序 / 分页

- **无筛选、无日期**（按 workDirId/repo_addr 定位）。
- **排序：全部客户端**（Commit 列表、参与者列表各自维护，null/0 沉底）。
- **分页**：无（后端一次取 ≤10000 commit）。

---

## 3. 边界与常见质疑应答

**Q：用户数/总费用/传统耗时/关联 Task 数都是 `-`，是数据问题吗？**
不是。本页复用仓库详情接口，这些字段后端在该视图下不提供，前端按约定硬降级为 `-`。汇报时说明"这些字段当前接口不提供"即可。

**Q：本页有提效比吗？**
无。本页唯一比例是 commit 级 AI 代码占比（silica，小数口径），与需求级 AI 占比不同（→§2.10）。

**Q：workDirId 怎么匹配提交？**
本页把 `workDirId` 当作 `repo_addr` 过滤提交（非按 `work_dir_id` 列）。若链接传入的标识与 repo_addr 不同名，可能匹配不到提交。

---

*行号基于 `feat/needs-user-search-multi-caliber` 分支当前快照。*
