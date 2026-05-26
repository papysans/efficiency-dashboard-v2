# Efficiency V2 E2E Fixtures

The v2 pipeline does not currently have production developer data. The apply
phase therefore uses deterministic mock data that exercises the data paths
described in `docs/plans/2026-05-21-提效比-*.md` and the OpenSpec change.

## Fixture Dimensions

Boundary scenarios:

- PR boundary with high confidence.
- Non-main branch boundary with high confidence.
- Issue identifier boundary with medium confidence.
- File-cluster boundary with low confidence.
- Orphan user-week boundary with very low confidence.

Session and activity scenarios:

- no-edit thinking session,
- edit-test-edit loop,
- final verification after last edit,
- degraded conversation-only diff event,
- multi-contributor overlap,
- long idle gap,
- wait-for-review interval.

Work accounting scenarios:

- uncovered commit,
- low AI code participation,
- high uncovered-work ratio,
- merged Need,
- active Need,
- abandoned Need.

Baseline and output scenarios:

- seeded anchors,
- empty-anchor fallback,
- disabled or failed LLM baseline,
- single-baseline low confidence,
- high outlier ratio,
- low or negative outlier ratio.

## Local Fixture Source of Truth

`kbcli/efficiency_v2_fixture.go` defines the deterministic fixture catalog and
seed helpers. Tests use this local fixture as the source of truth so CI and local
runs do not require network access.

The fixture helper initially seeds raw-ish legacy tables such as `sessions`,
`conversations`, `tasks`, `commits`, and `user_org`. It also seeds a temporary
fixture manifest table for anchors, coefficients, expected assertions, boundary
evidence, and baseline variants. Later sections replace or supplement this
manifest with the formal v2 tables and extend the E2E harness to assert
`conversation_events`, `session_stage_metrics`, `needs`, and
`user_productivity_v2`.

## Optional External Sample Data

An optional future fetch/transform command can load public benchmark data, for
example METR-style anchor records, into a cache or fixture directory. This path
is not required for tests. If network access is unavailable, the deterministic
fixture remains sufficient.

Planned behavior:

```text
fetch external source
  -> cache raw file
  -> transform to anchor_set-compatible records
  -> run E2E with seeded anchors
```

The command must be explicit and opt-in. No normal test or import path may fetch
network data implicitly.

## Target E2E Command Shape

The final validation sequence is expected to look like this:

```bash
go test ./...                              # narrow package tests first
go test -run TestEfficiencyV2Fixture ./... # deterministic fixture checks
kbcli import --config <fixture-config> --date 20260518
kbcli efficiency-v2 --config <fixture-config> --date 20260518
go test -tags integration ./backend -run TestEfficiencyV2
```

The exact command names may evolve during implementation, but the required path
does not change: fixture setup, legacy coexistence, v2 CLI output, database
assertions, and backend API assertions.
