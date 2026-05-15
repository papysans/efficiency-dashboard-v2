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
)

// ============================================================
// 响应类型（保持与原有JSON结构一致）
// ============================================================

type UserGroup struct {
	GroupID   string          `json:"group_id"`
	Name      string          `json:"name"`
	OrgName   string          `json:"org_name"`
	UserIDs   json.RawMessage `json:"user_ids" swaggertype:"string" example:"[\"user1\", \"user2\"]"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type TaskListItem struct {
	TaskId                   string    `json:"task_id"`
	SessionId                string    `json:"session_id"`
	CommitId                 string    `json:"commit_id"`
	Title                    string    `json:"title"`
	UserId                   string    `json:"user_id"`
	UserName                 string    `json:"user_name"`
	ClientId                 string    `json:"client_id"`
	ClientIde                string    `json:"client_ide"`
	ClientVersion            string    `json:"client_version"`
	ClientOs                 string    `json:"client_os"`
	ClientOsVersion          string    `json:"client_os_version"`
	Caller                   string    `json:"caller"`
	RepoAddr                 string    `json:"repo_addr"`
	RepoBranch               string    `json:"repo_branch"`
	WorkDir                  string    `json:"work_dir"`
	WorkDirId                string    `json:"work_dir_id"`
	StartTime                time.Time `json:"start_time"`
	EndTime                  time.Time `json:"end_time"`
	UpstreamTokens           int64     `json:"upstream_tokens"`
	DownstreamTokens         int64     `json:"downstream_tokens"`
	Cost                     float64   `json:"cost"`
	Silica                   float64   `json:"silica"`
	AcceptRatio              float64   `json:"accept_ratio"`
	DiffLines                int       `json:"diff_lines"`
	TaskAncientMinutes       float64   `json:"task_ancient_minutes"`
	TaskAncientReason        string    `json:"task_ancient_minutes_reason"`
	TaskAncientMinutesManual *float64  `json:"task_ancient_minutes_manual"`
	TaskAncientReasonManual  string    `json:"task_ancient_minutes_reason_manual"`
	TaskRealMinutes          float64   `json:"task_real_minutes"`
	TaskRealReason           string    `json:"task_real_minutes_reason"`
	TaskRealMinutesManual    *float64  `json:"task_real_minutes_manual"`
	TaskRealReasonManual     string    `json:"task_real_minutes_reason_manual"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
	EfficiencyRatio          float64   `json:"efficiency_ratio"`
	Org1                     string    `json:"org1"`
	Org2                     string    `json:"org2"`
	Org3                     string    `json:"org3"`
	Org4                     string    `json:"org4"`
	Org5                     string    `json:"org5"`
	Org6                     string    `json:"org6"`
	Org7                     string    `json:"org7"`
	Org8                     string    `json:"org8"`
	Org9                     string    `json:"org9"`
	OrgDisplay               string    `json:"org_display"`
}

type CommitListItem struct {
	CommitId                   string          `json:"commit_id"`
	CommitTime                 time.Time       `json:"commit_time"`
	RepoAddr                   string          `json:"repo_addr"`
	RepoBranch                 string          `json:"repo_branch"`
	GitUserName                string          `json:"git_user_name"`
	GitUserEmail               string          `json:"git_user_email"`
	UserId                     string          `json:"user_id"`
	UserName                   string          `json:"user_name"`
	ClientId                   string          `json:"client_id"`
	WorkDir                    string          `json:"work_dir"`
	DiffLines                  int             `json:"diff_lines"`
	CommitAncientMinutes       float64         `json:"commit_ancient_minutes"`
	CommitAncientReason        string          `json:"commit_ancient_minutes_reason"`
	CommitAncientMinutesManual *float64        `json:"commit_ancient_minutes_manual"`
	CommitAncientReasonManual  string          `json:"commit_ancient_minutes_reason_manual"`
	CommitRealMinutes          float64         `json:"commit_real_minutes"`
	CommitRealReason           string          `json:"commit_real_minutes_reason"`
	CommitRealMinutesManual    *float64        `json:"commit_real_minutes_manual"`
	CommitRealReasonManual     string          `json:"commit_real_minutes_reason_manual"`
	CommitRealAiMinutes        float64         `json:"commit_real_ai_minutes"`
	CommitRealAncientMinutes   float64         `json:"commit_real_ancient_minutes"`
	TaskIds                    json.RawMessage `json:"task_ids" swaggertype:"string" example:"[\"task1\"]"`
	TaskIdsSilica              json.RawMessage `json:"task_ids_silica" swaggertype:"string" example:"[\"1.0\"]"`
	TaskAcceptRatios           json.RawMessage `json:"task_accept_ratios" swaggertype:"string" example:"[\"0.5\"]"`
	Comment                    string          `json:"comment"`
	CreatedAt                  time.Time       `json:"created_at"`
	UpdatedAt                  time.Time       `json:"updated_at"`
	Cost                       float64         `json:"cost"`
	UpstreamTokens             int64           `json:"upstream_tokens"`
	DownstreamTokens           int64           `json:"downstream_tokens"`
	Silica                     float64         `json:"silica"`
	EfficiencyRatio            float64         `json:"efficiency_ratio"`
	Org1                       string          `json:"org1"`
	Org2                       string          `json:"org2"`
	Org3                       string          `json:"org3"`
	Org4                       string          `json:"org4"`
	Org5                       string          `json:"org5"`
	Org6                       string          `json:"org6"`
	Org7                       string          `json:"org7"`
	Org8                       string          `json:"org8"`
	Org9                       string          `json:"org9"`
	OrgDisplay                 string          `json:"org_display"`
}

