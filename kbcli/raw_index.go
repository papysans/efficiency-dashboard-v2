package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"kanban/core/models"
	"kanban/core/rawdump"
	"kanban/core/storage"
	"kanban/kbcli/internal/appconfig"
	"kanban/kbcli/internal/logx"
	"kanban/kbcli/internal/util"

	"gorm.io/gorm"
)

type rawIndexTaskConversationRow struct {
	TaskID        string    `gorm:"column:task_id"`
	S3Prefix      string    `gorm:"column:s3_prefix"`
	TotalChunks   int64     `gorm:"column:total_chunks"`
	TotalFileSize int64     `gorm:"column:total_file_size"`
	CreatedDate   time.Time `gorm:"column:created_date"`
}

type rawIndexTaskSummaryRow struct {
	TaskID      string    `gorm:"column:task_id"`
	S3Key       string    `gorm:"column:s3_key"`
	FileSize    int64     `gorm:"column:file_size"`
	CreatedDate time.Time `gorm:"column:created_date"`
}

type rawIndexCommitRow struct {
	S3Key       string    `gorm:"column:s3_key"`
	RepoAddr    string    `gorm:"column:repo_addr"`
	RepoBranch  string    `gorm:"column:repo_branch"`
	CommitID    string    `gorm:"column:commit_id"`
	CreatedDate time.Time `gorm:"column:created_date"`
}

func rawIndexEnabled() bool {
	return appconfig.Cfg != nil && appconfig.Cfg.RawIndex.Enabled
}

func openRawIndexDB() (*gorm.DB, func(), error) {
	if appconfig.Cfg == nil || !appconfig.Cfg.RawIndex.Enabled {
		return nil, func() {}, nil
	}
	if strings.TrimSpace(appconfig.Cfg.RawIndex.DSN) == "" {
		return nil, nil, fmt.Errorf("raw_index.enabled=true 但 raw_index.dsn 为空")
	}
	if strings.TrimSpace(appconfig.Cfg.RawIndex.S3Base) == "" {
		return nil, nil, fmt.Errorf("raw_index.enabled=true 但 raw_index.s3_base 为空")
	}
	if !storage.IsS3(appconfig.Cfg.RawIndex.S3Base) {
		return nil, nil, fmt.Errorf("raw_index.s3_base 必须是 s3:// 路径: %s", appconfig.Cfg.RawIndex.S3Base)
	}
	db, err := models.OpenGormDB(appconfig.Cfg.RawIndex.DSN)
	if err != nil {
		return nil, nil, fmt.Errorf("连接 raw_index 数据库失败: %w", err)
	}
	closeFn := func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	}
	return db, closeFn, nil
}

func rawIndexObjectLoc(s3Key string) string {
	base := strings.TrimRight(appconfig.Cfg.RawIndex.S3Base, "/")
	key := strings.TrimLeft(strings.TrimSpace(s3Key), "/")
	return storage.Join(base, key)
}

func rawIndexApplyDateRange(q *gorm.DB, startDate, endDate *time.Time) *gorm.DB {
	if startDate != nil {
		q = q.Where("created_date >= ?", startDate.Format("2006-01-02"))
	}
	if endDate != nil {
		q = q.Where("created_date < ?", endDate.Format("2006-01-02"))
	}
	return q
}

func scanConversationFilesFromRawIndex(rawDB *gorm.DB, startDate, endDate *time.Time) (map[string]convSource, error) {
	var rows []rawIndexTaskConversationRow
	q := rawDB.Table("s3_task_conversation_index").
		Select("task_id, s3_prefix, total_chunks, total_file_size, created_date")
	q = rawIndexApplyDateRange(q, startDate, endDate)
	if err := q.Order("created_date ASC, task_id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("查询 s3_task_conversation_index 失败: %w", err)
	}

	groups := make(map[string]convSource)
	for _, row := range rows {
		if row.TaskID == "" || row.S3Prefix == "" {
			continue
		}
		date := row.CreatedDate.Format("2006/01/02")
		paths := rawIndexConversationPaths(row.S3Prefix, row.TotalChunks)
		if len(paths) == 0 {
			continue
		}
		if existing, ok := groups[row.TaskID]; ok && existing.date >= date {
			continue
		}
		ref := rawdump.ConversationRef{SessionID: row.TaskID, Paths: paths}
		groups[row.TaskID] = convSource{ref: ref, date: date}
	}
	return groups, nil
}

