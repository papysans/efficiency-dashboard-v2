package efficiencyv2

import (
	"sort"
	"testing"
	"time"

	"kanban/core/models"
)

// 构建带 RegisteredEmpNos 的 emp map：行来自 commits JOIN dept_user，故每个 emp_no 在册。
func efficiencyV2AttrTestEmpMap(rows []efficiencyV2EmpCommitRow) *EfficiencyV2UserEmpMap {
	return BuildEfficiencyV2UserEmpMapFromRows(rows)
}

func efficiencyV2AttrTestCommit(id, gitEmail string, lines int, t time.Time) models.Commit {
	return models.Commit{
		CommitId:     id,
		GitUserEmail: gitEmail,
		DiffLines:    lines,
		CommitTime:   t,
	}
}

// indexAttrRows：按 emp_no 取行，便于断言。
func indexAttrRows(rows []models.NeedEmpAttribution) map[string]models.NeedEmpAttribution {
	out := make(map[string]models.NeedEmpAttribution, len(rows))
	for _, r := range rows {
		out[r.EmpNo] = r
	}
	return out
}

func sumAttrLoc(rows []models.NeedEmpAttribution) int64 {
	var s int64
	for _, r := range rows {
		s += r.LocNet
	}
	return s
}

// solo need（单工号）：所有交付物 + 努力整条归该工号一行，kind=solo。
func TestComputeNeedEmpAttribution_SoloSingleEmp(t *testing.T) {
	base := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	empMap := efficiencyV2AttrTestEmpMap([]efficiencyV2EmpCommitRow{
		// 工号 25163 三个碎片 user_id（add-kb-tyh-B 形态）
		{UserId: "u1", EmpNo: "25163"},
		{UserId: "u2", EmpNo: "25163"},
		{UserId: "u3", EmpNo: "25163"},
	})
	sessionA := efficiencyV2AggTestMetric("s-a", "u1", base, base.Add(60*time.Minute), 60)
	sessionB := efficiencyV2AggTestMetric("s-b", "u2", base.Add(2*time.Hour), base.Add(3*time.Hour), 45)
	// commit 落 sessionA 窗口内 → ai_covered
	commitCovered := efficiencyV2AttrTestCommit("c1", "25163@sangfor.com", 100, base.Add(30*time.Minute))
	// commit 远离任何 session → 未覆盖
	commitUncovered := efficiencyV2AttrTestCommit("c2", "25163@sangfor.com", 40, base.Add(10*time.Hour))

	need := efficiencyV2AggTestNeed("need-solo", efficiencyV2BoundaryBranch, efficiencyV2ConfidenceHigh, "u1", []string{"s-a", "s-b"}, []string{"c1", "c2"})
	need.EmpNoCount = 1

	rows := ComputeEfficiencyV2NeedEmpAttribution(need, []models.Commit{commitCovered, commitUncovered}, []models.SessionStageMetric{sessionA, sessionB}, empMap, EfficiencyV2Config{})
	if len(rows) != 1 {
		t.Fatalf("solo need 应 1 行，得 %d 行: %+v", len(rows), rows)
	}
	r := rows[0]
	if r.EmpNo != "25163" {
		t.Fatalf("emp_no = %q, want 25163", r.EmpNo)
	}
	if r.AttributionKind != efficiencyV2AttributionSolo {
		t.Fatalf("kind = %q, want solo", r.AttributionKind)
	}
	if r.CommitCount != 2 || r.LocNet != 140 {
		t.Fatalf("commit_count=%d loc_net=%d, want 2/140", r.CommitCount, r.LocNet)
	}
	if r.AICoveredLoc != 100 {
		t.Fatalf("ai_covered_loc=%d, want 100 (仅 c1 落窗)", r.AICoveredLoc)
	}
	// 两 user_id 不重叠时段 → 人时 = 60 + 45
	if r.ActiveWorkMin != 105 {
		t.Fatalf("active_work_min=%.2f, want 105 (60+45)", r.ActiveWorkMin)
	}
}

