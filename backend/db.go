package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"kanban/core/config"
	"kanban/core/models"
	"kanban/core/utils"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ============================================================
// 响应类型（保持与原有JSON结构一致）
// ============================================================

type StatTask struct {
	TaskID                         string     `json:"task_id"`
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
	DiffLines                      *int       `json:"diff_lines"`
	StartTime                      *time.Time `json:"start_time"`
	EndTime                        *time.Time `json:"end_time"`
	UpstreamTokens                 *int64     `json:"upstream_tokens"`
	DownstreamTokens               *int64     `json:"downstream_tokens"`
	Cost                           *float64   `json:"cost"`
	TaskRealMinutes                *float64   `json:"task_real_minutes"`
	TaskRealMinutesReason          *string    `json:"task_real_minutes_reason"`
	TaskRealMinutesManual          *float64   `json:"task_real_minutes_manual"`
	TaskRealMinutesReasonManual    *string    `json:"task_real_minutes_reason_manual"`
	TaskAncientMinutes             *float64   `json:"task_ancient_minutes"`
	TaskAncientMinutesReason       *string    `json:"task_ancient_minutes_reason"`
	TaskAncientMinutesManual       *float64   `json:"task_ancient_minutes_manual"`
	TaskAncientMinutesReasonManual *string    `json:"task_ancient_minutes_reason_manual"`
	Title                          *string    `json:"title"`
	CreatedAt                      *time.Time `json:"created_at"`
	UpdatedAt                      *time.Time `json:"updated_at"`
}

type StatTaskConversation struct {
	ID               int        `json:"id"`
	TaskID           string     `json:"task_id"`
	RequestID        string     `json:"request_id"`
	Sender           *string    `json:"sender"`
	PromptMode       *string    `json:"prompt_mode"`
	Mode             *string    `json:"mode"`
	Model            *string    `json:"model"`
	StartTime        *time.Time `json:"start_time"`
	EndTime          *time.Time `json:"end_time"`
	ProcessTime      *int64     `json:"process_time"`
	ProcessTTFT      *int64     `json:"process_ttft"`
	UpstreamTokens   *int64     `json:"upstream_tokens"`
	DownstreamTokens *int64     `json:"downstream_tokens"`
	Cost             *float64   `json:"cost"`
	RequestContent   *string    `json:"request_content"`
	ResponseContent  *string    `json:"response_content"`
	UserInput        *string    `json:"user_input"`
	Diff             *string    `json:"diff"`
	DiffLines        *int64     `json:"diff_lines"`
	ErrorCode        *string    `json:"error_code"`
	ErrorReason      *string    `json:"error_reason"`
	CreatedAt        *time.Time `json:"created_at"`
}

type StatCommit struct {
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
	TaskIDs                          json.RawMessage `json:"task_ids" swaggertype:"string" example:"[\"task1\", \"task2\"]"`
	TaskIDsSilica                    json.RawMessage `json:"task_ids_silica" swaggertype:"string" example:"[\"task1\", \"task2\"]"`
	UpstreamTokens                   *int64          `json:"upstream_tokens"`
	DownstreamTokens                 *int64          `json:"downstream_tokens"`
	Cost                             *float64        `json:"cost"`
	Silica                           *float64        `json:"silica"`
	CommitRealAIMinutes              *float64        `json:"commit_real_ai_minutes"`
	CommitRealAncientMinutes         *float64        `json:"commit_real_ancient_minutes"`
	CommitRealMinutes                *float64        `json:"commit_real_minutes"`
	CommitRealMinutesReason          *string         `json:"commit_real_minutes_reason"`
	CommitRealMinutesManual          *float64        `json:"commit_real_minutes_manual"`
	CommitRealMinutesReasonManual    *string         `json:"commit_real_minutes_reason_manual"`
	Comment                          *string         `json:"comment"`
	CreatedAt                        *time.Time      `json:"created_at"`
	UpdatedAt                        *time.Time      `json:"updated_at"`
}

type Project struct {
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
	UpstreamTokens                        int64           `json:"upstream_tokens"`
	DownstreamTokens                      int64           `json:"downstream_tokens"`
	Cost                                  float64         `json:"cost"`
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
}

type UserProductivity struct {
	UserProductivityID       string          `json:"user_productivity_id"`
	CreateTime               *time.Time      `json:"create_time"`
	UserID                   *string         `json:"user_id"`
	UserName                 *string         `json:"user_name"`
	TaskIDs                  json.RawMessage `json:"task_ids" swaggertype:"string"`
	WorkDirIDs               json.RawMessage `json:"work_dir_ids" swaggertype:"string"`
	TaskDiffLines            *int            `json:"task_diff_lines"`
	UpstreamTokens           *int64          `json:"upstream_tokens"`
	DownstreamTokens         *int64          `json:"downstream_tokens"`
	Cost                     *float64        `json:"cost"`
	TaskRealMinutes          *float64        `json:"task_real_minutes"`
	TaskAncientMinutes       *float64        `json:"task_ancient_minutes"`
	TaskEfficiencyRatio      *float64        `json:"task_efficiency_ratio"`
	CommitIDs                json.RawMessage `json:"commit_ids" swaggertype:"string"`
	CommitDiffLines          *int            `json:"commit_diff_lines"`
	CommitAncientMinutes     *float64        `json:"commit_ancient_minutes"`
	CommitRealAIMinutes      *float64        `json:"commit_real_ai_minutes"`
	CommitRealAncientMinutes *float64        `json:"commit_real_ancient_minutes"`
	CommitRealMinutes        *float64        `json:"commit_real_minutes"`
	CommitEfficiencyRatio    *float64        `json:"commit_efficiency_ratio"`
	CreatedAt                *time.Time      `json:"created_at"`
	UpdatedAt                *time.Time      `json:"updated_at"`
}

type UserGroup struct {
	GroupID   string          `json:"group_id"`
	Name      string          `json:"name"`
	OrgName   string          `json:"org_name"`
	UserIDs   json.RawMessage `json:"user_ids" swaggertype:"string" example:"[\"user1\", \"user2\"]"`
	CreatedAt *time.Time      `json:"created_at"`
	UpdatedAt *time.Time      `json:"updated_at"`
}

// ============================================================
// 聚合类型（非DB表，查询结果）
// ============================================================

type RepoAggregate struct {
	RepoAddr          *string
	RepoBranch        *string
	CommitCount       int
	StartTime         *time.Time
	EndTime           *time.Time
	SumAncientMinutes *float64
	SumRealMinutes    *float64
	TaskCount         int
	EfficiencyRatio   *float64
}

