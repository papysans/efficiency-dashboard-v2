# efficiency_v2_fullrun_output.json 数据分析

## 1. 结论

这次已经改用当前分支的新算法跑完，最终结果体现在两层：

- `needs.efficiency_ratio` / `needs.work_efficiency_ratio`
- `user_productivity_v2.efficiency_ratio` / `user_productivity_v2.work_efficiency_ratio`

和 main 不同，新算法确实产出了提效比：

| 指标 | 数量 |
|---|---:|
| Need 总数 | 605 |
| `needs.efficiency_ratio != 0` | 324 |
| `needs.work_efficiency_ratio != 0` | 541 |
| 周聚合总数 `user_productivity_v2` | 232 |
| `user_productivity_v2.efficiency_ratio != 0` | 114 |
| `user_productivity_v2.work_efficiency_ratio != 0` | 140 |

但这批结果整体置信度偏低：605 个 Need 里 602 个 `confidence_level=low`，320 个 Need 被标记为 outlier。原因不是没有算法结果，而是输入信号导致很多 Need 的日历跨度很短、边界低置信，提效比容易被放大。

## 2. 本次运行

| 项 | 值 |
|---|---|
| 分支 | `feat/efficiency-v2-pipeline` |
| 数据库 | `efficiency_v2_fullrun` |
| task 输入 | `工时估算数据/mnt/user-indicator/raw/task` |
| repo 输入 | `工时估算数据/mnt 2/user-indicator/raw/repo` |
| anchor 输入 | `docs/data/efficiency_v2_anchor_set.csv`，179 条 |
| AI LLM 估算 | 关闭，`ai_estimation.enabled=false` |
| 命令 | `import-conv --force` → `import-repo --force` → `import-org --from-csv` → anchor import → `efficiency-v2 --start-date 20251231 --end-date 20260522` |

完整输出：

- `docs/data/efficiency_v2_fullrun_output.json`

## 3. 表规模

| 表 | 行数 |
|---|---:|
| `sessions` | 12114 |
| `conversations` | 17073 |
| `commits` | 2616 |
| `conversation_events` | 17073 |
| `session_stage_metrics` | 12114 |
| `needs` | 605 |
| `user_productivity_v2` | 232 |
| `anchor_set` | 179 |
| `baseline_coefficients` | 1 |
| `baseline_fusion_weights` | 1 |
| `tasks` | 0 |
| `user_productivity` | 0 |

`tasks=0` 仍然成立，因为原始 conversation diff 没有形成 main/v1 的 silica task。但 v2 不依赖 `tasks` 才能出结果；它用 session stage、commit、Need 边界、baseline/fusion 来计算。

## 4. Need 边界分布

| boundary_source | confidence | status | Need 数 |
|---|---|---|---:|
| `lv1_pr` | high | merged | 86 |
| `lv2_branch` | high | active | 8 |
| `lv2_branch` | high | merged | 253 |
| `lv3_issue` | medium | merged | 6 |
| `lv4_cluster` | low | merged | 125 |
| `lv5_orphan` | very_low | active | 18 |
| `lv5_orphan` | very_low | merged | 109 |

高/中置信可用于正式覆盖口径的 Need 主要是：

- `lv1_pr high merged`: 86
- `lv2_branch high merged`: 253
- `lv3_issue medium merged`: 6

低置信部分仍然不少：

- `lv4_cluster low merged`: 125
- `lv5_orphan very_low merged`: 109
- `lv5_orphan very_low active`: 18

## 5. files 信号效果

这次当前分支已接入 commit raw 的 `files`：

- `commits.touched_files`
- `needs.touched_files`
- `lv4_cluster` 文件聚类

实际结果：

| 指标 | 数量 |
|---|---:|
| `needs.touched_files` 非空 | 239 |
| `total_files_touched > 0` | 239 |
| `lv4_cluster merged` | 125 |

按边界看有文件信号的 Need：

| boundary_source | status | Need 数 | 文件数合计 | loc 合计 |
|---|---|---:|---:|---:|
| `lv1_pr` | merged | 31 | 1251 | 69042 |
| `lv2_branch` | merged | 58 | 3603 | 353607 |
| `lv3_issue` | merged | 4 | 112 | 5359 |
| `lv4_cluster` | merged | 125 | 1742 | 126462 |
| `lv5_orphan` | merged | 21 | 46 | 4855 |

结论：`files` 对当前数据是有效信号。没有它，主干裸 commit 更容易落到 orphan；有它后至少 125 个 Need 被聚成 `lv4_cluster`。

## 6. 提效比结果

