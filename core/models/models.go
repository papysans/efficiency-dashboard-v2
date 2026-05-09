package models

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

type UserOrg struct {
	UserID       string    `gorm:"primaryKey;type:varchar(255)" json:"user_id"`
	UserName     string    `gorm:"type:varchar(255);index" json:"user_name"`
	Org1         string    `gorm:"type:varchar(255)" json:"org1"`
	Org2         string    `gorm:"type:varchar(255)" json:"org2"`
	Org3         string    `gorm:"type:varchar(255)" json:"org3"`
	Org4         string    `gorm:"type:varchar(255)" json:"org4"`
	Org5         string    `gorm:"type:varchar(255)" json:"org5"`
	Org6         string    `gorm:"type:varchar(255)" json:"org6"`
	Org7         string    `gorm:"type:varchar(255)" json:"org7"`
	Org8         string    `gorm:"type:varchar(255)" json:"org8"`
	Org9         string    `gorm:"type:varchar(255)" json:"org9"`
	GitUserName  string    `gorm:"type:varchar(255);index" json:"git_user_name"`
	GitUserEmail string    `gorm:"type:varchar(255);index" json:"git_user_email"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (UserOrg) TableName() string { return "user_org" }

type Commit struct {
	CommitID                         string     `gorm:"primaryKey;type:varchar(500)" json:"commit_id"`
	CommitTime                       *time.Time `gorm:"type:timestamptz;index" json:"commit_time"`
	RepoAddr                         string     `gorm:"type:text;index;index:idx_commits_repo_addr_branch" json:"repo_addr"`
	RepoBranch                       string     `gorm:"type:varchar(500);index:idx_commits_repo_addr_branch" json:"repo_branch"`
	GitUserName                      string     `gorm:"type:varchar(255)" json:"git_user_name"`
	GitUserEmail                     string     `gorm:"type:varchar(255)" json:"git_user_email"`
	UserID                           string     `gorm:"type:varchar(255);index" json:"user_id"`
	UserName                         string     `gorm:"type:varchar(255)" json:"user_name"`
	ClientID                         string     `gorm:"type:varchar(255)" json:"client_id"`
	WorkDir                          string     `gorm:"type:text" json:"work_dir"`
	DiffLines                        int        `gorm:"type:int" json:"diff_lines"`
	CommitAncientMinutes             *float64   `gorm:"type:float8" json:"commit_ancient_minutes"`
	CommitAncientMinutesReason       string     `gorm:"type:text" json:"commit_ancient_minutes_reason"`
	CommitAncientMinutesManual       *float64   `gorm:"type:float8" json:"commit_ancient_minutes_manual"`
	CommitAncientMinutesReasonManual string     `gorm:"type:text" json:"commit_ancient_minutes_reason_manual"`
	TaskIDs                          StringJSON `gorm:"type:jsonb" json:"task_ids"`
	TaskIDsSilica                    StringJSON `gorm:"type:jsonb" json:"task_ids_silica"`
	UpstreamTokens                   *int64     `gorm:"type:bigint" json:"upstream_tokens"`
	DownstreamTokens                 *int64     `gorm:"type:bigint" json:"downstream_tokens"`
	Cost                             *float64   `gorm:"type:float8" json:"cost"`
	Silica                           *float64   `gorm:"type:float8;default:0" json:"silica"`
	CommitRealAIMinutes              *float64   `gorm:"type:float8" json:"commit_real_ai_minutes"`
	CommitRealAncientMinutes         *float64   `gorm:"type:float8" json:"commit_real_ancient_minutes"`
	CommitRealMinutes                *float64   `gorm:"type:float8" json:"commit_real_minutes"`
	CommitRealMinutesReason          string     `gorm:"type:text" json:"commit_real_minutes_reason"`
	CommitRealMinutesManual          *float64   `gorm:"type:float8" json:"commit_real_minutes_manual"`
	CommitRealMinutesReasonManual    string     `gorm:"type:text" json:"commit_real_minutes_reason_manual"`
	Comment                          string     `gorm:"type:text" json:"comment"`
	CreatedAt                        time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt                        time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Commit) TableName() string { return "commits" }

type Task struct {
	TaskID                         string     `gorm:"primaryKey;type:varchar(500)" json:"task_id"`
	UserID                         string     `gorm:"type:varchar(255);index" json:"user_id"`
	UserName                       string     `gorm:"type:varchar(255)" json:"user_name"`
	ClientID                       string     `gorm:"column:client_id;type:varchar(255)" json:"client_id"`
	ClientIDE                      string     `gorm:"column:client_ide;type:varchar(100)" json:"client_ide"`
	ClientVersion                  string     `gorm:"type:varchar(100)" json:"client_version"`
	ClientOS                       string     `gorm:"column:client_os;type:varchar(100)" json:"client_os"`
	ClientOSVersion                string     `gorm:"column:client_os_version;type:varchar(100)" json:"client_os_version"`
	Caller                         string     `gorm:"type:varchar(100)" json:"caller"`
	RepoAddr                       string     `gorm:"type:text" json:"repo_addr"`
	RepoBranch                     string     `gorm:"type:varchar(500)" json:"repo_branch"`
	WorkDir                        string     `gorm:"type:text" json:"work_dir"`
	WorkDirID                      string     `gorm:"type:varchar(500);index" json:"work_dir_id"`
	DiffLines                      int        `gorm:"type:int" json:"diff_lines"`
	StartTime                      *time.Time `gorm:"type:timestamptz;index" json:"start_time"`
	EndTime                        *time.Time `gorm:"type:timestamptz" json:"end_time"`
	UpstreamTokens                 int64      `gorm:"type:bigint" json:"upstream_tokens"`
	DownstreamTokens               int64      `gorm:"type:bigint" json:"downstream_tokens"`
	Cost                           float64    `gorm:"type:float8" json:"cost"`
	TaskRealMinutes                float64    `gorm:"type:float8" json:"task_real_minutes"`
	TaskRealMinutesReason          string     `gorm:"type:text" json:"task_real_minutes_reason"`
	TaskRealMinutesManual          *float64   `gorm:"type:float8" json:"task_real_minutes_manual"`
	TaskRealMinutesReasonManual    string     `gorm:"type:text" json:"task_real_minutes_reason_manual"`
	TaskAncientMinutes             float64    `gorm:"type:float8" json:"task_ancient_minutes"`
	TaskAncientMinutesReason       string     `gorm:"type:text" json:"task_ancient_minutes_reason"`
	TaskAncientMinutesManual       *float64   `gorm:"type:float8" json:"task_ancient_minutes_manual"`
	TaskAncientMinutesReasonManual string     `gorm:"type:text" json:"task_ancient_minutes_reason_manual"`
	Title                          string     `gorm:"type:varchar(200)" json:"title"`
	CreatedAt                      time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt                      time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Task) TableName() string { return "tasks" }

type TaskConversation struct {
	ID               int        `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskID           string     `gorm:"type:varchar(500);not null;uniqueIndex:idx_task_request" json:"task_id"`
	RequestID        string     `gorm:"type:varchar(500);not null;uniqueIndex:idx_task_request" json:"request_id"`
	Sender           string     `gorm:"type:varchar(50)" json:"sender"`
	PromptMode       string     `gorm:"type:varchar(50)" json:"prompt_mode"`
	Mode             string     `gorm:"type:varchar(100)" json:"mode"`
	Model            string     `gorm:"type:varchar(200)" json:"model"`
	StartTime        *time.Time `gorm:"type:timestamptz;index" json:"start_time"`
	EndTime          *time.Time `gorm:"type:timestamptz" json:"end_time"`
	ProcessTime      int64      `gorm:"type:bigint" json:"process_time"`
	ProcessTTFT      int64      `gorm:"type:bigint" json:"process_ttft"`
	UpstreamTokens   int64      `gorm:"type:bigint" json:"upstream_tokens"`
	DownstreamTokens int64      `gorm:"type:bigint" json:"downstream_tokens"`
	Cost             float64    `gorm:"type:float8" json:"cost"`
	RequestContent   string     `gorm:"type:text" json:"request_content"`
	ResponseContent  string     `gorm:"type:text" json:"response_content"`
	UserInput        string     `gorm:"type:text" json:"user_input"`
	DiffLines        int64      `gorm:"type:bigint" json:"diff_lines"`
	ErrorCode        string     `gorm:"type:varchar(100)" json:"error_code"`
	ErrorReason      string     `gorm:"type:text" json:"error_reason"`
	CreatedAt        time.Time  `gorm:"autoCreateTime" json:"created_at"`
}