func rawIndexConversationPaths(s3Prefix string, totalChunks int64) []string {
	if totalChunks <= 0 {
		return []string{rawIndexObjectLoc(strings.TrimSuffix(s3Prefix, "/") + ".jsonl")}
	}
	paths := make([]string, 0, totalChunks)
	for i := int64(1); i <= totalChunks; i++ {
		paths = append(paths, rawIndexObjectLoc(fmt.Sprintf("%s/%06d.jsonl", strings.TrimSuffix(s3Prefix, "/"), i)))
	}
	return paths
}

func scanSessionFilesFromRawIndex(rawDB *gorm.DB, convMap map[string]convSource) (map[string]string, error) {
	sessionMap := make(map[string]string)
	if len(convMap) == 0 {
		return sessionMap, nil
	}
	taskIDs := make([]string, 0, len(convMap))
	for taskID := range convMap {
		taskIDs = append(taskIDs, taskID)
	}
	sort.Strings(taskIDs)

	const batchSize = 1000
	for start := 0; start < len(taskIDs); start += batchSize {
		end := start + batchSize
		if end > len(taskIDs) {
			end = len(taskIDs)
		}
		var rows []rawIndexTaskSummaryRow
		if err := rawDB.Raw(`
			SELECT DISTINCT ON (task_id)
				task_id, s3_key, file_size, created_date
			FROM s3_task_summary_index
			WHERE task_id IN ?
			ORDER BY task_id, created_date DESC, id DESC
		`, taskIDs[start:end]).Scan(&rows).Error; err != nil {
			return nil, fmt.Errorf("查询 s3_task_summary_index 失败: %w", err)
		}
		for _, row := range rows {
			if row.TaskID == "" || row.S3Key == "" {
				continue
			}
			sessionMap[row.TaskID] = rawIndexObjectLoc(row.S3Key)
		}
	}
	return sessionMap, nil
}

func scanRepoDirFromRawIndex(rawDB *gorm.DB, statDB *gorm.DB, force bool, startDate, endDate *time.Time) ([]repoFileMeta, int, error) {
	var rows []rawIndexCommitRow
	q := rawDB.Table("s3_commit_index").
		Select("s3_key, repo_addr, repo_branch, commit_id, created_date")
	q = rawIndexApplyDateRange(q, startDate, endDate)
	if err := q.Order("created_date ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("查询 s3_commit_index 失败: %w", err)
	}

	files := make([]repoFileMeta, 0, len(rows))
	skipCount := 0
	for _, row := range rows {
		if row.S3Key == "" || row.CommitID == "" {
			continue
		}
		if !util.IsActiveTimeInRange(row.CreatedDate, startDate, endDate) {
			skipCount++
			continue
		}
		if !force {
			var count int64
			if err := statDB.Model(&models.Commit{}).Where("commit_id = ?", row.CommitID).Count(&count).Error; err == nil && count > 0 {
				logx.Debugf("跳过(已存在): %s", row.S3Key)
				skipCount++
				continue
			}
		}

		meta := rawIndexCommitMeta(row)
		files = append(files, meta)
	}
	return files, skipCount, nil
}

func rawIndexCommitMeta(row rawIndexCommitRow) repoFileMeta {
	meta := repoFileMeta{
		Path:     rawIndexObjectLoc(row.S3Key),
		RelPath:  strings.TrimPrefix(strings.TrimLeft(row.S3Key, "/"), "repo/"),
		Repo:     row.RepoAddr,
		Branch:   row.RepoBranch,
		CommitId: row.CommitID,
	}
	relPath := filepath.ToSlash(meta.RelPath)
	if matches := reRepoPath.FindStringSubmatch(relPath); matches != nil {
		meta.Repo = matches[1]
		meta.Branch = matches[2]
		meta.Year = matches[3]
		meta.Month = matches[4]
		meta.Day = matches[5]
		meta.CommitId = matches[6]
		return meta
	}
	meta.Year = row.CreatedDate.Format("2006")
	meta.Month = row.CreatedDate.Format("01")
	meta.Day = row.CreatedDate.Format("02")
	return meta
}