// 多工号 need：交付物 + 努力按工号拆 N 行；Σloc_net 守恒；在册工号 kind=split。
func TestComputeNeedEmpAttribution_MultiEmpSplitConservesLoc(t *testing.T) {
	base := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	empMap := efficiencyV2AttrTestEmpMap([]efficiencyV2EmpCommitRow{
		{UserId: "ua", EmpNo: "10001"},
		{UserId: "ub", EmpNo: "20002"},
	})
	sessionA := efficiencyV2AggTestMetric("s-a", "ua", base, base.Add(60*time.Minute), 60)
	sessionB := efficiencyV2AggTestMetric("s-b", "ub", base.Add(2*time.Hour), base.Add(2*time.Hour+90*time.Minute), 90)

	commitA1 := efficiencyV2AttrTestCommit("ca1", "10001@sangfor.com", 200, base.Add(20*time.Minute)) // covered by s-a
	commitA2 := efficiencyV2AttrTestCommit("ca2", "10001@sangfor.com", 50, base.Add(20*time.Hour))    // uncovered
	commitB1 := efficiencyV2AttrTestCommit("cb1", "20002@sangfor.com", 300, base.Add(2*time.Hour+30*time.Minute))

	need := efficiencyV2AggTestNeed("need-multi", efficiencyV2BoundaryBranch, efficiencyV2ConfidenceHigh, "ua", []string{"s-a", "s-b"}, []string{"ca1", "ca2", "cb1"})
	need.EmpNoCount = 2

	commits := []models.Commit{commitA1, commitA2, commitB1}
	rows := ComputeEfficiencyV2NeedEmpAttribution(need, commits, []models.SessionStageMetric{sessionA, sessionB}, empMap, EfficiencyV2Config{})
	if len(rows) != 2 {
		t.Fatalf("两工号应 2 行，得 %d: %+v", len(rows), rows)
	}
	// 行按 emp_no 排序确定
	if rows[0].EmpNo != "10001" || rows[1].EmpNo != "20002" {
		t.Fatalf("行未按 emp_no 排序: %v", []string{rows[0].EmpNo, rows[1].EmpNo})
	}
	by := indexAttrRows(rows)
	if r := by["10001"]; r.CommitCount != 2 || r.LocNet != 250 || r.AICoveredLoc != 200 || r.ActiveWorkMin != 60 || r.AttributionKind != efficiencyV2AttributionSplit {
		t.Fatalf("工号 10001 行 = %+v", r)
	}
	if r := by["20002"]; r.CommitCount != 1 || r.LocNet != 300 || r.AICoveredLoc != 300 || r.ActiveWorkMin != 90 || r.AttributionKind != efficiencyV2AttributionSplit {
		t.Fatalf("工号 20002 行 = %+v", r)
	}
	// LOC 守恒：Σ 行 loc_net == Σ commit effective lines
	var totalCommitLoc int64
	for _, c := range commits {
		totalCommitLoc += c.GetEffectiveDiffLines()
	}
	if got := sumAttrLoc(rows); got != totalCommitLoc {
		t.Fatalf("LOC 守恒破坏：Σ行=%d Σcommit=%d", got, totalCommitLoc)
	}
}

// orphan / 非在册 committer → residual 交付物行；LOC 仍守恒。
func TestComputeNeedEmpAttribution_NonRegisteredCommitGoesResidual(t *testing.T) {
	base := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	empMap := efficiencyV2AttrTestEmpMap([]efficiencyV2EmpCommitRow{
		{UserId: "ua", EmpNo: "10001"},
	})
	sessionA := efficiencyV2AggTestMetric("s-a", "ua", base, base.Add(60*time.Minute), 60)
	commitReg := efficiencyV2AttrTestCommit("c1", "10001@sangfor.com", 200, base.Add(20*time.Minute))
	// 外部贡献者 / CI：前缀不在册 → residual
	commitExt := efficiencyV2AttrTestCommit("c2", "noreply@github.com", 80, base.Add(25*time.Minute))
	// 空邮箱 → residual
	commitNoEmail := efficiencyV2AttrTestCommit("c3", "", 30, base.Add(30*time.Minute))

	need := efficiencyV2AggTestNeed("need-residual-deliver", efficiencyV2BoundaryBranch, efficiencyV2ConfidenceHigh, "ua", []string{"s-a"}, []string{"c1", "c2", "c3"})
	need.EmpNoCount = 1 // 去重后只有在册工号 10001

	commits := []models.Commit{commitReg, commitExt, commitNoEmail}
	rows := ComputeEfficiencyV2NeedEmpAttribution(need, commits, []models.SessionStageMetric{sessionA}, empMap, EfficiencyV2Config{})

	by := indexAttrRows(rows)
	residual, ok := by[""]
	if !ok {
		t.Fatalf("应有 residual 行 (emp_no=\"\")，得 %+v", rows)
	}
	if residual.CommitCount != 2 || residual.LocNet != 110 {
		t.Fatalf("residual 交付物 = commit_count=%d loc_net=%d, want 2/110 (80+30)", residual.CommitCount, residual.LocNet)
	}
	// 单工号 need + 非在册 commit：residual 行必须标 residual（不能被 EmpNoCount=1 误盖 solo）。
	if residual.AttributionKind != efficiencyV2AttributionResidual {
		t.Fatalf("residual 行 kind=%q, want residual", residual.AttributionKind)
	}
	// 唯一在册工号行 R=1 → solo。
	if r := by["10001"]; r.LocNet != 200 || r.AttributionKind != efficiencyV2AttributionSolo {
		t.Fatalf("在册工号行 loc_net=%d kind=%q, want 200/solo", r.LocNet, r.AttributionKind)
	}
	// 守恒（含 residual）
	var totalCommitLoc int64
	for _, c := range commits {
		totalCommitLoc += c.GetEffectiveDiffLines()
	}
	if got := sumAttrLoc(rows); got != totalCommitLoc {
		t.Fatalf("含 residual LOC 守恒破坏：Σ行=%d Σcommit=%d", got, totalCommitLoc)
	}
}