func (TaskConversation) TableName() string { return "task_conversations" }

type UserProductivity struct {
	UserProductivityID       string     `gorm:"primaryKey;type:varchar(500)" json:"user_productivity_id"`
	CreateTime               *time.Time `gorm:"type:timestamptz;index" json:"create_time"`
	UserID                   string     `gorm:"type:varchar(255);index" json:"user_id"`
	UserName                 string     `gorm:"type:varchar(500)" json:"user_name"`
	TaskIDs                  StringJSON `gorm:"type:jsonb" json:"task_ids"`
	WorkDirIDs               StringJSON `gorm:"type:jsonb" json:"work_dir_ids"`
	TaskDiffLines            int        `gorm:"type:int" json:"task_diff_lines"`
	UpstreamTokens           int64      `gorm:"type:bigint" json:"upstream_tokens"`
	DownstreamTokens         int64      `gorm:"type:bigint" json:"downstream_tokens"`
	Cost                     float64    `gorm:"type:float8" json:"cost"`
	TaskRealMinutes          float64    `gorm:"type:float8" json:"task_real_minutes"`
	TaskAncientMinutes       float64    `gorm:"type:float8" json:"task_ancient_minutes"`
	TaskEfficiencyRatio      float64    `gorm:"type:float8" json:"task_efficiency_ratio"`
	CommitIDs                StringJSON `gorm:"type:jsonb" json:"commit_ids"`
	CommitDiffLines          int        `gorm:"type:int" json:"commit_diff_lines"`
	CommitAncientMinutes     float64    `gorm:"type:float8" json:"commit_ancient_minutes"`
	CommitRealAIMinutes      float64    `gorm:"type:float8" json:"commit_real_ai_minutes"`
	CommitRealAncientMinutes float64    `gorm:"column:commit_real_ancient_minutes;type:float8" json:"commit_real_ancient_minutes"`
	CommitRealMinutes        float64    `gorm:"type:float8" json:"commit_real_minutes"`
	CommitEfficiencyRatio    float64    `gorm:"column:commit_efficiency_ratio;type:float8" json:"commit_efficiency_ratio"`
	CreatedAt                time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt                time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (UserProductivity) TableName() string { return "user_productivity" }

type Project struct {
	ProjectID                             string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"project_id"`
	Name                                  string     `gorm:"type:varchar(500);not null;index" json:"name"`
	Description                           string     `gorm:"type:text" json:"description"`
	Repos                                 StringJSON `gorm:"type:jsonb;default:'[]'" json:"repos"`
	TaskIDs                               StringJSON `gorm:"type:jsonb;default:'[]'" json:"task_ids"`
	TaskIDsSilica                         StringJSON `gorm:"type:jsonb;default:'[]'" json:"task_ids_silica"`
	StartTime                             *time.Time `gorm:"type:timestamptz" json:"start_time"`
	EndTime                               *time.Time `gorm:"type:timestamptz" json:"end_time"`
	StartTimeManual                       *time.Time `gorm:"type:timestamptz" json:"start_time_manual"`
	EndTimeManual                         *time.Time `gorm:"type:timestamptz" json:"end_time_manual"`
	UpstreamTokens                        int64      `gorm:"type:bigint;default:0" json:"upstream_tokens"`
	DownstreamTokens                      int64      `gorm:"type:bigint;default:0" json:"downstream_tokens"`
	Cost                                  float64    `gorm:"type:float8;default:0" json:"cost"`
	ProjectAncientMinutes                 *float64   `gorm:"type:float8" json:"project_ancient_minutes"`
	ProjectAncientMinutesReason           string     `gorm:"type:text" json:"project_ancient_minutes_reason"`
	ProjectAncientMinutesManual           *float64   `gorm:"type:float8" json:"project_ancient_minutes_manual"`
	ProjectAncientMinutesReasonManual     string     `gorm:"type:text" json:"project_ancient_minutes_reason_manual"`
	ProjectRealProcessMinutes             *float64   `gorm:"type:float8" json:"project_real_process_minutes"`
	ProjectRealProcessMinutesReason       string     `gorm:"type:text" json:"project_real_process_minutes_reason"`
	ProjectRealProcessMinutesManual       *float64   `gorm:"type:float8" json:"project_real_process_minutes_manual"`
	ProjectRealProcessMinutesReasonManual string     `gorm:"type:text" json:"project_real_process_minutes_reason_manual"`
	ProjectRealLeadMinutes                *float64   `gorm:"type:float8" json:"project_real_lead_minutes"`
	ProjectRealLeadMinutesReason          string     `gorm:"type:text" json:"project_real_lead_minutes_reason"`
	ProjectRealLeadMinutesManual          *float64   `gorm:"type:float8" json:"project_real_lead_minutes_manual"`
	ProjectRealLeadMinutesReasonManual    string     `gorm:"type:text" json:"project_real_lead_minutes_reason_manual"`
	CreatedAt                             time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt                             time.Time  `gorm:"autoUpdateTime;index" json:"updated_at"`
}

func (Project) TableName() string { return "projects" }

type UserGroup struct {
	GroupID   string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"group_id"`
	Name      string     `gorm:"type:varchar(500);not null;index" json:"name"`
	OrgName   string     `gorm:"type:varchar(200);default:''" json:"org_name"`
	UserIDs   StringJSON `gorm:"type:jsonb;default:'[]'" json:"user_ids"`
	CreatedAt time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (UserGroup) TableName() string { return "user_groups" }

func OpenGormDB(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgresOpener(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	if err := sqlDB.Ping(); err != nil {
		return nil, err
	}
	if err := AutoMigrate(db); err != nil {
		return nil, fmt.Errorf("自动迁移数据库表结构失败: %w", err)
	}
	return db, nil
}

func AutoMigrate(db *gorm.DB) error {
	if err := db.Exec(`CREATE EXTENSION IF NOT EXISTS pgcrypto`).Error; err != nil {
		return fmt.Errorf("启用 pgcrypto 扩展失败: %w", err)
	}

	if err := db.AutoMigrate(
		&UserOrg{},
		&Task{},
		&TaskConversation{},
		&Commit{},
		&Project{},
		&UserProductivity{},
		&UserGroup{},
	); err != nil {
		return fmt.Errorf("AutoMigrate 失败: %w", err)
	}

	// execDDLIgnoreError(db, `ALTER TABLE commits RENAME COLUMN work_path TO work_dir`)

	return nil
}
