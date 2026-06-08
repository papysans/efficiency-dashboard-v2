# Efficiency V2 Apply Plan

This document is the implementation coordination plan for
`openspec/changes/need-efficiency-v2-pipeline`. It exists so the apply phase can
use parallel subagents without overlapping ownership or losing the E2E-first
validation thread.

## Success Criteria

- The repository can generate deterministic mock data for the v2 pipeline.
- The E2E harness can drive fixture setup, legacy efficiency, `efficiency-v2`,
  database assertions, and backend API assertions as the implementation grows.
- Each major task group is reviewed by a separate subagent before its checkboxes
  are marked complete.
- The final output remains this repository's natural product: CLI commands,
  database tables, API responses, logs, and docs. No frontend implementation is
  part of this change.

## Parallel Ownership Map

| Slice | Primary ownership | Write scope | Integration notes |
|---|---|---|---|
| Schema/config | v2 table models, AutoMigrate registration, config structs, migration tests | `core/models/*`, `kbcli/config.go`, config YAML examples | Owns shared model registration during section 2 |
| Ingestion/stages | event normalization, classifier, duration inference, stage splitter | new `kbcli/efficiency_v2_*` files, narrow importer hooks if needed | Must not change legacy import semantics |
| Need aggregation | boundary resolver, membership persistence, actual time, uncovered work, quality signals | new `kbcli/need_*` or `kbcli/efficiency_v2_need_*` files | Reads stage outputs and commits |
| Baselines/fusion | Baseline A/B/C, anchor loading, LLM v4 wrapper, fusion, confidence | new `kbcli/baseline_*` files, existing LLM transport only via small wrapper | Must not read legacy `commit_ancient_minutes` as v2 baseline input |
| CLI/API | `efficiency-v2` command, `import` mode, serve task registration, backend read endpoints | `kbcli/cmd_*`, `kbcli/exec_cmd.go`, `backend/*`, route registration | Shared files are coordinator-owned when multiple slices need edits |
| E2E validation | fixture builder, seed helpers, E2E harness, API/DB assertions, docs | `kbcli/efficiency_v2_fixture*`, `docs/*`, E2E tests | Starts first and remains the integration spine |

## Shared File Rules

- `core/models/models.go`: schema/config owner integrates model registration.
- `kbcli/config.go`: schema/config owner integrates config shape and defaults.
- `kbcli/cmd_import.go`, `kbcli/cmd_serve.go`, `kbcli/exec_cmd.go`: CLI/API owner integrates command wiring.
- `backend/main.go`: CLI/API owner integrates routes.
- `openspec/changes/need-efficiency-v2-pipeline/tasks.md`: coordinator updates checkboxes only after implementation and review evidence exists.

If two active workers need the same shared file, the coordinator serializes that
edit or assigns one explicit integrator. Workers should not revert or rewrite
another slice's changes.

## Review Gates

Every numbered task group has a review gate in `tasks.md`. The review subagent
receives:

- the completed task group number,
- the intended write scope,
- the tests or E2E command that were run,
- changed files in that group.

The review must report blocking findings before checkboxes are marked complete.
Blocking findings are fixed in the owning slice before dependent work continues.

## E2E-First Sequence

The E2E fixture spine starts before schema and algorithm work:

1. Build deterministic fixture descriptions and expected assertions.
2. Seed raw-ish legacy tables where possible.
3. Run legacy `efficiency` where applicable.
4. Run `efficiency-v2` once the command exists.
5. Assert v2 database outputs.
6. Assert backend API outputs.

Until later sections are implemented, the harness records the expected current
failure point instead of pretending the full pipeline exists.
