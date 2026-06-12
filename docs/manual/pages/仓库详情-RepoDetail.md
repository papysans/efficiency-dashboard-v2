# 页面说明书 · 仓库详情（RepoDetail）

> 路由 `/kanban/repo/:repoAddr/:repoBranch?`
> 组件 `frontend-react/src/pages/repos/RepoDetail.tsx`
> 数据接口 `GET /api/v2/repos/detail`（`endpoints.ts:180`）+ `/v2/repos/branches`

---

## 0. 这个页面是什么

单个仓库（可切分支）的明细：① 基础信息 → ② 度量信息 → ③ Commit 列表 → ④ Task 列表 → "添加到 Project"。

> ⚠️ **提效比/工时为 V1 古法百分比口径**；commit/task 行的"AI 代码占比"是 commit 级 `silica`（小数口径），与 need 级 `ai_code_ratio` 不同（→白皮书§12、§2.10）。

---

## 1. 字段速查表

### ② 度量信息（`RepoDetail.tsx:276-305`）
| 显示 | 含义 | 口径 | 空态 |
|---|---|---|---|
| 传统开发时长预估 | 各 commit 古法分钟之和（manual 优先） | 工时（古法） | 0/空→`-` |
| 实际耗时 | 各 commit 实际分钟之和 | 工时 | 0/空→`-` |
| 提效比 | `(古法−实际)/实际×100` | **百分比口径** | null→`-` |
| AI 代码占比 | 需求级 `ai_code_ratio`（needs 口径） | **小数口径** | null→`-` |
| 代码行数 / 总费用 / 贡献者 | commit diff 之和 / task 费用之和 / 去重人数 | 🔢 | — |

> 度量卡的提效比来自 commit 工时求和，AI 占比来自 needs 聚合（带资格+异常过滤、不同日期锚点）——同页两数不同源、不可对账。

### ③ Commit 列表（11 列）
Commit ID（可点）/ 时间 / 用户（git 名）/ 说明 / 代码行数 / 实际耗时 / 传统预估 / **AI 代码占比（commit 级 silica，小数×100）** / **提效比（`(古法−实际)/实际×100`，提升百分比）** / 费用 / Tokens。

> 提效比行口径已与度量卡/列表统一为"提升百分比"。manual 优先。仅 >0 才显，否则 `-`。

### ④ Task 列表（10 列）
仅当 `tasks.length>0` 渲染。列同 Commit 表对应项，提效比同样为提升百分比。来源：本仓 commit 反查的关联 task。

---

## 2. 筛选 / 排序 / 分页

- **分支切换**：标题栏下拉（选项来自 `/v2/repos/branches`），切换跳新分支路由。**直接访问 `/repo/:addr`（无分支）默认取分支字典序第一个**，可能与列表展示的"首选分支"那一行不是同一分支。
- **日期**：默认近 90 天（commit_time；AI 占比按 needs.dev_end_ts）。
- **Commit/Task 表排序：纯客户端**（按显示口径值，null 沉底，不重拉、不分页，后端一次取 ≤10000 commit）。

---

## 3. 边界与常见质疑应答

**Q：度量卡的工时是真实记录吗？**
是"真实记录 + 派生估算"的混合：当某 commit 古法/实际工时缺失时，后端用 Need 工时平摊 + 提交间隔法兜底（→白皮书§12.5）。`title` 悬浮会显示来源说明。

**Q：commit 行的"AI 代码占比"和度量卡的"AI 代码占比"是一回事吗？**
不是。commit 行是 commit 级 `silica`（该提交匹配 AI 产出的行占比），度量卡是需求级 `ai_code_ratio`（需求净行里 AI 覆盖占比）。文案相同、口径不同（→§2.10）。

**Q：直接打开仓库详情，数和列表对不上？**
列表用"首选分支评分"取分支，详情无分支参数时取字典序第一个分支。两者可能不同分支。请通过列表点进（会带 repo_branch），或在详情下拉选对分支。

---

*行号基于 `feat/needs-user-search-multi-caliber` 分支当前快照。*
