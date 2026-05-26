## 1. E2E Fixture Spine and Parallel Work Plan

- [x] 1.1 Define the parallel implementation ownership map for subagents: schema/config, ingestion/stages, Need aggregation, baselines/fusion, CLI/API, and E2E validation; record shared-file ownership before coding starts.
- [x] 1.2 Add deterministic fixture design covering PR, branch, issue, file-cluster, orphan, no-edit, edit-test-edit, uncovered commit, low AI participation, multi-contributor overlap, long idle gap, wait-for-review, active, merged, abandoned, baseline failure, and outlier scenarios.
- [x] 1.3 Add fixture generation or seed helpers that can populate a test database with raw-ish sessions, conversations, commits, org mappings, anchors, coefficients, and expected assertions.
- [x] 1.4 Add a first failing E2E test or command harness that represents the target path: fixture setup → legacy efficiency where applicable → `efficiency-v2` → v2 DB assertions → backend API assertions.
- [x] 1.5 Add optional external sample-data fetch/transform plan for benchmark anchors or public sample data, with local deterministic fixtures remaining sufficient when offline.
- [x] 1.6 Run an independent review session for the E2E fixture spine and parallel work plan before marking section 1 complete.

## 2. Schema and Configuration

- [x] 2.1 Add v2 GORM models for `conversation_events`, `session_stage_metrics`, `needs`, `user_productivity_v2`, `anchor_set`, `baseline_coefficients`, and `baseline_fusion_weights`.
- [x] 2.2 Register v2 models in `core/models.AutoMigrate` and add explicit DDL helpers for required indexes, JSON defaults, and uniqueness constraints that GORM cannot express safely.
- [x] 2.3 Add tests that verify all v2 tables, required columns, and key indexes are created in PostgreSQL.
- [x] 2.4 Extend kbcli config with v2 settings: `efficiency_mode`, stage gap/duration defaults, verification command patterns, uncovered commit margins, idle threshold, confidence thresholds, baseline defaults, and team profile.
- [x] 2.5 Add config loading defaults and tests proving omitted v2 config still produces deterministic cold-start values.
- [x] 2.6 Run the schema/config slice tests plus the E2E harness to the expected current failure point, then run independent review before marking section 2 complete.

## 3. Event Normalization and Stage Metrics

- [x] 3.1 Add tests for normalizing exact tool events, conversation diff events, and degraded conversation-only activity into `conversation_events`.
- [x] 3.2 Implement conversation event normalization as an idempotent upsert from existing `conversations` data.
- [x] 3.3 Add tests for event classification into message/read/edit/verify/other, including shell verification command patterns and non-verification shell commands.
- [x] 3.4 Implement deterministic event classifier and duration inference helpers.
- [x] 3.5 Add tests for the edit-driven stage state machine, including no-edit sessions, first-message edit sessions, edit-test-edit loops, final verification, and degraded confidence.
- [x] 3.6 Implement session stage splitter that writes `session_stage_metrics` with think/exec/verify/other minutes, feature counts, confidence, and reason fields.
- [x] 3.7 Add an idempotency test proving rerunning v2 event normalization and stage splitting does not create duplicate logical rows.
- [x] 3.8 Run ingestion/stage tests plus the E2E harness through `session_stage_metrics`, then run independent review before marking section 3 complete.

## 4. Need Boundary Resolution

- [x] 4.1 Add tests for boundary source precedence: PR, branch, issue identifier, file cluster, and orphan user-week fallback.
- [x] 4.2 Implement Need boundary resolver and deterministic Need ID generation.
- [x] 4.3 Add tests for filtering main/master/develop/release branches out of high-confidence branch Need detection.
- [x] 4.4 Add tests for splitting or flagging Needs whose span exceeds the configured 30-day maximum.
- [x] 4.5 Implement Need membership persistence for sessions, commits, contributors, repo, branch, dev start/end, merge time, status, and confidence fields.
- [x] 4.6 Add tests proving low and very-low boundary confidence Needs are persisted but excluded from team ratio eligibility.
- [x] 4.7 Run Need-boundary tests plus the E2E harness through `needs` membership output, then run independent review before marking section 4 complete.

