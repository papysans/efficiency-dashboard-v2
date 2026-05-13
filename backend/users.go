package main

import (
	"kanban/core/models"
	"kanban/core/utils"

	"gorm.io/gorm"
)

// GetUsersProductivity 根据用户列表查询用户生产力数据，并构建 OrgMemberItem 数组
func GetUsersProductivity(db *gorm.DB, matchedUsers []*models.UserOrg, startTime, endTime string) []OrgMemberItem {
	userIDs := make([]string, 0, len(matchedUsers))
	for _, u := range matchedUsers {
		userIDs = append(userIDs, u.UserId)
	}
	return GetProductivityByIds(db, userIDs, startTime, endTime)
}

// GetProductivityByIds 根据用户列表查询用户生产力数据，并构建 OrgMemberItem 数组
func GetProductivityByIds(db *gorm.DB, userids []string, startTime, endTime string) []OrgMemberItem {
	var members []OrgMemberItem
	daily, _, err := ListUserProductivity(db, UserFilter{UserIds: userids, StartTime: startTime, EndTime: endTime}, 1, 100000, "")
	if err != nil {
		return members
	}
	memberMap := make(map[string]*OrgMemberItem)
	for _, d := range daily {
		if d.CreateTime.IsZero() {
			continue
		}
		ma := memberMap[d.UserId]
		if ma == nil {
			ma = &OrgMemberItem{UserId: d.UserId, UserName: d.UserName}
			if om, ok := orgMappings[d.UserId]; ok {
				ma.UserName = om.UserName
				ma.Org1 = om.Org1
				ma.Org2 = om.Org2
				ma.Org3 = om.Org3
				ma.Org4 = om.Org4
				ma.Org5 = om.Org5
				ma.Org6 = om.Org6
				ma.Org7 = om.Org7
				ma.Org8 = om.Org8
				ma.Org9 = om.Org9
				ma.OrgDisplay = getOrgDisplay(om.Org1, om.Org2, om.Org3, om.Org4, om.Org5, om.Org6, om.Org7, om.Org8, om.Org9)
			}
		}
		ma.TaskDiffLines += d.TaskDiffLines
		ma.TaskRealMinutes += d.TaskRealMinutes
		ma.TaskAncientMinutes += d.TaskAncientMinutes
		ma.CommitDiffLines += d.CommitDiffLines
		ma.CommitRealMinutes += d.CommitRealMinutes
		ma.CommitAncientMinutes += d.CommitAncientMinutes
		ma.UpstreamTokens += d.UpstreamTokens
		ma.DownstreamTokens += d.DownstreamTokens
		ma.Cost += d.Cost
		memberMap[d.UserId] = ma
	}

	for _, ma := range memberMap {
		ma.TaskEfficiencyRatio = utils.CalcEfficiencyRatio(ma.TaskAncientMinutes, ma.TaskRealMinutes)
		ma.CommitEfficiencyRatio = utils.CalcEfficiencyRatio(ma.CommitAncientMinutes, ma.CommitRealMinutes)
		members = append(members, *ma)
	}

	return members
}