// 共享账号 session（一个 user_id 挂多工号）→ 努力归 residual 行，不单工号拆。
func TestComputeNeedEmpAttribution_SharedAccountEffortResidual(t *testing.T) {
	base := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	empMap := efficiencyV2AttrTestEmpMap([]efficiencyV2EmpCommitRow{
		{UserId: "ua", EmpNo: "10001"},
		// BUG018 形态：共享 user_id 挂两工号 → 标共享，EmpForUID 返回空
		{UserId: "ushared", EmpNo: "20002"},
		{UserId: "ushared", EmpNo: "30003"},
	})
	sessionReg := efficiencyV2AggTestMetric("s-a", "ua", base, base.Add(60*time.Minute), 60)
	sessionShared := efficiencyV2AggTestMetric("s-shared", "ushared", base.Add(2*time.Hour), base.Add(2*time.Hour+30*time.Minute), 30)
	sessionOrphan := efficiencyV2AggTestMetric("s-orphan", "u-unknown", base.Add(4*time.Hour), base.Add(4*time.Hour+15*time.Minute), 15)

	commitReg := efficiencyV2AttrTestCommit("c1", "10001@sangfor.com", 200, base.Add(20*time.Minute))
	need := efficiencyV2AggTestNeed("need-shared", efficiencyV2BoundaryBranch, efficiencyV2ConfidenceHigh, "ua", []string{"s-a", "s-shared", "s-orphan"}, []string{"c1"})
	need.EmpNoCount = 1 // 去重后仅在册工号 10001（共享/orphan 不计人）

	rows := ComputeEfficiencyV2NeedEmpAttribution(need, []models.Commit{commitReg}, []models.SessionStageMetric{sessionReg, sessionShared, sessionOrphan}, empMap, EfficiencyV2Config{})

	by := indexAttrRows(rows)
	// 唯一在册工号行（仅 10001 有 commit）R=1 → solo。
	if r := by["10001"]; r.ActiveWorkMin != 60 || r.LocNet != 200 || r.AttributionKind != efficiencyV2AttributionSolo {
		t.Fatalf("在册工号行 = active=%.2f loc=%d kind=%q, want 60/200/solo", r.ActiveWorkMin, r.LocNet, r.AttributionKind)
	}
	residual, ok := by[""]
	if !ok {
		t.Fatalf("共享 + orphan session 应汇入 residual 努力行，得 %+v", rows)
	}
	if residual.AttributionKind != efficiencyV2AttributionResidual {
		t.Fatalf("residual 努力行 kind=%q, want residual", residual.AttributionKind)
	}
	// 共享(30) 与 orphan(15) 两个不同 user_id 不重叠 → 人时 45，commit 侧 0
	if residual.ActiveWorkMin != 45 {
		t.Fatalf("residual active_work_min=%.2f, want 45 (30+15)", residual.ActiveWorkMin)
	}
	if residual.LocNet != 0 || residual.CommitCount != 0 {
		t.Fatalf("residual 不应有交付物：loc=%d commit=%d", residual.LocNet, residual.CommitCount)
	}
}