## 5. Need Aggregation and Actual-Time Fields

- [x] 5.1 Add tests for Need-level person-time, wall-time interval union, calendar span, and long-idle-gap exclusion.
- [x] 5.2 Implement Need actual-time aggregation from session stage metrics and commit membership.
- [x] 5.3 Add tests for uncovered commit detection using the configured pre/post session margins and file-overlap rule.
- [x] 5.4 Implement uncovered commit LOC and estimated uncovered human minutes using a v2 ancient-work estimator path.
- [x] 5.5 Add tests for ai-code-ratio, silica signal, uncovered-work-ratio, churn, duplication, revert, and post-generation deletion signal calculations where source data is available.
- [x] 5.6 Implement Need quality and confidence input signal aggregation with explicit reason strings for degraded or unavailable signals.
- [x] 5.7 Run actual-time aggregation tests plus the E2E harness through populated Need actual/stage/signal fields, then run independent review before marking section 5 complete.

## 6. Baseline A Algorithmic Estimator

- [x] 6.1 Add tests for think-stage coefficient estimation from user chars, read/search counts, turns, planning signals, and missing-feature behavior.
- [x] 6.2 Implement versioned think-stage baseline calculation.
- [x] 6.3 Add tests for execution baseline filtering of lock files, generated files, formatter-only changes, and large deletion review flags.
- [x] 6.4 Implement v2 execution baseline estimator without summing legacy `commit_ancient_minutes`.
- [x] 6.5 Add tests for verification-stage coefficient estimation from test/build counts, re-edits, review reads, and turns.
- [x] 6.6 Implement verification-stage baseline calculation.
- [x] 6.7 Add tests for persisted Baseline A component fields and total work minutes on `needs`.
- [x] 6.8 Run Baseline A tests plus the E2E harness through algorithmic baseline output, then run independent review before marking section 6 complete.

## 7. Baseline B Anchor-KNN

- [x] 7.1 Add tests for anchor feature-vector construction and validation of without-AI minute fields.
- [x] 7.2 Implement anchor loading from `anchor_set` with source, weight, feature vector, and human-labeled/without-AI minutes.
- [x] 7.3 Add tests for KNN distance calculation, top-K selection, inverse-distance weighting, and empty-anchor fallback.
- [x] 7.4 Implement Baseline B calculation and persistence of nullable output plus reason when anchors are unavailable.
- [x] 7.5 Add optional external sample-data fetch or transform command for benchmark anchor inputs, with deterministic cached/local fixture fallback.
- [x] 7.6 Run Anchor-KNN tests plus the E2E harness with seeded and empty-anchor fixtures, then run independent review before marking section 7 complete.

## 8. Baseline C Structured LLM

- [x] 8.1 Add tests for Need structured summary generation from stage metrics, production totals, key decisions, and per-session summaries.
- [x] 8.2 Implement deterministic template-rule session summaries as the zero-cost fallback.
- [x] 8.3 Add tests for structured LLM prompt construction that excludes full raw conversation transcripts.
- [x] 8.4 Implement `callAIForNeedEstimationV4` using existing AI config and LLM transport with strict JSON output parsing.
- [x] 8.5 Add tests for timeout, disabled config, invalid JSON, and failed-call behavior leaving Baseline C nullable with a reason.
- [x] 8.6 Run structured LLM tests plus the E2E harness with disabled/failing LLM fixtures, then run independent review before marking section 8 complete.

## 9. Fusion, Ratio, and Confidence

- [x] 9.1 Add tests for cold-start default fusion weights and team work density snapshot creation.
- [x] 9.2 Implement `baseline_fusion_weights` bootstrap and lookup logic.
- [x] 9.3 Add tests for fusing one, two, and three available baselines with correct low-confidence behavior for single-baseline cases.
- [x] 9.4 Implement fused work baseline, spread, baseline calendar conversion, efficiency ratio, band, work-efficiency drilldown, and null/reason behavior.
- [x] 9.5 Add tests for confidence level classification using spread, feature-dependency risk, silica, AI code ratio, and uncovered-work signals.
- [x] 9.6 Add tests proving ratios are not clipped and outlier flags are set for extreme high/low actual-to-baseline cases.
- [x] 9.7 Implement weekly hold-out/error snapshot fields for baseline support tables where anchors exist.
- [x] 9.8 Run fusion/confidence tests plus the E2E harness through Need ratio, band, confidence, outlier, and reason fields, then run independent review before marking section 9 complete.

