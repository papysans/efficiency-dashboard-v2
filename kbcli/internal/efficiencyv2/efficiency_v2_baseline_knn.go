package efficiencyv2

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"kanban/core/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	EfficiencyV2KNNDefaultK = 5
)

type EfficiencyV2KNNAnchor struct {
	AnchorID         string
	Source           string
	WithoutAIMinutes float64
	Weight           float64
	FeatureVector    map[string]float64
}

type EfficiencyV2KNNResult struct {
	Estimate    *float64
	NeighborIDs []string
	Reason      string
}

// LoadEfficiencyV2KNNAnchors reads usable anchor records into a deterministic
// in-memory list. Anchors missing `without_ai_minutes` or a feature vector are
// skipped with a logged reason.
func LoadEfficiencyV2KNNAnchors(db *gorm.DB) ([]EfficiencyV2KNNAnchor, error) {
	var rows []models.AnchorSet
	if err := db.Order("anchor_id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("load anchor set: %w", err)
	}
	anchors := make([]EfficiencyV2KNNAnchor, 0, len(rows))
	for _, row := range rows {
		if row.WithoutAIMinutes == nil {
			continue
		}
		// 设计 §4.3：kNN anchor 必须是真正的 without-AI 时间，否则退化为 peer-relative。
		// team_self_bootstrap anchor 的 without_ai_minutes 来自 LLM baseline 对自家
		// AI 辅助需求的估算（HumanLabeled=false），是 peer-relative、且与 fused 形成
		// 循环依赖，会把基线系统性拽低、提效比虚低、work_eff 压成负。排除之；当只剩
		// 这类 anchor 时 kNN 退出融合（§4.8：anchor 库无有效样本 → 只跑 A+C）。
		if row.Source == "team_self_bootstrap" && !row.HumanLabeled {
			continue
		}
		vector := efficiencyV2DecodeFeatureVector(row.FeatureVector)
		if len(vector) == 0 {
			continue
		}
		// Skip degenerate anchors whose feature_vector is all zeros — they would
		// give identical distance to every Need and degrade kNN to a global mean
		// (broken for v0 metr seed where feature_vector is "{loc:0,files:0,turns:0}").
		// Per design §4.3, kNN needs real features to find similar Needs.
		allZero := true
		for _, v := range vector {
			if v != 0 {
				allZero = false
				break
			}
		}
		if allZero {
			continue
		}
		weight := row.Weight
		if weight <= 0 {
			weight = 1
		}
		anchors = append(anchors, EfficiencyV2KNNAnchor{
			AnchorID:         row.AnchorId,
			Source:           row.Source,
			WithoutAIMinutes: *row.WithoutAIMinutes,
			Weight:           weight,
			FeatureVector:    vector,
		})
	}
	return anchors, nil
}

// UpsertEfficiencyV2KNNAnchorsFromFixture seeds anchors from the deterministic
// fixture catalog. It is intended for E2E spine setup.
func UpsertEfficiencyV2KNNAnchorsFromFixture(db *gorm.DB, fixture EfficiencyV2Fixture) error {
	for _, anchor := range fixture.Anchors {
		minutes := anchor.WithoutAIMinutes
		featureJSON, _ := json.Marshal(anchor.FeatureVector)
		row := models.AnchorSet{
			AnchorId:         anchor.AnchorID,
			Source:           anchor.Source,
			SourceVersion:    "fixture_v1",
			AnchorKind:       "fixture",
			WithoutAIMinutes: &minutes,
			HumanLabeled:     true,
			Weight:           1,
			FeatureVector:    models.ObjectJSON(featureJSON),
			Labels:           models.ObjectJSON("{}"),
		}
		if err := db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "anchor_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"source", "source_version", "anchor_kind", "without_ai_minutes",
				"human_labeled", "weight", "feature_vector", "labels", "updated_at",
			}),
		}).Create(&row).Error; err != nil {
			return fmt.Errorf("upsert anchor %s: %w", anchor.AnchorID, err)
		}
	}
	return nil
}