type ProjectAggregates struct {
	StartTime                       *time.Time
	EndTime                         *time.Time
	UpstreamTokens                  int64
	DownstreamTokens                int64
	Cost                            float64
	ProjectAncientMinutes           *float64
	ProjectAncientMinutesReason     string
	ProjectRealProcessMinutes       *float64
	ProjectRealProcessMinutesReason string
	ProjectRealLeadMinutes          *float64
	ProjectRealLeadMinutesReason    string
}

type CommitLightStats struct {
	UserName  *string
	DiffLines *int
}

// ============================================================
// 值→指针转换辅助（零值映射为 nil）
// ============================================================

func toStrPtr(s string) *string {
	if s == "" {
		return nil
	}
	s2 := s
	return &s2
}
func toIntPtr(i int) *int {
	if i == 0 {
		return nil
	}
	v := i
	return &v
}
func toInt64Ptr(i int64) *int64 {
	if i == 0 {
		return nil
	}
	v := i
	return &v
}
func toFloat64Ptr(f float64) *float64 {
	if f == 0 {
		return nil
	}
	v := f
	return &v
}
func toTimePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	v := t
	return &v
}

func strJSONToRaw(j models.StringJSON) json.RawMessage {
	if j == "" || j == "null" {
		return json.RawMessage("[]")
	}
	return json.RawMessage(j)
}

func rawToStrJSON(r json.RawMessage) models.StringJSON {
	if len(r) == 0 || string(r) == "null" {
		return "[]"
	}
	return models.StringJSON(r)
}

// ============================================================
// models → response 类型转换
// ============================================================

func toStatTask(t *models.Task) *StatTask {
	if t == nil {
		return nil
	}
	return &StatTask{
		TaskID:                         t.TaskID,
		UserID:                         toStrPtr(t.UserID),
		UserName:                       toStrPtr(t.UserName),
		ClientID:                       toStrPtr(t.ClientID),
		ClientIDE:                      toStrPtr(t.ClientIDE),
		ClientVersion:                  toStrPtr(t.ClientVersion),
		ClientOS:                       toStrPtr(t.ClientOS),
		ClientOSVersion:                toStrPtr(t.ClientOSVer),
		Caller:                         toStrPtr(t.Caller),
		RepoAddr:                       toStrPtr(t.RepoAddr),
		RepoBranch:                     toStrPtr(t.RepoBranch),
		WorkDir:                        toStrPtr(t.WorkDir),
		WorkDirID:                      toStrPtr(t.WorkDirID),
		DiffLines:                      toIntPtr(t.DiffLines),
		StartTime:                      t.StartTime,
		EndTime:                        t.EndTime,
		UpstreamTokens:                 toInt64Ptr(t.UpstreamTokens),
		DownstreamTokens:               toInt64Ptr(t.DownstreamTokens),
		Cost:                           toFloat64Ptr(t.Cost),
		TaskRealMinutes:                toFloat64Ptr(t.TaskRealMinutes),
		TaskRealMinutesReason:          toStrPtr(t.TaskRealMinutesReason),
		TaskRealMinutesManual:          t.TaskRealMinutesManual,
		TaskRealMinutesReasonManual:    t.TaskRealMinutesReasonManual,
		TaskAncientMinutes:             toFloat64Ptr(t.TaskAncientMinutes),
		TaskAncientMinutesReason:       toStrPtr(t.TaskAncientMinutesReason),
		TaskAncientMinutesManual:       t.TaskAncientMinutesManual,
		TaskAncientMinutesReasonManual: t.TaskAncientMinutesReasonManual,
		Title:                          toStrPtr(t.Title),
		CreatedAt:                      toTimePtr(t.CreatedAt),
		UpdatedAt:                      toTimePtr(t.UpdatedAt),
	}
}

func toStatTaskSlice(tasks []models.Task) []StatTask {
	result := make([]StatTask, len(tasks))
	for i, t := range tasks {
		result[i] = *toStatTask(&t)
	}
	return result
}

func toStatCommit(c *models.Commit) *StatCommit {
	if c == nil {
		return nil
	}
	return &StatCommit{
		CommitID:                         c.CommitID,
		CommitTime:                       c.CommitTime,
		RepoAddr:                         toStrPtr(c.RepoAddr),
		RepoBranch:                       toStrPtr(c.RepoBranch),
		GitUserName:                      toStrPtr(c.GitUserName),
		GitUserEmail:                     toStrPtr(c.GitUserEmail),
		UserID:                           toStrPtr(c.UserID),
		UserName:                         toStrPtr(c.UserName),
		ClientID:                         toStrPtr(c.ClientID),
		WorkDir:                          toStrPtr(c.WorkDir),
		DiffLines:                        toIntPtr(c.DiffLines),
		CommitAncientMinutes:             c.CommitAncientMinutes,
		CommitAncientMinutesReason:       c.CommitAncientMinutesReason,
		CommitAncientMinutesManual:       c.CommitAncientMinutesManual,
		CommitAncientMinutesReasonManual: c.CommitAncientMinutesReasonManual,
		TaskIDs:                          strJSONToRaw(c.TaskIDs),
		TaskIDsSilica:                    strJSONToRaw(c.TaskIDsSilica),
		UpstreamTokens:                   c.UpstreamTokens,
		DownstreamTokens:                 c.DownstreamTokens,
		Cost:                             c.Cost,
		Silica:                           c.Silica,
		CommitRealAIMinutes:              c.CommitRealAIMinutes,
		CommitRealAncientMinutes:         c.CommitRealAncientMinutes,
		CommitRealMinutes:                c.CommitRealMinutes,
		CommitRealMinutesReason:          c.CommitRealMinutesReason,
		CommitRealMinutesManual:          c.CommitRealMinutesManual,
		CommitRealMinutesReasonManual:    c.CommitRealMinutesReasonManual,
		Comment:                          toStrPtr(c.Comment),
		CreatedAt:                        toTimePtr(c.CreatedAt),
		UpdatedAt:                        toTimePtr(c.UpdatedAt),
	}
}

func toStatCommitSlice(commits []models.Commit) []StatCommit {
	result := make([]StatCommit, len(commits))
	for i, c := range commits {
		result[i] = *toStatCommit(&c)
	}
	return result
}

func toStatTaskConversation(c *models.TaskConversation) *StatTaskConversation {
	if c == nil {
		return nil
	}
	return &StatTaskConversation{
		ID:               c.ID,
		TaskID:           c.TaskID,
		RequestID:        c.RequestID,
		Sender:           toStrPtr(c.Sender),
		PromptMode:       toStrPtr(c.PromptMode),
		Mode:             toStrPtr(c.Mode),
		Model:            toStrPtr(c.Model),
		StartTime:        c.StartTime,
		EndTime:          c.EndTime,
		ProcessTime:      toInt64Ptr(c.ProcessTime),
		ProcessTTFT:      toInt64Ptr(c.ProcessTTFT),
		UpstreamTokens:   toInt64Ptr(c.UpstreamTokens),
		DownstreamTokens: toInt64Ptr(c.DownstreamTokens),
		Cost:             toFloat64Ptr(c.Cost),
		RequestContent:   toStrPtr(c.RequestContent),
		ResponseContent:  toStrPtr(c.ResponseContent),
		UserInput:        toStrPtr(c.UserInput),
		Diff:             nil,
		DiffLines:        toInt64Ptr(c.DiffLines),
		ErrorCode:        toStrPtr(c.ErrorCode),
		ErrorReason:      toStrPtr(c.ErrorReason),
		CreatedAt:        toTimePtr(c.CreatedAt),
	}
}

