# Efficiency V2 Pipeline

This document captures the v2 pipeline introduced by the OpenSpec change
`need-efficiency-v2-pipeline`. The legacy `kbcli efficiency` command, its
output tables (`tasks`, `commits`, `user_productivity`), and the
`utils.CalcEfficiencyRatio` formula are preserved unchanged. The v2 path
is additive.

## Commands

### `kbcli efficiency-v2`

Runs the v2 pipeline against the configured stats database.

```bash
kbcli efficiency-v2 --date YYYYMMDD
kbcli efficiency-v2 --start-date 20260518 --end-date 20260524
```

Steps performed (idempotent — rerun produces the same logical rows):

1. Ensure `baseline_coefficients` and a cold-start `baseline_fusion_weights` row exist.
2. Normalize `conversations` rows into `conversation_events`.
3. Build per-session `session_stage_metrics`.
4. Resolve Need boundaries and upsert into `needs`.
5. Aggregate Need actual time, stage totals, uncovered commits, and quality
   signals.
6. Compute Baseline A (algorithmic), Baseline B (anchor-KNN), Baseline C
   (structured LLM) for each Need. LLM is optional and gracefully degraded
   when AI estimation is disabled.
7. Fuse baselines using stored weights; compute calendar ratio, confidence
   band, work-efficiency ratio, confidence level, and outlier flag.
8. Aggregate `user_productivity_v2` rows.

### `kbcli import --efficiency-mode <legacy|new|both>`

The `import` command honors the `efficiency_mode` config (default `legacy`):

- `legacy`: runs legacy `efficiency` only (default — fully backwards compatible).
- `new`: runs `efficiency-v2` only.
- `both`: runs legacy then v2.

### `kbcli serve`

Accepts the `efficiency-v2` task type and `efficiency_mode` parameter on
`import` request bodies.

## Output Tables

| Table | Purpose |
|-------|---------|
| `conversation_events` | Normalized v2 event stream (`raw_tool`, `conversation_diff`, `synthetic`). |
| `session_stage_metrics` | Per-session think/exec/verify/other minutes + counts. |
| `needs` | One row per Need. Includes boundary source, actual time, baseline components, fusion outputs, confidence, and outlier flag. |
| `user_productivity_v2` | Per-user-per-week aggregates with coverage fields. |
| `anchor_set` | Anchor records used by Baseline B (Anchor-KNN). |
| `baseline_coefficients` | Versioned coefficients for Baseline A. |
| `baseline_fusion_weights` | Fusion weight snapshots (cold-start defaults seeded on first run). |

## Read API Contract (downstream consumers)

All endpoints return persisted v2 fields; baselines are never recomputed
during request handling.

- `GET /api/v2/needs` — paginated Need list with filters: `startDate`,
  `endDate`, `repoAddr`, `repoBranch`, `userId`, `status`, `boundarySource`,
  `boundaryConfidence`, `confidenceLevel`, `outlierOnly`.
- `GET /api/v2/needs/:needId` — Need detail including sessions, commits,
  stage metrics, baseline components, fusion inputs, quality/confidence
  signals, and outlier flag.
- `GET /api/v2/efficiency` — user-week aggregate output. Filters:
  `startDate`, `endDate`, `userId`.

### Aggregation 过滤约束（重要）

设计 doc 2026-05-21-提效比-design.md §6.2.1 明确规定：**`boundary_confidence ∈ {low, very_low}` 的 Need 不参与团队/用户聚合统计**。`user_productivity_v2` 已按 `status='merged' AND boundary_confidence IN ('high','medium')` 过滤。

对 `needs` 表做即席查询时（dashboard、报表、SQL ad-hoc），如果要计算聚合提效比，**必须**加：

```sql
WHERE status = 'merged'
  AND boundary_confidence IN ('high','medium')
  AND NOT outlier_flag
```

否则 active / very_low confidence 的 need 会污染 aggregate（它们的 dev_span 在涨但 baseline 因 LoC 少而偏小，把 ratio 拉负）。

## E2E Fixture

`kbcli/efficiency_v2_fixture.go` provides a deterministic local fixture
covering boundary variants (PR, branch, issue, file-cluster, orphan),
session shapes (no-edit, edit-test-edit, low AI, multi-contributor),
work-accounting variants (uncovered commit, idle gap, wait-for-review),
and baseline variants (A-only, A+B, A+B+C, anchor empty, LLM
disabled/failed). Tests in `efficiency_v2_*_integration_test.go` exercise
the full pipeline against the configured Postgres instance.

## Coefficient and Anchor Sourcing

- Coefficients default to deterministic cold-start values from
  `DefaultEfficiencyV2BaselineACoefficients()`. Operators can override per
  version by inserting rows into `baseline_coefficients` (key:
  `coef_version`).
- Anchor records are seeded from fixtures during local testing. The
  optional external sample-data fetch plan is documented at
  `EfficiencyV2OptionalAnchorFetchPlan()` and runs only when explicitly
  requested; deterministic local fixtures remain the source of truth for
  CI.

## Legacy Non-Regression Guarantees

- `kbcli efficiency` still writes `user_productivity` using the legacy
  `utils.CalcEfficiencyRatio` semantics.
- V2 code paths do not read `commit_ancient_minutes` from the `commits`
  table. The execution baseline is computed fresh from filtered LOC; see
  `computeEfficiencyV2BaselineExec` and the
  `TestEfficiencyV2DoesNotReadLegacyCommitAncientMinutes` regression test.

## Known Limitations Surfaced in Output

- **Covered-commit rule is temporal-only.** The spec calls for an "AND
  shares touched files" rule alongside the pre/post session margin, but
  `models.Commit` does not currently carry a touched-files list. The
  pipeline therefore classifies commits as covered/uncovered using
  session-time windows only, and surfaces `covered_rule=temporal_only`
  in every Need's `quality_signals.reason`.
- **Churn / duplication / post-generation deletion** ratios are not yet
  computed (no source data); they remain `NULL` and are excluded from the
  upsert column list to avoid clobbering future writers.
- **Hold-out MAE per baseline** (`baseline_fusion_weights.hold_out_mae_*`)
  is a placeholder. `ComputeEfficiencyV2WeeklyHoldOutError` returns `NaN`
  until anchor partitioning is implemented; callers persist `NULL` rather
  than misleading values.
- **`AggregateAndUpsertEfficiencyV2UserProductivity`** now respects the
  pipeline's date range and only touches Needs whose dev window overlaps
  the requested period. Reruns on a narrow window leave other weeks
  untouched.
- **Backend list date filter** accepts both `YYYYMMDD` and `YYYY-MM-DD`
  for `startDate`/`endDate` to match the CLI surface. The list query uses
  a permissive overlap predicate (`dev_end_ts >= start OR merge_ts >= start`
  combined with `dev_start_ts <= end OR dev_end_ts <= end`). If you need
  strict interval-overlap semantics, file an enhancement.
