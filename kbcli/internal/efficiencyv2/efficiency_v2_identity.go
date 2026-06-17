package efficiencyv2

import (
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"
)

// 身份层：工号(emp_no) 才是稳定的人身份，看板 user_id 是碎片——一个人用多个
// Costrict 账号/设备 → 一个工号挂多个 user_id（实测 add-kb-tyh-B 工号 25163 → 3
// user_id）；反向有共享/默认账号 → 一个 user_id 挂多个工号（实测 BUG018: 2 user_id
// → 11 工号）。贡献者计数 / 集成流降级 / 多人归属一律应按工号去重，不数 user_id。
// 桥：user_id ──commits.git_user_email 前缀(split_part @)──▶ emp_no，并对 dept_user
// 花名册校验（只认在册员工，开源/CI 作者留 orphan）。复用 dept-sync 既有口径
// （cmd_import_dept.go: split_part(git_user_email,'@',1)）。详见任务 research
// archive-data-mining.md。dept_user.universal_id 实测 0% 命中、弃用。

// EfficiencyV2UserEmpMap 持有 user_id→工号 的正向映射与共享账号集合。
type EfficiencyV2UserEmpMap struct {
	// EmpByUID：user_id → 该账号唯一归属的工号。仅含"该 user_id 的在册 commit 工号
	// 唯一"的条目；orphan（无在册 commit）与共享账号不在此 map。
	EmpByUID map[string]string
	// SharedAccountUIDs：commit 工号 ≥2 的 user_id（共享/默认账号）。其会话努力无法
	// 单工号归属，调用方应回退团队级，不计入个人。
	SharedAccountUIDs map[string]bool
	// RegisteredEmpNos：所有经 dept_user 花名册校验在册的工号（commit git_user_email
	// 前缀 JOIN dept_user 的并集）。供按 commit 拆交付物时校验工号是否在册——一个在册
	// 工号可能没有任何"干净" user_id 指向它（如共享账号的 committer），仍应认作工号。
	RegisteredEmpNos map[string]bool
}

// EmpForUID 返回 user_id 对应工号；orphan（未映射）或共享账号返回 ""。
func (m *EfficiencyV2UserEmpMap) EmpForUID(uid string) string {
	if m == nil {
		return ""
	}
	uid = strings.TrimSpace(uid)
	if m.SharedAccountUIDs[uid] {
		return ""
	}
	return m.EmpByUID[uid]
}

// EmpForCommit 按 commit 的 git_user_email 前缀(split_part @)取工号，并要求该工号在
// dept_user 花名册在册；不在册 / 空邮箱返回 ""（调用方归 residual）。拆交付物按 committer
// 的工号分组（数据已在 commits.git_user_email），不依赖 user_id 映射。
func (m *EfficiencyV2UserEmpMap) EmpForCommit(gitUserEmail string) string {
	emp := efficiencyV2EmpNoFromEmail(gitUserEmail)
	if emp == "" {
		return ""
	}
	if m == nil || !m.RegisteredEmpNos[emp] {
		return ""
	}
	return emp
}

// efficiencyV2EmpNoFromEmail 取 git_user_email 的 @ 前缀作工号（split_part(email,'@',1)），
// 镜像 dept-sync / LoadEfficiencyV2UserEmpMap 的提取口径，不做在册校验。
func efficiencyV2EmpNoFromEmail(gitUserEmail string) string {
	email := strings.TrimSpace(gitUserEmail)
	if email == "" {
		return ""
	}
	if idx := strings.IndexByte(email, '@'); idx >= 0 {
		return strings.TrimSpace(email[:idx])
	}
	return email
}

// DistinctEmpNos 把一组 user_id 去重为工号集（丢弃 orphan 与共享账号），用于按"人"
// 而非碎片 user_id 重新计数贡献者 / 重判集成流。结果排序保证确定性。
func (m *EfficiencyV2UserEmpMap) DistinctEmpNos(uids []string) []string {
	seen := make(map[string]bool, len(uids))
	out := make([]string, 0, len(uids))
	for _, uid := range uids {
		emp := m.EmpForUID(uid)
		if emp == "" || seen[emp] {
			continue
		}
		seen[emp] = true
		out = append(out, emp)
	}
	sort.Strings(out)
	return out
}

// efficiencyV2EmpCommitRow 是 (user_id, emp_no) 去重行的承载体（gorm Scan 目标）。
type efficiencyV2EmpCommitRow struct {
	UserId string
	EmpNo  string
}

// BuildEfficiencyV2UserEmpMapFromRows 是纯构建器（无 DB，便于单测）：
//   - 某 user_id 的工号集合恰为 1 → 写入 EmpByUID；
//   - 工号集合 ≥2 → 标记共享账号（不进 EmpByUID）。
//
// 空 user_id / 空 emp_no 行忽略。
func BuildEfficiencyV2UserEmpMapFromRows(rows []efficiencyV2EmpCommitRow) *EfficiencyV2UserEmpMap {
	empSetByUID := make(map[string]map[string]bool)
	registered := make(map[string]bool)
	for _, r := range rows {
		uid := strings.TrimSpace(r.UserId)
		emp := strings.TrimSpace(r.EmpNo)
		if emp == "" {
			continue
		}
		// 行来自 commits JOIN dept_user，故每个 emp_no 都在册（即便其 user_id 为空/碎片）。
		registered[emp] = true
		if uid == "" {
			continue
		}
		if empSetByUID[uid] == nil {
			empSetByUID[uid] = make(map[string]bool)
		}
		empSetByUID[uid][emp] = true
	}
	m := &EfficiencyV2UserEmpMap{
		EmpByUID:          make(map[string]string, len(empSetByUID)),
		SharedAccountUIDs: make(map[string]bool),
		RegisteredEmpNos:  registered,
	}
	for uid, set := range empSetByUID {
		if len(set) == 1 {
			for emp := range set {
				m.EmpByUID[uid] = emp
			}
			continue
		}
		m.SharedAccountUIDs[uid] = true
	}
	return m
}

// LoadEfficiencyV2UserEmpMap 从 commits 派生映射：emp_no = split_part(git_user_email,
// '@', 1)，并 JOIN dept_user 校验（只认在册工号；开源/CI 作者前缀不在 dept_user → 不
// 映射，留 orphan）。镜像 dept-sync 既有桥接口径。dept_user 未灌入时返回空映射（调用方
// 据此回退按 user_id 旧口径，不致 panic）。
func LoadEfficiencyV2UserEmpMap(db *gorm.DB) (*EfficiencyV2UserEmpMap, error) {
	var rows []efficiencyV2EmpCommitRow
	if err := db.Raw(`
		SELECT DISTINCT c.user_id AS user_id, split_part(c.git_user_email, '@', 1) AS emp_no
		FROM commits c
		JOIN dept_user d ON d.emp_no = split_part(c.git_user_email, '@', 1)
		WHERE c.user_id IS NOT NULL AND c.user_id <> ''
		  AND c.git_user_email IS NOT NULL AND c.git_user_email <> ''
	`).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("从 commits 构建 user→工号 映射失败: %w", err)
	}
	return BuildEfficiencyV2UserEmpMapFromRows(rows), nil
}