func toProject(p *models.Project) *Project {
	if p == nil {
		return nil
	}
	return &Project{
		ProjectID:                             p.ProjectID,
		Name:                                  p.Name,
		Description:                           p.Description,
		Repos:                                 strJSONToRaw(p.Repos),
		TaskIDs:                               strJSONToRaw(p.TaskIDs),
		TaskIDsSilica:                         strJSONToRaw(p.TaskIDsSilica),
		StartTime:                             p.StartTime,
		EndTime:                               p.EndTime,
		StartTimeManual:                       p.StartTimeManual,
		EndTimeManual:                         p.EndTimeManual,
		UpstreamTokens:                        p.UpstreamTokens,
		DownstreamTokens:                      p.DownstreamTokens,
		Cost:                                  p.Cost,
		ProjectAncientMinutes:                 p.ProjectAncientMinutes,
		ProjectAncientMinutesReason:           p.ProjectAncientMinutesReason,
		ProjectAncientMinutesManual:           p.ProjectAncientMinutesManual,
		ProjectAncientMinutesReasonManual:     p.ProjectAncientMinutesReasonManual,
		ProjectRealProcessMinutes:             p.ProjectRealProcessMinutes,
		ProjectRealProcessMinutesReason:       p.ProjectRealProcessMinutesReason,
		ProjectRealProcessMinutesManual:       p.ProjectRealProcessMinutesManual,
		ProjectRealProcessMinutesReasonManual: p.ProjectRealProcessMinutesReasonManual,
		ProjectRealLeadMinutes:                p.ProjectRealLeadMinutes,
		ProjectRealLeadMinutesReason:          p.ProjectRealLeadMinutesReason,
		ProjectRealLeadMinutesManual:          p.ProjectRealLeadMinutesManual,
		ProjectRealLeadMinutesReasonManual:    p.ProjectRealLeadMinutesReasonManual,
		CreatedAt:                             toTimePtr(p.CreatedAt),
		UpdatedAt:                             toTimePtr(p.UpdatedAt),
	}
}

func toProjectSlice(projects []models.Project) []Project {
	result := make([]Project, len(projects))
	for i, p := range projects {
		result[i] = *toProject(&p)
	}
	return result
}

func toUserProductivity(up *models.UserProductivity) *UserProductivity {
	if up == nil {
		return nil
	}
	return &UserProductivity{
		UserProductivityID:       up.UserProductivityID,
		CreateTime:               up.CreateTime,
		UserID:                   toStrPtr(up.UserID),
		UserName:                 toStrPtr(up.UserName),
		TaskIDs:                  strJSONToRaw(up.TaskIDs),
		WorkDirIDs:               strJSONToRaw(up.WorkDirIDs),
		TaskDiffLines:            toIntPtr(up.TaskDiffLines),
		UpstreamTokens:           toInt64Ptr(up.UpstreamTokens),
		DownstreamTokens:         toInt64Ptr(up.DownstreamTokens),
		Cost:                     toFloat64Ptr(up.Cost),
		TaskRealMinutes:          toFloat64Ptr(up.TaskRealMinutes),
		TaskAncientMinutes:       toFloat64Ptr(up.TaskAncientMinutes),
		TaskEfficiencyRatio:      toFloat64Ptr(up.TaskEfficiencyRatio),
		CommitIDs:                strJSONToRaw(up.CommitIDs),
		CommitDiffLines:          toIntPtr(up.CommitDiffLines),
		CommitAncientMinutes:     toFloat64Ptr(up.CommitAncientMinutes),
		CommitRealAIMinutes:      toFloat64Ptr(up.CommitRealAIMinutes),
		CommitRealAncientMinutes: toFloat64Ptr(up.CommitRealAncientMinutes),
		CommitRealMinutes:        toFloat64Ptr(up.CommitRealMinutes),
		CommitEfficiencyRatio:    toFloat64Ptr(up.CommitEfficiencyRatio),
		CreatedAt:                toTimePtr(up.CreatedAt),
		UpdatedAt:                toTimePtr(up.UpdatedAt),
	}
}

func toUserProductivitySlice(ups []models.UserProductivity) []UserProductivity {
	result := make([]UserProductivity, len(ups))
	for i, up := range ups {
		result[i] = *toUserProductivity(&up)
	}
	return result
}

func toUserGroup(g *models.UserGroup) *UserGroup {
	if g == nil {
		return nil
	}
	return &UserGroup{
		GroupID:   g.GroupID,
		Name:      g.Name,
		OrgName:   g.OrgName,
		UserIDs:   strJSONToRaw(g.UserIDs),
		CreatedAt: toTimePtr(g.CreatedAt),
		UpdatedAt: toTimePtr(g.UpdatedAt),
	}
}

// ============================================================
// 数据库初始化
// ============================================================

func InitDB(cfg config.DatabaseConfig) (*gorm.DB, error) {
	return models.OpenGormDB(cfg.DSN())
}

// ============================================================
// tasks CRUD (GORM)
// ============================================================

func GetStatTask(db *gorm.DB, taskID string) (*StatTask, error) {
	var t models.Task
	err := db.Where("task_id = ?", taskID).First(&t).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询 tasks 失败: %w", err)
	}
	return toStatTask(&t), nil
}

func BatchGetStatTasks(db *gorm.DB, taskIDs []string) (map[string]*StatTask, error) {
	result := make(map[string]*StatTask)
	if len(taskIDs) == 0 {
		return result, nil
	}
	var tasks []models.Task
	if err := db.Where("task_id IN ?", taskIDs).Find(&tasks).Error; err != nil {
		return nil, fmt.Errorf("批量查询 tasks 失败: %w", err)
	}
	for i := range tasks {
		result[tasks[i].TaskID] = toStatTask(&tasks[i])
	}
	return result, nil
}

