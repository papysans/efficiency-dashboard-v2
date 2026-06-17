package efficiencyv2

import (
	"fmt"
	"strings"

	"kanban/core/models"
	"kanban/core/utils"

	"gorm.io/gorm"
)

// EfficiencyV2ProjectNeedScope 一个 (repo_addr, repo_branch) 候选条件 + 该条目的时间窗与 Need 白/黑名单。
// clause builder（buildEfficiencyV2ProjectNeedScopeClause）与 backend/project_needs_handler.go 的
// projectNeedScope 同口径：同一关联路径 project.repos → needs 按 canon repo_addr[+branch+窗口] IN 反查。
// 但**行过滤口径不同**：ResolveEfficiencyV2ProjectNeeds 只取 merged+有会话（估算针对真实可判读 need），
// 不套展示 caliber（后者还排 main/master/develop/release 等主干分支）。
type EfficiencyV2ProjectNeedScope struct {
	RepoAddr         string
	RepoBranch       string
	StartTime        string
	EndTime          string
	ExcludeNeeds     []string
	IncludeOnlyNeeds []string
}

// buildEfficiencyV2ProjectNeedScopeClause 把候选池拼成 needs 表行选择条件。
// 与 backend 同口径：每个 (repo_addr, repo_branch) 一个 AND 组，组内叠时间窗(dev_end_ts ∈ [start,end])
// 与该条目 Need 白/黑名单；多组用 OR 连接。配置侧存原始地址，needs.repo_addr 是 CanonRepoAddr 规范化值，
// 匹配前对配置地址做同款 canon，否则精确等值恒空。scopes 为空(或全缺 repo_addr) 时返回 ("", nil)。
func buildEfficiencyV2ProjectNeedScopeClause(scopes []EfficiencyV2ProjectNeedScope) (string, []interface{}) {
	var groups []string
	var args []interface{}
	for _, s := range scopes {
		if strings.TrimSpace(s.RepoAddr) == "" {
			continue
		}
		cond := "(repo_addr = ?"
		args = append(args, utils.CanonRepoAddr(s.RepoAddr))
		if strings.TrimSpace(s.RepoBranch) != "" {
			cond += " AND repo_branch = ?"
			args = append(args, s.RepoBranch)
		}
		if s.StartTime != "" {
			cond += " AND dev_end_ts >= ?"
			args = append(args, s.StartTime)
		}
		if s.EndTime != "" {
			cond += " AND dev_end_ts <= ?"
			args = append(args, s.EndTime)
		}
		if len(s.IncludeOnlyNeeds) > 0 {
			cond += " AND need_id IN ?"
			args = append(args, s.IncludeOnlyNeeds)
		} else if len(s.ExcludeNeeds) > 0 {
			cond += " AND need_id NOT IN ?"
			args = append(args, s.ExcludeNeeds)
		}
		cond += ")"
		groups = append(groups, cond)
	}
	if len(groups) == 0 {
		return "", nil
	}
	return strings.Join(groups, " OR "), args
}

// ResolveEfficiencyV2ProjectNeeds 解析项目候选池内**可估算**的 need（项目级按需 LLM 的范围闸门）。
// 范围(PRD)：只取池内 status='merged' 且**有会话**(session_ids 非空数组) 的 need——
// 缺会话的 need 没有 LLM 可判读的努力信号，跑也只会得到低质量估算。
// 返回按 need_id 升序的 need 列表（稳定、便于进度展示与测试断言）。
func ResolveEfficiencyV2ProjectNeeds(db *gorm.DB, scopes []EfficiencyV2ProjectNeedScope) ([]models.Need, error) {
	clause, args := buildEfficiencyV2ProjectNeedScopeClause(scopes)
	if clause == "" {
		return nil, nil
	}
	var rows []models.Need
	q := db.Model(&models.Need{}).
		Where(clause, args...).
		Where("status = ?", "merged").
		// 有会话：session_ids 是 jsonb 数组，'[]' / null 视为无会话。
		Where("session_ids IS NOT NULL AND session_ids <> '[]'::jsonb AND jsonb_array_length(session_ids) > 0").
		Order("need_id ASC")
	if err := q.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("解析项目候选 Need 失败: %w", err)
	}
	return rows, nil
}
