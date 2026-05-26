## Context

This repository is the data production system for AI coding efficiency metrics. Its durable outputs are written by `kbcli` into PostgreSQL and exposed by `backend` APIs. The bundled `frontend/` exists, but this change does not make it the owner of the new v2 reporting experience.

The current production path is task/commit based:

- `kbcli import-conv` imports sessions and conversation rows into `sessions`, `conversations`, and `tasks`.
- `kbcli import-repo` imports commits and computes silica-derived commit fields.
- `kbcli efficiency` aggregates `tasks` and `commits` by user/day into `user_productivity`.
- `backend` exposes `/api/v2/*` read endpoints over these legacy tables.

The three planning documents in `docs/plans/2026-05-21-提效比-*.md` define a v2 target where the metric unit becomes a Need, session activity is split into think/execute/verify/other stages, without-AI baseline work is estimated by three methods, and the final output includes a calendar efficiency ratio, confidence band, confidence level, coverage, and reasons.

The key current gap is event granularity. The database has `conversations` with request/response text, timestamps, token counts, diff lines, and user input, but it does not have a normalized `tool_calls` table. A strict stage splitter cannot be correct unless the pipeline first normalizes tool/event data or explicitly marks degraded conversation-level inference.

## Goals / Non-Goals

**Goals:**

- Add a v2 pipeline that produces Need-level and user-week v2 data outputs from this repository.
- Preserve legacy `kbcli efficiency`, legacy fields, and legacy API behavior.
- Persist enough intermediate data to explain every v2 number: event/stage metrics, Need boundaries, baseline components, fusion weights, confidence signals, and null/outlier reasons.
- Make the pipeline idempotent for a date range and safe to rerun.
- Expose read-only backend APIs for downstream presentation repositories.
- Drive implementation from an end-to-end fixture that exercises raw conversation/repo-like input through CLI, database tables, and backend read APIs.
- Support parallel implementation by assigning disjoint code ownership slices and requiring independent review after each major node.
- Keep all frontend view work out of this change.

**Non-Goals:**

- No modifications to `frontend/src/views/*` for v2 visualization.
- No attempt to infer offline thinking, meetings, Slack, reviewer effort, or cross-Need knowledge transfer.
- No use of low-confidence Need boundaries in team-level efficiency ratio numerators or denominators.
- No clipping of extreme or negative efficiency ratios.
- No replacement or rewrite of the legacy task/commit estimation path.

## Decisions

### 1. Add a parallel v2 command, not a replacement

`kbcli efficiency-v2` will run independently of `kbcli efficiency`. The full import path can be configured to run legacy only, v2 only, or both, with `both` as the target production mode after rollout.

Alternatives considered:

- Modify `kbcli efficiency` in place. Rejected because the old table and formula must remain audit-compatible.
- Fork the repository into a separate metric producer. Rejected because the existing importer, silica code, config, and DB models are valuable shared inputs.

### 2. Normalize events before splitting stages

Add a v2 event table, tentatively `conversation_events`, with normalized event rows derived from raw conversation imports. Each event stores session/request identity, start/end time, event kind, tool name, command text when applicable, touched files when known, source, payload, and parse quality.

Event kinds:

- `message`: user or assistant text without a normalized tool action.
- `edit`: Edit/Write/MultiEdit/patch-like actions and conversation rows with diff output.
- `verify`: recognized test/build/typecheck/lint/check commands and verification reads after execution begins.
- `read`: read/search activities before or during review.
- `other`: shell/data/import/run actions that count toward activity but not the three primary stages.

When raw data lacks tool detail, the importer will create degraded synthetic events from `conversations`: message events from timestamps and text, edit events when `diff_lines > 0`, and other events when activity exists but cannot be classified. Degraded events must lower stage confidence and must be visible in persisted fields.

Alternatives considered:

- Split stages directly from `tasks` start/end and first/last diff. Rejected because it repeats the known error of classifying edit-test-edit loops as execution.
- Require perfect tool-call data before v2 can run. Rejected because current historical data must still produce explainable low-confidence outputs.

### 3. Model Need as the v2 metric unit

Add a `needs` table as the primary v2 output. Need boundary resolution uses the fallback chain from the design documents:

1. PR metadata when available.
2. Non-main branch from `repo_addr` + `repo_branch`.
3. Issue identifier parsed from commit messages or branch names.
4. Author + seven-day file-cluster grouping.
5. Orphan user-week bucket.

Each Need stores `boundary_source` and `boundary_confidence`. `low` and `very_low` Need records remain queryable but do not enter team-level ratio aggregation.

Alternatives considered:

- Use existing `projects` as Need. Rejected because projects are manually curated virtual groups and do not represent the automatic branch/PR boundary required by the v2 metric.
- Keep session/task as the metric unit. Rejected because one feature commonly spans multiple sessions and commits.

### 4. Separate actual-time concepts in storage

The v2 schema will store all actual-time concepts explicitly:

