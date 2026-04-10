package main

import (
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// createUserGroupHandler POST /api/v2/user-groups
func createUserGroupHandler(c *gin.Context) {
	var req struct {
		Name    string   `json:"name"`
		OrgName string   `json:"org_name"`
		UserIDs []string `json:"user_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体解析失败: " + err.Error()})
		return
	}
	if req.Name == "" || len(req.UserIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name 和 user_ids 为必填参数"})
		return
	}

	group, err := CreateUserGroup(statDB, req.Name, req.OrgName, req.UserIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建用户组失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, group)
}

// listUserGroupsHandler GET /api/v2/user-groups
func listUserGroupsHandler(c *gin.Context) {
	groups, err := ListUserGroups(statDB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询用户组列表失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": groups})
}

// deleteUserGroupHandler DELETE /api/v2/user-groups/:groupId
func deleteUserGroupHandler(c *gin.Context) {
	groupId := c.Param("groupId")

	err := DeleteUserGroup(statDB, groupId)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除用户组失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// getUserGroupDetailHandler GET /api/v2/user-groups/:groupId
func getUserGroupDetailHandler(c *gin.Context) {
	groupId := c.Param("groupId")

	group, err := GetUserGroup(statDB, groupId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询用户组失败: " + err.Error()})
		return
	}
	if group == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户组不存在"})
		return
	}

	startDate := c.Query("startDate")
	endDate := c.Query("endDate")

	var startTime, endTime string
	if startDate != "" {
		startT, err := parseDateParam(startDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "startDate 格式错误: " + err.Error()})
			return
		}
		startTime = startT.Format(time.RFC3339)
	}
	if endDate != "" {
		endT, err := parseDateParam(endDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "endDate 格式错误: " + err.Error()})
			return
		}
		endTime = endT.Add(23*time.Hour + 59*time.Minute + 59*time.Second).Format(time.RFC3339)
	}

	var userIDs []string
	if err := json.Unmarshal(group.UserIDs, &userIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析用户组 user_ids 失败: " + err.Error()})
		return
	}

	// 组级汇总
	var groupDayCount, groupTaskCount, groupCommitCount int
	var groupTaskDiffLines, groupCommitDiffLines int
	var groupUpTokens, groupDownTokens int64
	var groupCost, groupTaskRealMin, groupTaskAncientMin float64
	var groupCommitAncientMin, groupCommitRealMin float64

	members := make([]gin.H, 0, len(userIDs))
	for _, uid := range userIDs {
		daily, err := ListUserProductivity(statDB, uid, startTime, endTime, 1, 100000)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询用户产出失败: " + err.Error()})
			return
		}

		var dayCount, taskCount, commitCount int
		var taskDiffLines, commitDiffLines int
		var upTokens, downTokens int64
		var cost, taskRealMin, taskAncientMin float64
		var commitAncientMin, commitRealMin float64
		var userName string

		dayCount = len(daily)
		for _, d := range daily {
			if d.UserName != nil && userName == "" {
				userName = *d.UserName
			}
			if d.TaskIDs != nil {
				var ids []interface{}
				if json.Unmarshal(d.TaskIDs, &ids) == nil {
					taskCount += len(ids)
				}
			}
			if d.CommitIDs != nil {
				var ids []interface{}
				if json.Unmarshal(d.CommitIDs, &ids) == nil {
					commitCount += len(ids)
				}
			}
			if d.TaskDiffLines != nil {
				taskDiffLines += *d.TaskDiffLines
			}
			if d.UpstreamTokens != nil {
				upTokens += *d.UpstreamTokens
			}
			if d.DownstreamTokens != nil {
				downTokens += *d.DownstreamTokens
			}
			if d.Cost != nil {
				cost += *d.Cost
			}
			if d.TaskRealMinutes != nil {
				taskRealMin += *d.TaskRealMinutes
			}
			if d.TaskAncientMinutes != nil {
				taskAncientMin += *d.TaskAncientMinutes
			}
			if d.CommitDiffLines != nil {
				commitDiffLines += *d.CommitDiffLines
			}
			if d.CommitAncientMinutes != nil {
				commitAncientMin += *d.CommitAncientMinutes
			}
			if d.CommitRealMinutes != nil {
				commitRealMin += *d.CommitRealMinutes
			}
		}

		var taskEffRatio, commitEffRatio float64
		if taskRealMin > 0 {
			taskEffRatio = math.Round(taskAncientMin / taskRealMin * 100)
		}
		if commitRealMin > 0 {
			commitEffRatio = math.Round(commitAncientMin / commitRealMin * 100)
		}

		members = append(members, gin.H{
			"user_id":                 uid,
			"user_name":               userName,
			"day_count":               dayCount,
			"task_count":              taskCount,
			"commit_count":            commitCount,
			"task_diff_lines":         taskDiffLines,
			"upstream_tokens":         upTokens,
			"downstream_tokens":       downTokens,
			"cost":                    cost,
			"task_real_minutes":       taskRealMin,
			"task_ancient_minutes":    taskAncientMin,
			"task_efficiency_ratio":   taskEffRatio,
			"commit_diff_lines":       commitDiffLines,
			"commit_ancient_minutes":  commitAncientMin,
			"commit_real_minutes":     commitRealMin,
			"commit_efficiency_ratio": commitEffRatio,
		})

		// 累加到组级汇总
		groupDayCount += dayCount
		groupTaskCount += taskCount
		groupCommitCount += commitCount
		groupTaskDiffLines += taskDiffLines
		groupCommitDiffLines += commitDiffLines
		groupUpTokens += upTokens
		groupDownTokens += downTokens
		groupCost += cost
		groupTaskRealMin += taskRealMin
		groupTaskAncientMin += taskAncientMin
		groupCommitAncientMin += commitAncientMin
		groupCommitRealMin += commitRealMin
	}

	var groupTaskEffRatio, groupCommitEffRatio float64
	if groupTaskRealMin > 0 {
		groupTaskEffRatio = math.Round(groupTaskAncientMin / groupTaskRealMin * 100)
	}
	if groupCommitRealMin > 0 {
		groupCommitEffRatio = math.Round(groupCommitAncientMin / groupCommitRealMin * 100)
	}

	groupSummary := gin.H{
		"day_count":               groupDayCount,
		"task_count":              groupTaskCount,
		"commit_count":            groupCommitCount,
		"task_diff_lines":         groupTaskDiffLines,
		"upstream_tokens":         groupUpTokens,
		"downstream_tokens":       groupDownTokens,
		"cost":                    groupCost,
		"task_real_minutes":       groupTaskRealMin,
		"task_ancient_minutes":    groupTaskAncientMin,
		"task_efficiency_ratio":   groupTaskEffRatio,
		"commit_diff_lines":       groupCommitDiffLines,
		"commit_ancient_minutes":  groupCommitAncientMin,
		"commit_real_minutes":     groupCommitRealMin,
		"commit_efficiency_ratio": groupCommitEffRatio,
	}

	c.JSON(http.StatusOK, gin.H{
		"group":   group,
		"summary": groupSummary,
		"members": members,
	})
}
