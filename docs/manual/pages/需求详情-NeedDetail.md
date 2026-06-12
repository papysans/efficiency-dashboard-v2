# 页面说明书 · 需求详情（NeedDetail）

> 路由 `/kanban/needs/:needId`（`frontend-react/src/router.tsx:52`，basename `/kanban` 见 `:84`）
> 组件 `frontend-react/src/pages/needs/NeedDetail.tsx`
> 数据接口 `GET /api/v2/needs/{needId}`（`frontend-react/src/api/endpoints.ts:69`）

---

## 0. 这个页面是什么

单个**需求（Need）**的端到端提效明细页。一个 Need 代表"一项被识别出边界的开发工作"（通常对应一个特性分支/一段连续开发），系统把这段工作期间的 AI 编程会话（Session）、代码提交（Commit）聚合到一起，算出两套提效指标并展示其分解过程。

**谁看**：汇报人用顶部 6 张指标卡讲"这个需求 AI 提效多少"；技术对接方用下方基线分解表、工时明细、质量信号核对"这个数怎么来的"。

页面自上而下 9 个区块：① 标题与状态标签 → ② 6 张提效指标卡 → ③ 基础信息 → ④ 基线分解表 → ⑤ 阶段工作量 → ⑥ 代码与质量信号 → ⑦ 改动文件 → ⑧ 关联 Sessions → ⑨ 关联 Commits（默认收起）。

> **一个结构性事实**（影响全表理解）：详情接口不做逐字段 SELECT，而是把整行 `Need` 记录直接序列化返回（`backend/needs_v2_handler.go:424` `QueryNeedV2Detail`）。因此"响应字段 → 数据库列"的权威映射来自数据模型 `core/models/efficiency_v2_models.go` 的 GORM `column:` 标签，下表的 DB 列名均据此。后端 `sessions` 与 `stage_metrics` 是同一份数据（`needs_v2_handler.go:444`）。

---

## 1. 字段速查表

口径标记：📅=日历口径，🧑=人力/活跃口径，🔢=计数/比率，🏷️=状态标签。空态列说明该字段无数据时页面如何显示。

### ① 标题与状态标签（`NeedDetail.tsx:170-198`）

| 显示 | 含义 | 口径 | 空态 |
|---|---|---|---|
| 需求 ID（副标题） | 需求主键 `need_id` | — | 空→`-` |
| 状态标签 | merged/active 等需求状态 | 🏷️ | 有值才显；merged 绿/active 蓝 |
| 效率置信 X | 提效比的置信档（high/medium/low/very_low） | 🏷️ | 有值才显 → 见 §2.1 |
| 可计入 / 未计入 | 该需求是否计入提效统计 | 🏷️ | 恒显 → 见 §2.2 |
| 日历异常 | 日历口径提效比触发异常隔离 | 🏷️ | 仅 true 显 → 见 §2.3 |
| 工作量异常 | 人力口径提效比触发异常隔离 | 🏷️ | 仅 true 显 → 见 §2.3 |
| 异常样本 | 旧接口兜底异常标签 | 🏷️ | 仅旧数据显（无分口径 flag 时）|

### ② 提效指标卡（`NeedDetail.tsx:201-208`，6 张卡）

| 卡片 | 字段 | 含义 | 口径 |
|---|---|---|---|
| 日历提效 | `efficiency_ratio` | （传统周期预估 − 实际周期）/ 实际周期 | 📅 → §2.4 |
| 人力提效 | `work_efficiency_ratio` | （传统人力预估 − 实际人力）/ 实际人力 | 🧑 → §2.4 |
| 实际周期 | `total_calendar_min` | 自然时间跨度（扣长搁置） | 📅 → §2.5 |
| 传统周期预估 | `baseline_calendar_min` | 不用 AI 时该需求预计的自然周期 | 📅 → §2.4 |
| 实际人力 | `total_active_work_corrected_min` | 实际活跃投入 + 未覆盖补估 | 🧑 → §2.5 |
| 传统人力预估 | `baseline_fused_work_min` | 不用 AI 时预计的人力（多路融合） | 🧑 → §2.4 |

> 日历提效卡下方的"区间 X% ~ Y%"是提效比的置信区间（`efficiency_band_low/high`），两端皆空时不显示。

### ③ 基础信息（`NeedDetail.tsx:211-252`）

