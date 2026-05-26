## ADDED Requirements

### Requirement: Provide Deterministic E2E Fixtures
The system SHALL include deterministic fixture generation for the v2 pipeline so local and CI verification do not depend on production developer data.

#### Scenario: Fixture generation runs
- **WHEN** the v2 fixture generator is executed
- **THEN** it SHALL produce or seed conversations, sessions, commits, org mappings, anchors, coefficients, and expected assertions for a complete v2 pipeline run

#### Scenario: Fixture covers Need boundary variants
- **WHEN** the deterministic fixture set is generated
- **THEN** it SHALL include PR, branch, issue, file-cluster, and orphan boundary scenarios

#### Scenario: Fixture covers work-accounting variants
- **WHEN** the deterministic fixture set is generated
- **THEN** it SHALL include no-edit thinking, edit-test-edit verification, uncovered commit, low AI participation, multi-contributor overlap, long idle gap, wait-for-review, active, merged, and abandoned scenarios

#### Scenario: Fixture covers baseline variants
- **WHEN** the deterministic fixture set is generated
- **THEN** it SHALL include seeded anchors, empty-anchor fallback, disabled or failed LLM baseline, and outlier ratio scenarios

### Requirement: Support Optional External Sample Data Fetch
The system SHALL support optional external sample-data fetching for benchmark or anchor inputs without making network access mandatory for tests.

#### Scenario: External fetch is requested
- **WHEN** an operator runs the optional sample-data fetch command with a configured source
- **THEN** the system SHALL download or load the source into a cache or fixture directory and transform it into local v2 fixture or anchor records

#### Scenario: Network is unavailable
- **WHEN** tests run without network access
- **THEN** deterministic local fixtures SHALL still be sufficient to run the full E2E validation path

### Requirement: Verify Complete V2 Pipeline End-to-End
The system SHALL provide an end-to-end verification command or documented command sequence that validates the full v2 data production path.

#### Scenario: E2E verification runs
- **WHEN** the E2E verification is executed against a test database
- **THEN** it SHALL run the required import or seed steps, run legacy efficiency where applicable, run `efficiency-v2`, and assert v2 database outputs

#### Scenario: Backend API verification runs
- **WHEN** backend API verification is executed after v2 data is produced
- **THEN** it SHALL assert `GET /api/v2/needs`, `GET /api/v2/needs/:needId`, and `GET /api/v2/efficiency` return persisted fixture-derived values

#### Scenario: Legacy and v2 coexist
- **WHEN** the E2E validation runs both legacy and v2 paths
- **THEN** it SHALL assert legacy tables retain legacy semantics while v2 tables contain Need-level outputs

### Requirement: Require Review Gates for Major Implementation Nodes
The apply process SHALL include independent review checkpoints after major task groups before those groups are marked complete.

#### Scenario: Major task group is completed
- **WHEN** an implementation worker finishes a major task group
- **THEN** a separate review session SHALL inspect the completed slice, check relevant tests or E2E evidence, and report blocking findings before checkboxes are marked complete

#### Scenario: Review finds a blocking issue
- **WHEN** the review session reports a blocking issue
- **THEN** the owning implementation slice SHALL resolve it before dependent task groups proceed

### Requirement: Allow Parallel Subagent Implementation
The apply process SHALL support parallel subagent implementation only when write scopes are disjoint and integration points are explicit.

#### Scenario: Parallel work is delegated
- **WHEN** multiple subagents are used during apply
- **THEN** each subagent SHALL receive a bounded ownership scope and SHALL avoid editing files owned by another active subagent unless the coordinator explicitly reassigns ownership

#### Scenario: Shared file must be edited
- **WHEN** a shared file such as config, model registration, command registration, or route registration must be edited by multiple slices
- **THEN** the coordinator SHALL serialize that edit or assign one owner to integrate the shared file changes
