package main

import (
	"encoding/json"
	"time"
)

type ErrorResponse struct {
	Error string `json:"error" example:"参数错误"`
}

type StatusResponse struct {
	Status string `json:"status" example:"ok"`
}

type StatusMessageResponse struct {
	Message string `json:"message" example:"操作成功"`
}

// --- ES Handler ---

type RawDataHit struct {
	ID        string                 `json:"_id"`
	Source    map[string]interface{} `json:"_source"`
	Score     float64                `json:"_score"`
	Index     string                 `json:"_index"`
	Type      string                 `json:"_type"`
	Timestamp interface{}            `json:"@timestamp,omitempty"`
}

type RawDataResponse struct {
	Total    int64        `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"pageSize"`
	Hits     []RawDataHit `json:"hits"`
}

// --- VirtualGroup / Favorites ---

type VirtualGroupResponse struct {
	ID         int      `json:"id"`
	Name       string   `json:"name"`
	Dimension  string   `json:"dimension"`
	MemberKeys []string `json:"member_keys"`
}

type AggregateVirtualGroupResponse struct {
	Key             string  `json:"key"`
	Name            string  `json:"name"`
	UserInChars     float64 `json:"user_in_chars"`
	CodeLines       float64 `json:"code_lines"`
	APICount        float64 `json:"api_count"`
	APICost         float64 `json:"api_cost"`
	APIInTokens     float64 `json:"api_in_tokens"`
	APIOutTokens    float64 `json:"api_out_tokens"`
	TaskCount       float64 `json:"task_count"`
	AIEstimatedDays float64 `json:"ai_estimated_days"`
	StartTime       string  `json:"start_time"`
	EndTime         string  `json:"end_time"`
	LeadTime        float64 `json:"lead_time"`
	ProcessTime     float64 `json:"process_time"`
}

type FavoriteResponse struct {
	ID          int    `json:"id"`
	Dimension   string `json:"dimension"`
	ItemKey     string `json:"item_key"`
	DisplayName string `json:"display_name"`
}

type FavoriteItem struct {
	ID             int    `json:"id"`
	Dimension      string `json:"dimension"`
	ItemKey        string `json:"item_key"`
	DisplayName    string `json:"display_name"`
	VirtualGroupID *int64 `json:"virtual_group_id"`
	IsVirtual      bool   `json:"is_virtual"`
}

type CreateVirtualGroupRequest struct {
	Name       string   `json:"name" example:"前端组"`
	Dimension  string   `json:"dimension" example:"work_dir"`
	MemberKeys []string `json:"member_keys"`
}

type CreateFavoriteRequest struct {
	Dimension   string `json:"dimension" example:"work_dir"`
	ItemKey     string `json:"item_key" example:"project1"`
	DisplayName string `json:"display_name" example:"项目1"`
}

// --- Attribution ---

type TaskCommitMappingItem struct {
	TaskID      string   `json:"task_id"`
	CommitHash  string   `json:"commit_hash"`
	CodeSource  string   `json:"code_source"`
	UserID      *string  `json:"user_id,omitempty"`
	MatchScore  *float64 `json:"match_score,omitempty"`
	MatchReason *string  `json:"match_reason,omitempty"`
}

type TaskCommitMappingsResponse struct {
	Items []TaskCommitMappingItem `json:"items"`
}

type CodeAttributionSummary struct {
	TotalOurAILines int64 `json:"total_our_ai_lines"`
	TotalHumanLines int64 `json:"total_human_lines"`
}

type CodeAttributionDetail struct {
	CommitHash string  `json:"commit_hash"`
	OurAILines int64   `json:"our_ai_lines"`
	HumanLines int64   `json:"human_lines"`
	TaskID     *string `json:"task_id,omitempty"`
}

type CodeAttributionResponse struct {
	Summary CodeAttributionSummary  `json:"summary"`
	Details []CodeAttributionDetail `json:"details"`
}

type CodeSourceItem struct {
	Lines      int64   `json:"lines"`
	Percentage float64 `json:"percentage"`
}

type CodeSourceGroup struct {
	AICurrent CodeSourceItem `json:"ai_current"`
	Human     CodeSourceItem `json:"human"`
	AIOther   CodeSourceItem `json:"ai_other"`
	Unknown   CodeSourceItem `json:"unknown"`
}

type CodeSourceStatsResponse struct {
	CodeSource      CodeSourceGroup `json:"code_source"`
	MappedTaskCount int             `json:"mapped_task_count"`
}

// --- Git ---

type GitStatsInfo struct {
	CommitCount      *int   `json:"commit_count,omitempty"`
	ContributorCount *int   `json:"contributor_count,omitempty"`
	LinesAdded       *int64 `json:"lines_added,omitempty"`
	LinesDeleted     *int64 `json:"lines_deleted,omitempty"`
	FilesChanged     *int   `json:"files_changed,omitempty"`
}

type EstimationInfo struct {
	FromTask *float64 `json:"from_task,omitempty"`
	FromGit  *float64 `json:"from_git,omitempty"`
	Final    *float64 `json:"final,omitempty"`
}

type GitAnalysisResponse struct {
	RepoID          string          `json:"repo_id"`
	AnalysisDate    string          `json:"analysis_date"`
	GitStats        *GitStatsInfo   `json:"git_stats"`
	Estimation      *EstimationInfo `json:"estimation"`
	GitAnalysisFile *string         `json:"git_analysis_file,omitempty"`
}

