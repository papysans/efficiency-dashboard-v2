package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type ObjectJSON string

func (j *ObjectJSON) Scan(value interface{}) error {
	if value == nil {
		*j = ""
		return nil
	}
	switch v := value.(type) {
	case []byte:
		*j = ObjectJSON(v)
	case string:
		*j = ObjectJSON(v)
	}
	return nil
}

func (j ObjectJSON) Value() (driver.Value, error) {
	if j == "" {
		return "{}", nil
	}
	if !json.Valid([]byte(j)) {
		return nil, fmt.Errorf("invalid json object: %s", string(j))
	}
	return string(j), nil
}

func (j ObjectJSON) MarshalJSON() ([]byte, error) {
	if j == "" || j == "null" {
		return []byte("{}"), nil
	}
	return []byte(j), nil
}

func (j *ObjectJSON) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*j = "{}"
		return nil
	}
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*j = ObjectJSON(s)
		return nil
	}
	if !json.Valid(data) {
		return fmt.Errorf("invalid json object: %s", string(data))
	}
	*j = ObjectJSON(data)
	return nil
}

type ConversationEvent struct {
	EventId      string     `gorm:"primaryKey;type:varchar(255)" json:"event_id"`
	SessionId    string     `gorm:"type:varchar(255);not null;index" json:"session_id"`
	RequestId    string     `gorm:"type:varchar(255);not null;index" json:"request_id"`
	TaskId       string     `gorm:"type:varchar(500);index" json:"task_id"`
	UserId       string     `gorm:"type:varchar(255);index" json:"user_id"`
	RepoAddr     string     `gorm:"type:text" json:"repo_addr"`
	RepoBranch   string     `gorm:"type:varchar(500)" json:"repo_branch"`
	WorkDirId    string     `gorm:"type:varchar(500);index" json:"work_dir_id"`
	EventStartTs time.Time  `gorm:"column:event_start_ts;type:timestamptz;not null;index" json:"event_start_ts"`
	EventEndTs   *time.Time `gorm:"column:event_end_ts;type:timestamptz" json:"event_end_ts"`
	DurationSec  int64      `gorm:"type:bigint;default:0" json:"duration_sec"`
	EventKind    string     `gorm:"type:varchar(50);not null;index" json:"event_kind"`
	ToolName     string     `gorm:"type:varchar(100)" json:"tool_name"`
	CommandText  string     `gorm:"type:text" json:"command_text"`
	TouchedFiles StringJSON `gorm:"type:jsonb;default:'[]'" json:"touched_files"`
	Payload      ObjectJSON `gorm:"type:jsonb;default:'{}'" json:"payload"`
	Source       string     `gorm:"type:varchar(50);not null;index" json:"source"`
	ParseQuality string     `gorm:"type:varchar(50);not null;index" json:"parse_quality"`
	CreatedAt    time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (ConversationEvent) TableName() string { return "conversation_events" }

type SessionStageMetric struct {
	SessionId             string     `gorm:"primaryKey;type:varchar(255)" json:"session_id"`
	NeedId                string     `gorm:"type:varchar(500);index" json:"need_id"`
	UserId                string     `gorm:"type:varchar(255);index" json:"user_id"`
	RepoAddr              string     `gorm:"type:text" json:"repo_addr"`
	RepoBranch            string     `gorm:"type:varchar(500)" json:"repo_branch"`
	WorkDirId             string     `gorm:"type:varchar(500);index" json:"work_dir_id"`
	SessionStartTs        *time.Time `gorm:"column:session_start_ts;type:timestamptz;index" json:"session_start_ts"`
	SessionEndTs          *time.Time `gorm:"column:session_end_ts;type:timestamptz" json:"session_end_ts"`
	FirstEditTs           *time.Time `gorm:"column:first_edit_ts;type:timestamptz" json:"first_edit_ts"`
	LastEditTs            *time.Time `gorm:"column:last_edit_ts;type:timestamptz" json:"last_edit_ts"`
	TotalActiveMin        float64    `gorm:"type:float8;default:0" json:"total_active_min"`
	TotalWallMin          float64    `gorm:"type:float8;default:0" json:"total_wall_min"`
	ThinkActiveMin        float64    `gorm:"type:float8;default:0" json:"think_active_min"`
	ExecutionActiveMin    float64    `gorm:"column:exec_active_min;type:float8;default:0" json:"exec_active_min"`
	VerificationActiveMin float64    `gorm:"column:verify_active_min;type:float8;default:0" json:"verify_active_min"`
	OtherActiveMin        float64    `gorm:"type:float8;default:0" json:"other_active_min"`
	ThinkWallMin          float64    `gorm:"type:float8;default:0" json:"think_wall_min"`
	ExecutionWallMin      float64    `gorm:"column:exec_wall_min;type:float8;default:0" json:"exec_wall_min"`
	VerificationWallMin   float64    `gorm:"column:verify_wall_min;type:float8;default:0" json:"verify_wall_min"`
	OtherWallMin          float64    `gorm:"type:float8;default:0" json:"other_wall_min"`
	MessageEventCount     int64      `gorm:"type:bigint;default:0" json:"message_event_count"`
	ReadEventCount        int64      `gorm:"type:bigint;default:0" json:"read_event_count"`
	EditEventCount        int64      `gorm:"type:bigint;default:0" json:"edit_event_count"`
	VerifyEventCount      int64      `gorm:"type:bigint;default:0" json:"verify_event_count"`
	OtherEventCount       int64      `gorm:"type:bigint;default:0" json:"other_event_count"`
	DegradedEventCount    int64      `gorm:"type:bigint;default:0" json:"degraded_event_count"`
	EventKindCounts       ObjectJSON `gorm:"type:jsonb;default:'{}'" json:"event_kind_counts"`
	AITokenRatio          *float64   `gorm:"column:ai_token_ratio;type:float8" json:"ai_token_ratio"`
	RePromptCount         int64      `gorm:"type:bigint;default:0" json:"re_prompt_count"`
	RevertCount           int64      `gorm:"type:bigint;default:0" json:"revert_count"`
	CompactionCount       int64      `gorm:"type:bigint;default:0" json:"compaction_count"`
	TotalCostUSD          float64    `gorm:"column:total_cost_usd;type:float8;default:0" json:"total_cost_usd"`
	StageConfidence       string     `gorm:"type:varchar(50);index" json:"stage_confidence"`
	ConfidenceReason      string     `gorm:"type:text" json:"confidence_reason"`
	Summary               string     `gorm:"type:text" json:"summary"`
	SummarySource         string     `gorm:"type:varchar(50)" json:"summary_source"`
	CreatedAt             time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt             time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (SessionStageMetric) TableName() string { return "session_stage_metrics" }

type Need struct {
	NeedId                          string     `gorm:"primaryKey;type:varchar(500)" json:"need_id"`
	BoundarySource                  string     `gorm:"type:varchar(50);not null;index" json:"boundary_source"`
	BoundaryConfidence              string     `gorm:"type:varchar(50);not null;index" json:"boundary_confidence"`
	BoundaryKey                     string     `gorm:"type:varchar(1000);not null" json:"boundary_key"`
	BoundaryEvidence                ObjectJSON `gorm:"type:jsonb;default:'{}'" json:"boundary_evidence"`
	Status                          string     `gorm:"type:varchar(50);not null;index" json:"status"`
	RepoAddr                        string     `gorm:"type:text" json:"repo_addr"`
	RepoBranch                      string     `gorm:"type:varchar(500)" json:"repo_branch"`
	PrimaryUserId                   string     `gorm:"type:varchar(255);index" json:"primary_user_id"`
	ContributorUserIds              StringJSON `gorm:"type:jsonb;default:'[]'" json:"contributor_user_ids"`
	SessionIds                      StringJSON `gorm:"type:jsonb;default:'[]'" json:"session_ids"`
	CommitIds                       StringJSON `gorm:"type:jsonb;default:'[]'" json:"commit_ids"`
	TouchedFiles                    StringJSON `gorm:"type:jsonb;default:'[]'" json:"touched_files"`
	DevStartTs                      *time.Time `gorm:"column:dev_start_ts;type:timestamptz;index" json:"dev_start_ts"`
	DevEndTs                        *time.Time `gorm:"column:dev_end_ts;type:timestamptz;index" json:"dev_end_ts"`
	MergeTs                         *time.Time `gorm:"column:merge_ts;type:timestamptz" json:"merge_ts"`
	DevDurationMin                  float64    `gorm:"type:float8;default:0" json:"dev_duration_min"`
	CycleDurationMin                *float64   `gorm:"type:float8" json:"cycle_duration_min"`
	TotalSessionActivePersonMin     float64    `gorm:"type:float8;default:0" json:"total_session_active_person_min"`
	EstimateUncoveredHumanMin       float64    `gorm:"type:float8;default:0" json:"estimate_uncovered_human_min"`
	TotalActiveWorkCorrectedMin     float64    `gorm:"type:float8;default:0" json:"total_active_work_corrected_min"`
	TotalWallMin                    float64    `gorm:"type:float8;default:0" json:"total_wall_min"`
	TotalCalendarMin                float64    `gorm:"type:float8;default:0" json:"total_calendar_min"`
	WaitForReviewMin                float64    `gorm:"type:float8;default:0" json:"wait_for_review_min"`
	ThinkActiveMin                  float64    `gorm:"column:total_think_min;type:float8;default:0" json:"total_think_min"`
	ExecutionActiveMin              float64    `gorm:"column:total_exec_min;type:float8;default:0" json:"total_exec_min"`
	VerificationActiveMin           float64    `gorm:"column:total_verify_min;type:float8;default:0" json:"total_verify_min"`
	OtherActiveMin                  float64    `gorm:"column:total_other_min;type:float8;default:0" json:"total_other_min"`
	CommitCount                     int64      `gorm:"type:bigint;default:0" json:"commit_count"`
	ChangedLoc                      int64      `gorm:"column:total_loc_net;type:bigint;default:0" json:"total_loc_net"`
	FileCount                       int64      `gorm:"column:total_files_touched;type:bigint;default:0" json:"total_files_touched"`
	AICoveredLoc                    int64      `gorm:"column:ai_covered_loc;type:bigint;default:0" json:"ai_covered_loc"`
	UncoveredCommitIds              StringJSON `gorm:"type:jsonb;default:'[]'" json:"uncovered_commit_ids"`
	UncoveredLoc                    int64      `gorm:"type:bigint;default:0" json:"uncovered_loc"`
	UncoveredHumanMin               float64    `gorm:"type:float8;default:0" json:"uncovered_human_min"`
	UncoveredWorkRatio              *float64   `gorm:"type:float8" json:"uncovered_work_ratio"`
	AICodeRatio                     *float64   `gorm:"column:ai_code_ratio;type:float8" json:"ai_code_ratio"`
	Silica                          *float64   `gorm:"type:float8" json:"silica"`
	ChurnRatio                      *float64   `gorm:"type:float8" json:"churn_ratio"`
	DuplicationRatio                *float64   `gorm:"type:float8" json:"duplication_ratio"`
	RevertCount                     int64      `gorm:"type:bigint;default:0" json:"revert_count"`
	RevertRate                      *float64   `gorm:"type:float8" json:"revert_rate"`
	PostGenerationDeletionRatio     *float64   `gorm:"type:float8" json:"post_generation_deletion_ratio"`
	QualitySignals                  ObjectJSON `gorm:"type:jsonb;default:'{}'" json:"quality_signals"`
	ConfidenceSignals               ObjectJSON `gorm:"type:jsonb;default:'{}'" json:"confidence_signals"`
	BaselineAlgoThinkWorkMin        *float64   `gorm:"type:float8" json:"baseline_algo_think_work_min"`
	BaselineAlgoExecutionWorkMin    *float64   `gorm:"type:float8" json:"baseline_algo_execution_work_min"`
	BaselineAlgoVerificationWorkMin *float64   `gorm:"type:float8" json:"baseline_algo_verification_work_min"`
	BaselineAlgoTotalWorkMin        *float64   `gorm:"type:float8" json:"baseline_algo_total_work_min"`
	BaselineAnchorKnnWorkMin        *float64   `gorm:"column:baseline_anchor_knn_work_min;type:float8" json:"baseline_anchor_knn_work_min"`
	BaselineAnchorKnnReason         string     `gorm:"column:baseline_anchor_knn_reason;type:text" json:"baseline_anchor_knn_reason"`
	BaselineLLMThinkWorkMin         *float64   `gorm:"column:baseline_llm_think_work_min;type:float8" json:"baseline_llm_think_work_min"`
	BaselineLLMExecutionWorkMin     *float64   `gorm:"column:baseline_llm_execution_work_min;type:float8" json:"baseline_llm_execution_work_min"`
	BaselineLLMVerificationWorkMin  *float64   `gorm:"column:baseline_llm_verification_work_min;type:float8" json:"baseline_llm_verification_work_min"`
	BaselineLLMTotalWorkMin         *float64   `gorm:"column:baseline_llm_total_work_min;type:float8" json:"baseline_llm_total_work_min"`
	BaselineLLMConfidence           string     `gorm:"column:baseline_llm_confidence;type:varchar(50)" json:"baseline_llm_confidence"`
	BaselineLLMReason               string     `gorm:"column:baseline_llm_reason;type:text" json:"baseline_llm_reason"`
	BaselineFusedWorkMin            *float64   `gorm:"type:float8" json:"baseline_fused_work_min"`
	BaselineSpreadWorkMin           *float64   `gorm:"type:float8" json:"baseline_spread_work_min"`
	BaselineCalendarMin             *float64   `gorm:"type:float8" json:"baseline_calendar_min"`
	TeamWorkDensityUsed             *float64   `gorm:"type:float8" json:"team_work_density_used"`
	TeamProfileUsed                 string     `gorm:"type:varchar(100)" json:"team_profile_used"`
	EfficiencyRatio                 *float64   `gorm:"type:float8" json:"efficiency_ratio"`
	EfficiencyLowerBand             *float64   `gorm:"column:efficiency_band_low;type:float8" json:"efficiency_band_low"`
	EfficiencyUpperBand             *float64   `gorm:"column:efficiency_band_high;type:float8" json:"efficiency_band_high"`
	WorkEfficiencyRatio             *float64   `gorm:"type:float8" json:"work_efficiency_ratio"`
	ConfidenceLevel                 string     `gorm:"type:varchar(50);index" json:"confidence_level"`
	OutlierFlag                     bool       `gorm:"type:boolean;default:false;index" json:"outlier_flag"`
	CoverageEligible                bool       `gorm:"type:boolean;default:false;index" json:"coverage_eligible"`
	FeatureDependencyRisk           string     `gorm:"type:varchar(50)" json:"feature_dependency_risk"`
	SilicaSignal                    string     `gorm:"type:varchar(50)" json:"silica_signal"`
	AICodeRatioSignal               string     `gorm:"column:ai_code_ratio_signal;type:varchar(50)" json:"ai_code_ratio_signal"`
	UncoveredWorkSignal             string     `gorm:"type:varchar(50)" json:"uncovered_work_signal"`
	Reason                          string     `gorm:"type:text" json:"reason"`
	CreatedAt                       time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt                       time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Need) TableName() string { return "needs" }

type UserProductivityV2 struct {
	UserProductivityV2Id         string     `gorm:"primaryKey;type:varchar(255)" json:"user_productivity_v2_id"`
	WeekStart                    time.Time  `gorm:"type:date;not null;index" json:"week_start"`
	UserId                       string     `gorm:"type:varchar(255);not null;index" json:"user_id"`
	UserName                     string     `gorm:"type:varchar(500)" json:"user_name"`
	NeedIds                      StringJSON `gorm:"type:jsonb;default:'[]'" json:"need_ids"`
	MergedNeedCount              int64      `gorm:"type:bigint;default:0" json:"merged_need_count"`
	ActiveNeedCount              int64      `gorm:"type:bigint;default:0" json:"active_need_count"`
	AbandonedNeedCount           int64      `gorm:"type:bigint;default:0" json:"abandoned_need_count"`
	ActualCalendarMin            float64    `gorm:"type:float8;default:0" json:"actual_calendar_min"`
	BaselineCalendarMin          float64    `gorm:"type:float8;default:0" json:"baseline_calendar_min"`
	EfficiencyRatio              *float64   `gorm:"type:float8" json:"efficiency_ratio"`
	ActualActiveWorkCorrectedMin float64    `gorm:"type:float8;default:0" json:"actual_active_work_corrected_min"`
	BaselineFusedWorkMin         float64    `gorm:"type:float8;default:0" json:"baseline_fused_work_min"`
	WorkEfficiencyRatio          *float64   `gorm:"type:float8" json:"work_efficiency_ratio"`
	CoverageHigh                 float64    `gorm:"column:coverage_high_confidence;type:float8;default:0" json:"coverage_high_confidence"`
	CoverageMedium               float64    `gorm:"type:float8;default:0" json:"coverage_medium"`
	CoverageLowUnreported        float64    `gorm:"type:float8;default:0" json:"coverage_low_unreported"`
	CoverageAbandoned            float64    `gorm:"type:float8;default:0" json:"coverage_abandoned"`
	CoverageActive               float64    `gorm:"type:float8;default:0" json:"coverage_active"`
	ConfidenceLimited            bool       `gorm:"type:boolean;default:false;index" json:"confidence_limited"`
	ConfidenceReason             string     `gorm:"type:text" json:"confidence_reason"`
	// 与老 user_productivity 对齐：用户级 token/cost 累加，方便看板展示用量与成本
	UpstreamTokens   int64     `gorm:"type:bigint;default:0" json:"upstream_tokens"`
	DownstreamTokens int64     `gorm:"type:bigint;default:0" json:"downstream_tokens"`
	Cost             float64   `gorm:"type:float8;default:0" json:"cost"`
	CommitCount      int64     `gorm:"type:bigint;default:0" json:"commit_count"`
	CommitDiffLines  int64     `gorm:"type:bigint;default:0" json:"commit_diff_lines"`
	CreatedAt        time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (UserProductivityV2) TableName() string { return "user_productivity_v2" }

type AnchorSet struct {
	AnchorId            string     `gorm:"primaryKey;type:varchar(255)" json:"anchor_id"`
	Source              string     `gorm:"type:varchar(100);not null;index" json:"source"`
	SourceVersion       string     `gorm:"type:varchar(100);not null;default:''" json:"source_version"`
	AnchorKind          string     `gorm:"type:varchar(100)" json:"anchor_kind"`
	HumanLabeledMinutes *float64   `gorm:"type:float8" json:"human_labeled_minutes"`
	WithoutAIMinutes    *float64   `gorm:"column:without_ai_minutes;type:float8" json:"without_ai_minutes"`
	HumanLabeled        bool       `gorm:"type:boolean;default:false" json:"human_labeled"`
	Weight              float64    `gorm:"type:float8;default:1" json:"weight"`
	FeatureVector       ObjectJSON `gorm:"type:jsonb;default:'{}'" json:"feature_vector"`
	Labels              ObjectJSON `gorm:"type:jsonb;default:'{}'" json:"labels"`
	ValidFrom           *time.Time `gorm:"type:timestamptz;index" json:"valid_from"`
	ValidTo             *time.Time `gorm:"type:timestamptz" json:"valid_to"`
	CreatedAt           time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt           time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (AnchorSet) TableName() string { return "anchor_set" }

type BaselineCoefficient struct {
	CoefVersion   string     `gorm:"column:coef_version;primaryKey;type:varchar(100)" json:"coef_version"`
	CreatedTs     time.Time  `gorm:"column:created_ts;type:timestamptz;not null;index" json:"created_ts"`
	Algo          ObjectJSON `gorm:"type:jsonb;default:'{}'" json:"algo"`
	Metadata      ObjectJSON `gorm:"type:jsonb;default:'{}'" json:"metadata"`
	EffectiveFrom *time.Time `gorm:"type:timestamptz;index" json:"effective_from"`
	EffectiveTo   *time.Time `gorm:"type:timestamptz" json:"effective_to"`
	Source        string     `gorm:"type:varchar(100)" json:"source"`
	CreatedAt     time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (BaselineCoefficient) TableName() string { return "baseline_coefficients" }

type BaselineFusionWeight struct {
	FusionWeightId   string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"fusion_weight_id"`
	TeamId           string     `gorm:"type:varchar(255);not null;default:'default';index" json:"team_id"`
	SnapshotTs       time.Time  `gorm:"column:snapshot_ts;type:timestamptz;not null;index" json:"snapshot_ts"`
	WeekStart        time.Time  `gorm:"type:date;not null;index" json:"week_start"`
	WeightAlgo       float64    `gorm:"type:float8;not null;default:1" json:"weight_algo"`
	WeightKNN        float64    `gorm:"column:weight_knn;type:float8;not null;default:0" json:"weight_knn"`
	WeightLLM        float64    `gorm:"column:weight_llm;type:float8;not null;default:0" json:"weight_llm"`
	MADAlgo          *float64   `gorm:"column:mad_algo;type:float8" json:"mad_algo"`
	MADKNN           *float64   `gorm:"column:mad_knn;type:float8" json:"mad_knn"`
	MADLLM           *float64   `gorm:"column:mad_llm;type:float8" json:"mad_llm"`
	HoldOutMAEAlgo   *float64   `gorm:"column:hold_out_mae_algo;type:float8" json:"hold_out_mae_algo"`
	HoldOutMAEKNN    *float64   `gorm:"column:hold_out_mae_knn;type:float8" json:"hold_out_mae_knn"`
	HoldOutMAELLM    *float64   `gorm:"column:hold_out_mae_llm;type:float8" json:"hold_out_mae_llm"`
	TeamWorkDensity  float64    `gorm:"type:float8;not null;default:1" json:"team_work_density"`
	DensitySource    string     `gorm:"type:varchar(100)" json:"density_source"`
	ColdStartDefault bool       `gorm:"type:boolean;default:false" json:"cold_start_default"`
	SampleCount      int64      `gorm:"type:bigint;default:0" json:"sample_count"`
	Reason           string     `gorm:"type:text" json:"reason"`
	Metadata         ObjectJSON `gorm:"type:jsonb;default:'{}'" json:"metadata"`
	CreatedAt        time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (BaselineFusionWeight) TableName() string { return "baseline_fusion_weights" }

func migrateEfficiencyV2DDL(db *gorm.DB) error {
	statements := []struct {
		name string
		ddl  string
	}{
		{"commits touched files column", `ALTER TABLE commits ADD COLUMN IF NOT EXISTS touched_files jsonb DEFAULT '[]'::jsonb`},
		{"commits touched files default", `ALTER TABLE commits ALTER COLUMN touched_files SET DEFAULT '[]'::jsonb`},
		{"conversation_events.touched_files default", `ALTER TABLE conversation_events ALTER COLUMN touched_files SET DEFAULT '[]'::jsonb`},
		{"conversation_events.payload default", `ALTER TABLE conversation_events ALTER COLUMN payload SET DEFAULT '{}'::jsonb`},
		{"conversation_events logical unique index", `CREATE UNIQUE INDEX IF NOT EXISTS ux_conversation_events_logical ON conversation_events (session_id, request_id, event_start_ts, event_kind, source, COALESCE(tool_name, ''))`},
		{"conversation_events session start index", `CREATE INDEX IF NOT EXISTS idx_conversation_events_session_start ON conversation_events (session_id, event_start_ts)`},
		{"conversation_events task start index", `CREATE INDEX IF NOT EXISTS idx_conversation_events_task_start ON conversation_events (task_id, event_start_ts)`},
		{"conversation_events source quality index", `CREATE INDEX IF NOT EXISTS idx_conversation_events_source_quality ON conversation_events (source, parse_quality)`},
		{"session_stage_metrics event kind counts default", `ALTER TABLE session_stage_metrics ALTER COLUMN event_kind_counts SET DEFAULT '{}'::jsonb`},
		{"session_stage_metrics user start index", `CREATE INDEX IF NOT EXISTS idx_session_stage_metrics_user_start ON session_stage_metrics (user_id, session_start_ts)`},
		{"session_stage_metrics confidence index", `CREATE INDEX IF NOT EXISTS idx_session_stage_metrics_confidence ON session_stage_metrics (stage_confidence)`},
		{"needs boundary evidence default", `ALTER TABLE needs ALTER COLUMN boundary_evidence SET DEFAULT '{}'::jsonb`},
		{"needs contributor user ids default", `ALTER TABLE needs ALTER COLUMN contributor_user_ids SET DEFAULT '[]'::jsonb`},
		{"needs session ids default", `ALTER TABLE needs ALTER COLUMN session_ids SET DEFAULT '[]'::jsonb`},
		{"needs commit ids default", `ALTER TABLE needs ALTER COLUMN commit_ids SET DEFAULT '[]'::jsonb`},
		{"needs touched files default", `ALTER TABLE needs ALTER COLUMN touched_files SET DEFAULT '[]'::jsonb`},
		{"needs uncovered commit ids default", `ALTER TABLE needs ALTER COLUMN uncovered_commit_ids SET DEFAULT '[]'::jsonb`},
		{"needs quality signals default", `ALTER TABLE needs ALTER COLUMN quality_signals SET DEFAULT '{}'::jsonb`},
		{"needs confidence signals default", `ALTER TABLE needs ALTER COLUMN confidence_signals SET DEFAULT '{}'::jsonb`},
		{"needs boundary unique index", `CREATE UNIQUE INDEX IF NOT EXISTS ux_needs_boundary_key ON needs (boundary_source, boundary_key)`},
		{"needs repo branch index", `CREATE INDEX IF NOT EXISTS idx_needs_repo_branch ON needs (repo_addr, repo_branch)`},
		{"needs status confidence index", `CREATE INDEX IF NOT EXISTS idx_needs_status_confidence ON needs (status, boundary_confidence)`},
		{"needs primary user status index", `CREATE INDEX IF NOT EXISTS idx_needs_primary_user_status ON needs (primary_user_id, status)`},
		{"needs dev end index", `CREATE INDEX IF NOT EXISTS idx_needs_dev_end_ts ON needs (dev_end_ts)`},
		{"needs outlier index", `CREATE INDEX IF NOT EXISTS idx_needs_outlier_flag ON needs (outlier_flag)`},
		{"user_productivity_v2 need ids default", `ALTER TABLE user_productivity_v2 ALTER COLUMN need_ids SET DEFAULT '[]'::jsonb`},
		{"user_productivity_v2 user week unique index", `CREATE UNIQUE INDEX IF NOT EXISTS ux_user_productivity_v2_user_week ON user_productivity_v2 (user_id, week_start)`},
		{"user_productivity_v2 week index", `CREATE INDEX IF NOT EXISTS idx_user_productivity_v2_week_start ON user_productivity_v2 (week_start)`},
		{"user_productivity_v2 confidence limited index", `CREATE INDEX IF NOT EXISTS idx_user_productivity_v2_confidence_limited ON user_productivity_v2 (confidence_limited)`},
		{"anchor_set feature vector default", `ALTER TABLE anchor_set ALTER COLUMN feature_vector SET DEFAULT '{}'::jsonb`},
		{"anchor_set labels default", `ALTER TABLE anchor_set ALTER COLUMN labels SET DEFAULT '{}'::jsonb`},
		{"anchor_set source anchor unique index", `CREATE UNIQUE INDEX IF NOT EXISTS ux_anchor_set_source_anchor ON anchor_set (source, source_version, anchor_id)`},
		{"anchor_set source index", `CREATE INDEX IF NOT EXISTS idx_anchor_set_source ON anchor_set (source)`},
		{"anchor_set valid from index", `CREATE INDEX IF NOT EXISTS idx_anchor_set_valid_from ON anchor_set (valid_from)`},
		{"baseline_coefficients algo default", `ALTER TABLE baseline_coefficients ALTER COLUMN algo SET DEFAULT '{}'::jsonb`},
		{"baseline_coefficients metadata default", `ALTER TABLE baseline_coefficients ALTER COLUMN metadata SET DEFAULT '{}'::jsonb`},
		{"baseline_coefficients version unique index", `CREATE UNIQUE INDEX IF NOT EXISTS ux_baseline_coefficients_coef_version ON baseline_coefficients (coef_version)`},
		{"baseline_coefficients effective from index", `CREATE INDEX IF NOT EXISTS idx_baseline_coefficients_effective_from ON baseline_coefficients (effective_from)`},
		{"baseline_fusion_weights metadata default", `ALTER TABLE baseline_fusion_weights ALTER COLUMN metadata SET DEFAULT '{}'::jsonb`},
		{"baseline_fusion_weights team snapshot unique index", `CREATE UNIQUE INDEX IF NOT EXISTS ux_baseline_fusion_weights_team_snapshot ON baseline_fusion_weights (team_id, snapshot_ts)`},
		{"baseline_fusion_weights snapshot index", `CREATE INDEX IF NOT EXISTS idx_baseline_fusion_weights_snapshot_ts ON baseline_fusion_weights (snapshot_ts)`},
		{"baseline_fusion_weights week index", `CREATE INDEX IF NOT EXISTS idx_baseline_fusion_weights_week_start ON baseline_fusion_weights (week_start)`},
	}
	for _, stmt := range statements {
		if err := db.Exec(stmt.ddl).Error; err != nil {
			return fmt.Errorf("%s: %w", stmt.name, err)
		}
	}
	return nil
}