// 承重 case：共享账号 committer 作为第二在册工号 → EmpNoCount==1（EmpForUID 把共享账号
// 计为空、不入工号集），但 EmpForCommit 把该共享账号的 commit 归到有效在册工号 20002 →
// 产出 2 个在册工号行(10001 + 20002)，二者都必须 split。验证 attribution_kind 按实际行集
// 判而非 EmpNoCount——这正是旧实现 solo:=EmpNoCount<=1 的 bug 所在。
func TestComputeNeedEmpAttribution_SharedAccountCommitterSecondEmpSplits(t *testing.T) {
	base := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	empMap := efficiencyV2AttrTestEmpMap([]efficiencyV2EmpCommitRow{
		{UserId: "ua", EmpNo: "10001"},
		// 共享账号 ushared 挂两工号 20002/30003 → EmpForUID 返回空（不计入 EmpNoCount），
		// 但 20002 仍在 RegisteredEmpNos，故按 git_user_email 拆交付物时认作有效工号。
		{UserId: "ushared", EmpNo: "20002"},
		{UserId: "ushared", EmpNo: "30003"},
	})
	// 干净贡献者 ua 的会话 + commit（工号 10001）
	sessionReg := efficiencyV2AggTestMetric("s-a", "ua", base, base.Add(60*time.Minute), 60)
	commitReg := efficiencyV2AttrTestCommit("c1", "10001@sangfor.com", 200, base.Add(20*time.Minute))
	// 共享账号下一个 commit，git_user_email=20002（在册）→ 应归到第二在册工号行
	commitShared := efficiencyV2AttrTestCommit("c2", "20002@sangfor.com", 120, base.Add(25*time.Minute))

	need := efficiencyV2AggTestNeed("need-shared-committer", efficiencyV2BoundaryBranch, efficiencyV2ConfidenceHigh, "ua", []string{"s-a"}, []string{"c1", "c2"})
	need.EmpNoCount = 1 // EmpForUID 口径：仅干净工号 10001（共享账号计为空）

	rows := ComputeEfficiencyV2NeedEmpAttribution(need, []models.Commit{commitReg, commitShared}, []models.SessionStageMetric{sessionReg}, empMap, EfficiencyV2Config{})
	if len(rows) != 2 {
		t.Fatalf("应产出 2 个在册工号行，得 %d: %+v", len(rows), rows)
	}
	by := indexAttrRows(rows)
	// 两行都 split（EmpNoCount==1 不能把它们盖成 solo）
	if r := by["10001"]; r.LocNet != 200 || r.AttributionKind != efficiencyV2AttributionSplit {
		t.Fatalf("工号 10001 行 loc=%d kind=%q, want 200/split", r.LocNet, r.AttributionKind)
	}
	if r := by["20002"]; r.LocNet != 120 || r.AttributionKind != efficiencyV2AttributionSplit {
		t.Fatalf("工号 20002 行（共享账号 committer）loc=%d kind=%q, want 120/split", r.LocNet, r.AttributionKind)
	}
	// 不应出现 residual 行（两 commit 都归到在册工号）
	if _, ok := by[""]; ok {
		t.Fatalf("两 commit 均在册，不应有 residual 行: %+v", rows)
	}
	// LOC 守恒
	if got := sumAttrLoc(rows); got != 320 {
		t.Fatalf("LOC 守恒：Σ行=%d, want 320 (200+120)", got)
	}
}

