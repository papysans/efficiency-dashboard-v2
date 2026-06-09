package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"kanban/core/models"
	"kanban/core/utils"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	CommitId                   string    `json:"commit_id"`
	CommitTime                 time.Time `json:"commit_time"`
	RepoAddr                   string    `json:"repo_addr"`
	RepoBranch                 string    `json:"repo_branch"`
	GitUserName                string    `json:"git_user_name"`
	GitUserEmail               string    `json:"git_user_email"`
	UserId                     string    `json:"user_id"`
	UserName                   string    `json:"user_name"`
	ClientId                   string    `json:"client_id"`
	WorkDir                    string    `json:"work_dir"`
	DiffLines                  int       `json:"diff_lines"`
	CommitAncientMinutes       float64   `json:"commit_ancient_minutes"`
	CommitAncientReason        string    `json:"commit_ancient_minutes_reason"`
	CommitAncientMinutesManual *float64  `json:"commit_ancient_minutes_manual"`
	CommitAncientReasonManual  string    `json:"commit_ancient_minutes_reason_manual"`
	CommitRealMinutes          float64   `json:"commit_real_minutes"`
	CommitRealReason           string    `json:"commit_real_minutes_reason"`
	CommitRealMinutesManual    *float64  `json:"commit_real_minutes_manual"`
	CommitRealReasonManual     string    `json:"commit_real_minutes_reason_manual"`
	CommitRealAiMinutes        float64   `json:"commit_real_ai_minutes"`
	CommitRealAncientMinutes   float64   `json:"commit_real_ancient_minutes"`
	Comment                    string    `json:"comment"`
	CreatedAt                  time.Time `json:"created_at"`
	UpdatedAt                  time.Time `json:"updated_at"`
	Cost                       float64   `json:"cost"`
	UpstreamTokens             int64     `json:"upstream_tokens"`
	DownstreamTokens           int64     `json:"downstream_tokens"`
	Silica                     float64   `json:"silica"`
	EfficiencyRatio            float64   `json:"efficiency_ratio"`
	Org1                       string    `json:"org1"`
	Org2                       string    `json:"org2"`
	Org3                       string    `json:"org3"`
	Org4                       string    `json:"org4"`
	Org5                       string    `json:"org5"`
	Org6                       string    `json:"org6"`
	Org7                       string    `json:"org7"`
	Org8                       string    `json:"org8"`
	Org9                       string    `json:"org9"`
	OrgDisplay                 string    `json:"org_display"`
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
	CommitId       string
	UserName       string
	DiffLines      int
	AncientMinutes float64
	RealMinutes    float64
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
	ancient := commit.CommitAncientMinutes
	real := commit.CommitRealMinutes
	if ancient <= 0 || real <= 0 {
		derivedAncient, derivedReal, err := deriveCommitWorkMinutes(statDB, commit.CommitId)
		if err != nil {
			log.Printf("派生 commit %s 工时失败: %v", commit.CommitId, err)
		}
		if ancient <= 0 && derivedAncient > 0 {
			ancient = derivedAncient
		}
		if real <= 0 && derivedReal > 0 {
			real = derivedReal
		}
	}
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
		CommitAncientMinutes:       ancient,
		CommitAncientReason:        commit.CommitAncientReason,
		CommitAncientMinutesManual: commit.CommitAncientMinutesManual,
		CommitAncientReasonManual:  commit.CommitAncientReasonManual,
		CommitRealMinutes:          real,
		CommitRealReason:           commit.CommitRealReason,
		CommitRealMinutesManual:    commit.CommitRealMinutesManual,
		CommitRealReasonManual:     commit.CommitRealReasonManual,
		CommitRealAiMinutes:        commit.CommitRealAiMinutes,
		CommitRealAncientMinutes:   commit.CommitRealNonAiMinutes,
		Comment:                    commit.Comment,
		CreatedAt:                  commit.CreatedAt,
		UpdatedAt:                  commit.UpdatedAt,
		Cost:                       commit.Cost,
		UpstreamTokens:             commit.UpstreamTokens,
		DownstreamTokens:           commit.DownstreamTokens,
		Silica:                     commit.Silica,
	}
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

