## ADDED Requirements

### Requirement: Produce Need-Level Efficiency Outputs
The system SHALL persist business-facing and diagnostic v2 efficiency fields on each Need.

#### Scenario: Need has valid actual calendar and fused baseline
- **WHEN** `total_calendar_min` and `baseline_calendar_min` are positive
- **THEN** the system SHALL compute `efficiency_ratio = 1 - total_calendar_min / baseline_calendar_min` without clipping the result

#### Scenario: Need has spread and team density
- **WHEN** baseline spread and team density are available
- **THEN** the system SHALL compute lower and upper efficiency band fields from the converted calendar baseline bounds

#### Scenario: Need has corrected actual work and fused work baseline
- **WHEN** corrected actual work and fused work baseline are positive
- **THEN** the system SHALL compute `work_efficiency_ratio` as an internal diagnostic field

#### Scenario: Need result is invalid
- **WHEN** required formula inputs are missing or invalid
- **THEN** the Need SHALL store null ratio fields and a non-empty reason instead of writing a misleading zero

### Requirement: Produce Confidence and Outlier Signals
The system SHALL persist confidence, outlier, and explanatory signals for every v2 Need result.

#### Scenario: All confidence signals pass
- **WHEN** spread, feature-dependency, silica, AI code ratio, and uncovered-work signals satisfy configured thresholds
- **THEN** the Need SHALL be eligible for high confidence

#### Scenario: Extreme ratio is produced
- **WHEN** actual time is greater than five times fused baseline or less than one tenth of fused baseline
- **THEN** the system SHALL set `outlier_flag = true` and store a reason without removing or clipping the result

#### Scenario: AI participation is weak
- **WHEN** silica or AI code ratio is below the configured threshold
- **THEN** the system SHALL persist the low signal and reduce confidence according to the configured confidence rules

### Requirement: Produce User-Week V2 Aggregates
The system SHALL persist user-week v2 aggregates for downstream reporting.

#### Scenario: Weekly aggregate is computed
- **WHEN** merged high- or medium-boundary-confidence Needs exist in a week
- **THEN** `user_productivity_v2` SHALL sum actual calendar and baseline calendar terms first, then compute the aggregate ratio from the sums

#### Scenario: Coverage categories exist
- **WHEN** high, medium, low, very-low, active, or abandoned Need work exists in the week
- **THEN** the aggregate SHALL persist coverage fields that sum to the full observed work scope

#### Scenario: Coverage is too weak
- **WHEN** low/unreported or abandoned coverage exceeds configured thresholds
- **THEN** the aggregate SHALL mark the ratio as confidence-limited while preserving the raw aggregate terms

### Requirement: Expose V2 Read APIs
The backend SHALL expose read-only APIs for v2 Need and aggregate outputs.

#### Scenario: List Needs
- **WHEN** a client calls `GET /api/v2/needs` with supported filters and pagination
- **THEN** the backend SHALL return persisted Need summary fields including ratio, band, confidence, status, boundary source, stage totals, and coverage eligibility

#### Scenario: Get Need detail
- **WHEN** a client calls `GET /api/v2/needs/:needId`
- **THEN** the backend SHALL return persisted Need detail including sessions, commits, stage metrics, baseline components, fusion inputs, confidence signals, outlier flag, and reason fields

#### Scenario: Query v2 efficiency aggregate
- **WHEN** a client calls `GET /api/v2/efficiency`
- **THEN** the backend SHALL return persisted user-week or team aggregate v2 outputs without recomputing baselines during the request

### Requirement: Preserve Legacy Outputs
The v2 output pipeline SHALL not change legacy efficiency table semantics.

#### Scenario: Legacy command runs after v2 command
- **WHEN** `kbcli efficiency` runs after `kbcli efficiency-v2`
- **THEN** legacy `user_productivity`, `tasks`, and `commits` fields SHALL retain the same calculation semantics as before v2 existed

#### Scenario: Both mode is enabled
- **WHEN** the configured efficiency mode is `both`
- **THEN** the import pipeline SHALL run both legacy and v2 computations and SHALL write each result to its own table set