// --- Aggregate ---

type AggregateItem struct {
	Key             string  `json:"key"`
	UserInChars     float64 `json:"user_in_chars"`
	CodeLines       float64 `json:"code_lines"`
	APICount        float64 `json:"api_count"`
	APICost         float64 `json:"api_cost"`
	APIInTokens     float64 `json:"api_in_tokens"`
	APIOutTokens    float64 `json:"api_out_tokens"`
	TaskCount       float64 `json:"task_count"`
	AIEstimatedDays float64 `json:"ai_estimated_days"`
	StartTime       string  `json:"start_time"`
	EndTime         string  `json:"end_time"`
	LeadTime        float64 `json:"lead_time"`
	ProcessTime     float64 `json:"process_time"`
}

type AggregateResponse struct {
	Total    int             `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"pageSize"`
	Items    []AggregateItem `json:"items"`
}

type AggregateKeysResponse struct {
	Keys  []string `json:"keys"`
	Total int      `json:"total"`
}

// --- Efficiency ---

type AIEstimatedInfo struct {
	RawDays       float64  `json:"raw_days"`
	CorrectedDays *float64 `json:"corrected_days"`
	IsCorrected   bool     `json:"is_corrected"`
	Reasons       []string `json:"reasons"`
}

type ActualTimeUser struct {
	UserID        string `json:"user_id"`
	UserName      string `json:"user_name"`
	StartTime     string `json:"start_time"`
	EndTime       string `json:"end_time"`
	LeadTimeMs    int64  `json:"lead_time_ms"`
	ProcessTimeMs int64  `json:"process_time_ms"`
}

type ActualTimeInfo struct {
	TotalLeadTimeMs    int64            `json:"total_lead_time_ms"`
	TotalProcessTimeMs int64            `json:"total_process_time_ms"`
	TotalCodeLines     int64            `json:"total_code_lines"`
	UserCount          int              `json:"user_count"`
	StartTime          string           `json:"start_time"`
	EndTime            string           `json:"end_time"`
	Users              []ActualTimeUser `json:"users"`
}

type EfficiencyInfo struct {
	RatioLead    float64 `json:"ratio_lead"`
	RatioProcess float64 `json:"ratio_process"`
	Reason       string  `json:"reason"`
}

type CostInfo struct {
	APICost    float64 `json:"api_cost"`
	DailyRate  float64 `json:"daily_rate"`
	CostSaving float64 `json:"cost_saving"`
	ROI        float64 `json:"roi"`
}

type EfficiencyResponse struct {
	Dimension    string          `json:"dimension"`
	DimensionID  string          `json:"dimension_id"`
	AnalysisDate string          `json:"analysis_date"`
	AIEstimated  AIEstimatedInfo `json:"ai_estimated"`
	ActualTime   ActualTimeInfo  `json:"actual_time"`
	Efficiency   EfficiencyInfo  `json:"efficiency"`
	Cost         CostInfo        `json:"cost"`
	AnalysisFile string          `json:"analysis_file"`
}

type CorrectionHistoryItem struct {
	FieldName   string  `json:"field_name"`
	OldValue    *string `json:"old_value,omitempty"`
	NewValue    *string `json:"new_value,omitempty"`
	Reason      *string `json:"reason,omitempty"`
	CorrectedBy *string `json:"corrected_by,omitempty"`
	CorrectedAt *string `json:"corrected_at,omitempty"`
}

type EfficiencyHistoryResponse struct {
	Items []CorrectionHistoryItem `json:"items"`
}

// --- Org v2 ---

type OrgDataItem struct {
	OrgName               string  `json:"org_name"`
	UserCount             int     `json:"user_count"`
	TaskCount             int     `json:"task_count"`
	CommitCount           int     `json:"commit_count"`
	TaskDiffLines         int     `json:"task_diff_lines"`
	CommitDiffLines       int     `json:"commit_diff_lines"`
	TaskEfficiencyRatio   float64 `json:"task_efficiency_ratio"`
	CommitEfficiencyRatio float64 `json:"commit_efficiency_ratio"`
	TotalTokens           int64   `json:"total_tokens"`
	TotalCost             float64 `json:"total_cost"`
}

type OrgSeriesPoint struct {
	Period                string  `json:"period"`
	UserCount             int     `json:"user_count"`
	TaskCount             int     `json:"task_count"`
	CommitCount           int     `json:"commit_count"`
	TaskDiffLines         int     `json:"task_diff_lines"`
	CommitDiffLines       int     `json:"commit_diff_lines"`
	TaskEfficiencyRatio   float64 `json:"task_efficiency_ratio"`
	CommitEfficiencyRatio float64 `json:"commit_efficiency_ratio"`
	TotalTokens           int64   `json:"total_tokens"`
	TotalCost             float64 `json:"total_cost"`
}

type OrgSeriesItem struct {
	OrgName string           `json:"org_name"`
	Periods []string         `json:"periods"`
	Points  []OrgSeriesPoint `json:"points"`
}

type OrgListResponse struct {
	Data    []OrgDataItem   `json:"data"`
	Series  []OrgSeriesItem `json:"series"`
	Periods []string        `json:"periods,omitempty"`
}