// 努力工号有会话无 commit、交付物工号有 commit 无会话 → outer-join 两行都在。
func TestComputeNeedEmpAttribution_OuterJoinEffortOnlyAndDeliverOnly(t *testing.T) {
	base := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	empMap := efficiencyV2AttrTestEmpMap([]efficiencyV2EmpCommitRow{
		{UserId: "ua", EmpNo: "10001"},
		{UserId: "ub", EmpNo: "20002"},
	})
	// ua 只有会话无 commit；20002 只有 commit 无会话
	sessionA := efficiencyV2AggTestMetric("s-a", "ua", base, base.Add(60*time.Minute), 60)
	commitB := efficiencyV2AttrTestCommit("c1", "20002@sangfor.com", 150, base.Add(20*time.Minute))

	need := efficiencyV2AggTestNeed("need-outer", efficiencyV2BoundaryBranch, efficiencyV2ConfidenceHigh, "ua", []string{"s-a"}, []string{"c1"})
	need.EmpNoCount = 2

	rows := ComputeEfficiencyV2NeedEmpAttribution(need, []models.Commit{commitB}, []models.SessionStageMetric{sessionA}, empMap, EfficiencyV2Config{})
	if len(rows) != 2 {
		t.Fatalf("outer-join 应 2 行，得 %d: %+v", len(rows), rows)
	}
	by := indexAttrRows(rows)
	// 两个在册工号行（R=2）→ 都 split。
	if r := by["10001"]; r.ActiveWorkMin != 60 || r.CommitCount != 0 || r.LocNet != 0 || r.AttributionKind != efficiencyV2AttributionSplit {
		t.Fatalf("努力-only 工号 10001 = %+v, want kind=split", r)
	}
	if r := by["20002"]; r.CommitCount != 1 || r.LocNet != 150 || r.ActiveWorkMin != 0 || r.AttributionKind != efficiencyV2AttributionSplit {
		t.Fatalf("交付物-only 工号 20002 = %+v, want kind=split", r)
	}
}

// 空 need（无 commit 无 session）→ 0 行（无可归属，不写残行）。
func TestComputeNeedEmpAttribution_EmptyNeedNoRows(t *testing.T) {
	empMap := efficiencyV2AttrTestEmpMap(nil)
	need := efficiencyV2AggTestNeed("need-empty", efficiencyV2BoundaryBranch, efficiencyV2ConfidenceHigh, "ua", nil, nil)
	need.EmpNoCount = 0
	rows := ComputeEfficiencyV2NeedEmpAttribution(need, nil, nil, empMap, EfficiencyV2Config{})
	if len(rows) != 0 {
		t.Fatalf("空 need 应 0 行，得 %d: %+v", len(rows), rows)
	}
}

// 确定性：同输入多次计算行顺序与值一致（emp_no 排序）。
func TestComputeNeedEmpAttribution_Deterministic(t *testing.T) {
	base := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	empMap := efficiencyV2AttrTestEmpMap([]efficiencyV2EmpCommitRow{
		{UserId: "ua", EmpNo: "30003"},
		{UserId: "ub", EmpNo: "10001"},
		{UserId: "uc", EmpNo: "20002"},
	})
	sessions := []models.SessionStageMetric{
		efficiencyV2AggTestMetric("s-a", "ua", base, base.Add(30*time.Minute), 30),
		efficiencyV2AggTestMetric("s-b", "ub", base.Add(time.Hour), base.Add(time.Hour+30*time.Minute), 30),
		efficiencyV2AggTestMetric("s-c", "uc", base.Add(2*time.Hour), base.Add(2*time.Hour+30*time.Minute), 30),
	}
	commits := []models.Commit{
		efficiencyV2AttrTestCommit("c1", "30003@sangfor.com", 10, base.Add(5*time.Minute)),
		efficiencyV2AttrTestCommit("c2", "10001@sangfor.com", 20, base.Add(time.Hour+5*time.Minute)),
		efficiencyV2AttrTestCommit("c3", "20002@sangfor.com", 30, base.Add(2*time.Hour+5*time.Minute)),
	}
	need := efficiencyV2AggTestNeed("need-det", efficiencyV2BoundaryBranch, efficiencyV2ConfidenceHigh, "ua", []string{"s-a", "s-b", "s-c"}, []string{"c1", "c2", "c3"})
	need.EmpNoCount = 3

	first := ComputeEfficiencyV2NeedEmpAttribution(need, commits, sessions, empMap, EfficiencyV2Config{})
	gotOrder := make([]string, len(first))
	for i, r := range first {
		gotOrder[i] = r.EmpNo
	}
	if !sort.StringsAreSorted(gotOrder) {
		t.Fatalf("行未按 emp_no 升序: %v", gotOrder)
	}
	for i := 0; i < 5; i++ {
		again := ComputeEfficiencyV2NeedEmpAttribution(need, commits, sessions, empMap, EfficiencyV2Config{})
		for j := range first {
			if again[j] != first[j] {
				t.Fatalf("第 %d 次计算结果漂移: %+v vs %+v", i, again[j], first[j])
			}
		}
	}
}
