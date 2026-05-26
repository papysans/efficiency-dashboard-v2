## ADDED Requirements

### Requirement: Compute Algorithmic Baseline
The system SHALL compute a v2 algorithmic without-AI work baseline for each Need from stage metrics and commit outputs.

#### Scenario: Need has think-stage signals
- **WHEN** think-stage user text, file reads, grep/search counts, turns, or planning signals are available
- **THEN** the algorithmic baseline SHALL estimate think work minutes from versioned coefficients and persist the component value

#### Scenario: Need has execution output
- **WHEN** Need commits have filtered changed-line and file metadata
- **THEN** the algorithmic execution baseline SHALL estimate without-AI execution work from v2 coefficients and production signals rather than summing legacy `commit_ancient_minutes`

#### Scenario: Need has verification signals
- **WHEN** verification command counts, re-edit counts, or review reads are available
- **THEN** the algorithmic baseline SHALL estimate verification work minutes and persist the component value

### Requirement: Compute Anchor-KNN Baseline
The system SHALL compute an anchor-KNN baseline from persisted anchor feature vectors when sufficient anchors exist.

#### Scenario: Anchor set is populated
- **WHEN** anchor records with without-AI minutes are available
- **THEN** the system SHALL compute a Need feature vector, find nearest anchors, calculate a weighted estimate, and persist `baseline_anchor_knn_work_min`

#### Scenario: Anchor set is empty
- **WHEN** no valid anchors exist
- **THEN** the anchor-KNN baseline SHALL be nullable and the fusion reason SHALL record that Baseline B was unavailable

### Requirement: Compute Structured LLM Baseline
The system SHALL compute an optional structured LLM baseline at Need level without sending full raw conversation transcripts.

#### Scenario: LLM estimation is enabled
- **WHEN** AI estimation config is enabled and a Need has enough structured summary fields
- **THEN** the system SHALL call the v2 structured estimator once per Need and persist think, execution, verification, total, confidence, and reason fields

#### Scenario: LLM estimation fails
- **WHEN** the LLM request fails, times out, or returns invalid JSON
- **THEN** the system SHALL leave the LLM baseline nullable, record a reason, and continue fusion using available baselines

### Requirement: Fuse Baselines and Compute Uncertainty
The system SHALL fuse available baseline estimates using persisted fusion weights and SHALL persist uncertainty fields.

#### Scenario: Multiple baselines succeed
- **WHEN** at least two baseline estimates are available
- **THEN** the system SHALL compute `baseline_fused_work_min`, `baseline_spread_work_min`, and a confidence spread ratio from those baselines

#### Scenario: Only one baseline succeeds
- **WHEN** exactly one baseline estimate is available
- **THEN** the system SHALL compute a fused value from that baseline, mark confidence low, and record the single-baseline reason

#### Scenario: No baseline succeeds
- **WHEN** all baseline estimators fail or produce invalid values
- **THEN** the Need efficiency ratio SHALL be null and the Need SHALL have a non-empty reason

### Requirement: Maintain Baseline Support Tables
The system SHALL persist anchor records, coefficient versions, fusion weights, hold-out error fields, and team work density snapshots.

#### Scenario: Cold-start deployment has no stored weights
- **WHEN** no baseline fusion weight snapshot exists
- **THEN** the system SHALL use deterministic cold-start defaults and persist the default snapshot before calculating Need results

#### Scenario: Team work density is recomputed
- **WHEN** enough merged high-confidence Need data exists
- **THEN** the system SHALL compute `team_work_density` from corrected actual work divided by actual calendar span and persist the snapshot

#### Scenario: Hold-out validation runs
- **WHEN** anchor records are available for validation
- **THEN** the system SHALL store per-baseline hold-out error fields in the fusion-weight snapshot
