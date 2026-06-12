# 页面说明书 · 需求列表（NeedList）

> 路由 `/kanban/needs-v2`（`router.tsx:51`；旧路径 `/kanban/need` 等都重定向到此）
> 组件 `frontend-react/src/pages/needs/NeedList.tsx`
> 数据接口 `GET /api/v2/needs`（`endpoints.ts:66`），后端 `needs_v2_handler.go:95` `listNeedsV2`

---

## 0. 这个页面是什么

按**需求边界（Need）**维度列出提效比的总表。一眼看"哪些需求 AI 提效多少、谁做的、在哪个仓库分支"。字段口径与《需求详情》同源（同一 `models.Need` 持久化列），详情页字段释义见《需求详情-NeedDetail》，本页侧重列、筛选、排序、口径过滤。

> 提效相关列走 **V2 融合口径（小数）**（→《01-算法白皮书》§6.2）。

---

## 1. 列速查表（12 列）

后端 `summarizeNeed`（`needs_v2_handler.go:476-508`）从 `models.Need` 投影，**不重算基线**。

| 列 | 字段 | 含义 | 口径 | 空态 |
|---|---|---|---|---|
| Need ID | `need_id` | 需求主键，可点进详情 | — | 空→`-` |
| 日历提效 | `efficiency_ratio` | （传统周期−实际周期）/实际周期 | 📅 小数×100 | null→`-` |
| 人力提效 | `work_efficiency_ratio` | （传统人力−实际人力）/实际人力 | 🧑 小数×100 | null→`-` |
| AI 代码占比 | `ai_code_ratio` | AI 覆盖行/净代码行 | 🔢 小数×100 | null→`-` |
| 仓库 / 分支 | `repo_addr`/`repo_branch` | 仓库地址、分支 | — | 空→`-` |
| 主用户 | `primary_user_id` | 主贡献者，显示真名(工号)，可点 | — | 空→`-` |
| 实际周期 | `total_calendar_min` | 自然时间跨度（扣长搁置） | 📅 | 0/空→`-` |
| 传统周期预估 | `baseline_calendar_min` | 不用 AI 预计自然周期 | 📅 | 0/空→`-` |
| 实际人力 | `total_active_work_corrected_min` | 实际活跃投入+未覆盖补估 | 🧑 | 0/空→`-` |
| 传统人力预估 | `baseline_fused_work_min` | 不用 AI 预计人力（融合） | 🧑 | 0/空→`-` |
| 记录时间 | `dev_start_ts` | 开发开始时间 | — | 空→`-` |

> 主用户/UUID→真名：经 `GET /v2/user-names`（dept-sync 权威表）映射，命中显真名(工号)，否则回退原 UUID（`useUserNameMap`）。

---

## 2. 筛选 / 排序 / 分页

### 筛选（6 项，均服务端过滤）
日期范围 / 仓库地址（精确等值）/ 分支（精确等值）/ **用户 ID（多口径反查）** / 边界来源 / 仅异常 / 显示全部。输入框草稿态，点"查询"或回车才生效；所有输入三层 `.trim()`。

> **用户 ID 多口径反查**（`needs_v2_handler.go:390-422`）：搜索框输真名、工号、git 名或 UUID 都能查——后端把 term 经 `dept_user`（真名子串/工号精确/universal_id）和 `commits.git_user_name`（内网"AI_真名工号"格式）反查成主用户 UUID 候选集，再 `WHERE primary_user_id IN (候选集)`。候选集恒含原值，故已知 UUID 精确查询向后兼容。

### 排序（**全部服务端 ORDER BY**）
可排序 6 列：日历提效 / 人力提效 / AI 代码占比 / 实际周期 / 传统周期预估 / 记录时间（白名单 `needSortFields`，`sort.go:10`）。三态循环（无→升→降）。
- 排的是**数据库原值**（如 `efficiency_ratio` 的真实小数），不是界面显示的百分比字符串。
- **复合排序固定前缀**：孤儿边界/极低置信需求（`lv5_orphan` 或 `very_low`）**永远沉底**，用户选的排序只在该桶内生效（`needs_v2_handler.go:367-369`）。

### 分页（服务端）
pageSize 选项 20/50/100/200，默认 20，**上限 200**（前后端双重钳制）。

---

## 3. 看板口径与异常过滤

### 看板口径（默认开，"显示全部"放开）
未勾"显示全部"时，列表只显**已交付且非主干分支**的需求：`status≠'active'` 且 `repo_branch` 不在 `main/master/develop/release(/*)`（`db.go:1532-1535`）。理由：主干提交是落到兜底桶的散落提交，与"分支=需求"口径不一致。勾"显示全部"放开此过滤。

### 异常过滤（默认隐藏，不是标注）
**列表对异常需求是"过滤掉"而非打标签**（与详情页相反）：默认 `WHERE NOT outlier_flag`（撞了配置范围内异常类别的需求被排除）；勾"仅异常"则只看异常需求（排查用）。异常判定阈值见《需求详情》§2.3 与白皮书 §8。

---

## 4. 边界与常见质疑应答

**Q：列表里看不到某个需求？**
可能被看板口径（active/主干分支）或异常过滤隐藏了。勾"显示全部"放开看板口径，勾"仅异常"看被隔离的异常需求。

**Q：搜用户搜不到？**
确认无多余空格（已自动 trim）。多口径反查依赖三表 user 主键同源（dept-sync universal_id ↔ 看板 user_id），内网实测约 98.6% 命中；个别未同步的会落回 UUID 精确匹配。

**Q：列表和详情同一需求的"AI 代码占比"显示不同（0.0% vs -）？**
是显示约定差异：列表对 0 值显 `0.0%`，详情对 0 值显 `-`，底层是同一个 `ai_code_ratio`。

---

*行号基于 `feat/needs-user-search-multi-caliber` 分支当前快照。*