func ListStatTasks(db *gorm.DB, userID, workDirID, startTime, endTime string, page, pageSize int) ([]StatTask, error) {
	q := db.Model(&models.Task{})
	if userID != "" {
		q = q.Where("user_id = ?", userID)
	}
	if workDirID != "" {
		q = q.Where("work_dir_id = ?", workDirID)
	}
	if startTime != "" {
		q = q.Where("start_time >= ?", startTime)
	}
	if endTime != "" {
		q = q.Where("start_time <= ?", endTime)
	}
	var tasks []models.Task
	if err := q.Order("start_time DESC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&tasks).Error; err != nil {
		return nil, fmt.Errorf("查询 tasks 列表失败: %w", err)
	}
	result := toStatTaskSlice(tasks)
	return result, nil
}

func CountStatTasks(db *gorm.DB, userID, workDirID, startTime, endTime string) (int, error) {
	q := db.Model(&models.Task{})
	if userID != "" {
		q = q.Where("user_id = ?", userID)
	}
	if workDirID != "" {
		q = q.Where("work_dir_id = ?", workDirID)
	}
	if startTime != "" {
		q = q.Where("start_time >= ?", startTime)
	}
	if endTime != "" {
		q = q.Where("start_time <= ?", endTime)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("统计 tasks 总数失败: %w", err)
	}
	return int(count), nil
}

func UpdateStatTaskManual(db *gorm.DB, taskID string, realManual *float64, realReasonManual *string, ancientManual *float64, ancientReasonManual *string) error {
	updates := map[string]interface{}{
		"task_real_minutes_manual":           realManual,
		"task_real_minutes_reason_manual":    realReasonManual,
		"task_ancient_minutes_manual":        ancientManual,
		"task_ancient_minutes_reason_manual": ancientReasonManual,
		"updated_at":                         time.Now(),
	}
	result := db.Model(&models.Task{}).Where("task_id = ?", taskID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("更新 tasks manual 字段失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("tasks task_id=%s 不存在", taskID)
	}
	return nil
}

// ============================================================
// task_conversations CRUD (GORM)
// ============================================================

func ListStatTaskConversations(db *gorm.DB, taskID string) ([]StatTaskConversation, error) {
	var convs []models.TaskConversation
	if err := db.Where("task_id = ?", taskID).Order("start_time ASC").Find(&convs).Error; err != nil {
		return nil, fmt.Errorf("查询 task_conversations 列表失败: %w", err)
	}
	result := make([]StatTaskConversation, len(convs))
	for i := range convs {
		result[i] = *toStatTaskConversation(&convs[i])
	}
	return result, nil
}

// ============================================================
// commits CRUD (GORM)
// ============================================================

func GetStatCommitByID(db *gorm.DB, commitID string) (*StatCommit, error) {
	var c models.Commit
	err := db.Where("commit_id = ?", commitID).First(&c).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询 commits 失败: %w", err)
	}
	return toStatCommit(&c), nil
}

func ListStatCommits(db *gorm.DB, repoAddr, repoBranch, userID, startTime, endTime string, page, pageSize int, orgUserIDs []string) ([]StatCommit, error) {
	q := db.Model(&models.Commit{})
	if repoAddr != "" {
		q = q.Where("repo_addr = ?", repoAddr)
	}
	if repoBranch != "" {
		q = q.Where("repo_branch = ?", repoBranch)
	}
	if userID != "" {
		q = q.Where("user_id = ?", userID)
	}
	if len(orgUserIDs) > 0 {
		q = q.Where("user_id IN ?", orgUserIDs)
	}
	if startTime != "" {
		q = q.Where("commit_time >= ?", startTime)
	}
	if endTime != "" {
		q = q.Where("commit_time <= ?", endTime)
	}
	var commits []models.Commit
	if err := q.Order("commit_time DESC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&commits).Error; err != nil {
		return nil, fmt.Errorf("查询 commits 列表失败: %w", err)
	}
	result := toStatCommitSlice(commits)
	return result, nil
}

func CountStatCommits(db *gorm.DB, repoAddr, repoBranch, userID, startTime, endTime string, orgUserIDs []string) (int, error) {
	q := db.Model(&models.Commit{})
	if repoAddr != "" {
		q = q.Where("repo_addr = ?", repoAddr)
	}
	if repoBranch != "" {
		q = q.Where("repo_branch = ?", repoBranch)
	}
	if userID != "" {
		q = q.Where("user_id = ?", userID)
	}
	if len(orgUserIDs) > 0 {
		q = q.Where("user_id IN ?", orgUserIDs)
	}
	if startTime != "" {
		q = q.Where("commit_time >= ?", startTime)
	}
	if endTime != "" {
		q = q.Where("commit_time <= ?", endTime)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("统计 commits 总数失败: %w", err)
	}
	return int(count), nil
}