// UpsertEfficiencyV2SelfBootstrapAnchors persists eligible Needs as anchors so
// future v2 runs have real feature_vectors for kNN. Per design §4.3 + line 798
// ("v1+: 加你团队人工标注的少量 anchor"), the team's own merged + high-confidence
// Needs are valid anchors. We use the LLM baseline as the without_AI estimate
// (independent third-party estimate, not the team's actual time).
//
// Eligible Need = status=merged AND boundary_confidence='high' AND has a
// positive LLM baseline. We do NOT use medium-confidence (per design's
// stricter standard for anchor quality).
func UpsertEfficiencyV2SelfBootstrapAnchors(db *gorm.DB, needs []models.Need) (int, error) {
	count := 0
	for _, need := range needs {
		if strings.ToLower(need.Status) != "merged" {
			continue
		}
		if need.BoundaryConfidence != efficiencyV2ConfidenceHigh {
			continue
		}
		if need.BaselineLLMTotalWorkMin == nil || *need.BaselineLLMTotalWorkMin <= 0 {
			continue
		}
		// Build feature vector from need's measured features
		turns := float64(0)
		// Note: turns is also stored on need indirectly; we approximate via
		// session counts when sessions are loaded. For simplicity, use 0 here
		// (most discriminative features are loc + files).
		vector := map[string]float64{
			"loc":    float64(need.ChangedLoc),
			"files":  float64(need.FileCount),
			"turns":  turns,
			"think":  need.ThinkActiveMin,
			"exec":   need.ExecutionActiveMin,
			"verify": need.VerificationActiveMin,
		}
		// Skip degenerate (all-zero) features
		allZero := true
		for _, v := range vector {
			if v != 0 {
				allZero = false
				break
			}
		}
		if allZero {
			continue
		}
		featureJSON, _ := json.Marshal(vector)
		minutes := *need.BaselineLLMTotalWorkMin
		labels, _ := json.Marshal(map[string]interface{}{
			"source_need_id": need.NeedId,
			"llm_confidence": need.BaselineLLMConfidence,
			"team_origin":    true,
		})
		anchorID := "team:" + need.NeedId
		row := models.AnchorSet{
			AnchorId:         anchorID,
			Source:           "team_self_bootstrap",
			SourceVersion:    "v1",
			AnchorKind:       "team_need",
			WithoutAIMinutes: &minutes,
			HumanLabeled:     false,
			Weight:           1,
			FeatureVector:    models.ObjectJSON(featureJSON),
			Labels:           models.ObjectJSON(labels),
		}
		if err := db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "anchor_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"source", "source_version", "anchor_kind", "without_ai_minutes",
				"human_labeled", "weight", "feature_vector", "labels", "updated_at",
			}),
		}).Create(&row).Error; err != nil {
			return count, fmt.Errorf("upsert team anchor %s: %w", anchorID, err)
		}
		count++
	}
	return count, nil
}

// BuildEfficiencyV2NeedFeatureVector projects a Need + its sessions to the
// feature space anchors use for KNN.
func BuildEfficiencyV2NeedFeatureVector(need models.Need, sessions []models.SessionStageMetric) map[string]float64 {
	turns := int64(0)
	for _, s := range sessions {
		turns += s.MessageEventCount
	}
	aiCodeRatio := 0.0
	if need.AICodeRatio != nil {
		aiCodeRatio = *need.AICodeRatio
	}
	return map[string]float64{
		// 代码规模口径（与 METR/fixture 锚点共有；METR 当前为零特征被跳过）
		"loc":    float64(need.ChangedLoc),
		"files":  float64(need.FileCount),
		"turns":  float64(turns),
		"think":  need.ThinkActiveMin,
		"exec":   need.ExecutionActiveMin,
		"verify": need.VerificationActiveMin,
		// 工作量+采纳率口径（与 AI-Native 人工锚点共有）。kNN 距离按交集匹配，
		// 不同来源锚点各自命中对应的键。
		"log_work_min":  math.Log(need.TotalActiveWorkCorrectedMin + 1),
		"ai_code_ratio": aiCodeRatio,
	}
}