type OrgSummary struct {
	UserCount             int     `json:"user_count"`
	TaskDiffLines         int     `json:"task_diff_lines"`
	CommitDiffLines       int     `json:"commit_diff_lines"`
	TaskRealMinutes       float64 `json:"task_real_minutes"`
	TaskAncientMinutes    float64 `json:"task_ancient_minutes"`
	TaskEfficiencyRatio   float64 `json:"task_efficiency_ratio"`
	CommitRealMinutes     float64 `json:"commit_real_minutes"`
	CommitAncientMinutes  float64 `json:"commit_ancient_minutes"`
	CommitEfficiencyRatio float64 `json:"commit_efficiency_ratio"`
	UpstreamTokens        int64   `json:"upstream_tokens"`
	DownstreamTokens      int64   `json:"downstream_tokens"`
	Cost                  float64 `json:"cost"`
}

type OrgMemberItem struct {
	UserID                string  `json:"user_id"`
	UserName              string  `json:"user_name"`
	Org1                  string  `json:"org1"`
	Org2                  string  `json:"org2"`
	Org3                  string  `json:"org3"`
	Org4                  string  `json:"org4"`
	TaskDiffLines         int     `json:"task_diff_lines"`
	CommitDiffLines       int     `json:"commit_diff_lines"`
	TaskRealMinutes       float64 `json:"task_real_minutes"`
	TaskAncientMinutes    float64 `json:"task_ancient_minutes"`
	TaskEfficiencyRatio   float64 `json:"task_efficiency_ratio"`
	CommitRealMinutes     float64 `json:"commit_real_minutes"`
	CommitAncientMinutes  float64 `json:"commit_ancient_minutes"`
	CommitEfficiencyRatio float64 `json:"commit_efficiency_ratio"`
	UpstreamTokens        int64   `json:"upstream_tokens"`
	DownstreamTokens      int64   `json:"downstream_tokens"`
	Cost                  float64 `json:"cost"`
}

type CommitTimeSeriesItem struct {
	PeriodKey             string  `json:"period_key"`
	PeriodLabel           string  `json:"period_label"`
	CommitCount           int     `json:"commit_count"`
	TaskCount             int     `json:"task_count"`
	CommitDiffLines       int     `json:"commit_diff_lines"`
	CommitRealMinutes     float64 `json:"commit_real_minutes"`
	CommitAncientMinutes  float64 `json:"commit_ancient_minutes"`
	CommitEfficiencyRatio float64 `json:"commit_efficiency_ratio"`
	UpstreamTokens        int64   `json:"upstream_tokens"`
	DownstreamTokens      int64   `json:"downstream_tokens"`
	Cost                  float64 `json:"cost"`
}

type TaskTimeSeriesItem struct {
	PeriodKey           string  `json:"period_key"`
	PeriodLabel         string  `json:"period_label"`
	TaskCount           int     `json:"task_count"`
	CommitCount         int     `json:"commit_count"`
	TaskDiffLines       int     `json:"task_diff_lines"`
	TaskRealMinutes     float64 `json:"task_real_minutes"`
	TaskAncientMinutes  float64 `json:"task_ancient_minutes"`
	TaskEfficiencyRatio float64 `json:"task_efficiency_ratio"`
	UpstreamTokens      int64   `json:"upstream_tokens"`
	DownstreamTokens    int64   `json:"downstream_tokens"`
	Cost                float64 `json:"cost"`
}

type OrgDetailResponse struct {
	OrgPath     string                 `json:"org_path"`
	Summary     OrgSummary             `json:"summary"`
	Commits     []CommitTimeSeriesItem `json:"commits"`
	Tasks       []TaskTimeSeriesItem   `json:"tasks"`
	Members     []OrgMemberItem        `json:"members"`
	Granularity string                 `json:"granularity"`
}

type GroupSummary struct {
	TaskDiffLines         int     `json:"task_diff_lines"`
	CommitDiffLines       int     `json:"commit_diff_lines"`
	TaskRealMinutes       float64 `json:"task_real_minutes"`
	TaskAncientMinutes    float64 `json:"task_ancient_minutes"`
	TaskEfficiencyRatio   float64 `json:"task_efficiency_ratio"`
	CommitRealMinutes     float64 `json:"commit_real_minutes"`
	CommitAncientMinutes  float64 `json:"commit_ancient_minutes"`
	CommitEfficiencyRatio float64 `json:"commit_efficiency_ratio"`
	UpstreamTokens        int64   `json:"upstream_tokens"`
	DownstreamTokens      int64   `json:"downstream_tokens"`
	Cost                  float64 `json:"cost"`
}

type DailyDataItem struct {
	Date                  string  `json:"date"`
	TaskDiffLines         int     `json:"task_diff_lines"`
	CommitDiffLines       int     `json:"commit_diff_lines"`
	TaskRealMinutes       float64 `json:"task_real_minutes"`
	TaskAncientMinutes    float64 `json:"task_ancient_minutes"`
	TaskEfficiencyRatio   float64 `json:"task_efficiency_ratio"`
	CommitRealMinutes     float64 `json:"commit_real_minutes"`
	CommitAncientMinutes  float64 `json:"commit_ancient_minutes"`
	CommitEfficiencyRatio float64 `json:"commit_efficiency_ratio"`
	UpstreamTokens        int64   `json:"upstream_tokens"`
	DownstreamTokens      int64   `json:"downstream_tokens"`
	Cost                  float64 `json:"cost"`
}

