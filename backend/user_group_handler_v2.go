package main

import (
	"encoding/json"
	"kanban/core/utils"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type UserGroupMember struct {
	UserId                string  `json:"user_id"`
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

// createUserGroupHandler POST /api/v2/user-groups
// @Summary 创建用户组
// @Description 创建新的用户组
// @Tags UserGroups
// @Accept json
// @Produce json
// @Param group body CreateUserGroupRequest true "用户组信息"
// @Success 200 {object} UserGroup
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/user-groups [post]
func createUserGroupHandler(c *gin.Context) {
	var req CreateUserGroupRequest
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

// deleteUserGroupHandler DELETE /api/v2/user-groups/:groupId
// @Summary 删除用户组
// @Description 根据用户组ID删除用户组
// @Tags UserGroups
// @Produce json
// @Param groupId path string true "用户组ID"
// @Success 200 {object} StatusResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/user-groups/{groupId} [delete]
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

	c.JSON(http.StatusOK, StatusResponse{Status: "ok"})
}

// getUserGroupDetailHandler GET /api/v2/user-groups/:groupId
// @Summary 获取用户组详情
// @Description 根据用户组ID获取用户组详细信息
// @Tags UserGroups
// @Produce json
// @Param groupId path string true "用户组ID"
// @Param startDate query string false "开始日期(YYYYMMDD)"
// @Param endDate query string false "结束日期(YYYYMMDD)"
// @Success 200 {object} UserGroupDetailResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v2/user-groups/{groupId} [get]
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

	members := make([]UserGroupMember, 0, len(userIDs))
	for _, uid := range userIDs {
		daily, _, err := ListUserProductivity(statDB, UserFilter{UserIds: []string{uid}, StartTime: startTime, EndTime: endTime}, 1, 100000, "")
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
			if userName == "" {
				userName = d.UserName
			}
			if d.TaskIds != nil {
				var ids []interface{}
				if json.Unmarshal(d.TaskIds, &ids) == nil {
					taskCount += len(ids)
				}
			}
			if d.CommitIds != nil {
				var ids []interface{}
				if json.Unmarshal(d.CommitIds, &ids) == nil {
					commitCount += len(ids)
				}
			}
			taskDiffLines += d.TaskDiffLines
			upTokens += d.UpstreamTokens
			downTokens += d.DownstreamTokens
			cost += d.Cost
			taskRealMin += d.TaskRealMinutes
			taskAncientMin += d.TaskAncientMinutes
			commitDiffLines += d.CommitDiffLines
			commitAncientMin += d.CommitAncientMinutes
			commitRealMin += d.CommitRealMinutes
		}

		var taskEffRatio, commitEffRatio float64
		if taskRealMin > 0 {
			taskEffRatio = utils.CalcEfficiencyRatio(taskAncientMin, taskRealMin)
		}
		if commitRealMin > 0 {
			commitEffRatio = utils.CalcEfficiencyRatio(commitAncientMin, commitRealMin)
		}

		members = append(members, UserGroupMember{
			UserId: uid, UserName: userName, DayCount: dayCount,
			TaskCount: taskCount, CommitCount: commitCount,
			TaskDiffLines: taskDiffLines, UpstreamTokens: upTokens,
			DownstreamTokens: downTokens, Cost: cost,
			TaskRealMinutes: taskRealMin, TaskAncientMinutes: taskAncientMin,
			TaskEfficiencyRatio:   taskEffRatio,
			CommitDiffLines:       commitDiffLines,
			CommitAncientMinutes:  commitAncientMin,
			CommitRealMinutes:     commitRealMin,
			CommitEfficiencyRatio: commitEffRatio,
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
		groupTaskEffRatio = utils.CalcEfficiencyRatio(groupTaskAncientMin, groupTaskRealMin)
	}
	if groupCommitRealMin > 0 {
		groupCommitEffRatio = utils.CalcEfficiencyRatio(groupCommitAncientMin, groupCommitRealMin)
	}

	groupSummary := UserGroupSummary{
		DayCount: groupDayCount, TaskCount: groupTaskCount,
		CommitCount: groupCommitCount, TaskDiffLines: groupTaskDiffLines,
		UpstreamTokens: groupUpTokens, DownstreamTokens: groupDownTokens,
		Cost: groupCost, TaskRealMinutes: groupTaskRealMin,
		TaskAncientMinutes:    groupTaskAncientMin,
		TaskEfficiencyRatio:   groupTaskEffRatio,
		CommitDiffLines:       groupCommitDiffLines,
		CommitAncientMinutes:  groupCommitAncientMin,
		CommitRealMinutes:     groupCommitRealMin,
		CommitEfficiencyRatio: groupCommitEffRatio,
	}

	c.JSON(http.StatusOK, UserGroupDetailResponse{
		Group:   group,
		Summary: groupSummary,
		Members: members,
	})
}
