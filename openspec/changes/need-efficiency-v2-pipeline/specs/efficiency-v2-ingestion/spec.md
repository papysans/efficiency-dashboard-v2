## ADDED Requirements

### Requirement: Normalize Conversation Activity Events
The system SHALL persist a normalized event stream for v2 efficiency processing without removing or changing legacy `sessions`, `conversations`, or `tasks` outputs.

#### Scenario: Exact tool event is available
- **WHEN** the importer can identify a raw tool action from conversation data
- **THEN** it SHALL upsert a `conversation_events` row with `source = raw_tool`, `parse_quality = exact`, the normalized event kind, timestamps, session/request identity, and relevant payload fields

#### Scenario: Only conversation-level diff is available
- **WHEN** a conversation row has code diff output but no separate tool-call record
- **THEN** the v2 normalization SHALL create a synthetic edit event with `source = conversation_diff` and `parse_quality = degraded`

#### Scenario: Conversation has activity but no classifiable tool detail
- **WHEN** a conversation row has timestamps or text but no diff and no classifiable tool event
- **THEN** the v2 normalization SHALL create message or other events with degraded parse quality rather than dropping the activity silently

### Requirement: Classify Events for Stage Splitting
The system SHALL classify normalized events into `message`, `read`, `edit`, `verify`, or `other` using deterministic rules.

#### Scenario: Code modification action is detected
- **WHEN** an event corresponds to Edit, Write, MultiEdit, ApplyPatch, patch shell commands, or synthetic diff output
- **THEN** the event SHALL be classified as `edit`

#### Scenario: Verification command is detected
- **WHEN** an event command matches configured test, build, typecheck, lint, or check patterns
- **THEN** the event SHALL be classified as `verify`

#### Scenario: Shell command is not a verification command
- **WHEN** an event is a shell command that does not match verification patterns
- **THEN** the event SHALL be classified as `other` and counted in total active activity but not in think, execution, or verification stage active minutes

### Requirement: Compute Session Stage Metrics
The system SHALL compute one `session_stage_metrics` row per session from normalized events.

#### Scenario: Session contains no edit events
- **WHEN** a session has normalized events but none are classified as `edit`
- **THEN** all classifiable activity SHALL be assigned to the think stage, execution and verification minutes SHALL be zero, and the row SHALL remain valid

#### Scenario: Session contains edit-test-edit loop
- **WHEN** a session contains an edit event followed by a verification event followed by another edit event
- **THEN** the verification event SHALL contribute to verification metrics, not execution metrics

#### Scenario: Event duration is missing
- **WHEN** an event has no end timestamp
- **THEN** duration SHALL be inferred from the next event within the configured maximum gap or from deterministic minimum-duration defaults

#### Scenario: Session uses degraded events
- **WHEN** any session metric is computed from degraded event rows
- **THEN** the session metrics SHALL record degraded stage confidence and summary reason fields that downstream Need confidence can consume

### Requirement: Preserve Import Idempotency
The system SHALL make v2 event and stage metric generation rerunnable for the same raw conversation data.

#### Scenario: Import is rerun for the same session
- **WHEN** `import-conv` or the v2 normalization step runs twice for unchanged raw data
- **THEN** `conversation_events` and `session_stage_metrics` SHALL be upserted without duplicate logical rows

#### Scenario: Legacy data is inspected after v2 normalization
- **WHEN** v2 normalization completes
- **THEN** existing legacy task and conversation fields SHALL retain their prior meanings and remain usable by `kbcli efficiency`