func deriveCommitWorkMinutes(db *gorm.DB, commitID string) (float64, float64, error) {
	ancientMap, realMap, err := deriveCommitWorkMinutesBatch(db, []string{commitID})
	if err != nil {
		return 0, 0, err
	}
	return ancientMap[commitID], realMap[commitID], nil
}

// 提交间隔法（git-hours 思路）参数：相邻提交间隔上限 + 新会话首个提交默认耗时。
const (
	commitGapMaxMinutes       = 120.0
	commitFirstSessionMinutes = 30.0
)

func deriveCommitWorkMinutesBatch(db *gorm.DB, commitIDs []string) (map[string]float64, map[string]float64, error) {
	ancientByCommit := make(map[string]float64)
	realByCommit := make(map[string]float64)
	if len(commitIDs) == 0 {
		return ancientByCommit, realByCommit, nil
	}
	// 1) Need 归属（精确）：从包含该 commit 的 Need 把工作量按 commit 数平摊。
	// 注意：不能用 jsonb 的 `?|` 操作符——GORM 会把其中的 `?` 当成占位符导致 SQL 语法错误。
	// Need 数量很少（数十个），直接全量加载后在 Go 里按 commit 归属即可。
	var needs []models.Need
	if err := db.Select("commit_ids", "baseline_fused_work_min", "total_active_work_corrected_min").
		Find(&needs).Error; err != nil {
		return nil, nil, err
	}
	wanted := make(map[string]bool, len(commitIDs))
	for _, id := range commitIDs {
		wanted[id] = true
	}
	for _, need := range needs {
		needCommitIDs := efficiencyV2DecodeJSONStringSlice(need.CommitIds)
		if len(needCommitIDs) == 0 {
			continue
		}
		share := 1.0 / float64(len(needCommitIDs))
		var ancientShare float64
		if need.BaselineFusedWorkMin != nil {
			ancientShare = *need.BaselineFusedWorkMin * share
		}
		realShare := need.TotalActiveWorkCorrectedMin * share
		for _, id := range needCommitIDs {
			if !wanted[id] {
				continue
			}
			ancientByCommit[id] += ancientShare
			realByCommit[id] += realShare
		}
	}
	// 2) 提交间隔法兜底：Need 未覆盖到实际耗时的 commit，用同用户相邻提交间隔估算。
	missing := make([]string, 0, len(commitIDs))
	for _, id := range commitIDs {
		if realByCommit[id] <= 0 {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		gapReal, err := estimateCommitRealByGap(db, missing)
		if err != nil {
			return nil, nil, err
		}
		for id, v := range gapReal {
			if realByCommit[id] <= 0 && v > 0 {
				realByCommit[id] = v
			}
		}
	}
	return ancientByCommit, realByCommit, nil
}

// estimateCommitRealByGap 用「提交间隔法」估算实际耗时：同一用户的提交按时间排序，
// 相邻间隔 ≤ commitGapMaxMinutes 视为该提交的连续编码耗时；超过则视为新会话，
// 首个提交给 commitFirstSessionMinutes 默认值。仅依赖提交时间，可覆盖所有 commit。
func estimateCommitRealByGap(db *gorm.DB, commitIDs []string) (map[string]float64, error) {
	out := make(map[string]float64)
	type commitTime struct {
		CommitId   string
		UserId     string
		CommitTime time.Time
	}
	var heads []commitTime
	if err := db.Model(&models.Commit{}).Select("commit_id, user_id").
		Where("commit_id IN ?", commitIDs).Scan(&heads).Error; err != nil {
		return nil, err
	}
	userSet := make(map[string]bool)
	for _, h := range heads {
		if strings.TrimSpace(h.UserId) != "" {
			userSet[h.UserId] = true
		}
	}
	if len(userSet) == 0 {
		return out, nil
	}
	userList := make([]string, 0, len(userSet))
	for u := range userSet {
		userList = append(userList, u)
	}
	target := make(map[string]bool, len(commitIDs))
	for _, id := range commitIDs {
		target[id] = true
	}
	var all []commitTime
	if err := db.Model(&models.Commit{}).Select("commit_id, user_id, commit_time").
		Where("user_id IN ?", userList).
		Order("user_id, commit_time, commit_id").Scan(&all).Error; err != nil {
		return nil, err
	}
	var prevUser string
	var prevTime time.Time
	for _, c := range all {
		est := commitFirstSessionMinutes
		if c.UserId == prevUser {
			gap := c.CommitTime.Sub(prevTime).Minutes()
			if gap > 0 && gap <= commitGapMaxMinutes {
				est = gap
			}
		}
		if target[c.CommitId] {
			out[c.CommitId] = est
		}
		prevUser = c.UserId
		prevTime = c.CommitTime
	}
	return out, nil
}

func toCommitListItemSlice(commits []models.Commit) []CommitListItem {
	ids := make([]string, 0, len(commits))
	for i := range commits {
		if commits[i].CommitAncientMinutes <= 0 || commits[i].CommitRealMinutes <= 0 {
			ids = append(ids, commits[i].CommitId)
		}
	}
	derivedAncient, derivedReal, err := deriveCommitWorkMinutesBatch(statDB, ids)
	if err != nil {
		log.Printf("批量派生 commit 工时失败: %v", err)
	}
	result := make([]CommitListItem, len(commits))
	for i := range commits {
		if commits[i].CommitAncientMinutes <= 0 && derivedAncient[commits[i].CommitId] > 0 {
			commits[i].CommitAncientMinutes = derivedAncient[commits[i].CommitId]
		}
		if commits[i].CommitRealMinutes <= 0 && derivedReal[commits[i].CommitId] > 0 {
			commits[i].CommitRealMinutes = derivedReal[commits[i].CommitId]
		}
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

func ListRepoAggregates(db *gorm.DB, startTime, endTime string) ([]RepoAggregate, error) {
	var list []RepoAggregate

	q := db.Model(&models.Commit{}).
		Select(`commits.repo_addr, commits.repo_branch,
			COUNT(*) AS commit_count,
			MIN(commits.commit_time) AS start_time,
			MAX(commits.commit_time) AS end_time,
			SUM(commits.commit_ancient_minutes) AS sum_ancient_minutes,
			SUM(commits.commit_real_minutes) AS sum_real_minutes,
			COUNT(DISTINCT tasks.task_id) AS task_count`).
		Joins("LEFT JOIN tasks ON tasks.commit_id = commits.commit_id").
		Where("commits.repo_addr IS NOT NULL AND commits.repo_addr != ''")

	if startTime != "" {
		q = q.Where("commits.commit_time >= ?", startTime)
	}
	if endTime != "" {
		q = q.Where("commits.commit_time <= ?", endTime)
	}

	if err := q.Group("commits.repo_addr, commits.repo_branch").Order("commits.repo_addr, commits.repo_branch").Scan(&list).Error; err != nil {
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
// project_tasks CRUD (GORM)
// ============================================================

func AddProjectTasks(db *gorm.DB, projectID string, taskIDs []string, silicas []float64) error {
	if len(taskIDs) == 0 {
		return nil
	}
	pts := make([]models.ProjectTask, len(taskIDs))
	for i, tid := range taskIDs {
		s := 1.0
		if i < len(silicas) {
			s = silicas[i]
		}
		pts[i] = models.ProjectTask{
			ProjectId: projectID,
			TaskId:    tid,
			Silica:    s,
		}
	}
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "project_id"},
			{Name: "task_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"silica", "updated_at"}),
	}).Create(&pts).Error
}

func RemoveProjectTasks(db *gorm.DB, projectID string, taskIDs []string) error {
	if len(taskIDs) == 0 {
		return nil
	}
	return db.Where("project_id = ? AND task_id IN ?", projectID, taskIDs).Delete(&models.ProjectTask{}).Error
}

func UpdateProjectTaskSilica(db *gorm.DB, projectID, taskID string, silica float64) error {
	result := db.Model(&models.ProjectTask{}).Where("project_id = ? AND task_id = ?", projectID, taskID).Update("silica", silica)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("project_task (project_id=%s, task_id=%s) 不存在", projectID, taskID)
	}
	return nil
}

func ReplaceProjectTasks(db *gorm.DB, projectID string, taskIDs []string, silicas []float64) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("project_id = ?", projectID).Delete(&models.ProjectTask{}).Error; err != nil {
			return err
		}
		if len(taskIDs) == 0 {
			return nil
		}
		pts := make([]models.ProjectTask, len(taskIDs))
		for i, tid := range taskIDs {
			s := 1.0
			if i < len(silicas) {
				s = silicas[i]
			}
			pts[i] = models.ProjectTask{
				ProjectId: projectID,
				TaskId:    tid,
				Silica:    s,
			}
		}
		return tx.Create(&pts).Error
	})
}