// ComputeEfficiencyV2BaselineB performs inverse-distance weighted KNN over the
// supplied anchors. With zero anchors the result is null + reason.
func ComputeEfficiencyV2BaselineB(needFeatures map[string]float64, anchors []EfficiencyV2KNNAnchor, k int) EfficiencyV2KNNResult {
	if len(anchors) == 0 {
		return EfficiencyV2KNNResult{Reason: "knn:no_anchors"}
	}
	if k <= 0 {
		k = EfficiencyV2KNNDefaultK
	}

	type scored struct {
		anchor   EfficiencyV2KNNAnchor
		distance float64
	}
	scoredAnchors := make([]scored, 0, len(anchors))
	for _, anchor := range anchors {
		distance := efficiencyV2KNNDistance(needFeatures, anchor.FeatureVector)
		scoredAnchors = append(scoredAnchors, scored{anchor: anchor, distance: distance})
	}
	sort.SliceStable(scoredAnchors, func(i, j int) bool {
		if scoredAnchors[i].distance != scoredAnchors[j].distance {
			return scoredAnchors[i].distance < scoredAnchors[j].distance
		}
		return scoredAnchors[i].anchor.AnchorID < scoredAnchors[j].anchor.AnchorID
	})

	if k > len(scoredAnchors) {
		k = len(scoredAnchors)
	}
	selected := scoredAnchors[:k]

	var weightedSum, totalWeight float64
	ids := make([]string, 0, len(selected))
	for _, s := range selected {
		w := s.anchor.Weight / (s.distance + 1)
		weightedSum += s.anchor.WithoutAIMinutes * w
		totalWeight += w
		ids = append(ids, s.anchor.AnchorID)
	}
	if totalWeight <= 0 {
		return EfficiencyV2KNNResult{Reason: "knn:zero_weight"}
	}
	estimate := weightedSum / totalWeight
	return EfficiencyV2KNNResult{
		Estimate:    &estimate,
		NeighborIDs: ids,
		Reason:      fmt.Sprintf("knn:k=%d", k),
	}
}

func PersistEfficiencyV2BaselineBOnNeed(need *models.Need, result EfficiencyV2KNNResult) {
	need.BaselineAnchorKnnWorkMin = result.Estimate
	need.BaselineAnchorKnnReason = result.Reason
}

func efficiencyV2KNNDistance(a, b map[string]float64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return math.Inf(1)
	}
	// 只在两边共有的特征键上算距离（交集），而不是并集。need 的特征集是锚点的超集
	// （含 loc/files/turns + log_work_min/ai_code_ratio 等），不同来源的锚点只携带各自
	// 口径的键（METR/fixture 用 loc/files/turns；AI-Native 人工锚点用 log_work_min/
	// ai_code_ratio）。用并集会让 need 独有、锚点缺失的键当 0 主导距离，导致跨口径误配。
	// 交集为空 → 无可比特征 → +Inf（该锚点不参与）。
	sum := 0.0
	shared := 0
	for k, av := range a {
		if bv, ok := b[k]; ok {
			diff := av - bv
			sum += diff * diff
			shared++
		}
	}
	if shared == 0 {
		return math.Inf(1)
	}
	return math.Sqrt(sum)
}

func efficiencyV2DecodeFeatureVector(payload models.ObjectJSON) map[string]float64 {
	if payload == "" || string(payload) == "null" || string(payload) == "{}" {
		return nil
	}
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return nil
	}
	vec := make(map[string]float64, len(raw))
	for k, v := range raw {
		switch n := v.(type) {
		case float64:
			vec[k] = n
		case int:
			vec[k] = float64(n)
		case int64:
			vec[k] = float64(n)
		case json.Number:
			f, err := n.Float64()
			if err == nil {
				vec[k] = f
			}
		}
	}
	return vec
}

// FetchOptionalExternalAnchorsPlan describes the optional external sample-data
// fetch flow. The actual command is registered separately; this helper exists
// so tests and docs can exercise the deterministic local-fixture fallback.
type FetchOptionalExternalAnchorsPlan struct {
	Source          string
	CachePath       string
	Transformation  string
	OfflineFallback string
}

func EfficiencyV2OptionalAnchorFetchPlan() FetchOptionalExternalAnchorsPlan {
	return FetchOptionalExternalAnchorsPlan{
		Source:          "https://example.com/anchors.jsonl",
		CachePath:       "fixtures/anchors_cache.jsonl",
		Transformation:  "jsonl_to_anchor_set",
		OfflineFallback: strings.Join([]string{"local_fixture_anchors", time.Now().Format("2006-01-02")}, ":"),
	}
}