type GroupMemberItem struct {
	UserID                string  `json:"user_id"`
	UserName              string  `json:"user_name"`
	Org1                  string  `json:"org1"`
	Org2                  string  `json:"org2"`
	Org3                  string  `json:"org3"`
	Org4                  string  `json:"org4"`
	TaskDiffLines         int     `json:"task_diff_lines"`
	CommitDiffLines       int     `json:"commit_diff_lines"`
	TaskEfficiencyRatio   float64 `json:"task_efficiency_ratio"`
	CommitEfficiencyRatio float64 `json:"commit_efficiency_ratio"`
	Cost                  float64 `json:"cost"`
}

type GroupDetailResponse struct {
	OrgPath string            `json:"org_path"`
	Summary GroupSummary      `json:"summary"`
	Daily   []DailyDataItem   `json:"daily"`
	Members []GroupMemberItem `json:"members"`
}

// --- User v2 ---

type UserListItem struct {
	UserID                string  `json:"user_id"`
	UserName              string  `json:"user_name"`
	DayCount              int     `json:"day_count"`
	TaskCount             int     `json:"task_count"`
	CommitCount           int     `json:"commit_count"`
	TaskDiffLines         int     `json:"task_diff_lines"`
	CommitDiffLines       int     `json:"commit_diff_lines"`
	UpstreamTokens        int64   `json:"upstream_tokens"`
	DownstreamTokens      int64   `json:"downstream_tokens"`
	Cost                  float64 `json:"cost"`
	TaskRealMinutes       float64 `json:"task_real_minutes"`
	TaskAncientMinutes    float64 `json:"task_ancient_minutes"`
	TaskEfficiencyRatio   float64 `json:"task_efficiency_ratio"`
	CommitRealMinutes     float64 `json:"commit_real_minutes"`
	CommitAncientMinutes  float64 `json:"commit_ancient_minutes"`
	CommitEfficiencyRatio float64 `json:"commit_efficiency_ratio"`
	Org1                  string  `json:"org1"`
	Org2                  string  `json:"org2"`
	Org3                  string  `json:"org3"`
	Org4                  string  `json:"org4"`
	OrgDisplay            string  `json:"org_display"`
	IsVirtualGroup        bool    `json:"is_virtual_group"`
	OrgName               string  `json:"org_name"`
	GroupID               string  `json:"group_id,omitempty"`
}

type UserSeriesPoint struct {
	Period                string  `json:"period"`
	TaskCount             int     `json:"task_count"`
	CommitCount           int     `json:"commit_count"`
	TaskDiffLines         int     `json:"task_diff_lines"`
	CommitDiffLines       int     `json:"commit_diff_lines"`
	TaskEfficiencyRatio   float64 `json:"task_efficiency_ratio"`
	CommitEfficiencyRatio float64 `json:"commit_efficiency_ratio"`
	TotalTokens           int64   `json:"total_tokens"`
	TotalCost             float64 `json:"total_cost"`
	TaskRealMinutes       float64 `json:"task_real_minutes"`
	TaskAncientMinutes    float64 `json:"task_ancient_minutes"`
	CommitRealMinutes     float64 `json:"commit_real_minutes"`
	CommitAncientMinutes  float64 `json:"commit_ancient_minutes"`
}

type UserSeriesItem struct {
	UserID   string            `json:"user_id"`
	UserName string            `json:"user_name"`
	Periods  []string          `json:"periods"`
	Points   []UserSeriesPoint `json:"points"`
}

type UsersListResponse struct {
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"pageSize"`
	Data     []UserListItem   `json:"data"`
	Series   []UserSeriesItem `json:"series"`
	Periods  []string         `json:"periods"`
}

type UserDetailSummary struct {
	UserID                string  `json:"user_id"`
	UserName              string  `json:"user_name"`
	DayCount              int     `json:"day_count"`
	TaskCount             int     `json:"task_count"`
	CommitCount           int     `json:"commit_count"`
	TaskDiffLines         int     `json:"task_diff_lines"`
	CommitDiffLines       int     `json:"commit_diff_lines"`
	UpstreamTokens        int64   `json:"upstream_tokens"`
	DownstreamTokens      int64   `json:"downstream_tokens"`
	Cost                  float64 `json:"cost"`
	TaskRealMinutes       float64 `json:"task_real_minutes"`
	TaskAncientMinutes    float64 `json:"task_ancient_minutes"`
	TaskEfficiencyRatio   float64 `json:"task_efficiency_ratio"`
	CommitRealMinutes     float64 `json:"commit_real_minutes"`
	CommitAncientMinutes  float64 `json:"commit_ancient_minutes"`
	CommitEfficiencyRatio float64 `json:"commit_efficiency_ratio"`
}

type UserDetailResponse struct {
	Summary     UserDetailSummary      `json:"summary"`
	Daily       []UserProductivity     `json:"daily"`
	Commits     []CommitTimeSeriesItem `json:"commits"`
	Tasks       []TaskTimeSeriesItem   `json:"tasks"`
	Total       int                    `json:"total"`
	Granularity string                 `json:"granularity"`
}

// --- Dashboard ---

type DashboardSummaryResponse struct {
	TotalTasks              int      `json:"total_tasks"`
	TotalUsers              int      `json:"total_users"`
	TotalRepos              int      `json:"total_repos"`
	TotalCommits            int      `json:"total_commits"`
	TotalWorkDirs           int      `json:"total_work_dirs"`
	TotalCost               float64  `json:"total_cost"`
	TotalTokens             int64    `json:"total_tokens"`
	TotalDiffLines          int64    `json:"total_diff_lines"`
	TotalTaskAncientMinutes float64  `json:"total_task_ancient_minutes"`
	TotalRealMinutes        float64  `json:"total_real_minutes"`
	AvgEfficiencyRatio      *float64 `json:"avg_efficiency_ratio"`
}

// --- Repo v2 ---

type RepoListItem struct {
	RepoAddr          *string  `json:"repo_addr"`
	RepoBranch        *string  `json:"repo_branch"`
	CommitCount       int      `json:"commit_count"`
	StartTime         string   `json:"start_time"`
	EndTime           string   `json:"end_time"`
	SumAncientMinutes *float64 `json:"sum_ancient_minutes"`
	SumRealMinutes    *float64 `json:"sum_real_minutes"`
	TaskCount         int      `json:"task_count"`
	EfficiencyRatio   *float64 `json:"efficiency_ratio"`
}

type ReposListResponse struct {
	Total    int            `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
	Data     []RepoListItem `json:"data"`
}