| 显示 | 字段 | 含义 | 空态 |
|---|---|---|---|
| 边界来源 | `boundary_source` | 该需求边界是如何识别的（分支/episode 等） | 空→`-` |
| 边界置信 | `boundary_confidence` | 边界识别的置信档 | 空→`-` |
| 边界标识 | `boundary_key` | 边界唯一键 | 空→`-` |
| 仓库 | `repo_addr` | 仓库地址，可点击跳转仓库详情 | 空→`-` |
| 分支 | `repo_branch` | 分支名 | 空→`-` |
| 主用户 | `primary_user_id` | 主贡献者，显示真名（UUID 经映射），可点击 | 空→`-` |
| 协作人数 | `contributor_user_ids` 长度 | 协作者计数 | 非数组→`-` |
| 开始/结束时间 | `dev_start_ts`/`dev_end_ts` | 开发起止时间 | 空→`-` |
| 开发跨度 | `dev_duration_min` | 结束−开始的原始分钟（未扣搁置） | 0/空→`-` |

> 主用户/会话用户显示的是真名：UUID 经 `GET /v2/user-names` 映射为真名/工号（`endpoints.ts:136`），命中显真名，否则原样回显 UUID。

### ④ 基线分解表（`NeedDetail.tsx:255-281`）—— "传统人力预估"如何拼出来

三列：来源 / 估算（分钟）/ 说明。展示融合前的各路基线：

| 行 | 字段 | 含义 | 是否常显 |
|---|---|---|---|
| 算法基线 | `algo_total_min`（+思考/执行/验证分项） | 古法工时模型按代码量估时 | 常显 → §2.6 |
| 相似锚点 kNN | `anchor_knn_min` | 用历史相似需求的真实工时估时 | 常显 → §2.7 |
| LLM 估算 | `llm_total_min` | 大模型阅读上下文后的工时估计 | **仅有值时显**（禁用/失败则整行隐藏）→ §2.8 |
| 传统人力预估（融合） | `fused_work_min` | 上述各路加权融合的最终值 | 常显 → §2.4 |

表下脚注显示"传统周期预估（日历口径）"= `baseline_calendar_min`（仅有值时显）。

### ⑤ 阶段工作量（`NeedDetail.tsx:284-294`）

| 显示 | 字段 | 含义 | 空态 |
|---|---|---|---|
| 思考 | `total_think_min` | 思考阶段活跃分钟（粗略估算口径） | 0/空→`-` |
| 执行 | `total_exec_min` | 执行阶段活跃分钟（粗略估算口径） | 0/空→`-` |
| 验证 | `total_verify_min` | 验证阶段分钟 | **0/空→`—`（采集未覆盖）→ §2.9** |
| 其他 | `total_other_min` | 其他活跃分钟 | 0/空→`-` |
| 会话活跃人工 | `total_session_active_person_min` | 去重后的真实活跃人·分钟 | 0/空→`-` → §2.5 |
| 未覆盖人工估算 | `estimate_uncovered_human_min` | 采集未覆盖部分的补估 | 0/空→`-` → §2.5 |

### ⑥ 代码与质量信号（`NeedDetail.tsx:297-315`）

| 显示 | 字段 | 含义 | 空态 |
|---|---|---|---|
| 净代码行 | `total_loc_net` | 有效改动行数（治理后口径）| null→`-`，0 正常显示 |
| 改动文件 | `total_files_touched` | 触达文件数 | 同上 |
| 提交数 | `commit_count` | 关联 commit 数 | 同上 |
| AI 代码占比 | `ai_code_ratio` | AI 覆盖行 / 净代码行 | **null 或 0→`-`** |
| AI 覆盖行 | `ai_covered_loc` | 判定为 AI 产出的行数 | null→`-` |
| 未覆盖行 | `uncovered_loc` | 非 AI 覆盖行 | null→`-` |
| 未覆盖工作占比 | `uncovered_work_ratio` | 未覆盖行 / 净代码行 | **null 或 0→`-`** |
| AI 代码占比信号 / 未覆盖工作信号 | `ai_code_ratio_signal` / `uncovered_work_signal` | 上述比率的档位标签（ok/warn/risk）| 空→"未知" |

### ⑦ 改动文件（`NeedDetail.tsx:317-341`）

`touched_files` 文件名标签云，默认显示前 24 个，超出有"展开全部"按钮；空→"暂无改动文件"。

### ⑧ 关联 Sessions（`NeedDetail.tsx:343-400`）

每个 AI 编程会话一行：Session ID（截 8 位）/ 用户（真名，可点）/ 开始 / 结束 / 活跃工作量 / 思考 / 执行 / 验证（`—`）/ 阶段置信 / 摘要。
**服务端排序**：`total_active_min DESC, session_start_ts ASC`（`needs_v2_handler.go:439-440`，有执行活动的会话排前）。空→"暂无 Session"。