type RelatedTask struct {
	TaskId          string    `json:"task_id"`
	UserName        string    `json:"user_name"`
	StartTime       time.Time `json:"start_time"`
	TaskRealMinutes float64   `json:"task_real_minutes"`
	Silica          float64   `json:"silica"`
	Cost            float64   `json:"cost"`
	DiffLines       int       `json:"diff_lines"`
}

// ============================================================
// 聚合类型（非DB表，查询结果）
// ============================================================

type RepoAggregate struct {
	RepoAddr          string
	RepoBranch        string
	CommitCount       int
	StartTime         time.Time
	EndTime           time.Time
	SumAncientMinutes float64
	SumRealMinutes    float64
	TaskCount         int
	EfficiencyRatio   float64
}

type ProjectAggregates struct {
	StartTime                 *time.Time
	EndTime                   *time.Time
	UpstreamTokens            int64
	DownstreamTokens          int64
	Cost                      float64
	ProjectAncientMinutes     *float64
	ProjectAncientReason      string
	ProjectRealProcessMinutes *float64
	ProjectRealProcessReason  string
	ProjectRealLeadMinutes    *float64
	ProjectRealLeadReason     string
}

type CommitLightStats struct {
	UserName  string
	DiffLines int
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

func ptrToInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}
func ptrToFloat64(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}
func ptrToStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ============================================================
// models → response 类型转换
// ============================================================