type RepoEfficiency struct {
	RepoAncientMinutes       float64  `json:"repo_ancient_minutes"`
	RepoRealMinutes          float64  `json:"repo_real_minutes"`
	EfficiencyRatio          *float64 `json:"efficiency_ratio"`
	RepoAncientMinutesReason string   `json:"repo_ancient_minutes_reason"`
	RepoRealMinutesReason    string   `json:"repo_real_minutes_reason"`
}

type RepoSummary struct {
	CommitCount int `json:"commit_count"`
	TaskCount   int `json:"task_count"`
}

type RepoCommitItem struct {
	CommitID                         string          `json:"commit_id"`
	CommitTime                       *time.Time      `json:"commit_time"`
	RepoAddr                         *string         `json:"repo_addr"`
	RepoBranch                       *string         `json:"repo_branch"`
	GitUserName                      *string         `json:"git_user_name"`
	GitUserEmail                     *string         `json:"git_user_email"`
	UserID                           *string         `json:"user_id"`
	UserName                         *string         `json:"user_name"`
	ClientID                         *string         `json:"client_id"`
	WorkDir                          *string         `json:"work_dir"`
	DiffLines                        *int            `json:"diff_lines"`
	CommitAncientMinutes             *float64        `json:"commit_ancient_minutes"`
	CommitAncientMinutesReason       *string         `json:"commit_ancient_minutes_reason"`
	CommitAncientMinutesManual       *float64        `json:"commit_ancient_minutes_manual"`
	CommitAncientMinutesReasonManual *string         `json:"commit_ancient_minutes_reason_manual"`
	TaskIDs                          json.RawMessage `json:"task_ids" swaggertype:"string" example:"[\"task1\"]"`
	TaskIDsSilica                    json.RawMessage `json:"task_ids_silica" swaggertype:"string" example:"[\"1.0\"]"`
	CommitRealMinutes                *float64        `json:"commit_real_minutes"`
	CommitRealMinutesReason          *string         `json:"commit_real_minutes_reason"`
	CommitRealMinutesManual          *float64        `json:"commit_real_minutes_manual"`
	CommitRealMinutesReasonManual    *string         `json:"commit_real_minutes_reason_manual"`
	CommitRealAIMinutes              *float64        `json:"commit_real_ai_minutes"`
	CommitRealAncientMinutes         *float64        `json:"commit_real_ancient_minutes"`
	Comment                          *string         `json:"comment"`
	CreatedAt                        *time.Time      `json:"created_at"`
	UpdatedAt                        *time.Time      `json:"updated_at"`
	Cost                             float64         `json:"cost"`
	UpstreamTokens                   int64           `json:"upstream_tokens"`
	DownstreamTokens                 int64           `json:"downstream_tokens"`
	Silica                           *float64        `json:"silica"`
	EfficiencyRatio                  *float64        `json:"efficiency_ratio"`
}

type RepoDetailResponse struct {
	RepoAddr   string           `json:"repo_addr"`
	RepoBranch string           `json:"repo_branch"`
	Branches   []string         `json:"branches"`
	Commits    []RepoCommitItem `json:"commits"`
	Tasks      []StatTask       `json:"tasks"`
	Efficiency RepoEfficiency   `json:"efficiency"`
	Summary    RepoSummary      `json:"summary"`
}

type RepoBranchesResponse struct {
	Branches []string `json:"branches"`
}

// --- User Group v2 ---

type UserGroupMember struct {
	UserID                string  `json:"user_id"`
	UserName              string  `json:"user_name"`
	DayCount              int     `json:"day_count"`
	TaskCount             int     `json:"task_count"`
	CommitCount           int     `json:"commit_count"`
	TaskDiffLines         int     `json:"task_diff_lines"`
	UpstreamTokens        int64   `json:"upstream_tokens"`
	DownstreamTokens      int64   `json:"downstream_tokens"`
	Cost                  float64 `json:"cost"`
	TaskRealMinutes       float64 `json:"task_real_minutes"`
	TaskAncientMinutes    float64 `json:"task_ancient_minutes"`
	TaskEfficiencyRatio   float64 `json:"task_efficiency_ratio"`
	CommitDiffLines       int     `json:"commit_diff_lines"`
	CommitAncientMinutes  float64 `json:"commit_ancient_minutes"`
	CommitRealMinutes     float64 `json:"commit_real_minutes"`
	CommitEfficiencyRatio float64 `json:"commit_efficiency_ratio"`
}

