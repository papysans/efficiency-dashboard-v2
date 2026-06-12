# 页面说明书 · 提交详情（CommitDetail）

> 路由 `/kanban/commit/:commitId`
> 组件 `frontend-react/src/pages/commits/CommitDetail.tsx`
> 数据接口 `GET /api/v2/commits/{id}`（`endpoints.ts:166`），人工调整 `PUT /v2/commits/{id}/manual`

---

## 0. 这个页面是什么

单个提交的明细：① 基础信息 → ② 度量信息（含提效比）→ ③ 关联 Tasks。

> 提效比为 **V1 古法百分比口径**（→白皮书§12）。

---

## 1. 字段速查表

### ① 基础信息
Commit ID / 用户（真名，可点）/ Git 用户（`git_user_name <email>`）/ 仓库（可点）/ 分支 / 提交时间 / 提交说明。

### ② 度量信息（`CommitDetail.tsx:144-182`）
| 显示 | 含义 | 口径 | 空态 |
|---|---|---|---|
| 生成代码量 | diff 行数（原始） | 🔢 | null→`-` |
| 实际耗时 | manual 优先工时（含原值/理由） | 🧑 | manual/原值皆空→"(AI未出值)" |
| 传统开发时长预估 | manual 优先古法预估 | 🧑 古法 | 同上 |
| **提效比例** | `(古法−实际)/实际×100` | **百分比口径** | null→`-` |
| AI 代码占比 | commit 级 `silica`（该提交匹配 AI 产出行占比） | 🔢 小数×100 | null→`-` |
| 总 Tokens / 费用 | — | 🔢 | ≤0→`-` |

> 提效比口径 = 后端 `CalcEfficiencyRatioManual` = `(古法−实际)/实际×100`（manual 优先）。AI 代码占比是 commit 级 silica，与需求级 `ai_code_ratio` 不同（→§2.10）。

### ③ 关联 Tasks
本提交关联的 task 列表：Task ID（可点）/ 用户 / 开始时间 / 代码行 / 实际耗时 / **AI 代码占比（task 级 silica）** / 费用。空→"暂无关联 Task"。

---

## 2. 人工调整

标题栏"人工调整"按钮 → 4 字段弹窗（传统预估值+理由、实际耗时值+理由）→ `PUT /v2/commits/{id}/manual`。人工值优先于 AI 原值参与显示与提效比计算。

> 说明：Commit 与 Task 详情保留人工调整（项目详情已移除）。

---

## 3. 边界与常见质疑应答

**Q：提效比是怎么算的？口径和仓库列表一致吗？**
本页用 `(古法−实际)/实际×100`（提升百分比），与仓库列表、仓库详情度量卡、后端口径一致。

**Q：AI 代码占比 0.0% 是没用 AI 吗？**
commit 级 silica 默认 0；显示 `0.0%` 既可能是"该提交确无匹配 AI 产出"，也可能是 silica 未计算（无指纹）。结合代码量/关联 Task 判断。

---

*行号基于 `feat/needs-user-search-multi-caliber` 分支当前快照。*
