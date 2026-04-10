package main

import (
	"encoding/json"
	"math"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
)

// rebuildUserProductivity POST /api/v2/user-productivity/rebuild
func rebuildUserProductivity(c *gin.Context) {
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	if startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "startDate 和 endDate 为必填参数"})
		return
	}

	startT, err := parseDateParam(startDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "startDate 格式错误: " + err.Error()})
		return
	}
	endT, err := parseDateParam(endDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "endDate 格式错误: " + err.Error()})
		return
	}

	startTime := startT.Format(time.RFC3339)
	endTime := endT.Add(23*time.Hour + 59*time.Minute + 59*time.Second).Format(time.RFC3339)

	// 清理旧数据
	if err := DeleteUserProductivityByDate(statDB, startTime, endTime); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "清理旧数据失败: " + err.Error()})
		return
	}

	// 从 tasks 表聚合
	taskQuery := `SELECT user_id, DATE(start_time) as day,
		COALESCE(MAX(user_name), '') as user_name,
		array_agg(task_id) as task_ids,
		array_agg(DISTINCT work_dir_id) FILTER (WHERE work_dir_id IS NOT NULL) as work_dir_ids,
		COALESCE(SUM(diff_lines), 0) as task_diff_lines,
		COALESCE(SUM(upstream_tokens), 0) as upstream_tokens,
		COALESCE(SUM(downstream_tokens), 0) as downstream_tokens,
		COALESCE(SUM(cost), 0) as cost,
		COALESCE(SUM(task_real_minutes), 0) as task_real_minutes,
		COALESCE(SUM(task_ancient_minutes), 0) as task_ancient_minutes
		FROM tasks
		WHERE start_time >= $1 AND start_time <= $2
		GROUP BY user_id, DATE(start_time)`

	taskRows, err := statDB.Query(taskQuery, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询 tasks 聚合失败: " + err.Error()})
		return
	}
	defer taskRows.Close()

	mergeMap := make(map[string]*UserProductivity)
	for taskRows.Next() {
		var uid, userName string
		var day time.Time
		var taskIDs, workDirIDs []string
		var taskDiffLines int
		var upTokens, downTokens int64
		var cost, taskRealMin, taskAncientMin float64

		if err := taskRows.Scan(&uid, &day, &userName,
			pq.Array(&taskIDs), pq.Array(&workDirIDs),
			&taskDiffLines, &upTokens, &downTokens, &cost,
			&taskRealMin, &taskAncientMin); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "扫描 tasks 聚合行失败: " + err.Error()})
			return
		}

		key := uid + "_" + day.Format("20060102")
		taskIDsJSON, _ := json.Marshal(taskIDs)
		workDirIDsJSON, _ := json.Marshal(workDirIDs)

		mergeMap[key] = &UserProductivity{
			UserProductivityID: key,
			CreateTime:         ptrTime(day),
			UserID:             ptrString(uid),
			UserName:           ptrString(userName),
			TaskIDs:            json.RawMessage(taskIDsJSON),
			WorkDirIDs:         json.RawMessage(workDirIDsJSON),
			TaskDiffLines:      ptrInt(taskDiffLines),
			UpstreamTokens:     ptrInt64(upTokens),
			DownstreamTokens:   ptrInt64(downTokens),
			Cost:               ptrFloat64(cost),
			TaskRealMinutes:    ptrFloat64(taskRealMin),
			TaskAncientMinutes: ptrFloat64(taskAncientMin),
		}
	}
	if err := taskRows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "遍历 tasks 聚合结果失败: " + err.Error()})
		return
	}

	// 从 commits 表聚合
	commitQuery := `SELECT user_id, DATE(commit_time) as day,
		array_agg(commit_id) as commit_ids,
		COALESCE(SUM(diff_lines), 0) as commit_diff_lines,
		COALESCE(SUM(commit_ancient_minutes), 0) as commit_ancient_minutes,
		COALESCE(SUM(commit_real_ai_minutes), 0) as commit_real_ai_minutes,
		COALESCE(SUM(commit_real_ancient_minutes), 0) as commit_real_ancient_minutes,
		COALESCE(SUM(commit_real_minutes), 0) as commit_real_minutes
		FROM commits
		WHERE commit_time >= $1 AND commit_time <= $2
		GROUP BY user_id, DATE(commit_time)`

	commitRows, err := statDB.Query(commitQuery, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询 commits 聚合失败: " + err.Error()})
		return
	}
	defer commitRows.Close()

	for commitRows.Next() {
		var uid string
		var day time.Time
		var commitIDs []string
		var commitDiffLines int
		var commitAncientMin, commitRealAIMin, commitRealAncientMin, commitRealMin float64

		if err := commitRows.Scan(&uid, &day,
			pq.Array(&commitIDs),
			&commitDiffLines, &commitAncientMin, &commitRealAIMin,
			&commitRealAncientMin, &commitRealMin); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "扫描 commits 聚合行失败: " + err.Error()})
			return
		}

		key := uid + "_" + day.Format("20060102")
		commitIDsJSON, _ := json.Marshal(commitIDs)

		if rec, ok := mergeMap[key]; ok {
			rec.CommitIDs = json.RawMessage(commitIDsJSON)
			rec.CommitDiffLines = ptrInt(commitDiffLines)
			rec.CommitAncientMinutes = ptrFloat64(commitAncientMin)
			rec.CommitRealAIMinutes = ptrFloat64(commitRealAIMin)
			rec.CommitRealAncientMinutes = ptrFloat64(commitRealAncientMin)
			rec.CommitRealMinutes = ptrFloat64(commitRealMin)
		} else {
			mergeMap[key] = &UserProductivity{
				UserProductivityID:       key,
				CreateTime:               ptrTime(day),
				UserID:                   ptrString(uid),
				CommitIDs:                json.RawMessage(commitIDsJSON),
				CommitDiffLines:          ptrInt(commitDiffLines),
				CommitAncientMinutes:     ptrFloat64(commitAncientMin),
				CommitRealAIMinutes:      ptrFloat64(commitRealAIMin),
				CommitRealAncientMinutes: ptrFloat64(commitRealAncientMin),
				CommitRealMinutes:        ptrFloat64(commitRealMin),
			}
		}
	}
	if err := commitRows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "遍历 commits 聚合结果失败: " + err.Error()})
		return
	}

	// 计算效率比
	for _, rec := range mergeMap {
		if rec.TaskRealMinutes != nil && *rec.TaskRealMinutes > 0 && rec.TaskAncientMinutes != nil {
			rec.TaskEfficiencyRatio = ptrFloat64(math.Round(*rec.TaskAncientMinutes / *rec.TaskRealMinutes * 100))
		}
		if rec.CommitRealMinutes != nil && *rec.CommitRealMinutes > 0 && rec.CommitAncientMinutes != nil {
			rec.CommitEfficiencyRatio = ptrFloat64(math.Round(*rec.CommitAncientMinutes / *rec.CommitRealMinutes * 100))
		}
	}

	// 在事务中批量写入
	tx, err := statDB.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "开启事务失败: " + err.Error()})
		return
	}
	for _, rec := range mergeMap {
		_, err := tx.Exec(`
			INSERT INTO user_productivity (
				user_productivity_id, create_time, user_id, user_name,
				task_ids, work_dir_ids, task_diff_lines,
				upstream_tokens, downstream_tokens, cost,
				task_real_minutes, task_ancient_minutes, task_efficiency_ratio,
				commit_ids, commit_diff_lines,
				commit_ancient_minutes, commit_real_ai_minutes, commit_real_ancient_minutes,
				commit_real_minutes, commit_efficiency_ratio
			) VALUES (
				$1, $2, $3, $4,
				$5, $6, $7,
				$8, $9, $10,
				$11, $12, $13,
				$14, $15,
				$16, $17, $18,
				$19, $20
			)
			ON CONFLICT(user_productivity_id) DO UPDATE SET
				create_time = $2, user_id = $3, user_name = $4,
				task_ids = $5, work_dir_ids = $6, task_diff_lines = $7,
				upstream_tokens = $8, downstream_tokens = $9, cost = $10,
				task_real_minutes = $11, task_ancient_minutes = $12, task_efficiency_ratio = $13,
				commit_ids = $14, commit_diff_lines = $15,
				commit_ancient_minutes = $16, commit_real_ai_minutes = $17, commit_real_ancient_minutes = $18,
				commit_real_minutes = $19, commit_efficiency_ratio = $20,
				updated_at = CURRENT_TIMESTAMP`,
			rec.UserProductivityID, rec.CreateTime, rec.UserID, rec.UserName,
			jsonRawToString(rec.TaskIDs), jsonRawToString(rec.WorkDirIDs), rec.TaskDiffLines,
			rec.UpstreamTokens, rec.DownstreamTokens, rec.Cost,
			rec.TaskRealMinutes, rec.TaskAncientMinutes, rec.TaskEfficiencyRatio,
			jsonRawToString(rec.CommitIDs), rec.CommitDiffLines,
			rec.CommitAncientMinutes, rec.CommitRealAIMinutes, rec.CommitRealAncientMinutes,
			rec.CommitRealMinutes, rec.CommitEfficiencyRatio,
		)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "写入 user_productivity 失败: " + err.Error()})
			return
		}
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "提交事务失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "count": len(mergeMap)})
}
