## Why

The repository now produces v2 Need-level and user-week efficiency data, but the bundled dashboard still emphasizes legacy task/commit/repo/user views and does not expose the final v2 reporting contract. The downstream `costrict-web` kanban page already consumes this repository's metrics shape, so the local dashboard should migrate to the v2 contract and make the two final efficiency meanings explicit.

## What Changes

- Add v2 dashboard views that consume persisted `needs` and `user_productivity_v2` data through the existing backend APIs.
- Present calendar efficiency and work efficiency as separate metrics everywhere they appear; do not label either one as a generic "提效比".
- Treat calendar efficiency (`efficiency_ratio`) as the business-facing primary metric and work efficiency (`work_efficiency_ratio`) as a required diagnostic companion.
- Add Need list and Need detail views covering boundary confidence, status, actual calendar span, baseline calendar span, calendar efficiency band, actual work, fused baseline work, work efficiency, confidence, outlier flag, coverage eligibility, stage totals, baseline components, quality signals, sessions, and commits.
- Add a user-week v2 efficiency view covering actual/baseline calendar totals, actual/baseline work totals, calendar efficiency, work efficiency, coverage composition, confidence-limited state, usage cost, tokens, commits, and changed lines.
- Update navigation and dashboard entry points so users can reach v2 Need and user-week reporting from the main app.
- Add optional `/kanban/*` route aliases for the migrated panel shape so the local app can mirror the existing downstream panel paths without replacing current routes.
- Preserve existing legacy task, commit, repo, user, org, project, and manual correction pages.
- Verify the frontend against the documented v2 field contract and the existing read-only backend APIs; do not add new backend tables unless implementation discovers a documented field is unavailable.
- Ensure Docker packaging serves the migrated frontend through the existing compose/portal flow.

## Capabilities

### New Capabilities

- `v2-efficiency-dashboard`: Frontend reporting capability for Need-level and user-week v2 efficiency outputs, including separate calendar/work efficiency presentation and route-level access.

### Modified Capabilities

- None. There are no archived OpenSpec specs in `openspec/specs/`; legacy dashboard behavior remains available.

## Impact

- Affected frontend API code: `frontend/src/api/es.js` will add consumers for `/api/v2/needs`, `/api/v2/needs/:needId`, and `/api/v2/efficiency`.
- Affected frontend routes: `frontend/src/router/index.js` will add v2 Need, Need detail, v2 efficiency aggregate, and optional `/kanban/*` aliases.
- Affected frontend views: new Vue views will be added under `frontend/src/views/`; `Home.vue` and `App.vue` will add v2 dashboard entry points.
- Affected formatting/utilities: frontend formatting may add explicit calendar/work efficiency helpers so v2 ratios are displayed as fractional percentage metrics, not legacy 100/300 style multipliers.
- Affected deployment: compose/portal packaging must include the built frontend assets and continue proxying `/api` to the backend service.
- Affected validation: frontend tests/build must verify v2 route registration, API method contracts, calendar/work metric separation, and Docker packaging assumptions.