### ⑨ 关联 Commits（`NeedDetail.tsx:402-478`，默认收起）

每个提交一行：Commit ID（截 10 位，可点）/ 提交时间 / 用户（真名）/ 代码行 `diff_lines`（**原始行，非治理后**）/ AI 代码占比 `silica`（**0→`-`**）/ 提交说明 / 改动文件。
**服务端排序**：`commit_time DESC`（`needs_v2_handler.go:447`）。空→"暂无 Commit"。

> ⚠️ **口径区别**：⑨ 这里的"AI 代码占比"是 commit 级的 `silica`（单提交 AI 证据），与 ⑥ 需求级的 `ai_code_ratio` 不是同一指标，前端复用了同一文案。详见 §2.10。

---

## 2. 算法与出处（计算字段详解）

> 共享算法的完整推导见《01-算法白皮书》；本节给出本页字段用到的关键公式与代码出处。

### 2.1 效率置信档 `confidence_level`
`classifyEfficiencyV2Confidence`（`kbcli/internal/efficiencyv2/efficiency_v2_fusion.go:281-320`）。降档条件（任一命中即 low）：可用基线只有 1 条 / 融合值≤0 / 离散度 `spread/fused` > 0.30 / AI 代码占比 < 0.30 / 未覆盖工作占比 > 0.30 / silica < 0.30；离散度 > 0.15 → medium；否则 high。阈值见 `config.go:179` 等。

### 2.2 资格判定 `coverage_eligible`（可计入/未计入）
三步（《01-算法白皮书》详述）：
1. 初判 `status=="merged" 且 边界置信∈{high,medium}`（`efficiency_v2_need_boundary.go:607`，注意此处是**边界置信** `boundary_confidence` 而非效率置信）；
2. 若 `total_calendar_min<=0` 改回"未计入"，写 reason"有交付物但日历为 0"（`efficiency_v2_need_aggregate.go:153-163`）；
3. 若该需求无任何 Session（commit-only need）改回"未计入"（`:166-168`）。
→ 这解释了"已 merged 却显示未计入"，见 §3 常见质疑 Q2。

### 2.3 异常隔离 outlier（日历异常 / 工作量异常）
`efficiency_v2_fusion.go:238-271`：
- **工作量异常**：`实际人力/融合基线` > 5 或 < 0.10 → true（`:240-245`）；
- **日历异常**：`efficiency_ratio` > 10.0 或 < −2.0 → true（`:250-255`）；
- LOC 速率 `净代码行/日历分钟` > 7.0 → 两个 flag 都打（`:259-268`）。
阈值见 `config.go:206-216`。**异常只打标签 + 记 reason，不裁剪数值**（`efficiency_v2_fusion.go:249` 注释），所以指标卡仍显示极端原值。可配置的排除范围 `exclusion.scope` 默认三类全开（`config.go:53-57`），范围为空则永不打标。

### 2.4 两套提效比与融合基线（核心）
融合（`efficiency_v2_fusion.go:123-174`）：取 algo/knn/llm 三路中"值≥0"的可用基线，按权重归一加权 `fused += value*(w/Σw)`。默认权重 algo=0.30 / knn=0.45 / llm=0.25（`config.go:232-238`）。
- **传统周期预估** `baseline_calendar_min` = `(fused/density)*calib`，density 默认 0.25、calib 默认 1.0（`fusion.go:200`，`config.go:226,241`）。
- **日历提效** `efficiency_ratio` = `(baseline_calendar_min − total_calendar_min)/total_calendar_min`（`fusion.go:204`）。
- **人力提效** `work_efficiency_ratio` = `(fused − 实际人力)/实际人力`（`fusion.go:228`）。
- 无任何可用基线 → 置信=unknown，reason `fusion:no_baselines`（`:146-149`）。

### 2.5 实际工时（日历 vs 人力）
- **实际周期** `total_calendar_min` = 开发开始→结束，减去超过 idle 阈值（默认 3 天）的搁置（`computeEfficiencyV2DevCalendarMinutes`，`efficiency_v2_need_aggregate.go:146,186-248`）。
- **实际人力** `total_active_work_corrected_min` = `会话活跃人工 + 未覆盖人工补估`（`:380`）。
  - 会话活跃人工 `total_session_active_person_min`：按 user 分组、对并行 session 的活跃区间取并集去重（`efficiencyV2NeedPersonMinutes`，`:286-332`），避免多会话/多人重复计时。
  - 未覆盖人工补估 `estimate_uncovered_human_min` = 未覆盖代码行经算法折算的人工（`:379`）。