type UserGroupSummary struct {
	DayCount              int     `json:"day_count"`
	TaskCount             int     `json:"task_count"`
	CommitCount           int     `json:"commit_count"`
	TaskDiffLines         int     `json:"task_diff_lines"`
	UpstreamTokens        int64   `json:"upstream_tokens"`
	DownstreamTokens      int64   `json:"downstream_tokens"`
	Cost                  float64 `json:"cost"`
	TaskRealMinutes       float64 `json:"task_real_minutes"`
	TaskAncientMinutes    float64 `json:"task_ancient_minutes"`
	TaskEfficiencyRatio   float64 `json:"task_efficiency_ratio"`
	CommitDiffLines       int     `json:"commit_diff_lines"`
	CommitAncientMinutes  float64 `json:"commit_ancient_minutes"`
	CommitRealMinutes     float64 `json:"commit_real_minutes"`
	CommitEfficiencyRatio float64 `json:"commit_efficiency_ratio"`
}

type UserGroupDetailResponse struct {
	Group   *UserGroup        `json:"group"`
	Summary UserGroupSummary  `json:"summary"`
	Members []UserGroupMember `json:"members"`
}

type CreateUserGroupRequest struct {
	Name    string   `json:"name" example:"前端组"`
	OrgName string   `json:"org_name" example:"技术部"`
	UserIDs []string `json:"user_ids"`
}

// --- Task v2 ---

type TaskListItem struct {
	TaskID                         string     `json:"task_id"`
	Title                          *string    `json:"title"`
	UserID                         *string    `json:"user_id"`
	UserName                       *string    `json:"user_name"`
	ClientID                       *string    `json:"client_id"`
	ClientIDE                      *string    `json:"client_ide"`
	ClientVersion                  *string    `json:"client_version"`
	ClientOS                       *string    `json:"client_os"`
	ClientOSVersion                *string    `json:"client_os_version"`
	Caller                         *string    `json:"caller"`
	RepoAddr                       *string    `json:"repo_addr"`
	RepoBranch                     *string    `json:"repo_branch"`
	WorkDir                        *string    `json:"work_dir"`
	WorkDirID                      *string    `json:"work_dir_id"`
	StartTime                      *time.Time `json:"start_time"`
	EndTime                        *time.Time `json:"end_time"`
	UpstreamTokens                 *int64     `json:"upstream_tokens"`
	DownstreamTokens               *int64     `json:"downstream_tokens"`
	Cost                           *float64   `json:"cost"`
	DiffLines                      *int       `json:"diff_lines"`
	TaskAncientMinutes             *float64   `json:"task_ancient_minutes"`
	TaskAncientMinutesReason       *string    `json:"task_ancient_minutes_reason"`
	TaskAncientMinutesManual       *float64   `json:"task_ancient_minutes_manual"`
	TaskAncientMinutesReasonManual *string    `json:"task_ancient_minutes_reason_manual"`
	TaskRealMinutes                *float64   `json:"task_real_minutes"`
	TaskRealMinutesReason          *string    `json:"task_real_minutes_reason"`
	TaskRealMinutesManual          *float64   `json:"task_real_minutes_manual"`
	TaskRealMinutesReasonManual    *string    `json:"task_real_minutes_reason_manual"`
	CreatedAt                      *time.Time `json:"created_at"`
	UpdatedAt                      *time.Time `json:"updated_at"`
	EfficiencyRatio                *float64   `json:"efficiency_ratio"`
	Org1                           string     `json:"org1"`
	Org2                           string     `json:"org2"`
	Org3                           string     `json:"org3"`
	Org4                           string     `json:"org4"`
}