func toUserGroup(g *models.UserGroup) *UserGroup {
	if g == nil {
		return nil
	}
	return &UserGroup{
		GroupID:   g.GroupID,
		Name:      g.Name,
		OrgName:   g.OrgName,
		UserIDs:   json.RawMessage(g.UserIDs),
		CreatedAt: g.CreatedAt,
		UpdatedAt: g.UpdatedAt,
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

func toTaskListItem(t *models.Task) *TaskListItem {
	if t == nil {
		return nil
	}
	efficiencyRatio := utils.CalcEfficiencyRatioManual(t.TaskAncientMinutes,
		t.TaskRealMinutes, t.TaskAncientMinutesManual, t.TaskRealMinutesManual)

	it := &TaskListItem{
		TaskId:                   t.TaskId,
		SessionId:                t.SessionId,
		CommitId:                 t.CommitId,
		Title:                    t.Title,
		UserId:                   t.UserId,
		UserName:                 t.UserName,
		ClientId:                 t.ClientId,
		ClientIde:                t.ClientIde,
		ClientVersion:            t.ClientVersion,
		ClientOs:                 t.ClientOs,
		ClientOsVersion:          t.ClientOsVersion,
		Caller:                   t.Caller,
		RepoAddr:                 t.RepoAddr,
		RepoBranch:               t.RepoBranch,
		WorkDir:                  t.WorkDir,
		WorkDirId:                t.WorkDirId,
		StartTime:                t.StartTime,
		EndTime:                  t.EndTime,
		UpstreamTokens:           t.UpstreamTokens,
		DownstreamTokens:         t.DownstreamTokens,
		Cost:                     t.Cost,
		Silica:                   t.Silica,
		AcceptRatio:              t.AcceptRatio,
		DiffLines:                t.DiffLines,
		TaskRealMinutes:          t.TaskRealMinutes,
		TaskRealReason:           t.TaskRealReason,
		TaskRealMinutesManual:    t.TaskRealMinutesManual,
		TaskRealReasonManual:     t.TaskRealReasonManual,
		TaskAncientMinutes:       t.TaskAncientMinutes,
		TaskAncientReason:        t.TaskAncientReason,
		TaskAncientMinutesManual: t.TaskAncientMinutesManual,
		TaskAncientReasonManual:  t.TaskAncientReasonManual,
		EfficiencyRatio:          efficiencyRatio,
		CreatedAt:                t.CreatedAt,
		UpdatedAt:                t.UpdatedAt,
	}

	if t.UserId != "" {
		if om, ok := orgMappings[t.UserId]; ok {
			it.Org1 = om.Org1
			it.Org2 = om.Org2
			it.Org3 = om.Org3
			it.Org4 = om.Org4
			it.Org5 = om.Org5
			it.Org6 = om.Org6
			it.Org7 = om.Org7
			it.Org8 = om.Org8
			it.Org9 = om.Org9
			it.OrgDisplay = getOrgDisplay(om.Org1, om.Org2, om.Org3, om.Org4,
				om.Org5, om.Org6, om.Org7, om.Org8, om.Org9)
		}
	}
	return it
}

func toTaskListItemSlice(tasks []models.Task) []TaskListItem {
	result := make([]TaskListItem, len(tasks))
	for i, t := range tasks {
		result[i] = *toTaskListItem(&t)
	}
	return result
}

func toRelatedTask(t *models.Task) *RelatedTask {
	if t == nil {
		return nil
	}
	return &RelatedTask{
		TaskId:          t.TaskId,
		UserName:        t.UserName,
		StartTime:       t.StartTime,
		TaskRealMinutes: t.TaskRealMinutes,
		Cost:            t.Cost,
		DiffLines:       t.DiffLines,
	}
}

func toRelatedTaskSlice(tasks []models.Task) []RelatedTask {
	result := make([]RelatedTask, len(tasks))
	for i, t := range tasks {
		if rt := toRelatedTask(&t); rt != nil {
			result[i] = *rt
		}
	}
	return result
}

func GetRelatedTasks(db *gorm.DB, taskIds []string, taskSilicas []float64) []RelatedTask {
	var relatedTasks []RelatedTask
	for i, taskId := range taskIds {
		var rt RelatedTask
		task, err := GetTask(statDB, taskId)
		if err != nil {
			log.Printf("查询关联 task %s 失败: %v", taskId, err)
		}
		if task != nil {
			rt = *toRelatedTask(task)
		} else {
			rt.TaskId = taskId
		}
		if i < len(taskSilicas) {
			rt.Silica = taskSilicas[i]
		}
		relatedTasks = append(relatedTasks, rt)
	}
	return relatedTasks
}

func BatchGetTasks(db *gorm.DB, taskIds []string) (map[string]*models.Task, error) {
	result := make(map[string]*models.Task)
	if len(taskIds) == 0 {
		return result, nil
	}
	var tasks []models.Task
	if err := db.Where("task_id IN ?", taskIds).Find(&tasks).Error; err != nil {
		return nil, fmt.Errorf("批量查询 tasks 失败: %w", err)
	}
	for i := range tasks {
		result[tasks[i].TaskId] = &tasks[i]
	}
	return result, nil
}

func GetTask(db *gorm.DB, taskId string) (*models.Task, error) {
	var t models.Task
	err := db.Where("task_id = ?", taskId).First(&t).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询 tasks 失败: %w", err)
	}
	return &t, nil
}

func GetSession(db *gorm.DB, sessionId string) (*models.Session, error) {
	var s models.Session
	err := db.Where("session_id = ?", sessionId).First(&s).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询 sessions 失败: %w", err)
	}
	return &s, nil
}

type SessionFilter struct {
	UserId        string
	UserIds       []string
	UserName      string
	ClientId      string
	ClientIde     string
	ClientVersion string
	ClientOs      string
	StartTime     string
	EndTime       string
}

