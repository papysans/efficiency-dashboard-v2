package main

import "testing"

// TestEfficiencyV2RegisteredLegacyRemoved 确认 v2 任务类型在册、已下线的 V1 `efficiency` 不在册。
// （原在 efficiencyv2 包的 nonregression 测试，因测的是 main 的 validTaskTypes 注册表，迁回 main。）
func TestEfficiencyV2RegisteredLegacyRemoved(t *testing.T) {
	if validTaskTypes["efficiency"] {
		t.Fatalf("legacy `efficiency` task type should be removed")
	}
	if !validTaskTypes["efficiency-v2"] {
		t.Fatalf("`efficiency-v2` task type should be registered")
	}
}