func UpdateStatCommitManual(db *gorm.DB, commitID string, ancientManual *float64, ancientReasonManual *string, realManual *float64, realReasonManual *string) error {
	updates := map[string]interface{}{
		"commit_ancient_minutes_manual":        ancientManual,
		"commit_ancient_minutes_reason_manual": ancientReasonManual,
		"commit_real_minutes_manual":           realManual,
		"commit_real_minutes_reason_manual":    realReasonManual,
		"updated_at":                           time.Now(),
	}
	result := db.Model(&models.Commit{}).Where("commit_id = ?", commitID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("更新 commits manual 字段失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("commits commit_id=%s 不存在", commitID)
	}
	return nil
}

func UpdateStatCommitTaskAssoc(db *gorm.DB, commitID string, taskIDs, taskIDsSilica json.RawMessage, realMinutes *float64, realAIMinutes *float64, realAncientMinutes *float64, realReason *string) error {
	updates := map[string]interface{}{
		"task_ids":                    rawToStrJSON(taskIDs),
		"task_ids_silica":             rawToStrJSON(taskIDsSilica),
		"commit_real_minutes":         realMinutes,
		"commit_real_ai_minutes":      realAIMinutes,
		"commit_real_ancient_minutes": realAncientMinutes,
		"commit_real_minutes_reason":  realReason,
		"updated_at":                  time.Now(),
	}
	result := db.Model(&models.Commit{}).Where("commit_id = ?", commitID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("更新 commits task 关联信息失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("commits commit_id=%s 不存在", commitID)
	}
	return nil
}

func ListRepoAggregates(db *gorm.DB, startTime, endTime string) ([]RepoAggregate, error) {
	type rawRow struct {
		RepoAddr          *string
		RepoBranch        *string
		CommitCount       int
		StartTime         *time.Time
		EndTime           *time.Time
		SumAncientMinutes *float64
		SumRealMinutes    *float64
		TaskCount         int
	}
	var rows []rawRow

	q := db.Model(&models.Commit{}).
		Select(`repo_addr, repo_branch,
			COUNT(*) AS commit_count,
			MIN(commit_time) AS start_time,
			MAX(commit_time) AS end_time,
			SUM(commit_ancient_minutes) AS sum_ancient_minutes,
			SUM(commit_real_minutes) AS sum_real_minutes,
			SUM(CASE WHEN task_ids IS NOT NULL AND task_ids::text NOT IN ('null', '[]') THEN jsonb_array_length(task_ids) ELSE 0 END) AS task_count`).
		Where("repo_addr IS NOT NULL AND repo_addr != ''")

	if startTime != "" {
		q = q.Where("commit_time >= ?", startTime)
	}
	if endTime != "" {
		q = q.Where("commit_time <= ?", endTime)
	}

	if err := q.Group("repo_addr, repo_branch").Order("repo_addr, repo_branch").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("查询 commits 聚合失败: %w", err)
	}

	list := make([]RepoAggregate, len(rows))
	for i, row := range rows {
		item := RepoAggregate{
			RepoAddr:          row.RepoAddr,
			RepoBranch:        row.RepoBranch,
			CommitCount:       row.CommitCount,
			StartTime:         row.StartTime,
			EndTime:           row.EndTime,
			SumAncientMinutes: row.SumAncientMinutes,
			SumRealMinutes:    row.SumRealMinutes,
			TaskCount:         row.TaskCount,
		}
		if item.SumAncientMinutes != nil && item.SumRealMinutes != nil && *item.SumRealMinutes > 0 {
			ratio := utils.CalcEfficiencyRatio(*item.SumAncientMinutes, *item.SumRealMinutes)
			item.EfficiencyRatio = &ratio
		}
		list[i] = item
	}
	return list, nil
}

func ListBranchesByRepoAddr(db *gorm.DB, repoAddr string) ([]string, error) {
	var branches []string
	if err := db.Model(&models.Commit{}).
		Where("repo_addr = ? AND repo_branch IS NOT NULL AND repo_branch != ''", repoAddr).
		Distinct("repo_branch").
		Order("repo_branch").
		Pluck("repo_branch", &branches).Error; err != nil {
		return nil, fmt.Errorf("查询 commits 分支列表失败: %w", err)
	}
	return branches, nil
}

// ============================================================
// projects CRUD (GORM)
// ============================================================

func CreateProject(db *gorm.DB, p *Project) (string, error) {
	mp := models.Project{
		Name:          p.Name,
		Description:   p.Description,
		Repos:         rawToStrJSON(p.Repos),
		TaskIDs:       rawToStrJSON(p.TaskIDs),
		TaskIDsSilica: rawToStrJSON(p.TaskIDsSilica),
	}
	if err := db.Create(&mp).Error; err != nil {
		return "", fmt.Errorf("创建 project 失败: %w", err)
	}
	return mp.ProjectID, nil
}

func GetProject(db *gorm.DB, projectID string) (*Project, error) {
	var mp models.Project
	err := db.Where("project_id = ?", projectID).First(&mp).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询 project 失败: %w", err)
	}
	return toProject(&mp), nil
}

func ListProjects(db *gorm.DB) ([]Project, error) {
	var mps []models.Project
	if err := db.Order("updated_at DESC").Find(&mps).Error; err != nil {
		return nil, fmt.Errorf("查询 projects 列表失败: %w", err)
	}
	return toProjectSlice(mps), nil
}

func UpdateProject(db *gorm.DB, p *Project) error {
	updates := map[string]interface{}{
		"name":            p.Name,
		"description":     p.Description,
		"repos":           rawToStrJSON(p.Repos),
		"task_ids":        rawToStrJSON(p.TaskIDs),
		"task_ids_silica": rawToStrJSON(p.TaskIDsSilica),
		"updated_at":      time.Now(),
	}
	result := db.Model(&models.Project{}).Where("project_id = ?", p.ProjectID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("更新 project 失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("project project_id=%s 不存在", p.ProjectID)
	}
	return nil
}

func DeleteProject(db *gorm.DB, projectID string) error {
	result := db.Where("project_id = ?", projectID).Delete(&models.Project{})
	if result.Error != nil {
		return fmt.Errorf("删除 project 失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("project project_id=%s 不存在", projectID)
	}
	return nil
}

func UpdateProjectManual(db *gorm.DB, projectID string, req UpdateProjectManualRequest) error {
	updates := map[string]interface{}{}
	if req.ProjectAncientMinutesManual != nil {
		updates["project_ancient_minutes_manual"] = *req.ProjectAncientMinutesManual
	}
	if req.ProjectAncientMinutesReasonManual != nil {
		updates["project_ancient_minutes_reason_manual"] = *req.ProjectAncientMinutesReasonManual
	}
	if req.ProjectRealProcessMinutesManual != nil {
		updates["project_real_process_minutes_manual"] = *req.ProjectRealProcessMinutesManual
	}
	if req.ProjectRealProcessMinutesReasonManual != nil {
		updates["project_real_process_minutes_reason_manual"] = *req.ProjectRealProcessMinutesReasonManual
	}
	if req.ProjectRealLeadMinutesManual != nil {
		updates["project_real_lead_minutes_manual"] = *req.ProjectRealLeadMinutesManual
	}
	if req.ProjectRealLeadMinutesReasonManual != nil {
		updates["project_real_lead_minutes_reason_manual"] = *req.ProjectRealLeadMinutesReasonManual
	}
	if req.StartTimeManual != nil {
		updates["start_time_manual"] = *req.StartTimeManual
	}
	if req.EndTimeManual != nil {
		updates["end_time_manual"] = *req.EndTimeManual
	}

	if len(updates) == 0 {
		return fmt.Errorf("没有需要更新的字段")
	}
	updates["updated_at"] = time.Now()

	result := db.Model(&models.Project{}).Where("project_id = ?", projectID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("更新 project manual 字段失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("project project_id=%s 不存在", projectID)
	}
	return nil
}

func UpdateProjectAggregates(db *gorm.DB, projectID string, agg *ProjectAggregates) error {
	updates := map[string]interface{}{
		"start_time":                          agg.StartTime,
		"end_time":                            agg.EndTime,
		"upstream_tokens":                     agg.UpstreamTokens,
		"downstream_tokens":                   agg.DownstreamTokens,
		"cost":                                agg.Cost,
		"project_ancient_minutes":             agg.ProjectAncientMinutes,
		"project_ancient_minutes_reason":      agg.ProjectAncientMinutesReason,
		"project_real_process_minutes":        agg.ProjectRealProcessMinutes,
		"project_real_process_minutes_reason": agg.ProjectRealProcessMinutesReason,
		"project_real_lead_minutes":           agg.ProjectRealLeadMinutes,
		"project_real_lead_minutes_reason":    agg.ProjectRealLeadMinutesReason,
		"updated_at":                          time.Now(),
	}
	result := db.Model(&models.Project{}).Where("project_id = ?", projectID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("更新 project 聚合数据失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("project project_id=%s 不存在", projectID)
	}
	return nil
}

// ============================================================
// user_productivity CRUD (GORM)
// ============================================================

func UpsertUserProductivity(db *gorm.DB, up *UserProductivity) error {
	mup := models.UserProductivity{
		UserProductivityID:       up.UserProductivityID,
		CreateTime:               up.CreateTime,
		UserID:                   strPtrToVal(up.UserID),
		UserName:                 strPtrToVal(up.UserName),
		TaskIDs:                  rawToStrJSON(up.TaskIDs),
		WorkDirIDs:               rawToStrJSON(up.WorkDirIDs),
		TaskDiffLines:            intPtrToVal(up.TaskDiffLines),
		UpstreamTokens:           int64PtrToVal(up.UpstreamTokens),
		DownstreamTokens:         int64PtrToVal(up.DownstreamTokens),
		Cost:                     float64PtrToVal(up.Cost),
		TaskRealMinutes:          float64PtrToVal(up.TaskRealMinutes),
		TaskAncientMinutes:       float64PtrToVal(up.TaskAncientMinutes),
		TaskEfficiencyRatio:      float64PtrToVal(up.TaskEfficiencyRatio),
		CommitIDs:                rawToStrJSON(up.CommitIDs),
		CommitDiffLines:          intPtrToVal(up.CommitDiffLines),
		CommitAncientMinutes:     float64PtrToVal(up.CommitAncientMinutes),
		CommitRealAIMinutes:      float64PtrToVal(up.CommitRealAIMinutes),
		CommitRealAncientMinutes: float64PtrToVal(up.CommitRealAncientMinutes),
		CommitRealMinutes:        float64PtrToVal(up.CommitRealMinutes),
		CommitEfficiencyRatio:    float64PtrToVal(up.CommitEfficiencyRatio),
	}
	err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_productivity_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"create_time", "user_id", "user_name",
			"task_ids", "work_dir_ids", "task_diff_lines",
			"upstream_tokens", "downstream_tokens", "cost",
			"task_real_minutes", "task_ancient_minutes", "task_efficiency_ratio",
			"commit_ids", "commit_diff_lines",
			"commit_ancient_minutes", "commit_real_ai_minutes", "commit_real_ancient_minutes",
			"commit_real_minutes", "commit_efficiency_ratio",
			"updated_at",
		}),
	}).Create(&mup).Error
	if err != nil {
		return fmt.Errorf("upsert user_productivity 失败: %w", err)
	}
	return nil
}

func strPtrToVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func intPtrToVal(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}

func int64PtrToVal(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}

func float64PtrToVal(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}

func ListUserProductivity(db *gorm.DB, userId, startTime, endTime string, page, pageSize int) ([]UserProductivity, error) {
	q := db.Model(&models.UserProductivity{})
	if userId != "" {
		q = q.Where("user_id = ?", userId)
	}
	if startTime != "" {
		q = q.Where("create_time >= ?", startTime)
	}
	if endTime != "" {
		q = q.Where("create_time <= ?", endTime)
	}
	var ups []models.UserProductivity
	if err := q.Order("create_time DESC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&ups).Error; err != nil {
		return nil, fmt.Errorf("查询 user_productivity 列表失败: %w", err)
	}
	return toUserProductivitySlice(ups), nil
}

func CountUserProductivity(db *gorm.DB, userId, startTime, endTime string) (int, error) {
	q := db.Model(&models.UserProductivity{})
	if userId != "" {
		q = q.Where("user_id = ?", userId)
	}
	if startTime != "" {
		q = q.Where("create_time >= ?", startTime)
	}
	if endTime != "" {
		q = q.Where("create_time <= ?", endTime)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("统计 user_productivity 总数失败: %w", err)
	}
	return int(count), nil
}

// ============================================================
// user_groups CRUD (GORM)
// ============================================================

func CreateUserGroup(db *gorm.DB, name string, orgName string, userIDs []string) (*UserGroup, error) {
	userIDsJSON, err := json.Marshal(userIDs)
	if err != nil {
		return nil, fmt.Errorf("序列化 user_ids 失败: %w", err)
	}
	g := models.UserGroup{
		Name:    name,
		OrgName: orgName,
		UserIDs: models.StringJSON(userIDsJSON),
	}
	if err := db.Create(&g).Error; err != nil {
		return nil, fmt.Errorf("创建 user_group 失败: %w", err)
	}
	// 重新查询以获取完整数据
	return GetUserGroup(db, g.GroupID)
}

func ListUserGroups(db *gorm.DB) ([]UserGroup, error) {
	var gs []models.UserGroup
	if err := db.Order("created_at DESC").Find(&gs).Error; err != nil {
		return nil, fmt.Errorf("查询 user_groups 列表失败: %w", err)
	}
	result := make([]UserGroup, len(gs))
	for i := range gs {
		result[i] = *toUserGroup(&gs[i])
	}
	return result, nil
}

func GetUserGroup(db *gorm.DB, groupId string) (*UserGroup, error) {
	var g models.UserGroup
	err := db.Where("group_id = ?", groupId).First(&g).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询 user_group 失败: %w", err)
	}
	return toUserGroup(&g), nil
}

func DeleteUserGroup(db *gorm.DB, groupId string) error {
	result := db.Where("group_id = ?", groupId).Delete(&models.UserGroup{})
	if result.Error != nil {
		return fmt.Errorf("删除 user_group 失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user group not found: %s", groupId)
	}
	return nil
}

// ============================================================
// 轻量查询 commits
// ============================================================

func ListCommitLightByRepoRange(db *gorm.DB, repoAddr, repoBranch, startTime, endTime string) ([]CommitLightStats, error) {
	type row struct {
		UserName  *string
		DiffLines *int
	}
	q := db.Model(&models.Commit{}).
		Select("COALESCE(user_name, git_user_name) AS user_name, diff_lines")

	if repoAddr != "" {
		q = q.Where("repo_addr = ?", repoAddr)
	}
	if repoBranch != "" {
		q = q.Where("repo_branch = ?", repoBranch)
	}
	if startTime != "" {
		q = q.Where("commit_time >= ?", startTime)
	}
	if endTime != "" {
		q = q.Where("commit_time <= ?", endTime)
	}

	var rows []row
	if err := q.Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("轻量查询 commits 失败: %w", err)
	}
	list := make([]CommitLightStats, len(rows))
	for i, r := range rows {
		list[i] = CommitLightStats{UserName: r.UserName, DiffLines: r.DiffLines}
	}
	return list, nil
}

// ============================================================
// 仪表盘聚合查询
// ============================================================

type dashboardTaskAgg struct {
	TotalTasks          int
	TotalUsers          int
	TotalRepos          int
	TotalCost           float64
	TotalTokens         int64
	TotalAiDays         float64
	TotalRealMinutes    float64
	TotalAncientMinutes float64
}