func (f *SessionFilter) applyToQuery(q *gorm.DB) *gorm.DB {
	if f.UserId != "" {
		q = q.Where("user_id = ?", f.UserId)
	}
	if len(f.UserIds) > 0 {
		q = q.Where("user_id IN ?", f.UserIds)
	}
	if f.UserName != "" {
		q = q.Where("user_name = ?", f.UserName)
	}
	if f.ClientId != "" {
		q = q.Where("client_id = ?", f.ClientId)
	}
	if f.ClientIde != "" {
		q = q.Where("client_ide = ?", f.ClientIde)
	}
	if f.ClientVersion != "" {
		q = q.Where("client_version = ?", f.ClientVersion)
	}
	if f.ClientOs != "" {
		q = q.Where("client_os = ?", f.ClientOs)
	}
	if f.StartTime != "" {
		q = q.Where("create_time >= ?", f.StartTime)
	}
	if f.EndTime != "" {
		q = q.Where("create_time <= ?", f.EndTime)
	}
	return q
}

func ListSessions(db *gorm.DB, filter SessionFilter, page, pageSize int, orderClause string) ([]models.Session, int, error) {
	q := filter.applyToQuery(db.Model(&models.Session{}))
	var sessions []models.Session
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return []models.Session{}, 0, fmt.Errorf("统计 sessions 总数失败: %w", err)
	}
	if orderClause != "" {
		q = q.Order(orderClause)
	}
	if pageSize > 0 {
		q = q.Limit(pageSize).Offset((page - 1) * pageSize)
	}
	if err := q.Find(&sessions).Error; err != nil {
		return nil, int(count), fmt.Errorf("查询 sessions 列表失败: %w", err)
	}
	return sessions, int(count), nil
}

type TaskFilter struct {
	OrgsFilter
	UserId     string
	UserName   string
	ClientId   string
	ClientIde  string
	ClientOs   string
	Caller     string
	RepoAddr   string
	RepoBranch string
	WorkDirId  string
	StartTime  string
	EndTime    string
	TaskIds    []string
}

func (f *TaskFilter) applyToQuery(q *gorm.DB) *gorm.DB {
	if f.UserId != "" {
		q = q.Where("user_id = ?", f.UserId)
	}
	if f.UserName != "" {
		q = q.Where("user_name = ?", f.UserName)
	}
	if f.ClientId != "" {
		q = q.Where("client_id = ?", f.ClientId)
	}
	if f.ClientIde != "" {
		q = q.Where("client_ide = ?", f.ClientIde)
	}
	if f.ClientOs != "" {
		q = q.Where("client_os = ?", f.ClientOs)
	}
	if f.Caller != "" {
		q = q.Where("caller = ?", f.Caller)
	}
	if f.RepoAddr != "" {
		q = q.Where("repo_addr = ?", f.RepoAddr)
	}
	if f.RepoBranch != "" {
		q = q.Where("repo_branch = ?", f.RepoBranch)
	}
	if f.WorkDirId != "" {
		q = q.Where("work_dir_id = ?", f.WorkDirId)
	}
	q = f.ApplyOrgsToQuery(q)
	if f.StartTime != "" {
		q = q.Where("start_time >= ?", f.StartTime)
	}
	if f.EndTime != "" {
		q = q.Where("start_time <= ?", f.EndTime)
	}
	if f.TaskIds != nil {
		if len(f.TaskIds) > 0 {
			q = q.Where("task_id IN ?", f.TaskIds)
		} else {
			q = q.Where("1 = 0")
		}
	}
	return q
}

func ListTasks(db *gorm.DB, filter TaskFilter, page, pageSize int, orderClause string) ([]models.Task, int, error) {
	q := filter.applyToQuery(db.Model(&models.Task{}))
	var tasks []models.Task
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return []models.Task{}, 0, fmt.Errorf("统计 tasks 总数失败: %w", err)
	}
	if orderClause != "" {
		q = q.Order(orderClause)
	}
	if pageSize > 0 {
		q = q.Limit(pageSize).Offset((page - 1) * pageSize)
	}
	if err := q.Find(&tasks).Error; err != nil {
		return nil, int(count), fmt.Errorf("查询 tasks 列表失败: %w", err)
	}
	return tasks, int(count), nil
}

