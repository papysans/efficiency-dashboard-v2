## 1. API and Formatting Foundation

- [x] 1.1 Add `getNeedsV2`, `getNeedDetailV2`, and `getEfficiencyV2` wrappers in `frontend/src/api/es.js`.
- [x] 1.2 Add v2-specific ratio formatting helpers that render fractional ratios as percentages and do not alter legacy ratio formatting.
- [x] 1.3 Add duration/number helpers needed by v2 calendar and work fields, reusing existing helpers where compatible.
- [x] 1.4 Add focused tests or structure checks proving v2 ratios are labeled/formatted separately from legacy task/commit ratios.

## 2. Need List View

- [x] 2.1 Create a Vue Need list view that calls `/api/v2/needs` through the new API wrapper.
- [x] 2.2 Render required Need columns: need id, status, boundary source, boundary confidence, repo, branch, primary user, dates, calendar terms, work terms,日历提效, 工作量提效, confidence, coverage, and outlier flag.
- [x] 2.3 Implement server-backed filters for date range, repo, branch, user, status, boundary source, boundary confidence, confidence level, and outlier-only.
- [x] 2.4 Implement row navigation from Need list to Need detail while preserving useful date query context.

## 3. Need Detail View

- [x] 3.1 Create a Vue Need detail view that calls `/api/v2/needs/:needId`.
- [x] 3.2 Render summary sections for calendar efficiency, work efficiency, status, boundary confidence, confidence level, outlier flag, coverage eligibility, and reason.
- [x] 3.3 Render actual/baseline calendar and actual/baseline work terms next to the corresponding ratio sections.
- [x] 3.4 Render stage totals, baseline components, team density, quality signals, confidence signals, related sessions, and related commits.
- [x] 3.5 Handle missing/null fields explicitly as unavailable instead of displaying misleading zeros.

## 4. User-Week V2 Efficiency View

- [x] 4.1 Create a Vue user-week v2 efficiency view that calls `/api/v2/efficiency`.
- [x] 4.2 Render user, week, need counts, calendar terms, work terms,日历提效, 工作量提效, coverage composition, confidence-limited state, tokens, cost, commit count, and changed lines.
- [x] 4.3 Add date and user filters supported by the backend API.
- [x] 4.4 Display `confidence_reason` near ratio fields when `confidence_limited` is true.

## 5. Routing and Navigation

- [x] 5.1 Add canonical routes `/needs-v2`, `/needs/:needId`, and `/efficiency-v2`.
- [x] 5.2 Add non-breaking `/kanban/need`, `/kanban/need/:needId`, and `/kanban/efficiency` compatibility aliases or redirects that preserve query parameters.
- [x] 5.3 Add App navigation entries for v2 Need and v2 user-week efficiency reporting without removing legacy entries.
- [x] 5.4 Add Home page entry points for Need-level v2 reporting and user-week v2 efficiency reporting.
- [x] 5.5 Ensure active menu behavior remains coherent for canonical and compatibility routes.

## 6. Docker and Delivery Path

- [x] 6.1 Verify current frontend build output path and compose portal static asset mount path.
- [x] 6.2 Add or document the command path that packages built frontend assets into the portal service used by compose.
- [x] 6.3 Verify nginx SPA fallback serves direct v2 route requests and `/api` remains proxied to the backend service.

## 7. Verification

- [x] 7.1 Run frontend unit/structure tests with `npm test`.
- [x] 7.2 Run frontend production build with `npm run build`.
- [x] 7.3 Run route-level manual or browser verification for `/needs-v2`, `/needs/:needId` when data exists, `/efficiency-v2`, and compatibility aliases.
- [x] 7.4 Verify API requests use `/api/v2/needs`, `/api/v2/needs/:needId`, and `/api/v2/efficiency` and do not call legacy endpoints for v2 views.
- [x] 7.5 Run OpenSpec validation for `migrate-v2-efficiency-dashboard`.
