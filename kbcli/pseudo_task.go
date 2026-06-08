package main

import (
	"fmt"
	"kanban/core/models"
	"kanban/core/utils"
	"kanban/kbcli/internal/logx"
	"sort"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// createPseudoTasks 为所有 Session 创建伪任务。
// 伪任务不关联 Commit，但关联该 Session 的所有 Conversation。
// 用于统计那些没有 Commit 关联的 Session 的工作量。
func createPseudoTasks(db *gorm.DB) error {
	var sessions []models.Session
	if err := db.Find(&sessions).Error; err != nil {
		return fmt.Errorf("查询sessions失败: %w", err)
	}

	logx.Infof("开始为 %d 个session创建伪任务", len(sessions))

	var successCount, failCount int
	for _, session := range sessions {
		var conversations []models.Conversation
		if err := db.Where("session_id = ?", session.SessionId).Order("start_time ASC").Find(&conversations).Error; err != nil {
			logx.Warnf("查询session[%s]的conversations失败: %v", session.SessionId, err)
			failCount++
			continue
		}

		if len(conversations) == 0 {
			continue
		}

		task := buildPseudoTask(session, conversations)

		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "task_id"}},
			DoUpdates: clause.AssignmentColumns(pseudoTaskUpdateColumns()),
		}).Create(&task).Error; err != nil {
			logx.Warnf("保存伪任务[%s]失败: %v", task.TaskId, err)
			failCount++
			continue
		}

		if err := db.Model(&models.Conversation{}).
			Where("session_id = ?", session.SessionId).
			Update("task_id", task.TaskId).Error; err != nil {
			logx.Warnf("更新conversation的task_id失败 [%s]: %v", task.TaskId, err)
		}

		successCount++
	}

	logx.Infof("伪任务创建完成: 成功 %d 个，失败 %d 个", successCount, failCount)
	return nil
}

func buildPseudoTask(session models.Session, conversations []models.Conversation) models.Task {
	sort.Slice(conversations, func(i, j int) bool {
		return conversations[i].StartTime.Before(conversations[j].StartTime)
	})

	firstConv := conversations[0]
	lastConv := conversations[len(conversations)-1]

	var upstreamTokens int64
	var downstreamTokens int64
	var cost float64
	var diffLines int64
	var totalInchars int
	var validTimes []time.Time

	for _, c := range conversations {
		upstreamTokens += c.UpstreamTokens
		downstreamTokens += c.DownstreamTokens
		cost += c.Cost
		diffLines += c.DiffLines
		totalInchars += len(c.UserInput)
		if !c.StartTime.IsZero() {
			validTimes = append(validTimes, c.StartTime)
		}
	}

	realMinutes, realReason := calcTaskRealMinutes(validTimes, cfg.TaskStatistics.GapThresholdMinutes, cfg.TaskStatistics.ExtensionMinutes)
	ancientMinutes, ancientReason := estimateTaskAncientMinutes(&cfg.AlgoEstimation, float64(totalInchars), float64(diffLines), realMinutes)

	workDirID := ""
	if firstConv.WorkDir != "" {
		workDirID = utils.GenerateWorkDirID(session.ClientId, firstConv.WorkDir)
	}

	task := models.Task{
		TaskId:             session.SessionId,
		CommitId:           "",
		SessionId:          session.SessionId,
		UserId:             session.UserId,
		UserName:           session.UserName,
		ClientId:           session.ClientId,
		ClientIde:          session.ClientIde,
		ClientVersion:      session.ClientVersion,
		ClientOs:           session.ClientOs,
		ClientOsVersion:    session.ClientOsVersion,
		Caller:             firstConv.Sender,
		RepoAddr:           firstConv.RepoAddr,
		RepoBranch:         firstConv.RepoBranch,
		WorkDir:            firstConv.WorkDir,
		WorkDirId:          workDirID,
		StartTime:          firstConv.StartTime,
		EndTime:            lastConv.EndTime,
		DiffLines:          int(diffLines),
		Silica:             0,
		AcceptRatio:        1,
		UpstreamTokens:     upstreamTokens,
		DownstreamTokens:   downstreamTokens,
		Cost:               cost,
		TaskRealMinutes:    realMinutes,
		TaskRealReason:     realReason,
		TaskAncientMinutes: ancientMinutes,
		TaskAncientReason:  ancientReason,
		SessionDate:        session.SessionDate,
		ConversationDate:   session.ConversationDate,
	}

	return task
}

func pseudoTaskUpdateColumns() []string {
	return []string{
		"commit_id", "session_id", "user_id", "user_name",
		"client_id", "client_ide", "client_version", "client_os", "client_os_version",
		"caller", "repo_addr", "repo_branch", "work_dir", "work_dir_id",
		"start_time", "end_time", "diff_lines", "silica", "accept_ratio",
		"upstream_tokens", "downstream_tokens", "cost",
		"task_real_minutes", "task_real_reason",
		"task_real_minutes_manual", "task_real_reason_manual",
		"task_ancient_minutes", "task_ancient_reason",
		"task_ancient_minutes_manual", "task_ancient_reason_manual",
		"title", "session_date", "conversation_date", "updated_at",
	}
}