- `total_session_active_person_min`: contributor activity summed without overlap deduplication.
- `estimate_uncovered_human_min`: uncovered commit correction.
- `total_active_work_corrected_min`: actual person work for internal work-efficiency drilldown and density fitting.
- `total_wall_min`: union of session intervals for display.
- `total_calendar_min`: development span minus long idle gaps for the business-facing calendar formula.
- `wait_for_review_min`: merge wait time as a collaboration signal, not part of the formula.

The main v2 business metric is:

```text
efficiency_ratio = 1 - total_calendar_min / baseline_calendar_min
baseline_calendar_min = baseline_fused_work_min / team_work_density_used
```

The internal diagnostic metric is:

```text
work_efficiency_ratio = 1 - total_active_work_corrected_min / baseline_fused_work_min
```

Alternatives considered:

- Use person-time as the main numerator. Rejected for this change because the planning documents settle on calendar efficiency as the business-facing output.
- Use wall-clock as the numerator. Rejected because it creates false gains for simultaneous contributors.

### 5. Keep baseline methods independent in storage

The v2 pipeline stores all three baseline outputs separately before fusion:

- Baseline A: algorithmic stage model. Think and verify use coefficient-based estimates; execution uses a v2 ancient-exec estimator based on filtered commit LOC, files, complexity delta where available, and re-edit/iteration signals. It reuses existing estimator code only as a lower-level primitive where appropriate and does not sum legacy `commit_ancient_minutes`.
- Baseline B: anchor-KNN over METR/internal anchor feature vectors.
- Baseline C: structured LLM estimator at Need level. It reuses existing AI config and LLM transport, but uses a v2 structured prompt and output schema rather than feeding full conversation transcripts.

Fusion uses persisted weights from `baseline_fusion_weights`. Cold start uses deterministic defaults and records that fallback in `reason`.

Alternatives considered:

- Only keep the LLM estimator. Rejected because it is the current single-trust-source problem.
- Only keep the algorithmic estimator. Rejected because it cannot express uncertainty well for high-thinking or high-verification Needs.

### 6. Treat frontend as a consumer, not an implementation target

Backend APIs will expose v2 list/detail/weekly aggregate contracts. Another repository can build the dedicated reporting UI from those contracts. This repository's acceptance is based on generated DB records, API responses, command logs, and tests.

Alternatives considered:

- Add frontend pages here. Rejected by product direction for this change.

### 7. Make E2E validation the spine of the apply phase

The first implementation milestone must create a deterministic E2E fixture harness before the full pipeline exists. The harness will seed or generate raw-ish task conversations, session records, repo/commit inputs, org mappings, anchor records, coefficient defaults, and expected assertions. As implementation progresses, the same harness evolves from direct table seeding to real command execution:

```text
fixture generation
  -> import-conv / import-repo where possible
  -> efficiency legacy
  -> efficiency-v2
  -> DB assertions
  -> backend API assertions
```

The mock data must cover the dimensions that real data is expected to stress:

- Boundary sources: PR, branch, issue, file-cluster, orphan.
- Session shapes: no-edit thinking, edit-test-edit loop, final verification, degraded conversation-only data.
- Work accounting: uncovered commit, low AI code ratio, multi-contributor overlap, long idle gap, wait-for-review.
- Outcomes: merged, active, abandoned, outlier high/low, missing baseline, low confidence.
- Baselines: A-only cold start, A+B, A+B+C, failed LLM, empty anchors, seeded anchors.

The harness may include an optional fetch step for external public sample data such as METR anchor files or other declared benchmark fixtures, but tests must not require network access. Fetched data is cached or transformed into local fixtures so CI and local runs remain deterministic.

Alternatives considered:

- Unit-test-first only. Rejected because the highest risk is pipeline integration across import, DB, CLI, and API boundaries.
- Depend on production developer data. Rejected because there is no current real development dataset and it would make implementation non-reproducible.

### 8. Plan for parallel subagent implementation with review gates

The apply phase should use multiple subagents where available, but only with disjoint write scopes:

- Schema/config worker: `core/models`, config structs, migration/index tests.
- Ingestion/stage worker: v2 event normalization, classifier, stage splitter.
- Need aggregation worker: boundary resolver, actual-time aggregation, uncovered work, quality signals.
- Baseline worker: Baseline A/B/C and fusion math.
- CLI/API worker: `efficiency-v2` command, `import` mode wiring, serve task registration, backend read endpoints.
- E2E validation worker: fixture builder, seed data, E2E commands, DB/API assertions, documentation.

After each major node in `tasks.md`, a separate review subagent should inspect only the completed slice, run or request the relevant tests, and report blocking findings before the task group is marked complete. Review findings must be resolved in the owning slice before the next dependent group proceeds.

Alternatives considered:

- One sequential implementation thread. Rejected because this change is intentionally large and has separable surfaces.
- Unbounded parallel edits. Rejected because overlapping edits to shared files such as `core/models/models.go`, `kbcli/config.go`, and route registration would create avoidable merge conflicts.

## Data Model

The exact Go struct names can follow local style, but the v2 schema must include these logical tables.

### `conversation_events`

Normalized event stream used by the stage splitter.