func UpdateStatTaskManual(db *gorm.DB, taskId string, realManual *float64, realReasonManual *string, ancientManual *float64, ancientReasonManual *string) error {
	updates := map[string]interface{}{
		"task_real_minutes_manual":           realManual,
		"task_real_minutes_reason_manual":    realReasonManual,
		"task_ancient_minutes_manual":        ancientManual,
		"task_ancient_minutes_reason_manual": ancientReasonManual,
		"updated_at":                         time.Now(),
	}
	result := db.Model(&models.Task{}).Where("task_id = ?", taskId).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("更新 tasks manual 字段失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("tasks task_id=%s 不存在", taskId)
	}
	return nil
}

// ============================================================
// conversations CRUD (GORM)
// ============================================================

func ListConversations(db *gorm.DB, taskId string) ([]models.Conversation, error) {
	var convs []models.Conversation
	if err := db.Where("task_id = ?", taskId).Order("start_time ASC").Find(&convs).Error; err != nil {
		return nil, fmt.Errorf("查询 conversations 列表失败: %w", err)
	}
	return convs, nil
}

type UserFilter struct {
	OrgsFilter
	StartTime   string
	EndTime     string
	Granularity string
	UserIds     []string
}

func (f *UserFilter) applyToQuery(q *gorm.DB) *gorm.DB {
	userIds := f.UserIds
	orgUserIds := f.GetFilter()
	if orgUserIds != nil {
		if userIds == nil {
			userIds = orgUserIds
		} else if len(userIds) > 0 {
			orgSet := make(map[string]bool, len(orgUserIds))
			for _, uid := range orgUserIds {
				orgSet[uid] = true
			}
			intersection := make([]string, 0)
			for _, uid := range userIds {
				if orgSet[uid] {
					intersection = append(intersection, uid)
				}
			}
			userIds = intersection
		}
	}
	if userIds != nil {
		if len(userIds) > 0 {
			q = q.Where("user_id IN ?", userIds)
		} else {
			q = q.Where("1 = 0")
		}
	}
	if f.StartTime != "" {
		q = q.Where("create_time >= ?", f.StartTime)
	}
	if f.EndTime != "" {
		q = q.Where("create_time <= ?", f.EndTime)
	}
	return q
}

// ============================================================
// commits CRUD (GORM)
// ============================================================

type CommitFilter struct {
	OrgsFilter
	RepoAddr    string
	RepoBranch  string
	GitUserName string
	UserId      string
	UserIds     []string
	UserName    string
	ClientId    string
	WorkDir     string
	WorkDirId   string
	StartTime   string
	EndTime     string
}

func intersectUserIdFilter(orgUserIds []string, userId string, userIds []string) []string {
	hasOrgFilter := orgUserIds != nil
	hasUserId := userId != ""
	hasUserIds := userIds != nil

	if !hasOrgFilter && !hasUserId && !hasUserIds {
		return nil
	}

	var set map[string]bool

	if hasOrgFilter {
		if len(orgUserIds) == 0 {
			return []string{}
		}
		set = make(map[string]bool, len(orgUserIds))
		for _, uid := range orgUserIds {
			set[uid] = true
		}
	}

	if hasUserId {
		if set == nil {
			set = map[string]bool{userId: true}
		} else if !set[userId] {
			return []string{}
		} else {
			set = map[string]bool{userId: true}
		}
	}

	if hasUserIds {
		if len(userIds) == 0 {
			return []string{}
		}
		if set == nil {
			set = make(map[string]bool, len(userIds))
			for _, uid := range userIds {
				set[uid] = true
			}
		} else {
			newSet := make(map[string]bool)
			for _, uid := range userIds {
				if set[uid] {
					newSet[uid] = true
				}
			}
			set = newSet
			if len(set) == 0 {
				return []string{}
			}
		}
	}

	ids := make([]string, 0, len(set))
	for uid := range set {
		ids = append(ids, uid)
	}
	return ids
}

func (f *CommitFilter) applyUserIdFilter(q *gorm.DB) *gorm.DB {
	ids := intersectUserIdFilter(f.GetFilter(), f.UserId, f.UserIds)
	if ids == nil {
		return q
	}
	if len(ids) == 0 {
		return q.Where("1 = 0")
	}
	if len(ids) == 1 {
		return q.Where("user_id = ?", ids[0])
	}
	return q.Where("user_id IN ?", ids)
}