func ListProjectTasks(db *gorm.DB, projectID string) ([]models.ProjectTask, error) {
	var pts []models.ProjectTask
	if err := db.Where("project_id = ?", projectID).Find(&pts).Error; err != nil {
		return nil, fmt.Errorf("查询 project_tasks 失败: %w", err)
	}
	return pts, nil
}

// ============================================================
// user_productivity CRUD (GORM)
// ============================================================

// ListUserProductivity 返回按 (user_id, 自然日) 聚合的生产力 daily 行。
//
// V1 user_productivity 预聚合表已下线，本函数改为从 tasks/commits 基表实时聚合，
// 口径与原 kbcli efficiency 写侧 (calculateUserProductivity) 完全一致：
//   - tasks 按 DATE(start_time)、commits 按 DATE(commit_time) 分日；
//   - real/ancient 分钟数 manual 值优先 (COALESCE(xxx_manual, xxx))；
//   - tokens/cost 仅来自 tasks（与 V1 一致）；
//   - 效能比 = utils.CalcEfficiencyRatio(ancient, real)；
//   - 用户名三级回退：user_org(orgMappings) > tasks > commits。
//
// 下游消费方（org/group/user-group 详情、project members）对 daily 行的用法不变，
// 故 API 输出契约保持不变。orderClause 现已无底表可排，按 (日期,user_id) 默认稳定排序。
func ListUserProductivity(db *gorm.DB, filter UserFilter, page, pageSize int, _ string) ([]models.UserProductivity, int, error) {
	// 复用 UserFilter 的 user_id 过滤口径（org 交集 + 显式 UserIds），见 applyToQuery。
	userIds := filter.UserIds
	if orgUserIds := filter.GetFilter(); orgUserIds != nil {
		if userIds == nil {
			userIds = orgUserIds
		} else {
			orgSet := make(map[string]bool, len(orgUserIds))
			for _, uid := range orgUserIds {
				orgSet[uid] = true
			}
			inter := make([]string, 0, len(userIds))
			for _, uid := range userIds {
				if orgSet[uid] {
					inter = append(inter, uid)
				}
			}
			userIds = inter
		}
	}
	if userIds != nil && len(userIds) == 0 { // 过滤后空集 → 无数据
		return []models.UserProductivity{}, 0, nil
	}

	type dayKey struct{ uid, day string }
	rowMap := make(map[dayKey]*models.UserProductivity)
	order := make([]dayKey, 0)
	getRow := func(uid string, day time.Time) *models.UserProductivity {
		dk := dayKey{uid: uid, day: day.Format("2006-01-02")}
		r := rowMap[dk]
		if r == nil {
			ct := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
			r = &models.UserProductivity{
				UserProductivityId: uid + "_" + ct.Format("20060102"),
				CreateTime:         ct,
				UserId:             uid,
			}
			rowMap[dk] = r
			order = append(order, dk)
		}
		return r
	}

	// ---- tasks 聚合（tokens/cost 仅此处计入）----
	type taskAggRow struct {
		UserId             string
		Day                time.Time
		TaskCount          int
		TaskDiffLines      int
		UpstreamTokens     int64
		DownstreamTokens   int64
		Cost               float64
		TaskRealMinutes    float64
		TaskAncientMinutes float64
		UserName           string
	}
	tq := db.Table("tasks").
		Select(`user_id,
			DATE(start_time) AS day,
			COUNT(*) AS task_count,
			COALESCE(SUM(diff_lines),0) AS task_diff_lines,
			COALESCE(SUM(upstream_tokens),0) AS upstream_tokens,
			COALESCE(SUM(downstream_tokens),0) AS downstream_tokens,
			COALESCE(SUM(cost),0) AS cost,
			COALESCE(SUM(COALESCE(task_real_minutes_manual, task_real_minutes)),0) AS task_real_minutes,
			COALESCE(SUM(COALESCE(task_ancient_minutes_manual, task_ancient_minutes)),0) AS task_ancient_minutes,
			COALESCE(MAX(user_name),'') AS user_name`).
		Where("user_id IS NOT NULL AND user_id <> ''").
		Group("user_id, DATE(start_time)")
	if len(userIds) > 0 {
		tq = tq.Where("user_id IN ?", userIds)
	}
	if filter.StartTime != "" {
		tq = tq.Where("DATE(start_time) >= DATE(?)", filter.StartTime)
	}
	if filter.EndTime != "" {
		tq = tq.Where("DATE(start_time) <= DATE(?)", filter.EndTime)
	}
	var taskRows []taskAggRow
	if err := tq.Scan(&taskRows).Error; err != nil {
		return nil, 0, fmt.Errorf("聚合 tasks 生产力失败: %w", err)
	}
	taskName := make(map[string]string)
	for _, tr := range taskRows {
		r := getRow(tr.UserId, tr.Day)
		r.TaskCount = tr.TaskCount
		r.TaskDiffLines = tr.TaskDiffLines
		r.UpstreamTokens = tr.UpstreamTokens
		r.DownstreamTokens = tr.DownstreamTokens
		r.Cost = tr.Cost
		r.TaskRealMinutes = tr.TaskRealMinutes
		r.TaskAncientMinutes = tr.TaskAncientMinutes
		if tr.UserName != "" {
			taskName[tr.UserId] = tr.UserName
		}
	}

	// ---- commits 聚合 ----
	type commitAggRow struct {
		UserId                 string
		Day                    time.Time
		CommitCount            int
		CommitDiffLines        int
		CommitAncientMinutes   float64
		CommitRealAiMinutes    float64
		CommitRealNonAiMinutes float64
		CommitRealMinutes      float64
		UserName               string
	}
	cq := db.Table("commits").
		Select(`user_id,
			DATE(commit_time) AS day,
			COUNT(*) AS commit_count,
			COALESCE(SUM(diff_lines),0) AS commit_diff_lines,
			COALESCE(SUM(COALESCE(commit_ancient_minutes_manual, commit_ancient_minutes)),0) AS commit_ancient_minutes,
			COALESCE(SUM(commit_real_ai_minutes),0) AS commit_real_ai_minutes,
			COALESCE(SUM(commit_real_non_ai_minutes),0) AS commit_real_non_ai_minutes,
			COALESCE(SUM(COALESCE(commit_real_minutes_manual, commit_real_minutes)),0) AS commit_real_minutes,
			COALESCE(MAX(COALESCE(NULLIF(user_name,''), git_user_name)),'') AS user_name`).
		Where("user_id IS NOT NULL AND user_id <> ''").
		Group("user_id, DATE(commit_time)")
	if len(userIds) > 0 {
		cq = cq.Where("user_id IN ?", userIds)
	}
	if filter.StartTime != "" {
		cq = cq.Where("DATE(commit_time) >= DATE(?)", filter.StartTime)
	}
	if filter.EndTime != "" {
		cq = cq.Where("DATE(commit_time) <= DATE(?)", filter.EndTime)
	}
	var commitRows []commitAggRow
	if err := cq.Scan(&commitRows).Error; err != nil {
		return nil, 0, fmt.Errorf("聚合 commits 生产力失败: %w", err)
	}
	commitName := make(map[string]string)
	for _, cr := range commitRows {
		r := getRow(cr.UserId, cr.Day)
		r.CommitCount = cr.CommitCount
		r.CommitDiffLines = cr.CommitDiffLines
		r.CommitAncientMinutes = cr.CommitAncientMinutes
		r.CommitRealAiMinutes = cr.CommitRealAiMinutes
		r.CommitRealNonAiMinutes = cr.CommitRealNonAiMinutes
		r.CommitRealMinutes = cr.CommitRealMinutes
		if cr.UserName != "" {
			commitName[cr.UserId] = cr.UserName
		}
	}

	// 用户名三级回退（user_org 优先）+ 效能比
	for _, r := range rowMap {
		name := taskName[r.UserId]
		if name == "" {
			name = commitName[r.UserId]
		}
		if om, ok := orgMappings[r.UserId]; ok && om.UserName != "" {
			name = om.UserName
		}
		r.UserName = name
		r.TaskEfficiencyRatio = utils.CalcEfficiencyRatio(r.TaskAncientMinutes, r.TaskRealMinutes)
		r.CommitEfficiencyRatio = utils.CalcEfficiencyRatio(r.CommitAncientMinutes, r.CommitRealMinutes)
	}

	sort.Slice(order, func(i, j int) bool {
		if order[i].day != order[j].day {
			return order[i].day < order[j].day
		}
		return order[i].uid < order[j].uid
	})
	all := make([]models.UserProductivity, 0, len(order))
	for _, dk := range order {
		all = append(all, *rowMap[dk])
	}

	count := len(all)
	if pageSize > 0 { // 调用方多传 page=1,pageSize 极大 → 等价全量
		start := (page - 1) * pageSize
		if start < 0 {
			start = 0
		}
		if start >= count {
			return []models.UserProductivity{}, count, nil
		}
		end := start + pageSize
		if end > count {
			end = count
		}
		all = all[start:end]
	}
	return all, count, nil
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
		Select("commit_id, COALESCE(user_name, git_user_name) AS user_name, diff_lines, commit_ancient_minutes AS ancient_minutes, commit_real_minutes AS real_minutes")

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
	ids := make([]string, 0, len(list))
	for i := range list {
		if list[i].AncientMinutes <= 0 || list[i].RealMinutes <= 0 {
			ids = append(ids, list[i].CommitId)
		}
	}
	derivedAncient, derivedReal, err := deriveCommitWorkMinutesBatch(db, ids)
	if err != nil {
		log.Printf("批量派生轻量 commit 工时失败: %v", err)
	}
	for i := range list {
		if list[i].AncientMinutes <= 0 {
			list[i].AncientMinutes = derivedAncient[list[i].CommitId]
		}
		if list[i].RealMinutes <= 0 {
			list[i].RealMinutes = derivedReal[list[i].CommitId]
		}
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
	TotalUsers          int
	TotalRealMinutes    float64
	TotalAncientMinutes float64
}

func queryDashboardCommitAgg(db *gorm.DB, startTime, endTime string) (*dashboardCommitAgg, error) {
	var agg dashboardCommitAgg
	q := db.Model(&models.Commit{}).Select(`COUNT(*) as total_commits,
		COALESCE(SUM(diff_lines), 0) as total_diff_lines,
		COUNT(DISTINCT NULLIF(user_id, '')) as total_users,
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

// dashboardNeedAgg 汇总 Need 维度（v2）指标，供首页综合提效展示。
// 提效相关只统计可计入且非异常的 Need，避免孤儿/异常样本污染。
type dashboardNeedAgg struct {
	TotalNeeds          int
	MergedNeeds         int
	EligibleNeeds       int
	ActualCalendarMin   float64
	BaselineCalendarMin float64
	ActualWorkMin       float64
	BaselineWorkMin     float64
	AICoveredLoc        int64
	TotalLocNet         int64
}

// applyNeedCaliberFilter 限定到看板口径：已交付(非 active) + 非主干分支。
// main/master/develop/release 等主干提交不形成 branch Need，是落到 cluster/orphan 兜底桶的
// 散落提交，与 "branch=Need" 口径不一致，列表与首页计数都按此口径收口。
func applyNeedCaliberFilter(q *gorm.DB) *gorm.DB {
	return q.Where("status <> ?", "active").
		Where("NOT (LOWER(TRIM(COALESCE(repo_branch,''))) IN ('main','master','develop','release') OR LOWER(TRIM(COALESCE(repo_branch,''))) LIKE 'release/%')")
}

func queryDashboardNeedAgg(db *gorm.DB, startTime, endTime string) (*dashboardNeedAgg, error) {
	var agg dashboardNeedAgg
	// 按口径分别剔除 outlier：日历 SUM 用 NOT calendar_outlier_flag、工作量 SUM 用 NOT
	// work_outlier_flag，与 kbcli 个人周表投影口径一致(design.md §4)。eligible_needs 取两侧
	// 均干净(NOT outlier_flag)作总览计数。
	q := applyNeedCaliberFilter(db.Model(&models.Need{})).Select(`COUNT(*) as total_needs,
		COUNT(*) FILTER (WHERE status = 'merged') as merged_needs,
		COUNT(*) FILTER (WHERE coverage_eligible AND NOT outlier_flag) as eligible_needs,
		COALESCE(SUM(total_calendar_min) FILTER (WHERE coverage_eligible AND NOT calendar_outlier_flag), 0) as actual_calendar_min,
		COALESCE(SUM(baseline_calendar_min) FILTER (WHERE coverage_eligible AND NOT calendar_outlier_flag), 0) as baseline_calendar_min,
		COALESCE(SUM(total_active_work_corrected_min) FILTER (WHERE coverage_eligible AND NOT work_outlier_flag), 0) as actual_work_min,
		COALESCE(SUM(baseline_fused_work_min) FILTER (WHERE coverage_eligible AND NOT work_outlier_flag), 0) as baseline_work_min,
		` + needAICodeAggSelect())
	if startTime != "" {
		q = q.Where("dev_end_ts >= ?", startTime)
	}
	if endTime != "" {
		q = q.Where("dev_end_ts <= ?", endTime)
	}
	if err := q.Scan(&agg).Error; err != nil {
		return nil, fmt.Errorf("查询 needs 仪表盘聚合失败: %w", err)
	}
	return &agg, nil
}

// needsDistributionAgg 一次 SQL 取出"提效分布"面板所需的全部标量：
// 计入/剔除计数、calendar/work 提效比分位数、提效比直方图(6桶×kept/excluded)、
// 剔除原因计数、LOC 速率分档。供 GET /api/v2/needs/distribution 直接组装返回。
// 全部按看板口径(applyNeedCaliberFilter)+ 日期窗口(dev_end_ts ∈ [start, end))统计。
type needsDistributionAgg struct {
	KeptCount      int64    `gorm:"column:kept_count"`
	ExcludedCount  int64    `gorm:"column:excluded_count"`
	CalendarMedian *float64 `gorm:"column:calendar_median"`
	CalendarP25    *float64 `gorm:"column:calendar_p25"`
	CalendarP75    *float64 `gorm:"column:calendar_p75"`
	WorkMedian     *float64 `gorm:"column:work_median"`

	H0Kept int64 `gorm:"column:h0_kept"`
	H0Excl int64 `gorm:"column:h0_excl"`
	H1Kept int64 `gorm:"column:h1_kept"`
	H1Excl int64 `gorm:"column:h1_excl"`
	H2Kept int64 `gorm:"column:h2_kept"`
	H2Excl int64 `gorm:"column:h2_excl"`
	H3Kept int64 `gorm:"column:h3_kept"`
	H3Excl int64 `gorm:"column:h3_excl"`
	H4Kept int64 `gorm:"column:h4_kept"`
	H4Excl int64 `gorm:"column:h4_excl"`
	H5Kept int64 `gorm:"column:h5_kept"`
	H5Excl int64 `gorm:"column:h5_excl"`

	ReasonLoc int64 `gorm:"column:reason_loc"`
	ReasonEff int64 `gorm:"column:reason_eff"`
	ReasonAtb int64 `gorm:"column:reason_atb"`

	Lb1 int64 `gorm:"column:lb1"`
	Lb2 int64 `gorm:"column:lb2"`
	Lb3 int64 `gorm:"column:lb3"`
	Lb4 int64 `gorm:"column:lb4"`
}

func queryNeedsDistributionAgg(db *gorm.DB, startTime, endTime string) (*needsDistributionAgg, error) {
	var agg needsDistributionAgg
	// calendar 提效分布(分位数/直方图/kept-excl)按日历口径剔除 → calendar_outlier_flag；
	// work_median 按工作量口径 → work_outlier_flag；reason_* 异常原因诊断计数保持 outlier_flag
	// (任一异常的原因分布)。详见 design.md §4。
	q := applyNeedCaliberFilter(db.Model(&models.Need{})).Select(`
		COUNT(*) FILTER (WHERE coverage_eligible AND NOT calendar_outlier_flag) AS kept_count,
		COUNT(*) FILTER (WHERE coverage_eligible AND calendar_outlier_flag)     AS excluded_count,
		PERCENTILE_CONT(0.5)  WITHIN GROUP (ORDER BY efficiency_ratio)      FILTER (WHERE coverage_eligible AND NOT calendar_outlier_flag) AS calendar_median,
		PERCENTILE_CONT(0.25) WITHIN GROUP (ORDER BY efficiency_ratio)      FILTER (WHERE coverage_eligible AND NOT calendar_outlier_flag) AS calendar_p25,
		PERCENTILE_CONT(0.75) WITHIN GROUP (ORDER BY efficiency_ratio)      FILTER (WHERE coverage_eligible AND NOT calendar_outlier_flag) AS calendar_p75,
		PERCENTILE_CONT(0.5)  WITHIN GROUP (ORDER BY work_efficiency_ratio) FILTER (WHERE coverage_eligible AND NOT work_outlier_flag) AS work_median,
		COUNT(*) FILTER (WHERE coverage_eligible AND NOT calendar_outlier_flag AND efficiency_ratio < 0)               AS h0_kept,
		COUNT(*) FILTER (WHERE coverage_eligible AND calendar_outlier_flag     AND efficiency_ratio < 0)               AS h0_excl,
		COUNT(*) FILTER (WHERE coverage_eligible AND NOT calendar_outlier_flag AND efficiency_ratio >= 0 AND efficiency_ratio < 0.5) AS h1_kept,
		COUNT(*) FILTER (WHERE coverage_eligible AND calendar_outlier_flag     AND efficiency_ratio >= 0 AND efficiency_ratio < 0.5) AS h1_excl,
		COUNT(*) FILTER (WHERE coverage_eligible AND NOT calendar_outlier_flag AND efficiency_ratio >= 0.5 AND efficiency_ratio < 1) AS h2_kept,
		COUNT(*) FILTER (WHERE coverage_eligible AND calendar_outlier_flag     AND efficiency_ratio >= 0.5 AND efficiency_ratio < 1) AS h2_excl,
		COUNT(*) FILTER (WHERE coverage_eligible AND NOT calendar_outlier_flag AND efficiency_ratio >= 1 AND efficiency_ratio < 2)   AS h3_kept,
		COUNT(*) FILTER (WHERE coverage_eligible AND calendar_outlier_flag     AND efficiency_ratio >= 1 AND efficiency_ratio < 2)   AS h3_excl,
		COUNT(*) FILTER (WHERE coverage_eligible AND NOT calendar_outlier_flag AND efficiency_ratio >= 2 AND efficiency_ratio < 5)   AS h4_kept,
		COUNT(*) FILTER (WHERE coverage_eligible AND calendar_outlier_flag     AND efficiency_ratio >= 2 AND efficiency_ratio < 5)   AS h4_excl,
		COUNT(*) FILTER (WHERE coverage_eligible AND NOT calendar_outlier_flag AND efficiency_ratio >= 5)              AS h5_kept,
		COUNT(*) FILTER (WHERE coverage_eligible AND calendar_outlier_flag     AND efficiency_ratio >= 5)              AS h5_excl,
		COUNT(*) FILTER (WHERE coverage_eligible AND outlier_flag AND reason LIKE '%impossible_loc_rate%') AS reason_loc,
		COUNT(*) FILTER (WHERE coverage_eligible AND outlier_flag AND reason LIKE '%efficiency_ratio%')     AS reason_eff,
		COUNT(*) FILTER (WHERE coverage_eligible AND outlier_flag AND reason LIKE '%actual_to_baseline%')   AS reason_atb,
		COUNT(*) FILTER (WHERE coverage_eligible AND total_calendar_min > 0 AND total_loc_net::float / NULLIF(total_calendar_min,0) <= 7)  AS lb1,
		COUNT(*) FILTER (WHERE coverage_eligible AND total_calendar_min > 0 AND total_loc_net::float / NULLIF(total_calendar_min,0) > 7  AND total_loc_net::float / NULLIF(total_calendar_min,0) <= 21) AS lb2,
		COUNT(*) FILTER (WHERE coverage_eligible AND total_calendar_min > 0 AND total_loc_net::float / NULLIF(total_calendar_min,0) > 21 AND total_loc_net::float / NULLIF(total_calendar_min,0) <= 50) AS lb3,
		COUNT(*) FILTER (WHERE coverage_eligible AND total_calendar_min > 0 AND total_loc_net::float / NULLIF(total_calendar_min,0) > 50) AS lb4`)
	if startTime != "" {
		q = q.Where("dev_end_ts >= ?", startTime)
	}
	if endTime != "" {
		q = q.Where("dev_end_ts <= ?", endTime)
	}
	if err := q.Scan(&agg).Error; err != nil {
		return nil, fmt.Errorf("查询 needs 提效分布聚合失败: %w", err)
	}
	return &agg, nil
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

func QueryDashboardNeedAgg(db *gorm.DB, startTime, endTime string) (*dashboardNeedAgg, error) {
	return queryDashboardNeedAgg(db, startTime, endTime)
}