Required columns include:

- `event_id`, `session_id`, `request_id`, `task_id`
- `user_id`, `repo_addr`, `repo_branch`, `work_dir_id`
- `event_start_ts`, `event_end_ts`, `duration_sec`
- `event_kind`, `tool_name`, `command_text`
- `touched_files` JSON, `payload` JSON
- `source` (`raw_tool`, `conversation_diff`, `synthetic`, `external`)
- `parse_quality` (`exact`, `partial`, `degraded`)
- timestamps for idempotent upsert

### `session_stage_metrics`

One row per session. Stores stage active/wall minutes, feature counts, stage confidence, summary, and summary source.

### `needs`

One row per Need. Stores boundary, status, session/commit IDs, time fields, stage totals, production totals, silica and uncovered-work signals, baseline values, ratio/band/confidence, outlier flag, and reason.

### `user_productivity_v2`

One row per user-week. Stores summed actual/baseline calendar terms before ratio calculation, merged/active/abandoned coverage, high/medium/low boundary coverage, confidence-limited flag, and work-efficiency drilldown.

### `anchor_set`, `baseline_coefficients`, `baseline_fusion_weights`

Support baseline training, cold-start defaults, hold-out error reporting, time decay, and team work density snapshots.

## Pipeline

1. `import-conv` continues to write legacy `sessions`, `conversations`, and `tasks`.
2. V2 event normalization reads `conversations` and raw retained fields, then upserts `conversation_events`.
3. Stage splitter reads `conversation_events`, applies the edit-driven state machine, computes session stage metrics, and writes `session_stage_metrics`.
4. Need resolver reads sessions, commits, optional PR metadata when available, and issue/file signals, then upserts `needs` boundary and membership fields.
5. Need aggregator computes person/wall/calendar time, stage totals, uncovered commit correction, silica signals, quality signals, and status.
6. Baseline estimators compute A/B/C work baselines and persist all component outputs.
7. Fusion computes `baseline_fused_work_min`, `baseline_spread_work_min`, `baseline_calendar_min`, efficiency band, confidence level, outlier flag, and reason.
8. User-week aggregation writes `user_productivity_v2` by summing calendar numerator and denominator terms before computing ratios.
9. Backend read APIs expose Need list/detail and user-week v2 aggregates.

## API Contract

Add read-only endpoints under existing `/api/v2`:

- `GET /api/v2/needs`: paginated Need list with filters for date range, repo, branch, user, status, confidence, boundary source, outlier flag.
- `GET /api/v2/needs/:needId`: Need detail with sessions, commits, stage metrics, baseline components, fusion weights, confidence reasons, and coverage eligibility.
- `GET /api/v2/efficiency`: user-week or team aggregate v2 output for downstream reporting.

These endpoints return persisted values. They must not recompute baselines on request.

## Risks / Trade-offs

- [Risk] Historical data lacks tool-call detail. → Mitigation: persist degraded event parse quality, lower confidence, and keep stage split explainable rather than silently pretending precision.
- [Risk] Branch-only Need boundaries are inaccurate for long-lived branches. → Mitigation: split Needs longer than 30 days and mark lower confidence when fallback rules are used.
- [Risk] LOC-heavy AI output can bias all three baselines together. → Mitigation: persist churn, duplication, post-generation deletion, and feature-dependency risk flags; use anchor hold-out metrics for confidence.
- [Risk] LLM estimator can be unavailable or costly. → Mitigation: make Baseline C nullable, fuse available baselines, and record reason plus low confidence when only one baseline succeeds.
- [Risk] AutoMigrate may be insufficient for complex indexes or JSON defaults. → Mitigation: keep GORM models plus explicit DDL helpers where needed; tests must verify required columns and indexes.
- [Risk] Running v2 in cron could surprise operators. → Mitigation: introduce mode/config gates and default legacy behavior until `both` is intentionally enabled in deployment config.

## Migration Plan

1. Add v2 models and migrations. Existing tables remain untouched.
2. Add event normalization and stage metric computation. Run it against existing imported data without changing legacy outputs.
3. Add Need resolver and Need aggregation.
4. Add baseline estimators and fusion with cold-start defaults.
5. Add `kbcli efficiency-v2` and optional `efficiency_mode` integration.
6. Add backend read APIs.
7. Add tests for idempotency and legacy non-regression.
8. Run the deterministic E2E fixture through CLI, DB assertions, and backend API assertions.
9. Enable `both` mode in config only after local and integration verification.

Rollback is to stop scheduling `efficiency-v2` and continue using legacy `efficiency`. V2 tables are append/upsert outputs and do not block legacy reads.

## Open Questions

- Whether PR metadata will be imported in V1 or only supported when already present in commit data.
- Whether external benchmark anchor files will live in `kbcli` fixtures, config paths, or a seed table loaded by a command.
- Exact LLM model choice for session summaries and Baseline C; this can use existing `ai_estimation` config initially.
- Exact command name for optional external sample-data fetch; default implementation should make this optional and keep deterministic local fixtures as the source of truth.