func (f *CommitFilter) applyToQuery(q *gorm.DB) *gorm.DB {
	if f.RepoAddr != "" {
		q = q.Where("repo_addr = ?", f.RepoAddr)
	}
	if f.RepoBranch != "" {
		q = q.Where("repo_branch = ?", f.RepoBranch)
	}
	if f.GitUserName != "" {
		q = q.Where("git_user_name = ?", f.GitUserName)
	}

	q = f.applyUserIdFilter(q)

	if f.UserName != "" {
		q = q.Where("user_name = ?", f.UserName)
	}
	if f.ClientId != "" {
		q = q.Where("client_id = ?", f.ClientId)
	}
	if f.WorkDir != "" {
		q = q.Where("work_dir = ?", f.WorkDir)
	}
	if f.WorkDirId != "" {
		q = q.Where("work_dir_id = ?", f.WorkDirId)
	}
	if f.StartTime != "" {
		q = q.Where("commit_time >= ?", f.StartTime)
	}
	if f.EndTime != "" {
		q = q.Where("commit_time <= ?", f.EndTime)
	}
	return q
}

func toCommitListItem(commit *models.Commit) CommitListItem {
	item := CommitListItem{
		CommitId:                   commit.CommitId,
		CommitTime:                 commit.CommitTime,
		RepoAddr:                   commit.RepoAddr,
		RepoBranch:                 commit.RepoBranch,
		GitUserName:                commit.GitUserName,
		GitUserEmail:               commit.GitUserEmail,
		UserId:                     commit.UserId,
		UserName:                   commit.UserName,
		ClientId:                   commit.ClientId,
		WorkDir:                    commit.WorkDir,
		DiffLines:                  commit.DiffLines,
		CommitAncientMinutes:       commit.CommitAncientMinutes,
		CommitAncientReason:        commit.CommitAncientReason,
		CommitAncientMinutesManual: commit.CommitAncientMinutesManual,
		CommitAncientReasonManual:  commit.CommitAncientReasonManual,
		CommitRealMinutes:          commit.CommitRealMinutes,
		CommitRealReason:           commit.CommitRealReason,
		CommitRealMinutesManual:    commit.CommitRealMinutesManual,
		CommitRealReasonManual:     commit.CommitRealReasonManual,
		CommitRealAiMinutes:        commit.CommitRealAiMinutes,
		CommitRealAncientMinutes:   commit.CommitRealNonAiMinutes,
		TaskIds:                    json.RawMessage(commit.TaskIds),
		TaskIdsSilica:              json.RawMessage(commit.TaskIdsSilica),
		TaskAcceptRatios:           json.RawMessage(commit.TaskAcceptRatios),
		Comment:                    commit.Comment,
		CreatedAt:                  commit.CreatedAt,
		UpdatedAt:                  commit.UpdatedAt,
		Cost:                       commit.Cost,
		UpstreamTokens:             commit.UpstreamTokens,
		DownstreamTokens:           commit.DownstreamTokens,
		Silica:                     commit.Silica,
	}
	ancient := commit.CommitAncientMinutes
	real := commit.CommitRealMinutes
	item.EfficiencyRatio = utils.CalcEfficiencyRatioManual(ancient,
		real,
		commit.CommitAncientMinutesManual,
		commit.CommitRealMinutesManual)

	if commit.UserId != "" {
		if om, ok := orgMappings[commit.UserId]; ok {
			item.Org1 = om.Org1
			item.Org2 = om.Org2
			item.Org3 = om.Org3
			item.Org4 = om.Org4
			item.Org5 = om.Org5
			item.Org6 = om.Org6
			item.Org7 = om.Org7
			item.Org8 = om.Org8
			item.Org9 = om.Org9
			item.OrgDisplay = getOrgDisplay(om.Org1, om.Org2, om.Org3, om.Org4, om.Org5, om.Org6, om.Org7, om.Org8, om.Org9)
		}
	}
	return item
}

func toCommitListItemSlice(commits []models.Commit) []CommitListItem {
	result := make([]CommitListItem, len(commits))
	for i := range commits {
		result[i] = toCommitListItem(&commits[i])
	}
	return result
}

