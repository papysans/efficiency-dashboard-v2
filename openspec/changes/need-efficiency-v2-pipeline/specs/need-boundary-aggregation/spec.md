## ADDED Requirements

### Requirement: Resolve Need Boundaries
The system SHALL group sessions and commits into Need records using a deterministic fallback chain and record the source and confidence of each boundary.

#### Scenario: PR metadata is available
- **WHEN** one or more commits can be associated with a specific PR
- **THEN** the system SHALL create or update a Need with `boundary_source = lv1_pr` and high boundary confidence

#### Scenario: Non-main branch is available
- **WHEN** PR metadata is unavailable and activity belongs to a non-main repository branch
- **THEN** the system SHALL create or update a Need keyed by repo and branch with `boundary_source = lv2_branch` and high boundary confidence

#### Scenario: Issue identifier is available
- **WHEN** PR and branch boundaries are unavailable but commit messages or branch names contain an issue identifier
- **THEN** the system SHALL create or update a Need with `boundary_source = lv3_issue` and medium boundary confidence

#### Scenario: Only weak grouping signals exist
- **WHEN** only author, time-window, and file-overlap grouping is available
- **THEN** the system SHALL create a low-confidence file-cluster Need or a very-low-confidence orphan user-week Need

### Requirement: Persist Need Membership and Status
The system SHALL persist Need membership, contributors, repository metadata, and lifecycle status.

#### Scenario: Need has associated sessions and commits
- **WHEN** sessions and commits are assigned to a Need
- **THEN** the Need record SHALL store session IDs, commit IDs, contributor user IDs, primary user ID, repo, branch, dev start, and dev end fields

#### Scenario: Merge time is known
- **WHEN** a Need has a merge timestamp after development completion
- **THEN** the system SHALL store `merge_ts` and `wait_for_review_min` without including review wait time in efficiency ratio calculation

#### Scenario: Need is abandoned
- **WHEN** branch or PR evidence indicates work was closed, discarded, or force-pushed away without merge
- **THEN** the Need SHALL be marked `abandoned`, SHALL keep all computed metrics, SHALL count in coverage, and SHALL not enter team efficiency ratio aggregation

### Requirement: Compute Actual Time and Stage Aggregates
The system SHALL compute Need-level actual time fields and stage totals using conceptually separate person-time, wall-time, and calendar-time fields.

#### Scenario: Multiple contributors overlap in time
- **WHEN** two contributors have overlapping active sessions on the same Need
- **THEN** `total_session_active_person_min` SHALL sum both contributors' active minutes and `total_wall_min` SHALL store the de-duplicated wall-clock interval union

#### Scenario: Need has a long idle gap
- **WHEN** a Need has an activity gap greater than the configured idle threshold
- **THEN** `total_calendar_min` SHALL exclude the long idle gap while shorter gaps remain included

#### Scenario: Need has stage metrics
- **WHEN** session stage metrics exist for assigned sessions
- **THEN** the Need SHALL store total think, execution, verification, and other minutes by summing session metrics in person-time terms

### Requirement: Correct Uncovered Human Work
The system SHALL detect commits that are not covered by AI sessions and include a conservative uncovered-work correction.

#### Scenario: Commit matches a session window and file overlap
- **WHEN** a commit falls within the configured session margin and shares touched files with a session
- **THEN** the commit SHALL be considered session-associated and SHALL not add uncovered human minutes

#### Scenario: Commit does not match any session
- **WHEN** a commit matches neither the session window nor file overlap criteria
- **THEN** the system SHALL classify it as uncovered, add its LOC to `uncovered_loc`, estimate uncovered human minutes, and include those minutes in `total_active_work_corrected_min`

#### Scenario: Uncovered work is high
- **WHEN** uncovered work ratio exceeds the configured threshold
- **THEN** the Need SHALL record a low uncovered-work signal that can reduce confidence

### Requirement: Exclude Low-Confidence Boundaries From Team Ratios
The system SHALL exclude low and very-low boundary confidence Needs from team efficiency ratio numerators and denominators while still reporting their coverage.

#### Scenario: Team weekly aggregate is computed
- **WHEN** a weekly team aggregate is calculated
- **THEN** only merged Needs with high or medium boundary confidence SHALL contribute to the efficiency ratio terms

#### Scenario: Low-confidence work exists
- **WHEN** low or very-low confidence Needs exist in the reporting period
- **THEN** their work SHALL be reported in coverage fields instead of being silently omitted