### 2.6 算法基线（古法工时）
`efficiency_v2_baseline_algo.go:136-149`：思考 + 执行 + 验证求和。执行阶段按"有效代码行 ÷ 每分钟行数"估时，并用 `classifyEfficiencyV2ExecFiles` 过滤 lock/生成文件（`:194-247,327-364`）。

### 2.7 相似锚点 kNN（Method B）
`efficiency_v2_baseline_knn.go:254-265`：取 k 个相似历史需求，逆距离加权 `estimate = Σ(value·w)/Σw`，权重 `w = Weight/(distance+1)`。说明列形如 `knn:k=5`。详见《01-算法白皮书》。

### 2.8 LLM 估算
`efficiency_v2_baseline_llm.go`：大模型读取需求上下文后给出工时估计与置信。**禁用或调用失败时 `llm_total_min` 为 null，④ 基线表整行隐藏**（`NeedDetail.tsx:143`），reason 形如 `llm:disabled` / `llm:call_failed`。

### 2.9 验证阶段为何恒为 `—`
`total_verify_min` 用 `formatVerifyMin`（`formatters.ts:107`）：0/空→全角 `—`。根因：**当前采集口径不记录"命令执行"类事件**，验证阶段普遍为 0。这是数据采集覆盖的结构性缺失，非计算错误。tip 文案见 `formatters.ts:103`。→ §3 常见质疑 Q4。

### 2.10 commit 级 `silica` vs 需求级 `ai_code_ratio`
- 需求级 `ai_code_ratio` = `ai_covered_loc / total_loc_net`（`efficiency_v2_need_aggregate.go:420`）。
- commit 级 `silica` 是单提交的 AI 代码匹配证据（`core/models/models.go:101`，float64 默认 0）。
前端 ⑥ 和 ⑨ 都用"AI 代码占比"文案，但底层是两个不同口径的量。**汇报时应区分措辞**：需求页主指标用 `ai_code_ratio`；commit 行的是单提交证据。

---

## 3. 边界与常见质疑应答

> 以下都是"看起来像 bug、实为数据限制或设计选择"的点，汇报时最可能被追问。每条给出触发条件与代码出处。

**Q1：提效比是个夸张的大数（比如 +1500%），是不是算错了？**
不是算错，是该需求触发了异常隔离但**指标不裁剪**。当 `efficiency_ratio` > 10 或人力比 > 5 时，系统打"日历异常/工作量异常"标签并把它移出统计池，但卡片仍显示真实原值（`efficiency_v2_fusion.go:238-271`，`:249` 明确不 clip）。极端值通常源于一侧数据不完整（活跃工时偏小或日历偏短）。看到异常标签即说明该样本已被隔离、不污染总体。

**Q2：这个需求明明已经 merged，为什么显示"未计入"？**
资格判定在初判通过后还有两道否决：实际周期 `total_calendar_min<=0`（有交付物但日历为 0，常见于 commit-only 的需求），或该需求下没有任何 AI 会话 Session。命中任一即改回"未计入"（`efficiency_v2_need_aggregate.go:153-168`）。这是为了不让缺少时间侧数据的样本虚增提效。

**Q3：净代码行 `total_loc_net` 比 commit 的 `diff_lines` 之和小很多？**
是的，二者口径不同。⑨ commit 行展示的 `diff_lines` 是**原始改动行**；⑥ 的 `total_loc_net` 是**治理后的有效行**——治理会对超大 build/merge/生成文件做软上限、对纯注释/纯文档降权或排除。差额即被治理挤掉的"水分"。

**Q4：验证阶段（思考/执行/验证里的验证）为什么全是 `—`？**
当前数据采集不记录"命令执行"类事件，验证阶段无数据来源，故恒为 0、显示为 `—`（`formatters.ts:107`，`:103`）。这是采集覆盖的结构性缺失，思考/执行为粗略估算口径、有兜底所以总有值。补齐需上游采集增加相应事件。

**Q5：AI 代码占比 / commit 的 silica 显示 `-`，是没数据吗？**
不一定。这些字段用 `fmtPct`（`NeedDetail.tsx:95`，0 也当 `-`）；commit 的 `silica` 是非指针 float64、默认 0。所以"真值为 0"和"无数据"在界面上都呈现 `-`。需结合是否有 commit/是否 `total_loc_net>0` 判断（无 commit 时后端会把这些字段置 null + reason `no_commits`/`zero_loc_commits`，`efficiency_v2_need_aggregate.go:402-418`）。

---

*出处行号基于 `feat/needs-user-search-multi-caliber` 分支当前快照。*