type TaskListResponse struct {
	Total    int            `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
	Data     []TaskListItem `json:"data"`
}

type TaskDetailResponse struct {
	Task            *StatTask              `json:"task"`
	Conversations   []StatTaskConversation `json:"conversations"`
	TimeSegments    []TimeSegment          `json:"time_segments"`
	EfficiencyRatio *float64               `json:"efficiency_ratio"`
}

type EstimateAncientResult struct {
	TaskID  string  `json:"task_id"`
	Minutes float64 `json:"minutes"`
	Reason  string  `json:"reason"`
	Error   string  `json:"error,omitempty"`
}

type EstimateAncientResponse struct {
	Status  string                  `json:"status"`
	Total   int                     `json:"total"`
	Success int                     `json:"success"`
	Results []EstimateAncientResult `json:"results"`
}

type UpdateTaskManualRequest struct {
	TaskRealMinutesManual          *float64 `json:"task_real_minutes_manual"`
	TaskRealMinutesReasonManual    *string  `json:"task_real_minutes_reason_manual"`
	TaskAncientMinutesManual       *float64 `json:"task_ancient_minutes_manual"`
	TaskAncientMinutesReasonManual *string  `json:"task_ancient_minutes_reason_manual"`
}

// --- Project v2 ---

type ProjectListItem struct {
	ProjectID                             string          `json:"project_id"`
	Name                                  string          `json:"name"`
	Description                           *string         `json:"description"`
	Repos                                 json.RawMessage `json:"repos" swaggertype:"string"`
	TaskIDs                               json.RawMessage `json:"task_ids" swaggertype:"string"`
	TaskIDsSilica                         json.RawMessage `json:"task_ids_silica" swaggertype:"string"`
	StartTime                             *time.Time      `json:"start_time"`
	EndTime                               *time.Time      `json:"end_time"`
	StartTimeManual                       *time.Time      `json:"start_time_manual"`
	EndTimeManual                         *time.Time      `json:"end_time_manual"`
	UpstreamTokens                        *int64          `json:"upstream_tokens"`
	DownstreamTokens                      *int64          `json:"downstream_tokens"`
	Cost                                  *float64        `json:"cost"`
	ProjectAncientMinutes                 *float64        `json:"project_ancient_minutes"`
	ProjectAncientMinutesReason           *string         `json:"project_ancient_minutes_reason"`
	ProjectAncientMinutesManual           *float64        `json:"project_ancient_minutes_manual"`
	ProjectAncientMinutesReasonManual     *string         `json:"project_ancient_minutes_reason_manual"`
	ProjectRealProcessMinutes             *float64        `json:"project_real_process_minutes"`
	ProjectRealProcessMinutesReason       *string         `json:"project_real_process_minutes_reason"`
	ProjectRealProcessMinutesManual       *float64        `json:"project_real_process_minutes_manual"`
	ProjectRealProcessMinutesReasonManual *string         `json:"project_real_process_minutes_reason_manual"`
	ProjectRealLeadMinutes                *float64        `json:"project_real_lead_minutes"`
	ProjectRealLeadMinutesReason          *string         `json:"project_real_lead_minutes_reason"`
	ProjectRealLeadMinutesManual          *float64        `json:"project_real_lead_minutes_manual"`
	ProjectRealLeadMinutesReasonManual    *string         `json:"project_real_lead_minutes_reason_manual"`
	CreatedAt                             *time.Time      `json:"created_at"`
	UpdatedAt                             *time.Time      `json:"updated_at"`
	RepoCount                             int             `json:"repo_count"`
	TaskCount                             int             `json:"task_count"`
	UserCount                             int             `json:"user_count"`
	TotalCodeLines                        int64           `json:"total_code_lines"`
	ActualLinesPerDay                     *float64        `json:"actual_lines_per_day"`
	EfficiencyRatio                       *float64        `json:"efficiency_ratio"`
}

type ProjectListResponse struct {
	Data []ProjectListItem `json:"data"`
}

type ProjectCommitItem struct {
	CommitID                   string     `json:"commit_id"`
	CommitTime                 *time.Time `json:"commit_time"`
	RepoAddr                   string     `json:"repo_addr"`
	RepoBranch                 string     `json:"repo_branch"`
	UserName                   *string    `json:"user_name"`
	GitUserName                *string    `json:"git_user_name"`
	DiffLines                  *int       `json:"diff_lines"`
	Comment                    *string    `json:"comment"`
	CommitAncientMinutes       *float64   `json:"commit_ancient_minutes"`
	CommitAncientMinutesManual *float64   `json:"commit_ancient_minutes_manual"`
	CommitRealMinutes          *float64   `json:"commit_real_minutes"`
	CommitRealMinutesManual    *float64   `json:"commit_real_minutes_manual"`
	Silica                     *float64   `json:"silica"`
}

type ProjectTaskItem struct {
	TaskID                   string     `json:"task_id"`
	UserName                 *string    `json:"user_name"`
	StartTime                *time.Time `json:"start_time"`
	EndTime                  *time.Time `json:"end_time"`
	UpstreamTokens           *int64     `json:"upstream_tokens"`
	DownstreamTokens         *int64     `json:"downstream_tokens"`
	Cost                     *float64   `json:"cost"`
	TaskAncientMinutes       *float64   `json:"task_ancient_minutes"`
	TaskAncientMinutesManual *float64   `json:"task_ancient_minutes_manual"`
	TaskRealMinutes          *float64   `json:"task_real_minutes"`
	TaskRealMinutesManual    *float64   `json:"task_real_minutes_manual"`
	Title                    *string    `json:"title"`
	WorkDir                  *string    `json:"work_dir"`
	Silica                   float64    `json:"silica"`
}

type ProjectDetailResponse struct {
	Project         *Project            `json:"project"`
	Commits         []ProjectCommitItem `json:"commits"`
	Tasks           []ProjectTaskItem   `json:"tasks"`
	EfficiencyRatio *float64            `json:"efficiency_ratio"`
	UserCount       int                 `json:"user_count"`
}

type ProjectConflict struct {
	CommitID    string `json:"commit_id"`
	ProjectID   string `json:"project_id"`
	ProjectName string `json:"project_name"`
}

type ProjectConflictsResponse struct {
	Conflicts []ProjectConflict `json:"conflicts"`
}

type CreateProjectRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

type UpdateProjectRequest struct {
	Name          string          `json:"name"`
	Description   *string         `json:"description"`
	Repos         json.RawMessage `json:"repos" swaggertype:"string"`
	TaskIDs       json.RawMessage `json:"task_ids" swaggertype:"string"`
	TaskIDsSilica json.RawMessage `json:"task_ids_silica" swaggertype:"string"`
}

type AddTasksRequest struct {
	TaskIDs       []string  `json:"task_ids"`
	TaskIDsSilica []float64 `json:"task_ids_silica"`
}

type RemoveTasksRequest struct {
	TaskIDs []string `json:"task_ids"`
}

type UpdateTaskSilicaRequest struct {
	TaskID string  `json:"task_id"`
	Silica float64 `json:"silica"`
}

type CheckProjectConflictsRequest struct {
	CommitIDs []string `json:"commit_ids"`
}

type UpdateProjectManualRequest struct {
	ProjectAncientMinutesManual           *float64   `json:"project_ancient_minutes_manual"`
	ProjectAncientMinutesReasonManual     *string    `json:"project_ancient_minutes_reason_manual"`
	ProjectRealProcessMinutesManual       *float64   `json:"project_real_process_minutes_manual"`
	ProjectRealProcessMinutesReasonManual *string    `json:"project_real_process_minutes_reason_manual"`
	ProjectRealLeadMinutesManual          *float64   `json:"project_real_lead_minutes_manual"`
	ProjectRealLeadMinutesReasonManual    *string    `json:"project_real_lead_minutes_reason_manual"`
	StartTimeManual                       *time.Time `json:"start_time_manual"`
	EndTimeManual                         *time.Time `json:"end_time_manual"`
}

type RepoFilter struct {
	RepoAddr           string   `json:"repo_addr"`
	RepoBranch         string   `json:"repo_branch"`
	StartTime          *string  `json:"start_time"`
	EndTime            *string  `json:"end_time"`
	ExcludeCommits     []string `json:"exclude_commits"`
	IncludeOnlyCommits []string `json:"include_only_commits"`
}

// --- Commit v2 ---

type CommitListItem struct {
	CommitID                         string          `json:"commit_id"`
	CommitTime                       *time.Time      `json:"commit_time"`
	RepoAddr                         *string         `json:"repo_addr"`
	RepoBranch                       *string         `json:"repo_branch"`
	GitUserName                      *string         `json:"git_user_name"`
	GitUserEmail                     *string         `json:"git_user_email"`
	UserID                           *string         `json:"user_id"`
	UserName                         *string         `json:"user_name"`
	ClientID                         *string         `json:"client_id"`
	WorkDir                          *string         `json:"work_dir"`
	DiffLines                        *int            `json:"diff_lines"`
	CommitAncientMinutes             *float64        `json:"commit_ancient_minutes"`
	CommitAncientMinutesReason       *string         `json:"commit_ancient_minutes_reason"`
	CommitAncientMinutesManual       *float64        `json:"commit_ancient_minutes_manual"`
	CommitAncientMinutesReasonManual *string         `json:"commit_ancient_minutes_reason_manual"`
	CommitRealMinutes                *float64        `json:"commit_real_minutes"`
	CommitRealMinutesReason          *string         `json:"commit_real_minutes_reason"`
	CommitRealMinutesManual          *float64        `json:"commit_real_minutes_manual"`
	CommitRealMinutesReasonManual    *string         `json:"commit_real_minutes_reason_manual"`
	CommitRealAIMinutes              *float64        `json:"commit_real_ai_minutes"`
	CommitRealAncientMinutes         *float64        `json:"commit_real_ancient_minutes"`
	TaskIDs                          json.RawMessage `json:"task_ids" swaggertype:"string" example:"[\"task1\"]"`
	TaskIDsSilica                    json.RawMessage `json:"task_ids_silica" swaggertype:"string" example:"[\"1.0\"]"`
	Comment                          *string         `json:"comment"`
	CreatedAt                        *time.Time      `json:"created_at"`
	UpdatedAt                        *time.Time      `json:"updated_at"`
	Cost                             float64         `json:"cost"`
	UpstreamTokens                   int64           `json:"upstream_tokens"`
	DownstreamTokens                 int64           `json:"downstream_tokens"`
	Silica                           *float64        `json:"silica"`
	EfficiencyRatio                  *float64        `json:"efficiency_ratio"`
	Org1                             string          `json:"org1"`
	Org2                             string          `json:"org2"`
	Org3                             string          `json:"org3"`
	Org4                             string          `json:"org4"`
	OrgDisplay                       string          `json:"org_display"`
}

type CommitListResponse struct {
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"pageSize"`
	Data     []CommitListItem `json:"data"`
}