func queryDashboardTaskAgg(db *gorm.DB, startTime, endTime string) (*dashboardTaskAgg, error) {
	var agg dashboardTaskAgg
	q := db.Model(&models.Task{}).
		Select(`COUNT(task_id) as total_tasks,
			COUNT(DISTINCT user_id) as total_users,
			COUNT(DISTINCT work_dir_id) as total_repos,
			COALESCE(SUM(cost), 0) as total_cost,
			COALESCE(SUM(upstream_tokens + downstream_tokens), 0) as total_tokens,
			COALESCE(SUM(task_ancient_minutes), 0) as total_ai_days,
			COALESCE(SUM(CASE WHEN task_real_minutes_manual IS NOT NULL THEN task_real_minutes_manual ELSE task_real_minutes END), 0) as total_real_minutes,
			COALESCE(SUM(CASE WHEN task_ancient_minutes_manual IS NOT NULL THEN task_ancient_minutes_manual ELSE task_ancient_minutes END), 0) as total_ancient_minutes`)
	if startTime != "" {
		q = q.Where("start_time >= ?", startTime)
	}
	if endTime != "" {
		q = q.Where("start_time <= ?", endTime)
	}
	if err := q.Scan(&agg).Error; err != nil {
		return nil, fmt.Errorf("查询 tasks 仪表盘聚合失败: %w", err)
	}
	return &agg, nil
}

type dashboardCommitAgg struct {
	TotalCommits   int
	TotalDiffLines int64
}

func queryDashboardCommitAgg(db *gorm.DB, startTime, endTime string) (*dashboardCommitAgg, error) {
	var agg dashboardCommitAgg
	q := db.Model(&models.Commit{}).
		Select(`COUNT(*) as total_commits,
			COALESCE(SUM(diff_lines), 0) as total_diff_lines`)
	if startTime != "" {
		q = q.Where("commit_time >= ?", startTime)
	}
	if endTime != "" {
		q = q.Where("commit_time <= ?", endTime)
	}
	if err := q.Scan(&agg).Error; err != nil {
		return nil, fmt.Errorf("查询 commits 仪表盘聚合失败: %w", err)
	}
	return &agg, nil
}

func countDistinctWorkDirs(db *gorm.DB) (int, error) {
	var count int
	if err := db.Raw(`SELECT COUNT(*) FROM (SELECT DISTINCT repo_addr, repo_branch FROM commits WHERE repo_addr IS NOT NULL AND repo_addr != '') sub`).Scan(&count).Error; err != nil {
		return 0, fmt.Errorf("查询 work_dirs 聚合失败: %w", err)
	}
	return count, nil
}

// ============================================================
// user_productivity 聚合查询 (用于 user list / org list)
// ============================================================

type userProdAggRow struct {
	UserID               string
	UserName             string
	DayCount             int
	TaskCount            int
	CommitCount          int
	TaskDiffLines        int
	CommitDiffLines      int
	UpstreamTokens       int64
	DownstreamTokens     int64
	Cost                 float64
	TaskRealMinutes      float64
	TaskAncientMinutes   float64
	CommitRealMinutes    float64
	CommitAncientMinutes float64
}

func queryUserProdAgg(db *gorm.DB, startTime, endTime string) ([]userProdAggRow, error) {
	var rows []userProdAggRow
	q := db.Model(&models.UserProductivity{}).
		Select(`user_id,
			COALESCE(MAX(user_name), '') as user_name,
			COUNT(*) as day_count,
			COALESCE(SUM(jsonb_array_length(task_ids)), 0) as task_count,
			COALESCE(SUM(jsonb_array_length(commit_ids)), 0) as commit_count,
			COALESCE(SUM(task_diff_lines), 0) as task_diff_lines,
			COALESCE(SUM(commit_diff_lines), 0) as commit_diff_lines,
			COALESCE(SUM(upstream_tokens), 0) as upstream_tokens,
			COALESCE(SUM(downstream_tokens), 0) as downstream_tokens,
			COALESCE(SUM(cost), 0) as cost,
			COALESCE(SUM(task_real_minutes), 0) as task_real_minutes,
			COALESCE(SUM(task_ancient_minutes), 0) as task_ancient_minutes,
			COALESCE(SUM(commit_real_minutes), 0) as commit_real_minutes,
			COALESCE(SUM(commit_ancient_minutes), 0) as commit_ancient_minutes`)
	if startTime != "" {
		q = q.Where("create_time >= ?", startTime)
	}
	if endTime != "" {
		q = q.Where("create_time <= ?", endTime)
	}
	if err := q.Group("user_id").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("查询 user_productivity 聚合失败: %w", err)
	}
	return rows, nil
}

func queryUserProdAggForIDs(db *gorm.DB, userIDs []string, startTime, endTime string) ([]userProdAggRow, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	var rows []userProdAggRow
	q := db.Model(&models.UserProductivity{}).
		Select(`user_id,
			COALESCE(SUM(jsonb_array_length(task_ids)), 0) as task_count,
			COALESCE(SUM(jsonb_array_length(commit_ids)), 0) as commit_count,
			COALESCE(SUM(task_diff_lines), 0) as task_diff_lines,
			COALESCE(SUM(commit_diff_lines), 0) as commit_diff_lines,
			COALESCE(SUM(upstream_tokens), 0) as upstream_tokens,
			COALESCE(SUM(downstream_tokens), 0) as downstream_tokens,
			COALESCE(SUM(cost), 0) as cost,
			COALESCE(SUM(task_real_minutes), 0) as task_real_minutes,
			COALESCE(SUM(task_ancient_minutes), 0) as task_ancient_minutes,
			COALESCE(SUM(commit_real_minutes), 0) as commit_real_minutes,
			COALESCE(SUM(commit_ancient_minutes), 0) as commit_ancient_minutes`).
		Where("user_id IN ?", userIDs)
	if startTime != "" {
		q = q.Where("create_time >= ?", startTime)
	}
	if endTime != "" {
		q = q.Where("create_time <= ?", endTime)
	}
	if err := q.Group("user_id").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("查询 user_productivity 聚合失败: %w", err)
	}
	return rows, nil
}

type userProdTimeSeriesRow struct {
	UserID               string
	CreateTime           time.Time
	TaskCount            int
	CommitCount          int
	TaskDiffLines        int
	CommitDiffLines      int
	UpstreamTokens       int64
	DownstreamTokens     int64
	Cost                 float64
	TaskRealMinutes      float64
	TaskAncientMinutes   float64
	CommitRealMinutes    float64
	CommitAncientMinutes float64
}

