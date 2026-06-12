# 页面说明书 · 总览（Overview）

> 路由 `/kanban/`（首页，`frontend-react/src/router.tsx:49`）
> 组件 `frontend-react/src/pages/Overview.tsx`
> 数据接口 `GET /api/v2/dashboard/summary`（`endpoints.ts:57`）+ `/v2/needs`（全量）+ `/v2/users` + `/v2/config` + chat 代理 `/stats/global/daily`

---

## 0. 这个页面是什么

面向管理者的**提效总览大屏**。把全平台（默认最近 90 天，`Overview.tsx:14-17`）的提效成果浓缩成 5 个区块：① Hero 省人天/净节省 → ② 提效趋势 → ③ AI 代码占比 → ④ Top 提效榜 → ⑤ 规模概览。

**谁看**：汇报开场用 Hero 的"省人天/净节省"和"综合提效"讲整体成效；用趋势和 Top 榜讲走势与标杆。

> 口径前提：本页提效相关数字走 **V2 融合口径（小数）**（→《01-算法白皮书》§6.2），分母均在后端 SQL 内剔除不可计入（§9）与异常（§8）样本。**AI 花费/成本来自外部 chat 服务**，口径不在本系统内（见 §4）。

---

## 1. 字段速查表

### ① Hero（`HeroSaving.tsx`）
| 显示 | 含义 | 口径/来源 | 空态 |
|---|---|---|---|
| 为团队节省（人天） | 省下的自然周期，折算人天 | 📅 `max(0, 合计传统周期预估 − 合计实际周期) ÷ 480`（`HeroSaving.tsx:54-55`；后端两数皆 FILTER 可计入且非日历异常，`db.go:1545-1546`） | `≤0`→`-` |
| AI 花费（¥） | 同区间全平台 AI 成本 | 外部 chat `/stats/global/daily` 的 `estimated_total_cost` 求和（`HeroSaving.tsx:48-51`） | chat 未启用→不显（降级三格） |
| 净节省（¥） | 毛节省 − AI 花费 | `毛节省 = 省人天 × 人天单价`，人天单价取 `/v2/config cost_per_person_day`（默认 2000，`HeroSaving.tsx:19,32`） | 负数标红 |
| 综合日历提效（%） | Need 维度的整体日历提效比 | 📅 `need_calendar_ratio`（`dashboard_handler_v2.go:108`，→§6.2） | null→`-` |

> Hero 视 chat 开关切四格（省人天/AI花费/净节省/综合提效）或三格（省人天/折合节省成本/综合提效），见 §4。

### ② 提效趋势 TrendCard
| 显示 | 含义 | 口径/来源 |
|---|---|---|
| 每周平均提效率折线 | 按 ISO 周聚合的"可计入需求平均日历提效" | 📅 客户端聚合：仅 `coverage_eligible && efficiency_ratio≠null` 的需求，按 `dev_end_ts` 周分桶，桶内算术平均（`TrendCard.tsx:25-40`） |

> 注：趋势是"各可计入需求日历提效的周算术平均"，与 Hero 的"综合提效"（合计基线÷合计实际）是不同聚合方式，数值不必相等——前者看典型需求、后者看整体盘子。

### ③ AI 代码占比 AdoptionCard
| 显示 | 含义 | 口径/来源 | 空态 |
|---|---|---|---|
| AI 覆盖代码占比 | 需求口径 AI 覆盖行 / 净代码行 | 🔢 `ai_code_ratio`（仅统计可计入、非异常、`total_loc_net>0` 的需求，`ai_code_ratio.go:50-52`，→§3.3） | null→`-` |
| 可计入需求（X/Y） | 可计入 / 总需求 | 🔢 `eligible_needs`/`total_needs`（→§9） | `0` |
| 已合并需求 | merged 状态需求数 | 🔢 `merged_needs` | `0` |

### ④ Top 提效榜 TopRankCard
| 显示 | 含义 | 口径/来源 |
|---|---|---|
| 需求榜（Top6） | 可计入需求按日历提效降序 | 📅 `efficiency_ratio`（客户端筛选+排序，`TopRankCard.tsx:40-41`） |
| 人榜（Top6） | 用户按日历提效降序 | 📅 用户 `calendar_ratio` |

### ⑤ 规模概览 CountsCard
总仓库 / 分支 / 总用户 / 需求（hint：已合并·可计入）/ 总 Commit / 总代码行 / 活跃用户。均来自 `/v2/dashboard/summary` 的计数聚合（`dashboard_handler_v2.go:117-145`），代码行口径为 commit 表原始 `diff_lines` 之和。

---

## 2. 筛选 / 排序 / 分页

- **无交互筛选器**，唯一隐式过滤是固定的最近 90 天窗口（`Overview.tsx:14-16`），对所有卡片统一生效。
- **Top 榜为客户端排序**（按真实数值降序取 Top6，null 沉底，`TopRankCard.tsx:30-46`）；趋势按周一升序。
- Top 榜需求用"翻页拉全"再客户端取榜（后端单页钳到 200，逐页合并，`endpoints.ts:79-98`）。

---

## 3. 边界与常见质疑应答

**Q：Hero 有时是 4 格、有时 3 格？**
取决于"平台指标服务（chat）"是否启用且取数成功（`HeroSaving.tsx:52`）。启用且成功 → 四格（含 AI 花费、净节省）；否则降级三格（省人天/折合节省成本/综合提效）。chat 请求失败静默，绝不拖垮主数据（主数据走独立接口）。

**Q：省人天会不会是负数？**
不会。省人天对 `传统周期预估 − 实际周期` 做了 `max(0,…)` 下钳（`HeroSaving.tsx:54`），算不出正节省时显示 `-`。

**Q：趋势折线的"周平均提效"和 Hero 的"综合提效"对不上？**
正常。趋势是"每个可计入需求各自日历提效的周算术平均"（看典型需求），Hero 综合是"合计传统周期 ÷ 合计实际周期"（看整体盘子）。两种聚合方式不同，数值不必相等。

**Q：这些卡片的数都剔除异常/不可计入了吗？**
是。省人天、AI 占比、可计入需求等分母均在后端 SQL 内用 `FILTER (coverage_eligible AND NOT *outlier_flag …)` 剔除（`db.go:1544-1548`、`ai_code_ratio.go:51-52`），前端拿到的已是干净聚合值（→§8/§9）。

**Q：AI 花费/成本是谁算的？**
来自外部 chat-indicator-statistics 服务的 `estimated_total_cost`（按其价格表估算），本系统只做展示与区间求和，成本计算口径不在本系统内（见《平台》册）。

---

*行号基于 `feat/needs-user-search-multi-caliber` 分支当前快照。*