func GetCommitByID(db *gorm.DB, commitID string) (*models.Commit, error) {
	var c models.Commit
	err := db.Where("commit_id = ?", commitID).First(&c).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询 commits 失败: %w", err)
	}
	return &c, nil
}

func ListCommits(db *gorm.DB, filter CommitFilter, page, pageSize int, orderClause string) ([]models.Commit, int, error) {
	q := filter.applyToQuery(db.Model(&models.Commit{}))
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("统计 commits 总数失败: %w", err)
	}
	var commits []models.Commit
	if err := q.Order(orderClause).Limit(pageSize).Offset((page - 1) * pageSize).Find(&commits).Error; err != nil {
		return nil, 0, fmt.Errorf("查询 commits 列表失败: %w", err)
	}
	return commits, int(total), nil
}

func UpdateCommitManual(db *gorm.DB, commitID string, ancientManual *float64, ancientReasonManual *string, realManual *float64, realReasonManual *string) error {
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

func UpdateCommitTaskAssoc(db *gorm.DB, commitID string, taskIDs, taskIDsSilica json.RawMessage, realMinutes *float64, realAIMinutes *float64, realAncientMinutes *float64, realReason *string) error {
	tids := models.StringJSON(taskIDs)
	if tids == "" || tids == "null" {
		tids = "[]"
	}
	ts := models.StringJSON(taskIDsSilica)
	if ts == "" || ts == "null" {
		ts = "[]"
	}
	updates := map[string]interface{}{
		"task_ids":                    tids,
		"task_ids_silica":             ts,
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
	var list []RepoAggregate

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

	if err := q.Group("repo_addr, repo_branch").Order("repo_addr, repo_branch").Scan(&list).Error; err != nil {
		return nil, fmt.Errorf("查询 commits 聚合失败: %w", err)
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

func CreateProject(db *gorm.DB, p *models.Project) (string, error) {
	if err := db.Create(p).Error; err != nil {
		return "", fmt.Errorf("创建 project 失败: %w", err)
	}
	return p.ProjectId, nil
}

func GetProject(db *gorm.DB, projectID string) (*models.Project, error) {
	var mp models.Project
	err := db.Where("project_id = ?", projectID).First(&mp).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询 project 失败: %w", err)
	}
	return &mp, nil
}

func ListProjects(db *gorm.DB) ([]models.Project, error) {
	var mps []models.Project
	if err := db.Order("updated_at DESC").Find(&mps).Error; err != nil {
		return nil, fmt.Errorf("查询 projects 列表失败: %w", err)
	}
	return mps, nil
}

func UpdateProject(db *gorm.DB, p *models.Project) error {
	result := db.Model(&models.Project{}).Where("project_id = ?", p.ProjectId).Updates(p)
	if result.Error != nil {
		return fmt.Errorf("更新 project 失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("project project_id=%s 不存在", p.ProjectId)
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
	if req.ProjectAncientReasonManual != nil {
		updates["project_ancient_minutes_reason_manual"] = *req.ProjectAncientReasonManual
	}
	if req.ProjectRealProcessMinutesManual != nil {
		updates["project_real_process_minutes_manual"] = *req.ProjectRealProcessMinutesManual
	}
	if req.ProjectRealProcessReasonManual != nil {
		updates["project_real_process_minutes_reason_manual"] = *req.ProjectRealProcessReasonManual
	}
	if req.ProjectRealLeadMinutesManual != nil {
		updates["project_real_lead_minutes_manual"] = *req.ProjectRealLeadMinutesManual
	}
	if req.ProjectRealLeadReasonManual != nil {
		updates["project_real_lead_minutes_reason_manual"] = *req.ProjectRealLeadReasonManual
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
		"project_ancient_minutes_reason":      agg.ProjectAncientReason,
		"project_real_process_minutes":        agg.ProjectRealProcessMinutes,
		"project_real_process_minutes_reason": agg.ProjectRealProcessReason,
		"project_real_lead_minutes":           agg.ProjectRealLeadMinutes,
		"project_real_lead_minutes_reason":    agg.ProjectRealLeadReason,
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

func ListUserProductivity(db *gorm.DB, filter UserFilter, page, pageSize int, orderClause string) ([]models.UserProductivity, int, error) {
	q := filter.applyToQuery(db.Model(&models.UserProductivity{}))
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return nil, 0, fmt.Errorf("统计 user_productivity 总数失败: %w", err)
	}
	if orderClause != "" {
		q = q.Order(orderClause)
	}
	if pageSize > 0 {
		q = q.Limit(pageSize).Offset((page - 1) * pageSize)
	}
	var ups []models.UserProductivity
	if err := q.Find(&ups).Error; err != nil {
		return nil, int(count), fmt.Errorf("查询 user_productivity 列表失败: %w", err)
	}
	return ups, int(count), nil
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

	var list []CommitLightStats
	if err := q.Scan(&list).Error; err != nil {
		return nil, fmt.Errorf("轻量查询 commits 失败: %w", err)
	}
	return list, nil
}

// ============================================================
// 仪表盘聚合查询
// ============================================================

type dashboardTaskAgg struct {
	TotalTasks          int
	TotalUsers          int
	TotalWorkDirs       int
	TotalCost           float64
	TotalTokens         int64
	TotalLines          int64
	TotalAiDays         float64
	TotalRealMinutes    float64
	TotalAncientMinutes float64
	AvgEfficiencyRatio  float64
}

func queryDashboardTaskAgg(db *gorm.DB, startTime, endTime string) (*dashboardTaskAgg, error) {
	var agg dashboardTaskAgg
	q := db.Model(&models.Task{}).
		Select(`COUNT(task_id) as total_tasks,
			COUNT(DISTINCT user_id) as total_users,
			COUNT(DISTINCT work_dir_id) as total_work_dirs,
			COALESCE(SUM(cost), 0) as total_cost,
			COALESCE(SUM(diff_lines), 0) as total_lines,
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
	TotalRepos          int
	TotalCommits        int
	TotalDiffLines      int64
	TotalBranchs        int
	TotalRealMinutes    float64
	TotalAncientMinutes float64
}

func queryDashboardCommitAgg(db *gorm.DB, startTime, endTime string) (*dashboardCommitAgg, error) {
	var agg dashboardCommitAgg
	q := db.Model(&models.Commit{}).Select(`COUNT(*) as total_commits,
		COALESCE(SUM(diff_lines), 0) as total_diff_lines,
		COUNT(DISTINCT NULLIF(repo_addr, '')) as total_repos,
		(SELECT COUNT(*) FROM (SELECT DISTINCT repo_addr, repo_branch FROM commits WHERE repo_addr IS NOT NULL AND repo_addr != '') sub) as total_branchs,
		COALESCE(SUM(CASE WHEN commit_real_minutes_manual IS NOT NULL THEN commit_real_minutes_manual ELSE commit_real_minutes END), 0) as total_real_minutes,
		COALESCE(SUM(CASE WHEN commit_ancient_minutes_manual IS NOT NULL THEN commit_ancient_minutes_manual ELSE commit_ancient_minutes END), 0) as total_ancient_minutes`)
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

// ============================================================
// user_productivity 聚合查询 (用于 user list / org list)
// ============================================================

type userProdAggRow struct {
	UserId               string
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
	UserId               string
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
	if err := q.Order("create_time DESC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ============================================================
// user_org 查询
// ============================================================

func LoadUserOrgs(db *gorm.DB) (map[string]*models.UserOrg, error) {
	var uos []models.UserOrg
	if err := db.Find(&uos).Error; err != nil {
		return nil, fmt.Errorf("查询 user_org 表失败: %w", err)
	}
	result := make(map[string]*models.UserOrg, len(uos))
	for i := range uos {
		if uos[i].UserId == "" {
			continue
		}
		result[uos[i].UserId] = &uos[i]
	}
	return result, nil
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

func QueryUserProdAgg(db *gorm.DB, startTime, endTime string) ([]userProdAggRow, error) {
	return queryUserProdAgg(db, startTime, endTime)
}

func QueryUserProdAggForIDs(db *gorm.DB, userIDs []string, startTime, endTime string) ([]userProdAggRow, error) {
	return queryUserProdAggForIDs(db, userIDs, startTime, endTime)
}

func QueryUserProdTimeSeries(db *gorm.DB, userIDs []string, startTime, endTime string) ([]userProdTimeSeriesRow, error) {
	return queryUserProdTimeSeries(db, userIDs, startTime, endTime)
}
