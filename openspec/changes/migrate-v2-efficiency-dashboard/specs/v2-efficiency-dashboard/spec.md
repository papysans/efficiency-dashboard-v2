## ADDED Requirements

### Requirement: Provide V2 API Consumption
The frontend SHALL expose client methods for the v2 Need and user-week efficiency APIs without changing existing legacy API methods.

#### Scenario: List Needs API is requested
- **WHEN** a v2 Need list view requests data with supported filters
- **THEN** the frontend SHALL call `GET /api/v2/needs` with `startDate`, `endDate`, pagination, and any supplied repo, branch, user, status, boundary, confidence, or outlier filters

#### Scenario: Need detail API is requested
- **WHEN** a user opens a Need detail route
- **THEN** the frontend SHALL call `GET /api/v2/needs/:needId` and render persisted response fields without recomputing baselines

#### Scenario: User-week efficiency API is requested
- **WHEN** a v2 efficiency aggregate view requests data
- **THEN** the frontend SHALL call `GET /api/v2/efficiency` with supported date and user filters

### Requirement: Distinguish Calendar and Work Efficiency
The v2 dashboard SHALL present calendar efficiency and work efficiency as separate metrics with explicit labels and formulas implied by their source fields.

#### Scenario: Need list displays v2 ratios
- **WHEN** a Need row contains `efficiency_ratio` and `work_efficiency_ratio`
- **THEN** the list SHALL label them as `日历提效` and `工作量提效` respectively

#### Scenario: Need detail displays v2 ratio inputs
- **WHEN** a Need detail response contains calendar and work terms
- **THEN** the detail view SHALL show actual calendar, baseline calendar, actual corrected work, and fused baseline work next to the corresponding ratio labels

#### Scenario: User-week aggregate displays v2 ratios
- **WHEN** a user-week aggregate row contains `efficiency_ratio` and `work_efficiency_ratio`
- **THEN** the aggregate view SHALL show both ratios and the underlying actual/baseline calendar and work totals

#### Scenario: V2 ratio value is formatted
- **WHEN** a v2 ratio value is `0.0667`
- **THEN** the frontend SHALL render it as approximately `6.7%` and SHALL NOT use the legacy task/commit ratio thresholds that treat `300` as `300%`

### Requirement: Provide Need List Reporting
The dashboard SHALL provide a paginated Need list for v2 reporting.

#### Scenario: Need list route is opened
- **WHEN** a user opens the canonical Need list route
- **THEN** the dashboard SHALL render a table of Need records with Need ID, status, boundary source, boundary confidence, repo, branch, primary user, development dates, calendar terms, work terms, calendar efficiency, work efficiency, confidence level, coverage eligibility, and outlier flag

#### Scenario: Need list filters are applied
- **WHEN** a user changes date, repo, branch, user, status, boundary source, boundary confidence, confidence level, or outlier-only filters
- **THEN** the dashboard SHALL refresh the list from the backend using matching query parameters and reset pagination to the first page

#### Scenario: Need row is selected
- **WHEN** a user selects a Need row
- **THEN** the dashboard SHALL navigate to the Need detail route while preserving useful date query context

### Requirement: Provide Need Detail Reporting
The dashboard SHALL provide a Need detail view for explaining a v2 result.

#### Scenario: Need detail summary is rendered
- **WHEN** a Need detail response is loaded
- **THEN** the dashboard SHALL render summary sections for calendar efficiency, work efficiency, status, boundary confidence, confidence level, outlier flag, coverage eligibility, and reason

#### Scenario: Baseline components are rendered
- **WHEN** baseline component fields are present
- **THEN** the dashboard SHALL render algorithmic, anchor-KNN, LLM, fused work, spread, calendar conversion, and team density fields without recalculating them

#### Scenario: Stage metrics are rendered
- **WHEN** stage metric fields are present
- **THEN** the dashboard SHALL render think, execution, verification, and other stage totals in a compact diagnostic section

#### Scenario: Related records are rendered
- **WHEN** sessions or commits are present in the detail response
- **THEN** the dashboard SHALL render related session and commit tables with identifiers, timestamps, user/repo context, and relevant totals

### Requirement: Provide User-Week V2 Efficiency Reporting
The dashboard SHALL provide a user-week aggregate view based on `user_productivity_v2`.

#### Scenario: Aggregate route is opened
- **WHEN** a user opens the v2 efficiency aggregate route
- **THEN** the dashboard SHALL render rows with user, week, merged/active/abandoned Need counts, actual calendar, baseline calendar, calendar efficiency, actual work, baseline work, work efficiency, coverage fields, confidence-limited state, tokens, cost, commit count, and changed lines

#### Scenario: Confidence is limited
- **WHEN** an aggregate row has `confidence_limited = true`
- **THEN** the dashboard SHALL display the limited state and `confidence_reason` near the ratio fields

#### Scenario: Coverage composition exists
- **WHEN** coverage fields are present
- **THEN** the dashboard SHALL expose high, medium, low/unreported, abandoned, and active coverage values so the official ratio can be interpreted with selection bias visible

### Requirement: Preserve Legacy Dashboard Access
The v2 dashboard migration SHALL be additive and SHALL NOT remove existing legacy pages or routes.

#### Scenario: Existing route is opened
- **WHEN** a user opens an existing legacy route such as `/repo-v2`, `/task-v2`, `/commit-v2`, `/user-v2`, `/org-v2`, or `/project-v2`
- **THEN** the existing view SHALL remain routable

#### Scenario: Main navigation is rendered
- **WHEN** the app shell renders navigation
- **THEN** it SHALL include v2 reporting entries without hiding existing legacy dashboard entries

#### Scenario: Home page is rendered
- **WHEN** the home page renders navigation cards or metric entry points
- **THEN** it SHALL include entry points for Need-level v2 reporting and user-week v2 efficiency reporting

### Requirement: Support Kanban Route Compatibility
The dashboard SHALL support compatibility routes for the migrated kanban reporting surface when they do not conflict with canonical routes.

#### Scenario: Kanban Need route is opened
- **WHEN** a user opens a `/kanban/need` compatibility route
- **THEN** the dashboard SHALL render or redirect to the v2 Need list while preserving query parameters

#### Scenario: Kanban efficiency route is opened
- **WHEN** a user opens a `/kanban/efficiency` compatibility route
- **THEN** the dashboard SHALL render or redirect to the v2 user-week efficiency view while preserving query parameters

### Requirement: Package V2 Dashboard for Docker Delivery
The migrated frontend SHALL be included in the repository's Docker/compose delivery path.

#### Scenario: Frontend build completes
- **WHEN** `npm run build` is executed in `frontend`
- **THEN** the build SHALL include the v2 routes and views without compile errors

#### Scenario: Portal serves SPA routes
- **WHEN** the portal nginx service receives a direct request for a v2 dashboard route
- **THEN** it SHALL serve the frontend SPA fallback and continue proxying `/api` requests to the backend service

#### Scenario: Docker packaging is verified
- **WHEN** the implementation is complete
- **THEN** the repository SHALL document or provide the command path that packages the built frontend into the portal service used by compose
