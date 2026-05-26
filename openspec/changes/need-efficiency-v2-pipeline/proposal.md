## Why

The current efficiency pipeline reports task-level and commit-level ratios with mixed business meanings, so the final numbers can be mathematically valid but operationally misleading. The repository already collects conversations, sessions, tasks, commits, silica, and productivity aggregates; this change turns those existing inputs into a Need-level v2 output pipeline that downstream reporting systems can consume without requiring this repository to own the final frontend.

## What Changes

- Add a new `kbcli efficiency-v2` pipeline that runs alongside the existing `kbcli efficiency` path and does not overwrite legacy `tasks`, `commits`, or `user_productivity` semantics.
- Add persistent v2 tables for session stage metrics, Need aggregates, anchor data, baseline coefficients, fusion weights, user-week productivity aggregates, and any tool/event records required to support stage splitting.
- Derive Need boundaries from repository activity using a fallback chain: PR when available, then non-main branch, then issue identifier, then file cluster, then orphan weekly bucket.
- Compute session-level stage metrics for think, execution, verification, and other activity, using tool/event data when available and an explicit degraded path when only conversation-level data exists.
- Aggregate sessions and commits into Need-level records with person-time, wall time, calendar span, uncovered commit corrections, silica participation signals, confidence signals, and status.
- Implement three v2 baseline estimators: algorithmic stage model, anchor-KNN model, and structured LLM estimator, then fuse them using stored weights and report uncertainty.
- Compute Need-level and user-week v2 output fields: calendar efficiency ratio, confidence band, confidence level, coverage metrics, outlier flag, reasons, and internal work-efficiency drilldown fields.
- Add backend read APIs for v2 outputs so other presentation repositories can render the data. This repository will expose data contracts only; it will not add or modify the existing frontend views for this change.
- Add an E2E-first validation harness with deterministic fixture generation, optional external sample-data fetch, multi-angle mock scenarios, and repeatable commands that prove the repository can produce v2 data from raw-ish inputs through database outputs and backend APIs.
- Structure the apply work so multiple subagents can implement disjoint slices in parallel, with an independent review subagent gate after each major implementation node before tasks are marked complete.
- Keep the old LLM/task-based estimation path as a legacy comparison rail and keep its command, tables, and formula behavior unchanged.

## Capabilities

### New Capabilities

- `efficiency-v2-ingestion`: Captures or derives the event and session metrics required for v2 stage splitting without breaking existing conversation import.
- `need-boundary-aggregation`: Groups sessions and commits into Need records with boundary confidence, status, time fields, uncovered commit correction, and coverage eligibility.
- `efficiency-v2-baselines`: Estimates without-AI baseline work using algorithmic, anchor-KNN, and structured LLM methods, then stores fusion weights, coefficients, uncertainty, and reasons.
- `efficiency-v2-outputs`: Produces v2 Need and user-week result tables plus backend read APIs for downstream reporting systems.
- `efficiency-v2-validation`: Provides deterministic fixture generation, external sample-data ingestion hooks, and E2E verification commands for the complete v2 data pipeline.

### Modified Capabilities

- None. There are no existing OpenSpec specs in this repository; legacy efficiency behavior remains unchanged by contract.

## Impact

- Affected CLI code: `kbcli/cmd_efficiency.go` remains legacy; new v2 command and supporting modules will be added under `kbcli/`.
- Affected shared models: `core/models/models.go` will gain v2 table models and migration registration.
- Affected backend API: `backend/` will add read-only v2 handlers and query helpers for Need-level and user-week v2 data.
- Affected configuration: `kbcli-config.yaml`, compose, and Helm config may add an `efficiency_mode` or v2 scheduling option, defaulting to legacy-safe behavior.
- Affected tests: Go tests will cover stage splitting, Need boundary resolution, baseline calculations, fusion math, DB models, CLI idempotency, and backend v2 API contracts.
- Affected validation data: new fixture builders and mock datasets will cover PR/branch/issue/file-cluster/orphan boundaries, no-edit thinking sessions, edit-test-edit loops, uncovered commits, low silica, abandoned work, long idle gaps, multi-contributor overlap, baseline failures, and outlier ratios.
- Out of scope: frontend dashboard changes in `frontend/`; downstream presentation is handled by another repository.