func queryUserProdTimeSeries(db *gorm.DB, userIDs []string, startTime, endTime string) ([]userProdTimeSeriesRow, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	var rows []userProdTimeSeriesRow
	q := db.Model(&models.UserProductivity{}).
		Select(`user_id, create_time,
			COALESCE(jsonb_array_length(task_ids), 0) as task_count,
			COALESCE(jsonb_array_length(commit_ids), 0) as commit_count,
			COALESCE(task_diff_lines, 0) as task_diff_lines,
			COALESCE(commit_diff_lines, 0) as commit_diff_lines,
			COALESCE(upstream_tokens, 0) as upstream_tokens,
			COALESCE(downstream_tokens, 0) as downstream_tokens,
			COALESCE(cost, 0) as cost,
			COALESCE(task_real_minutes, 0) as task_real_minutes,
			COALESCE(task_ancient_minutes, 0) as task_ancient_minutes,
			COALESCE(commit_real_minutes, 0) as commit_real_minutes,
			COALESCE(commit_ancient_minutes, 0) as commit_ancient_minutes`).
		Where("user_id IN ?", userIDs)
	if startTime != "" {
		q = q.Where("create_time >= ?", startTime)
	}
	if endTime != "" {
		q = q.Where("create_time <= ?", endTime)
	}
	if err := q.Order("create_time").Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ============================================================
// 任务内联操作
// ============================================================

func UpdateTaskRealMinutes(db *gorm.DB, taskID string, minutes float64, reason string) error {
	result := db.Model(&models.Task{}).Where("task_id = ?", taskID).
		Updates(map[string]interface{}{"task_real_minutes": minutes, "task_real_minutes_reason": reason})
	if result.Error != nil {
		log.Printf("更新 task_real_minutes 失败: %v", result.Error)
		return result.Error
	}
	return nil
}

func UpdateTaskTitle(db *gorm.DB, taskID string, title string) error {
	result := db.Model(&models.Task{}).Where("task_id = ?", taskID).
		Updates(map[string]interface{}{"title": title, "updated_at": time.Now()})
	if result.Error != nil {
		log.Printf("回写title失败: %v", result.Error)
		return result.Error
	}
	return nil
}

func GetTaskIDsForEstimation(db *gorm.DB, specificTaskID string) ([]string, error) {
	var taskIDs []string
	q := db.Model(&models.Task{}).Select("task_id").
		Where("task_ancient_minutes IS NULL AND task_ancient_minutes_manual IS NULL")
	if specificTaskID != "" {
		q = q.Where("task_id = ?", specificTaskID)
	} else {
		q = q.Order("start_time DESC").Limit(50)
	}
	if err := q.Pluck("task_id", &taskIDs).Error; err != nil {
		return nil, err
	}
	return taskIDs, nil
}

func GetConvInputForEstimation(db *gorm.DB, taskID string) ([]string, []string, int64, int64, error) {
	type row struct {
		UserInput        *string
		Diff             *string
		DiffLines        *int64
		UpstreamTokens   *int64
		DownstreamTokens *int64
	}
	var rows []row
	if err := db.Model(&models.TaskConversation{}).
		Select("user_input, diff, diff_lines, upstream_tokens, downstream_tokens").
		Where("task_id = ?", taskID).
		Order("start_time").
		Scan(&rows).Error; err != nil {
		return nil, nil, 0, 0, err
	}

	var userInputs []string
	var codeOutputs []string
	var totalLines int64
	var totalChars int64
	for _, r := range rows {
		if r.UserInput != nil && *r.UserInput != "" {
			userInputs = append(userInputs, *r.UserInput)
			totalChars += int64(len(*r.UserInput))
		}
		if r.Diff != nil && *r.Diff != "" {
			codeOutputs = append(codeOutputs, *r.Diff)
		}
		if r.DiffLines != nil {
			totalLines += *r.DiffLines
		}
	}
	return userInputs, codeOutputs, totalChars, totalLines, nil
}

func UpdateTaskAncientEstimation(db *gorm.DB, taskID string, minutes float64, reason string) error {
	result := db.Model(&models.Task{}).Where("task_id = ?", taskID).
		Updates(map[string]interface{}{
			"task_ancient_minutes":        minutes,
			"task_ancient_minutes_reason": reason,
			"updated_at":                  time.Now(),
		})
	if result.Error != nil {
		return fmt.Errorf("db update failed: %w", result.Error)
	}
	return nil
}

// ============================================================
// user_org 查询
// ============================================================

func loadUserOrgs(db *gorm.DB) (map[string]*models.UserOrg, error) {
	var uos []models.UserOrg
	if err := db.Find(&uos).Error; err != nil {
		return nil, fmt.Errorf("查询 user_org 表失败: %w", err)
	}
	result := make(map[string]*models.UserOrg, len(uos))
	for i := range uos {
		if uos[i].UserID == "" {
			continue
		}
		result[uos[i].UserID] = &uos[i]
	}
	return result, nil
}

// LoadUserOrgs 从 user_org 表加载组织映射（用于启动时加载到 orgMappings）
func LoadUserOrgs(db *gorm.DB) (map[string]*OrgMapping, error) {
	uos, err := loadUserOrgs(db)
	if err != nil {
		return nil, err
	}
	maps := make(map[string]*OrgMapping, len(uos))
	for id, uo := range uos {
		maps[id] = &OrgMapping{
			UserID:       uo.UserID,
			UserName:     uo.UserName,
			Org1:         uo.Org1,
			Org2:         uo.Org2,
			Org3:         uo.Org3,
			Org4:         uo.Org4,
			Org5:         uo.Org5,
			Org6:         uo.Org6,
			Org7:         uo.Org7,
			Org8:         uo.Org8,
			Org9:         uo.Org9,
			GitUserName:  uo.GitUserName,
			GitUserEmail: uo.GitUserEmail,
		}
	}
	return maps, nil
}

// ============================================================
// 仪表盘汇总查询辅助
// ============================================================

func QueryDashboardTaskAgg(db *gorm.DB, startTime, endTime string) (*dashboardTaskAgg, error) {
	return queryDashboardTaskAgg(db, startTime, endTime)
}

func QueryDashboardCommitAgg(db *gorm.DB, startTime, endTime string) (*dashboardCommitAgg, error) {
	return queryDashboardCommitAgg(db, startTime, endTime)
}

func QueryDistinctWorkDirs(db *gorm.DB) (int, error) {
	return countDistinctWorkDirs(db)
}

func QueryUserProdAgg(db *gorm.DB, startTime, endTime string) ([]userProdAggRow, error) {
	return queryUserProdAgg(db, startTime, endTime)
}

func QueryUserProdAggForIDs(db *gorm.DB, userIDs []string, startTime, endTime string) ([]userProdAggRow, error) {
	return queryUserProdAggForIDs(db, userIDs, startTime, endTime)
}

func QueryUserProdTimeSeries(db *gorm.DB, userIDs []string, startTime, endTime string) ([]userProdTimeSeriesRow, error) {
	return queryUserProdTimeSeries(db, userIDs, startTime, endTime)
}
