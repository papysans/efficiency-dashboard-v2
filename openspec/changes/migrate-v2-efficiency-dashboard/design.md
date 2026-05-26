## Context

The v2 data pipeline already persists Need-level outputs and user-week aggregates. The documented new tables are `needs` and `user_productivity_v2`, with support tables for events, stage metrics, anchors, coefficients, and fusion weights. The backend already exposes read-only contracts for `GET /api/v2/needs`, `GET /api/v2/needs/:needId`, and `GET /api/v2/efficiency`.

The current bundled Vue dashboard still centers on legacy task/commit/repo/user views and displays legacy ratios as generic "提效比" percentages. That is no longer precise enough for v2. In v2, `efficiency_ratio` is the calendar efficiency business metric, while `work_efficiency_ratio` is a separate work-efficiency diagnostic metric. They answer different questions and must not be collapsed into one label.

The downstream `costrict-web` kanban panel uses this repository's metrics as its source. This change makes the local dashboard a first-class consumer of the same v2 reporting contract while preserving legacy views.

## Goals / Non-Goals

**Goals:**

- Add a Vue + Element Plus v2 reporting surface for Need-level and user-week outputs.
- Display calendar efficiency and work efficiency as separate fields in list, detail, and aggregate views.
- Use persisted backend fields only; do not recompute baselines in the frontend.
- Preserve existing legacy routes and pages.
- Add route aliases that can mirror the existing `/kanban/*` consumer shape when practical.
- Keep the visual style consistent with the existing work-focused dashboard: dense tables, restrained cards, clear filters, and explicit labels.
- Verify frontend tests/build and Docker/compose serving assumptions.

**Non-Goals:**

- No change to the v2 calculation formulas.
- No new backend tables unless a documented v2 field is missing from the existing API contract.
- No replacement of legacy task/commit/repo/user/project pages.
- No direct SolidJS component copy from `costrict-web`.
- No mock-only validation as the final proof of correctness.

## Decisions

### 1. Build native Vue views instead of copying the downstream SolidJS panel

The local frontend uses Vue 3, Element Plus, ECharts, `KbFilterTable`, and the existing `/api` axios client. The downstream kanban code is SolidJS and Tailwind-style. Reusing it directly would introduce a second frontend stack and create maintenance cost.

Implementation should migrate the behavior and data contract, not the component source. New views should reuse local components such as `KbFilterTable`, `DateRangePicker`, existing chart helpers, and formatting utilities.

Alternative considered: embed or port the SolidJS tree wholesale. Rejected because it conflicts with the repository's frontend stack and would make Docker packaging larger and harder to verify.

### 2. Treat calendar efficiency as primary and work efficiency as required companion

The v2 business metric is:

```text
calendar_efficiency = efficiency_ratio
                    = (baseline_calendar_min - actual_calendar_min) / actual_calendar_min
```

At Need level, actual calendar is `total_calendar_min`; at user-week level, actual calendar is `actual_calendar_min`. Baseline calendar is `baseline_calendar_min` in both surfaces.

The diagnostic work metric is:

```text
work_efficiency = work_efficiency_ratio
                = (baseline_fused_work_min - actual_work_min) / actual_work_min
```

At Need level, actual work is `total_active_work_corrected_min`; at user-week level, actual work is `actual_active_work_corrected_min`. Baseline work is `baseline_fused_work_min`.

UI labels must use "日历提效" and "工作量提效". Existing generic "提效比" labels may remain only on legacy pages where the old task/commit meaning is unchanged.

Alternative considered: show only calendar efficiency because it is the primary business metric. Rejected because short calendar spans can create extreme values; the work metric is required context.

### 3. Add three v2 frontend surfaces

The implementation should add:

- Need list: paginated table over `GET /api/v2/needs`.
- Need detail: detail view over `GET /api/v2/needs/:needId`.
- User-week v2 efficiency: aggregate table over `GET /api/v2/efficiency`.

Need list should support filters already supported by the backend: date range, repo, branch, user, status, boundary source, boundary confidence, confidence level, and outlier-only. It should default to a broad recent date range consistent with existing v2 pages.

Need detail should be structured as compact sections:

- summary metrics:日历提效, 工作量提效, actual/baseline calendar, actual/baseline work, confidence, status, outlier, coverage;
- stage totals: think, execution, verification, other when available;
- baseline components: algorithmic, anchor-KNN, LLM, fused work, spread, team density;
- quality/confidence signals and reason fields;
- related sessions and commits.

User-week v2 efficiency should expose per-week/person metrics and coverage composition. It should not imply that low-confidence or abandoned coverage contributes to the official aggregate ratio; those fields are displayed as confidence context.

### 4. Add explicit v2 API wrappers

`frontend/src/api/es.js` should add named wrappers:

- `getNeedsV2(params)`
- `getNeedDetailV2(needId)`
- `getEfficiencyV2(params)`

Wrappers should follow the existing axios pattern and keep query parameters trimmed before sending when they originate from text filters.

### 5. Keep route compatibility additive

Existing routes such as `/repo-v2`, `/task-v2`, and `/user-v2` must keep working.

New canonical routes should be explicit:

- `/needs-v2`
- `/needs/:needId`
- `/efficiency-v2`

Optional aliases may map:

- `/kanban` to the updated dashboard home or v2 overview
- `/kanban/need` to `/needs-v2`
- `/kanban/need/:needId` to `/needs/:needId`
- `/kanban/efficiency` to `/efficiency-v2`

If aliases are implemented, they should preserve `startDate` and `endDate` query parameters.

### 6. Format v2 ratios as fractional percentages

v2 ratios are fractional values where `0.25` means `25%`; legacy table ratios often use values like `300` to mean `300%`. V2 views must use a v2-specific formatter that multiplies by 100 and preserves signs.

Examples:

- `0` -> `0.0%`
- `0.0667` -> `6.7%`
- `-0.0911` -> `-9.1%`
- `1` -> `100.0%`

This formatter must not be reused blindly on legacy task/commit/repo pages.

### 7. Docker packaging remains nginx + built static assets unless explicitly changed

Current compose uses an nginx `portal` service that serves `./data` as `/var/www` and proxies `/api` to `server:9990`. Implementation should either document and preserve the static asset publish path or add a focused frontend image/build step if the existing package path is insufficient.

Acceptance should verify that a built frontend can be served by the portal configuration and that SPA routes fall back to `index.html`.

## Risks / Trade-offs

- V2 ratio unit confusion -> Add dedicated v2 formatting helpers and tests that assert `0.0667` renders near `6.7%`, not `0.1%` or `0.0667%`.
- Extreme calendar ratios look like UI bugs -> Display `outlier_flag`, confidence, boundary confidence, and work efficiency adjacent to calendar efficiency.
- Backend list summary omits some detail fields -> Use Need detail API for drilldown fields and only show summary fields in the list; add backend fields only if a documented list requirement cannot be satisfied.
- Route aliases can create confusing active navigation -> Keep canonical routes in the main menu and treat `/kanban/*` as compatibility aliases.
- Docker compose currently expects prebuilt static files under `compose/portal/data` -> Treat packaging as an explicit task and validate the build output is copied or served by the configured portal path.
- Full browser verification may require real data -> Use available local API data if present; otherwise verify rendering with backend-empty responses plus build/tests, and document any unavailable real-data check.

## Migration Plan

1. Add frontend API wrappers and v2 ratio/duration formatting helpers.
2. Add Need list, Need detail, and user-week v2 efficiency views.
3. Register canonical routes and compatibility aliases.
4. Add main navigation and home-page entry points.
5. Add focused tests for route/API registration and v2 ratio label/format separation.
6. Run `npm test` and `npm run build`.
7. Start the local frontend or compose portal, then verify the v2 routes render and `/api` requests target existing backend paths.

Rollback is additive: remove the new routes/menu entries and v2 view files. Existing legacy dashboard routes remain untouched.

## Open Questions

- Whether `/kanban` should route to the existing home page, a new v2 overview, or the Need list. The proposal assumes additive aliases and keeps canonical routes explicit.
- Whether production packaging should keep mounting built assets into `compose/portal/data` or introduce a dedicated frontend Dockerfile. This should be decided during implementation after checking current deployment scripts.
