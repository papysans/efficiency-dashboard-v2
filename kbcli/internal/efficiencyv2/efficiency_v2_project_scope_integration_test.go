//go:build integration

package efficiencyv2

import (
	"testing"

	"kanban/core/models"
)

// ResolveEfficiencyV2ProjectNeeds 端到端：canon 反查 + merged + 有会话 三重闸门。
// 配置侧给原始地址，needs 行存 canon 地址；只命中 merged 且 session_ids 非空的 need。
func TestResolveEfficiencyV2ProjectNeeds_CanonMergedWithSessionsOnly(t *testing.T) {
	db := openEfficiencyV2IntegrationDB(t)

	// needs.repo_addr 落 canon 值（与 kbcli 写侧一致）。
	const canonAddr = "example.com/acme/billing"
	seed := []models.Need{
		{ // ✓ 命中：merged + 有会话
			NeedId: "proj-need-merged-sess", BoundarySource: "pr", BoundaryConfidence: "high",
			BoundaryKey: "pr:9001", Status: "merged", RepoAddr: canonAddr, RepoBranch: "feature/x",
			SessionIds: models.StringJSON(`["s-1"]`),
		},
		{ // ✗ merged 但无会话
			NeedId: "proj-need-merged-nosess", BoundarySource: "pr", BoundaryConfidence: "high",
			BoundaryKey: "pr:9002", Status: "merged", RepoAddr: canonAddr, RepoBranch: "feature/x",
			SessionIds: models.StringJSON(`[]`),
		},
		{ // ✗ 有会话但 active
			NeedId: "proj-need-active", BoundarySource: "pr", BoundaryConfidence: "high",
			BoundaryKey: "pr:9003", Status: "active", RepoAddr: canonAddr, RepoBranch: "feature/x",
			SessionIds: models.StringJSON(`["s-2"]`),
		},
		{ // ✗ 别的仓库
			NeedId: "proj-need-otherrepo", BoundarySource: "pr", BoundaryConfidence: "high",
			BoundaryKey: "pr:9004", Status: "merged", RepoAddr: "example.com/acme/other", RepoBranch: "feature/x",
			SessionIds: models.StringJSON(`["s-3"]`),
		},
	}
	for i := range seed {
		if err := db.Where("need_id = ?", seed[i].NeedId).Delete(&models.Need{}).Error; err != nil {
			t.Fatalf("cleanup need %s: %v", seed[i].NeedId, err)
		}
	}
	if err := db.Create(&seed).Error; err != nil {
		t.Fatalf("seed needs: %v", err)
	}
	t.Cleanup(func() {
		for i := range seed {
			_ = db.Where("need_id = ?", seed[i].NeedId).Delete(&models.Need{}).Error
		}
	})

	// 配置侧给原始（未规范化）地址，验证 canon 后才匹配上。
	scopes := []EfficiencyV2ProjectNeedScope{
		{RepoAddr: "git@example.com:Acme/Billing.git", RepoBranch: "feature/x"},
	}
	got, err := ResolveEfficiencyV2ProjectNeeds(db, scopes)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(got) != 1 || got[0].NeedId != "proj-need-merged-sess" {
		ids := make([]string, len(got))
		for i, n := range got {
			ids[i] = n.NeedId
		}
		t.Fatalf("resolved = %v, want exactly [proj-need-merged-sess]", ids)
	}
}