type RelatedTask struct {
	TaskID          string     `json:"task_id"`
	UserName        *string    `json:"user_name"`
	StartTime       *time.Time `json:"start_time"`
	TaskRealMinutes *float64   `json:"task_real_minutes"`
	Silica          *float64   `json:"silica"`
	Cost            *float64   `json:"cost"`
	DiffLines       *int       `json:"diff_lines"`
}

type UpdateCommitManualRequest struct {
	CommitAncientMinutesManual       *float64 `json:"commit_ancient_minutes_manual"`
	CommitAncientMinutesReasonManual *string  `json:"commit_ancient_minutes_reason_manual"`
	CommitRealMinutesManual          *float64 `json:"commit_real_minutes_manual"`
	CommitRealMinutesReasonManual    *string  `json:"commit_real_minutes_reason_manual"`
}

type CommitDetailResponse struct {
	Commit           *StatCommit   `json:"commit"`
	RelatedTasks     []RelatedTask `json:"related_tasks"`
	EfficiencyRatio  *float64      `json:"efficiency_ratio"`
	TotalCost        float64       `json:"total_cost"`
	Silica           *float64      `json:"silica"`
	UpstreamTokens   int64         `json:"upstream_tokens"`
	DownstreamTokens int64         `json:"downstream_tokens"`
}
