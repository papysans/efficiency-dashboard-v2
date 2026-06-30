package models

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type UserOrg struct {
	UserId       string    `gorm:"primaryKey;type:varchar(255)" json:"user_id"`
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

type Session struct {
	SessionId        string    `gorm:"primaryKey;type:varchar(255)" json:"session_id"`
	CreateTime       time.Time `gorm:"type:timestamptz;index" json:"create_time"`
	UserId           string    `gorm:"type:varchar(255);index" json:"user_id"`
	UserName         string    `gorm:"type:varchar(255)" json:"user_name"`
	ClientId         string    `gorm:"column:client_id;type:varchar(255)" json:"client_id"`
	ClientIde        string    `gorm:"column:client_ide;type:varchar(100)" json:"client_ide"`
	ClientVersion    string    `gorm:"type:varchar(100)" json:"client_version"`
	ClientOs         string    `gorm:"column:client_os;type:varchar(100)" json:"client_os"`
	ClientOsVersion  string    `gorm:"column:client_os_version;type:varchar(100)" json:"client_os_version"`
	SessionDate      string    `gorm:"type:varchar(10)" json:"session_date"`
	ConversationDate string    `gorm:"type:varchar(10)" json:"conversation_date"`
	CreatedAt        time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Session) TableName() string { return "sessions" }

type Conversation struct {
	ID               int        `gorm:"primaryKey;autoIncrement" json:"id"`
	SessionId        string     `gorm:"type:varchar(255);not null;uniqueIndex:idx_session_request" json:"session_id"`
	RequestId        string     `gorm:"type:varchar(255);not null;uniqueIndex:idx_session_request" json:"request_id"`
	TaskId           string     `gorm:"type:varchar(255);index" json:"task_id"`
	Sender           string     `gorm:"type:varchar(50)" json:"sender"`
	PromptMode       string     `gorm:"type:varchar(50)" json:"prompt_mode"`
	Mode             string     `gorm:"type:varchar(100)" json:"mode"`
	Model            string     `gorm:"type:varchar(200)" json:"model"`
	StartTime        time.Time  `gorm:"type:timestamptz;index" json:"start_time"`
	EndTime          time.Time  `gorm:"type:timestamptz" json:"end_time"`
	ProcessTime      int64      `gorm:"type:bigint" json:"process_time"`
	ProcessTtft      int64      `gorm:"type:bigint" json:"process_ttft"`
	UpstreamTokens   int64      `gorm:"type:bigint" json:"upstream_tokens"`
	DownstreamTokens int64      `gorm:"type:bigint" json:"downstream_tokens"`
	Cost             float64    `gorm:"type:float8" json:"cost"`
	DiffLines        int64      `gorm:"type:bigint" json:"diff_lines"`
	RepoAddr         string     `gorm:"type:text" json:"repo_addr"`
	RepoBranch       string     `gorm:"type:varchar(500)" json:"repo_branch"`
	WorkDir          string     `gorm:"type:text" json:"work_dir"`
	WorkDirId        string     `gorm:"type:varchar(500);index" json:"work_dir_id"`
	UserInput        string     `gorm:"type:text" json:"user_input"`
	RequestContent   string     `gorm:"type:text" json:"request_content"`
	ResponseContent  string     `gorm:"type:text" json:"response_content"`
	ToolEvents       StringJSON `gorm:"type:jsonb;default:'[]'" json:"tool_events"`
	ErrorCode        string     `gorm:"type:varchar(100)" json:"error_code"`
	ErrorReason      string     `gorm:"type:text" json:"error_reason"`
	// 正文卸载(WS-A)：ContentLocation 非空表示 request/response_content/user_input 已卸载到磁盘/对象存储，
	// 三列在 DB 置空，按需经 storage.ReadFile 回灌。UserInputChars 预存 len(UserInput)，
	// 卸载后供 pseudo_task 古代估时(原读 len(c.UserInput))与 efficiency-v2 活跃判定用，避免回灌全文。
	ContentLocation string    `gorm:"type:text" json:"-"`          // 内部存储指针，json:"-" 不外发(防泄露 bucket/磁盘布局)
	UserInputChars  int       `gorm:"type:int;default:0" json:"-"` // 内部估时用，不外发
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (Conversation) TableName() string { return "conversations" }

type Commit struct {
	CommitId                   string     `gorm:"primaryKey;type:varchar(255)" json:"commit_id"`
	CommitTime                 time.Time  `gorm:"type:timestamptz;index" json:"commit_time"`
	RepoAddr                   string     `gorm:"type:text;index;index:idx_commits_repo_addr_branch" json:"repo_addr"`
	RepoBranch                 string     `gorm:"type:varchar(500);index:idx_commits_repo_addr_branch" json:"repo_branch"`
	GitUserName                string     `gorm:"type:varchar(255)" json:"git_user_name"`
	GitUserEmail               string     `gorm:"type:varchar(255)" json:"git_user_email"`
	UserId                     string     `gorm:"type:varchar(255);index" json:"user_id"`
	UserName                   string     `gorm:"type:varchar(255)" json:"user_name"`
	ClientId                   string     `gorm:"type:varchar(255)" json:"client_id"`
	WorkDir                    string     `gorm:"type:text" json:"work_dir"`
	WorkDirId                  string     `gorm:"type:varchar(500);index" json:"work_dir_id"`
	DiffLines                  int        `gorm:"type:int" json:"diff_lines"`
	TouchedFiles               StringJSON `gorm:"type:jsonb;default:'[]'" json:"touched_files"`
	CommitAncientMinutes       float64    `gorm:"type:float8" json:"commit_ancient_minutes"`
	CommitAncientReason        string     `gorm:"type:text" json:"commit_ancient_minutes_reason"`
	CommitAncientMinutesManual *float64   `gorm:"type:float8" json:"commit_ancient_minutes_manual"`
	CommitAncientReasonManual  string     `gorm:"type:text" json:"commit_ancient_minutes_reason_manual"`
	UpstreamTokens             int64      `gorm:"type:bigint" json:"upstream_tokens"`
	DownstreamTokens           int64      `gorm:"type:bigint" json:"downstream_tokens"`
	Cost                       float64    `gorm:"type:float8" json:"cost"`
	Silica                     float64    `gorm:"type:float8;default:0" json:"silica"`
	CommitRealAiMinutes        float64    `gorm:"type:float8" json:"commit_real_ai_minutes"`
	CommitRealNonAiMinutes     float64    `gorm:"type:float8" json:"commit_real_ancient_minutes"`
	CommitRealMinutes          float64    `gorm:"type:float8" json:"commit_real_minutes"`
	CommitRealReason           string     `gorm:"type:text" json:"commit_real_minutes_reason"`
	CommitRealMinutesManual    *float64   `gorm:"type:float8" json:"commit_real_minutes_manual"`
	CommitRealReasonManual     string     `gorm:"type:text" json:"commit_real_minutes_reason_manual"`
	Comment                    string     `gorm:"type:text" json:"comment"`
	// 治理列（commit governance pass 写入，排除=打标记不删数据）
	ExcludedFlag       bool      `gorm:"default:false;index" json:"excluded_flag"` // 是否被治理排除（聚合口径处过滤）
	ExcludedReason     string    `gorm:"type:text" json:"excluded_reason"`         // 排除原因（命中的规则说明）
	IsMerge            bool      `gorm:"default:false" json:"is_merge"`            // 是否 merge commit
	ReplayOf           string    `gorm:"type:varchar(255)" json:"replay_of"`       // 重放（rebase/cherry-pick）的原 commit_id
	EffectiveDiffLines *int64    `json:"effective_diff_lines"`                     // 治理后的有效行数（nil=未治理，回退 DiffLines）
	CreatedAt          time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt          time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Commit) TableName() string { return "commits" }

// GetEffectiveDiffLines 返回治理后的有效代码行数：被排除的 commit 记 0；
// 治理 pass 写入了 effective_diff_lines（软上限/降权/去重折算）则用之；否则回退原始 DiffLines。
// loc/估时聚合统一走本方法取行数。
func (c Commit) GetEffectiveDiffLines() int64 {
	if c.ExcludedFlag {
		return 0
	}
	if c.EffectiveDiffLines != nil {
		return *c.EffectiveDiffLines
	}
	return int64(c.DiffLines)
}

type Task struct {
	TaskId                   string    `gorm:"primaryKey;type:varchar(500)" json:"task_id"`
	CommitId                 string    `gorm:"type:varchar(255);index" json:"commit_id"`
	SessionId                string    `gorm:"type:varchar(255);index" json:"session_id"`
	UserId                   string    `gorm:"type:varchar(255);index" json:"user_id"`
	UserName                 string    `gorm:"type:varchar(255)" json:"user_name"`
	ClientId                 string    `gorm:"column:client_id;type:varchar(255)" json:"client_id"`
	ClientIde                string    `gorm:"column:client_ide;type:varchar(100)" json:"client_ide"`
	ClientVersion            string    `gorm:"type:varchar(100)" json:"client_version"`
	ClientOs                 string    `gorm:"column:client_os;type:varchar(100)" json:"client_os"`
	ClientOsVersion          string    `gorm:"column:client_os_version;type:varchar(100)" json:"client_os_version"`
	Caller                   string    `gorm:"type:varchar(100)" json:"caller"`
	RepoAddr                 string    `gorm:"type:text" json:"repo_addr"`
	RepoBranch               string    `gorm:"type:varchar(500)" json:"repo_branch"`
	WorkDir                  string    `gorm:"type:text" json:"work_dir"`
	WorkDirId                string    `gorm:"type:varchar(500);index" json:"work_dir_id"`
	StartTime                time.Time `gorm:"type:timestamptz;index" json:"start_time"`
	EndTime                  time.Time `gorm:"type:timestamptz" json:"end_time"`
	DiffLines                int       `gorm:"type:int" json:"diff_lines"`
	Silica                   float64   `gorm:"type:float8" json:"silica"`
	AcceptRatio              float64   `gorm:"type:float8" json:"accept_ratio"`
	UpstreamTokens           int64     `gorm:"type:bigint" json:"upstream_tokens"`
	DownstreamTokens         int64     `gorm:"type:bigint" json:"downstream_tokens"`
	Cost                     float64   `gorm:"type:float8" json:"cost"`
	TaskRealMinutes          float64   `gorm:"type:float8" json:"task_real_minutes"`
	TaskRealReason           string    `gorm:"type:text" json:"task_real_minutes_reason"`
	TaskRealMinutesManual    *float64  `gorm:"type:float8" json:"task_real_minutes_manual"`
	TaskRealReasonManual     string    `gorm:"type:text" json:"task_real_minutes_reason_manual"`
	TaskAncientMinutes       float64   `gorm:"type:float8" json:"task_ancient_minutes"`
	TaskAncientReason        string    `gorm:"type:text" json:"task_ancient_minutes_reason"`
	TaskAncientMinutesManual *float64  `gorm:"type:float8" json:"task_ancient_minutes_manual"`
	TaskAncientReasonManual  string    `gorm:"type:text" json:"task_ancient_minutes_reason_manual"`
	Title                    string    `gorm:"type:varchar(200)" json:"title"`
	SessionDate              string    `gorm:"type:varchar(10)" json:"session_date"`
	ConversationDate         string    `gorm:"type:varchar(10)" json:"conversation_date"`
	CreatedAt                time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt                time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Task) TableName() string { return "tasks" }

// UserProductivity 现为内存 DTO：V1 user_productivity 表已下线(不再 AutoMigrate)，
// backend ListUserProductivity 从 tasks/commits 基表实时聚合后填充此结构。表的物理删除需手动 DROP。
type UserProductivity struct {
	UserProductivityId     string    `gorm:"primaryKey;type:varchar(255)" json:"user_productivity_id"`
	CreateTime             time.Time `gorm:"type:timestamptz;index" json:"create_time"`
	UserId                 string    `gorm:"type:varchar(255);index" json:"user_id"`
	UserName               string    `gorm:"type:varchar(500)" json:"user_name"`
	UpstreamTokens         int64     `gorm:"type:bigint" json:"upstream_tokens"`
	DownstreamTokens       int64     `gorm:"type:bigint" json:"downstream_tokens"`
	Cost                   float64   `gorm:"type:float8" json:"cost"`
	TaskCount              int       `gorm:"type:int;default:0" json:"task_count"`
	TaskDiffLines          int       `gorm:"type:int" json:"task_diff_lines"`
	TaskRealMinutes        float64   `gorm:"type:float8" json:"task_real_minutes"`
	TaskAncientMinutes     float64   `gorm:"type:float8" json:"task_ancient_minutes"`
	TaskEfficiencyRatio    float64   `gorm:"type:float8" json:"task_efficiency_ratio"`
	CommitCount            int       `gorm:"type:int;default:0" json:"commit_count"`
	CommitDiffLines        int       `gorm:"type:int" json:"commit_diff_lines"`
	CommitAncientMinutes   float64   `gorm:"type:float8" json:"commit_ancient_minutes"`
	CommitRealAiMinutes    float64   `gorm:"type:float8" json:"commit_real_ai_minutes"`
	CommitRealNonAiMinutes float64   `gorm:"column:commit_real_ancient_minutes;type:float8" json:"commit_real_ancient_minutes"`
	CommitRealMinutes      float64   `gorm:"type:float8" json:"commit_real_minutes"`
	CommitEfficiencyRatio  float64   `gorm:"column:commit_efficiency_ratio;type:float8" json:"commit_efficiency_ratio"`
	CreatedAt              time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt              time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (UserProductivity) TableName() string { return "user_productivity" }

type Project struct {
	ProjectId                       string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"project_id"`
	Name                            string     `gorm:"type:varchar(500);not null;index" json:"name"`
	Description                     string     `gorm:"type:text" json:"description"`
	Repos                           StringJSON `gorm:"type:jsonb;default:'[]'" json:"repos"`
	TaskIds                         StringJSON `gorm:"type:jsonb;default:'[]'" json:"task_ids"`
	TaskIdsSilica                   StringJSON `gorm:"type:jsonb;default:'[]'" json:"task_ids_silica"`
	StartTime                       time.Time  `gorm:"type:timestamptz" json:"start_time"`
	EndTime                         time.Time  `gorm:"type:timestamptz" json:"end_time"`
	StartTimeManual                 *time.Time `gorm:"type:timestamptz" json:"start_time_manual"`
	EndTimeManual                   *time.Time `gorm:"type:timestamptz" json:"end_time_manual"`
	UpstreamTokens                  int64      `gorm:"type:bigint;default:0" json:"upstream_tokens"`
	DownstreamTokens                int64      `gorm:"type:bigint;default:0" json:"downstream_tokens"`
	Cost                            float64    `gorm:"type:float8;default:0" json:"cost"`
	ProjectAncientMinutes           float64    `gorm:"type:float8" json:"project_ancient_minutes"`
	ProjectAncientReason            string     `gorm:"type:text" json:"project_ancient_minutes_reason"`
	ProjectAncientMinutesManual     *float64   `gorm:"type:float8" json:"project_ancient_minutes_manual"`
	ProjectAncientReasonManual      string     `gorm:"type:text" json:"project_ancient_minutes_reason_manual"`
	ProjectRealProcessMinutes       float64    `gorm:"type:float8" json:"project_real_process_minutes"`
	ProjectRealProcessReason        string     `gorm:"type:text" json:"project_real_process_minutes_reason"`
	ProjectRealProcessMinutesManual *float64   `gorm:"type:float8" json:"project_real_process_minutes_manual"`
	ProjectRealProcessReasonManual  string     `gorm:"type:text" json:"project_real_process_minutes_reason_manual"`
	ProjectRealLeadMinutes          float64    `gorm:"type:float8" json:"project_real_lead_minutes"`
	ProjectRealLeadReason           string     `gorm:"type:text" json:"project_real_lead_minutes_reason"`
	ProjectRealLeadMinutesManual    *float64   `gorm:"type:float8" json:"project_real_lead_minutes_manual"`
	ProjectRealLeadReasonManual     string     `gorm:"type:text" json:"project_real_lead_minutes_reason_manual"`
	CreatedAt                       time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt                       time.Time  `gorm:"autoUpdateTime;index" json:"updated_at"`
}

func (Project) TableName() string { return "projects" }

type ProjectTask struct {
	ProjectId   string    `gorm:"primaryKey;type:uuid" json:"project_id"`
	TaskId      string    `gorm:"primaryKey;type:varchar(500)" json:"task_id"`
	Silica      float64   `gorm:"type:float8;default:1" json:"silica"`
	AcceptRatio float64   `gorm:"type:float8;default:0" json:"accept_ratio"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (ProjectTask) TableName() string { return "project_tasks" }

type ProjectRepo struct {
	ProjectId          string     `gorm:"primaryKey;type:uuid" json:"project_id"`
	RepoAddr           string     `gorm:"primaryKey;type:text" json:"repo_addr"`
	RepoBranch         string     `gorm:"primaryKey;type:varchar(500)" json:"repo_branch"`
	StartTime          *time.Time `gorm:"type:timestamptz" json:"start_time"`
	EndTime            *time.Time `gorm:"type:timestamptz" json:"end_time"`
	ExcludeCommits     StringJSON `gorm:"type:jsonb;default:'[]'" json:"exclude_commits"`
	IncludeOnlyCommits StringJSON `gorm:"type:jsonb;default:'[]'" json:"include_only_commits"`
	CreatedAt          time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt          time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (ProjectRepo) TableName() string { return "project_repos" }

type ProjectCommit struct {
	ProjectId  string    `gorm:"primaryKey;type:uuid" json:"project_id"`
	CommitId   string    `gorm:"primaryKey;type:varchar(255)" json:"commit_id"`
	RepoAddr   string    `gorm:"type:text" json:"repo_addr"`
	RepoBranch string    `gorm:"type:varchar(500)" json:"repo_branch"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (ProjectCommit) TableName() string { return "project_commits" }

type UserGroup struct {
	GroupID   string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"group_id"`
	Name      string     `gorm:"type:varchar(500);not null;index" json:"name"`
	OrgName   string     `gorm:"type:varchar(200);default:''" json:"org_name"`
	UserIDs   StringJSON `gorm:"type:jsonb;default:'[]'" json:"user_ids"`
	CreatedAt time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (UserGroup) TableName() string { return "user_groups" }

// Dept 部门树（来自 dept-sync 的 department，邻接表 parent_dept_id + 物化路径 dept_path）
type Dept struct {
	DeptId         string    `gorm:"primaryKey;type:varchar(64)" json:"dept_id"`
	DeptName       string    `gorm:"type:varchar(128)" json:"dept_name"`
	ParentDeptId   string    `gorm:"type:varchar(64);index" json:"parent_dept_id"`
	DeptPath       string    `gorm:"type:varchar(1024);index" json:"dept_path"`
	DeptLevel      int       `gorm:"type:int" json:"dept_level"`
	OrderNum       int       `gorm:"type:int;default:0" json:"order_num"`
	LeaderId       string    `gorm:"type:varchar(64)" json:"leader_id"` // 工号口径
	ChildDeptCount int       `gorm:"type:int;default:0" json:"child_dept_count"`
	Status         int       `gorm:"type:int;default:1" json:"status"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Dept) TableName() string { return "dept" }

// DeptUser 人员归属（来自 dept-sync 的 user_department）。
// 主键 EmpNo（工号）= dept-sync.user_department.user_id，是与看板 commits.git_user_email 前缀对接的 JOIN 锚点。
// UniversalId 仅留存，不参与 JOIN（实测与看板 user_id 0% 命中）。
type DeptUser struct {
	EmpNo       string    `gorm:"primaryKey;type:varchar(64)" json:"emp_no"`
	RealName    string    `gorm:"type:varchar(128);index" json:"real_name"`
	UniversalId string    `gorm:"type:varchar(64)" json:"universal_id"`
	DeptId      string    `gorm:"type:varchar(64);index" json:"dept_id"`
	Position    string    `gorm:"type:varchar(128)" json:"position"`
	IsMain      int       `gorm:"type:int;default:1" json:"is_main"`
	EntryTime   string    `gorm:"type:varchar(20)" json:"entry_time"`
	Status      int       `gorm:"type:int;default:1" json:"status"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (DeptUser) TableName() string { return "dept_user" }

func OpenGormDB(dsn string) (*gorm.DB, error) {
	db, err := openGormDBConn(dsn)
	if err != nil {
		return nil, err
	}
	if err := AutoMigrate(db); err != nil {
		return nil, fmt.Errorf("自动迁移数据库表结构失败: %w", err)
	}
	return db, nil
}

// OpenGormDBReadOnly 连库但**不跑 AutoMigrate**，供只读诊断命令（db-diag）用——不加列/不建索引/
// 不取 schema 锁，真正只读不改库。前提：schema 已由常驻 server/kbcli 启动期或迁移命令迁过。
func OpenGormDBReadOnly(dsn string) (*gorm.DB, error) {
	return openGormDBConn(dsn)
}

func openGormDBConn(dsn string) (*gorm.DB, error) {
	// gorm logger 设 Silent：默认 logger(Warn 级)会把「慢 SQL >=200ms」连同完整绑定值打出来——
	// 导入对话时 request/response_content 含粘贴图片的 base64,INSERT 又大又慢,会刷屏整条 SQL(含 base64)。
	// DB 错误仍由各调用方检查返回的 err 并打简短「导入失败: <err>」日志,信息不丢、噪声不进盘。
	db, err := gorm.Open(postgresOpener(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
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
	return db, nil
}

func AutoMigrate(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	lock, err := AcquireAdvisoryLock(context.Background(), sqlDB, AdvisoryLockAutoMigrate)
	if err != nil {
		return fmt.Errorf("获取 AutoMigrate 互斥锁失败: %w", err)
	}
	defer func() {
		_ = lock.Release(context.Background())
	}()

	if err := db.Exec(`CREATE EXTENSION IF NOT EXISTS pgcrypto`).Error; err != nil {
		return fmt.Errorf("启用 pgcrypto 扩展失败: %w", err)
	}

	if err := db.AutoMigrate(
		&UserOrg{},
		&Task{},
		&Session{},
		&Conversation{},
		&Commit{},
		&Project{},
		&ProjectTask{},
		&ProjectRepo{},
		&ProjectCommit{},
		&UserGroup{},
		&ConversationEvent{},
		&SessionStageMetric{},
		&Need{},
		&NeedEmpAttribution{},
		&UserProductivityV2{},
		&AnchorSet{},
		&BaselineCoefficient{},
		&BaselineFusionWeight{},
		&Dept{},
		&DeptUser{},
	); err != nil {
		return fmt.Errorf("AutoMigrate 失败: %w", err)
	}

	if err := migrateEfficiencyV2DDL(db); err != nil {
		return fmt.Errorf("迁移 v2 效率表结构失败: %w", err)
	}

	return nil
}