## 10. CLI Pipeline

- [x] 10.1 Add tests for `kbcli efficiency-v2` date, start-date, and end-date parameter parsing.
- [x] 10.2 Implement `cmd_efficiency_v2.go` with pipeline steps: normalize events, split stages, resolve Needs, aggregate Needs, compute baselines, fuse, and aggregate user-week outputs.
- [x] 10.3 Add tests proving `kbcli efficiency-v2` can rerun idempotently for the same date range.
- [x] 10.4 Extend `kbcli import` to honor `efficiency_mode = legacy | new | both` while preserving current legacy behavior by default.
- [x] 10.5 Extend `kbcli serve` valid task types and request bodies to support `efficiency-v2` and `import` mode parameters.
- [x] 10.6 Update kbcli Swagger docs generation path if the new serve endpoint annotations require regeneration.
- [x] 10.7 Run the E2E harness through real `kbcli` command invocation, then run independent review before marking section 10 complete.

## 11. User-Week V2 Aggregates

- [x] 11.1 Add tests for `user_productivity_v2` aggregation that sums actual and baseline calendar terms before calculating ratio.
- [x] 11.2 Implement merged high/medium confidence aggregation for ratio terms.
- [x] 11.3 Add tests for coverage fields across high, medium, low-unreported, abandoned, and active work.
- [x] 11.4 Implement coverage-limited flags and reason fields when configured coverage thresholds are breached.
- [x] 11.5 Add tests proving abandoned Needs keep individual metrics but do not enter aggregate ratio terms.
- [x] 11.6 Run aggregate tests plus the E2E harness through `user_productivity_v2`, then run independent review before marking section 11 complete.

## 12. Backend Read APIs

- [x] 12.1 Add backend query helpers and tests for listing v2 Needs with date, repo, branch, user, status, confidence, boundary source, and outlier filters.
- [x] 12.2 Implement `GET /api/v2/needs` returning persisted summary fields without recomputing baselines.
- [x] 12.3 Add backend tests for `GET /api/v2/needs/:needId` including sessions, commits, stage metrics, baseline components, confidence signals, outlier flag, and reason fields.
- [x] 12.4 Implement Need detail endpoint.
- [x] 12.5 Add backend tests for `GET /api/v2/efficiency` user-week/team aggregate output.
- [x] 12.6 Implement v2 efficiency aggregate endpoint.
- [x] 12.7 Register routes in `backend/main.go` and update Swagger docs if annotations are added.
- [x] 12.8 Run backend API tests plus the E2E harness through API assertions, then run independent review before marking section 12 complete.

## 13. Final End-to-End Verification and Documentation

- [x] 13.1 Add or update tests proving `kbcli efficiency` still writes legacy `user_productivity` fields with the existing `CalcEfficiencyRatio` semantics.
- [x] 13.2 Add tests proving v2 code does not read legacy `commit_ancient_minutes` as a v2 baseline input.
- [x] 13.3 Run `go test ./...` in `kbcli`, `backend`, and `core` after implementation.
- [x] 13.4 Run the full deterministic E2E fixture: generate or seed mock data, run import or seed steps, run legacy efficiency, run `efficiency-v2`, verify legacy/v2 table coexistence, and verify backend APIs.
- [x] 13.5 Run the optional external sample-data fetch/transform path if network and source configuration are available; otherwise record the offline deterministic fixture evidence.
- [x] 13.6 Document the v2 command, fixture/E2E command, output tables, and downstream API contract in repository docs without adding frontend implementation instructions.
- [x] 13.7 Run a final independent review session over the full change, including E2E logs, DB assertions, API assertions, and legacy non-regression evidence, before declaring the change ready for archive.