Need 级：

| 指标 | 数量 |
|---|---:|
| `efficiency_ratio != 0` | 324 |
| `work_efficiency_ratio != 0` | 541 |
| `efficiency_ratio > 10` | 110 |
| `efficiency_ratio > 100` | 41 |
| `efficiency_ratio < 0` | 91 |
| `work_efficiency_ratio > 10` | 106 |
| `work_efficiency_ratio < 0` | 171 |

周聚合：

| 指标 | 数量 |
|---|---:|
| 周聚合行数 | 232 |
| `efficiency_ratio != 0` | 114 |
| `work_efficiency_ratio != 0` | 140 |
| `efficiency_ratio > 10` | 38 |
| `efficiency_ratio > 100` | 16 |
| `efficiency_ratio < 0` | 30 |

总量口径：

| 指标 | 值 |
|---|---:|
| Need 实际工作量 `total_active_work_corrected_min` | 2462918.43 |
| Need 融合基线工作量 `baseline_fused_work_min` | 2170535.93 |
| Need 实际日历 `total_calendar_min` | 1420310.45 |
| Need 基线日历 `baseline_calendar_min` | 6201531.22 |
| Need 总 LOC | 1229519 |
| Need uncovered LOC | 1227962 |

## 7. 为什么有些提效比极端

提效比按日历口径计算：

`efficiency_ratio = (baseline_calendar_min - total_calendar_min) / total_calendar_min`

所以当 `total_calendar_min` 很小，例如几分钟甚至 0.05 分钟，基线日历只要是几百分钟，结果就会被放大到几千倍。

典型例子：

| Need | source | status | efficiency_ratio | actual calendar | baseline calendar |
|---|---|---|---:|---:|---:|
| `branch:http://100.125.254.48:30080/gdunicom/cims-web.git:xurencheng-` | lv2_branch | merged | 29647.36 | 1.33 | 39531.14 |
| `orphan:syn-3b51515e4b74:2026w21` | lv5_orphan | active | 16606.99 | 0.05 | 830.40 |
| `branch:https://code-ouc.cmhk.com/cmhk/CM-MFS-SIT/manage-config-front.git:zoucc` | lv2_branch | merged | 8764.42 | 0.25 | 2191.36 |

这些不是“没有结果”，而是结果被短日历跨度放大。分析时应该优先看：

- `coverage_eligible=true`
- `boundary_confidence in high/medium`
- `outlier_flag=false`
- `confidence_level != low`
- 同时看 `work_efficiency_ratio`

## 8. 周聚合样例

按 `user_productivity_v2.efficiency_ratio` 排名前几项：

| week_start | user | merged_need_count | efficiency_ratio | work_efficiency_ratio | confidence_limited |
|---|---|---:|---:|---:|---|
| 2026-05-04 | `48c4c432...` | 3 | 30263.46 | -0.17 | false |
| 2026-04-27 | `25ed8c0c...` | 1 | 3316.46 | -0.21 | false |
| 2026-04-27 | `dcff5ea1...` | 5 | 1269.37 | 6.27 | true |
| 2026-04-27 | `a1b49887...` | 5 | 880.24 | 2.93 | false |
| 2026-04-27 | `bcfe6bdd...` | 4 | 473.95 | 0.83 | false |

这里也能看到同一个问题：日历提效比容易被短跨度拉高，因此不能只按 `efficiency_ratio` 排序解读。

## 9. 和 main 结果的区别

main 跑出来：

- `needs` 不存在
- `user_productivity_v2` 不存在
- `tasks=0`
- `silica=0`
- `user_productivity.commit_efficiency_ratio=0`
- 不保存 `files`

v2 跑出来：

- `needs=605`
- `user_productivity_v2=232`
- `needs.efficiency_ratio` 有 324 个非零
- `user_productivity_v2.efficiency_ratio` 有 114 个非零
- `needs.touched_files` 有 239 个非空
- `lv4_cluster=125`

所以“最终算法结果”在 v2 里已经体现出来了；之前 main 没有体现，是因为 main 的算法链路没有 Need 层，也没有在这批数据上形成 task/silica。

## 10. 当前风险

1. 低置信结果太多：605 个 Need 里 602 个 `confidence_level=low`。
2. outlier 太多：320 个 Need 标记为 `outlier_flag=true`。
3. active/orphan 的极端提效比不可直接作为正式结论。
4. 原始 conversation 没有 diff_lines，AI 覆盖率仍然薄弱。
5. 输出 JSON 仍包含完整 repo 地址，部分 URL 有 token-like 信息，外发前必须脱敏。

