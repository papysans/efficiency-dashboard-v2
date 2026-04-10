package main

import (
	"encoding/json"
	"testing"
	"time"
)

// ── 工具函数 ──────────────────────────────────────────────────────────────

func emptyOrgProvider() *OrgProvider {
	return &OrgProvider{
		userIDMap:   make(map[string]OrgInfo),
		userNameMap: make(map[string]OrgInfo),
	}
}

func defaultPrices() map[string]ModelPrice {
	return map[string]ModelPrice{
		"GLM-4.7": {InPrice: 0.5, OutPrice: 1.0},
		"GLM-5":   {InPrice: 1.0, OutPrice: 2.0},
		"Auto":    {InPrice: 0.0, OutPrice: 0.0},
	}
}

// minimalJSON 构造一条最小合法的 rawJSON 字节串，允许覆写任意字段
func buildRawJSON(overrides map[string]interface{}) []byte {
	base := map[string]interface{}{
		"identity": map[string]interface{}{
			"task_id":       "task-001",
			"request_id":    "req-001",
			"client_id":     "clientid1234567890abcdef",
			"client_ide":    "vscode",
			"client_version": "2.5.3",
			"client_os":     "Windows",
			"user_name":     "fallback_user",
			"project_path":  "/workspace/proj",
			"caller":        "chat",
			"sender":        "user",
			"user_info": map[string]interface{}{
				"uuid":        "uuid-001",
				"phone":       "13800138000",
				"github_name": "",
				"name":        "张三",
			},
		},
		"timestamp": "2026-03-31T09:39:02.474731526+08:00",
		"tokens": map[string]interface{}{
			"original": map[string]interface{}{
				"system_tokens": 3942,
				"user_tokens":   990,
			},
		},
		"latency": map[string]interface{}{
			"total_latency_ms":       18040,
			"first_token_latency_ms": 805,
		},
		"params": map[string]interface{}{
			"model": "GLM-4.7",
			"llm_params": map[string]interface{}{
				"extra_body": map[string]interface{}{
					"mode":        "code",
					"prompt_mode": "vibe",
				},
				"messages": []interface{}{
					map[string]interface{}{
						"role":    "system",
						"content": "you are a helper",
					},
					map[string]interface{}{
						"role":    "user",
						"content": "<user_message>hello world</user_message>",
					},
				},
			},
		},
		"response_content": map[string]interface{}{
			"tool_calls": nil,
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     int64(1000),
			"completion_tokens": int64(500),
		},
	}
	for k, v := range overrides {
		base[k] = v
	}
	b, _ := json.Marshal(base)
	return b
}

// ── TestParseRawJSON_Normal ──────────────────────────────────────────────

// TP-01: 正常场景 - 字段全部正确解析
func TestParseRawJSON_Normal(t *testing.T) {
	data := buildRawJSON(nil)
	doc, err := ParseRawJSON(data, defaultPrices(), emptyOrgProvider())
	if err != nil {
		t.Fatalf("ParseRawJSON 返回错误: %v", err)
	}
	if doc.TaskID != "task-001" {
		t.Errorf("TaskID: want task-001, got %s", doc.TaskID)
	}
	if doc.UserName != "张三" {
		t.Errorf("UserName: want 张三, got %s", doc.UserName)
	}
	if doc.Model != "GLM-4.7" {
		t.Errorf("Model: want GLM-4.7, got %s", doc.Model)
	}
	if doc.SystemTokens != 3942 {
		t.Errorf("SystemTokens: want 3942, got %d", doc.SystemTokens)
	}
	if doc.UserTokens != 990 {
		t.Errorf("UserTokens: want 990, got %d", doc.UserTokens)
	}
	if doc.APIProcessTime != 18040 {
		t.Errorf("APIProcessTime: want 18040, got %d", doc.APIProcessTime)
	}
	if doc.APITtft != 805 {
		t.Errorf("APITtft: want 805, got %d", doc.APITtft)
	}
}

// TP-02: timestamp 解析 - RFC3339Nano 格式（带纳秒+时区偏移）
func TestParseRawJSON_TimestampRFC3339Nano(t *testing.T) {
	data := buildRawJSON(nil)
	doc, err := ParseRawJSON(data, defaultPrices(), emptyOrgProvider())
	if err != nil {
		t.Fatalf("ParseRawJSON 返回错误: %v", err)
	}
	// 时间应被转换为 UTC
	if doc.Timestamp.Location() != time.UTC {
		t.Errorf("Timestamp 应为 UTC, got %v", doc.Timestamp.Location())
	}
	// 2026-03-31T09:39:02 +08:00 → UTC = 2026-03-31T01:39:02Z
	wantHour := 1
	if doc.Timestamp.Hour() != wantHour {
		t.Errorf("Timestamp UTC hour: want %d, got %d", wantHour, doc.Timestamp.Hour())
	}
}

// TP-03: api_end_time = api_request_time + total_latency_ms
func TestParseRawJSON_EndTimeCalculation(t *testing.T) {
	data := buildRawJSON(nil)
	doc, err := ParseRawJSON(data, defaultPrices(), emptyOrgProvider())
	if err != nil {
		t.Fatalf("ParseRawJSON 返回错误: %v", err)
	}
	diff := doc.APIEndTime.Sub(doc.APIRequestTime).Milliseconds()
	if diff != 18040 {
		t.Errorf("APIEndTime - APIRequestTime: want 18040ms, got %dms", diff)
	}
}

// TP-04: timestamp 解析 - RFC3339 格式（无纳秒）
func TestParseRawJSON_TimestampRFC3339(t *testing.T) {
	data := buildRawJSON(map[string]interface{}{
		"timestamp": "2026-03-31T09:39:02+08:00",
	})
	doc, err := ParseRawJSON(data, defaultPrices(), emptyOrgProvider())
	if err != nil {
		t.Fatalf("RFC3339 格式解析失败: %v", err)
	}
	if doc.Timestamp.IsZero() {
		t.Error("Timestamp 不应为零值")
	}
}

// TP-05: timestamp 格式不合法 → 返回 error
func TestParseRawJSON_InvalidTimestamp(t *testing.T) {
	data := buildRawJSON(map[string]interface{}{
		"timestamp": "not-a-time",
	})
	_, err := ParseRawJSON(data, defaultPrices(), emptyOrgProvider())
	if err == nil {
		t.Error("期望返回 error，但未返回")
	}
}

// TP-06: JSON 格式不合法 → 返回 error
func TestParseRawJSON_InvalidJSON(t *testing.T) {
	_, err := ParseRawJSON([]byte("{invalid json}"), defaultPrices(), nil)
	if err == nil {
		t.Error("期望返回 error，但未返回")
	}
}

// ── TestUsername_Fallback ────────────────────────────────────────────────

// TP-07: username fallback - name 优先
func TestUsernameFromName(t *testing.T) {
	data := buildRawJSON(map[string]interface{}{
		"identity": map[string]interface{}{
			"task_id": "t1", "request_id": "r1",
			"client_id": "cid1234567890abc", "client_ide": "vscode",
			"client_version": "1.0", "client_os": "Windows",
			"user_name": "fallback", "project_path": "/p",
			"caller": "chat", "sender": "user",
			"user_info": map[string]interface{}{
				"uuid": "u1", "phone": "13800000001",
				"github_name": "gh_user", "name": "正式姓名",
			},
		},
	})
	doc, err := ParseRawJSON(data, defaultPrices(), emptyOrgProvider())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.UserName != "正式姓名" {
		t.Errorf("UserName: want 正式姓名, got %s", doc.UserName)
	}
}

// TP-08: username fallback - name 为空则用 phone
func TestUsernameFromPhone(t *testing.T) {
	data := buildRawJSON(map[string]interface{}{
		"identity": map[string]interface{}{
			"task_id": "t1", "request_id": "r1",
			"client_id": "cid1234567890abc", "client_ide": "vscode",
			"client_version": "1.0", "client_os": "Windows",
			"user_name": "fallback", "project_path": "/p",
			"caller": "chat", "sender": "user",
			"user_info": map[string]interface{}{
				"uuid": "u1", "phone": "13800000001",
				"github_name": "gh_user", "name": "",
			},
		},
	})
	doc, err := ParseRawJSON(data, defaultPrices(), emptyOrgProvider())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.UserName != "13800000001" {
		t.Errorf("UserName: want 13800000001, got %s", doc.UserName)
	}
}

// TP-09: username fallback - name/phone 均为空则用 github_name
func TestUsernameFromGithubName(t *testing.T) {
	data := buildRawJSON(map[string]interface{}{
		"identity": map[string]interface{}{
			"task_id": "t1", "request_id": "r1",
			"client_id": "cid1234567890abc", "client_ide": "vscode",
			"client_version": "1.0", "client_os": "Windows",
			"user_name": "fallback", "project_path": "/p",
			"caller": "chat", "sender": "user",
			"user_info": map[string]interface{}{
				"uuid": "u1", "phone": "",
				"github_name": "gh_user", "name": "",
			},
		},
	})
	doc, err := ParseRawJSON(data, defaultPrices(), emptyOrgProvider())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.UserName != "gh_user" {
		t.Errorf("UserName: want gh_user, got %s", doc.UserName)
	}
}

// TP-10: username fallback - 全部为空则用 user_name
func TestUsernameFromUserName(t *testing.T) {
	data := buildRawJSON(map[string]interface{}{
		"identity": map[string]interface{}{
			"task_id": "t1", "request_id": "r1",
			"client_id": "cid1234567890abc", "client_ide": "vscode",
			"client_version": "1.0", "client_os": "Windows",
			"user_name": "fallback_user", "project_path": "/p",
			"caller": "chat", "sender": "user",
			"user_info": map[string]interface{}{
				"uuid": "u1", "phone": "",
				"github_name": "", "name": "",
			},
		},
	})
	doc, err := ParseRawJSON(data, defaultPrices(), emptyOrgProvider())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.UserName != "fallback_user" {
		t.Errorf("UserName: want fallback_user, got %s", doc.UserName)
	}
}

// ── TestProjectID ────────────────────────────────────────────────────────

// TP-11: project_id = client_id 前10字符 + ":" + project_path
func TestProjectID_FromClientIDAndPath(t *testing.T) {
	// client_id = "clientid1234567890abcdef" (前10字符: "clientid12")
	data := buildRawJSON(nil)
	doc, err := ParseRawJSON(data, defaultPrices(), emptyOrgProvider())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "clientid12:/workspace/proj"
	if doc.ProjectID != want {
		t.Errorf("ProjectID: want %q, got %q", want, doc.ProjectID)
	}
}

// TP-12: project_id - client_id 短于10字符时截取全部
func TestProjectID_ShortClientID(t *testing.T) {
	data := buildRawJSON(map[string]interface{}{
		"identity": map[string]interface{}{
			"task_id": "t1", "request_id": "r1",
			"client_id": "abc", // 只有3字符
			"client_ide": "vscode", "client_version": "1.0", "client_os": "Windows",
			"user_name": "u", "project_path": "/proj",
			"caller": "chat", "sender": "user",
			"user_info": map[string]interface{}{
				"uuid": "u1", "phone": "13800000001",
				"github_name": "", "name": "用户",
			},
		},
	})
	doc, err := ParseRawJSON(data, defaultPrices(), emptyOrgProvider())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "abc:/proj"
	if doc.ProjectID != want {
		t.Errorf("ProjectID: want %q, got %q", want, doc.ProjectID)
	}
}

// ── TestCalcUserInChars ──────────────────────────────────────────────────

// TP-13: sender=user，最后一条消息含 <user_message> 标签，中英文混合
func TestCalcUserInChars_Normal(t *testing.T) {
	// "hello world" = 11 英文字母+空格，可见ASCII: h,e,l,l,o,' ',w,o,r,l,d → 空格 0x20 不计，英文字母11-1=10？
	// countChars 规则: r > 0x20 && r < 0x7F 才计，空格 0x20 不计
	// "hello world" -> h,e,l,l,o = 5, w,o,r,l,d = 5 → 10
	// 但 space = 0x20, 0x20 不满足 > 0x20，所以空格不计
	text := "hello world" // 10个可见英文字符
	msgs := []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}{
		{Role: "system", Content: json.RawMessage(`"system prompt"`)},
		{Role: "user", Content: json.RawMessage(`"<user_message>` + text + `</user_message>"`)},
	}
	got := calcUserInChars("user", msgs)
	if got != 10 {
		t.Errorf("calcUserInChars: want 10, got %d", got)
	}
}

// TP-14: sender=user, 纯中文消息
func TestCalcUserInChars_CJK(t *testing.T) {
	// "你好" = 2个中文字符，每个计2 → 4
	msgs := []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}{
		{Role: "user", Content: json.RawMessage(`"<user_message>你好</user_message>"`)},
	}
	got := calcUserInChars("user", msgs)
	if got != 4 {
		t.Errorf("calcUserInChars CJK: want 4, got %d", got)
	}
}

// TP-15: sender=user，消息不含 <user_message> 标签 → 返回0
func TestCalcUserInChars_NoTag(t *testing.T) {
	msgs := []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}{
		{Role: "user", Content: json.RawMessage(`"plain message without tag"`)},
	}
	got := calcUserInChars("user", msgs)
	if got != 0 {
		t.Errorf("calcUserInChars no tag: want 0, got %d", got)
	}
}

// TP-16: sender=system → 返回0（不计算）
func TestCalcUserInChars_SenderSystem(t *testing.T) {
	msgs := []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}{
		{Role: "user", Content: json.RawMessage(`"<user_message>hello</user_message>"`)},
	}
	got := calcUserInChars("system", msgs)
	if got != 0 {
		t.Errorf("calcUserInChars sender=system: want 0, got %d", got)
	}
}

// TP-17: messages 为空 → 返回0
func TestCalcUserInChars_EmptyMessages(t *testing.T) {
	msgs := []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}{}
	got := calcUserInChars("user", msgs)
	if got != 0 {
		t.Errorf("calcUserInChars empty: want 0, got %d", got)
	}
}

// TP-18: <user_message> 无结束标签 → 提取到末尾
func TestCalcUserInChars_NoEndTag(t *testing.T) {
	// "abc" = 3
	msgs := []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}{
		{Role: "user", Content: json.RawMessage(`"<user_message>abc"`)},
	}
	got := calcUserInChars("user", msgs)
	if got != 3 {
		t.Errorf("calcUserInChars no end tag: want 3, got %d", got)
	}
}

// TP-19: content 是 []object 格式（非字符串）
func TestCalcUserInChars_ArrayContent(t *testing.T) {
	msgs := []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}{
		{Role: "user", Content: json.RawMessage(`[{"type":"text","text":"<user_message>hi</user_message>"}]`)},
	}
	got := calcUserInChars("user", msgs)
	if got != 2 { // h,i = 2
		t.Errorf("calcUserInChars array content: want 2, got %d", got)
	}
}

// ── TestCountChars ───────────────────────────────────────────────────────

// TP-20: countChars - 纯英文（小写 ASCII）
func TestCountChars_ASCII(t *testing.T) {
	got := countChars("hello") // 5
	if got != 5 {
		t.Errorf("countChars ASCII: want 5, got %d", got)
	}
}

// TP-21: countChars - 空格不计入
func TestCountChars_SpaceNotCounted(t *testing.T) {
	got := countChars("a b") // a=1, space不计, b=1 → 2
	if got != 2 {
		t.Errorf("countChars space: want 2, got %d", got)
	}
}

// TP-22: countChars - 中文每字2分
func TestCountChars_CJK(t *testing.T) {
	got := countChars("中文") // 4
	if got != 4 {
		t.Errorf("countChars CJK: want 4, got %d", got)
	}
}

// TP-23: countChars - 空字符串
func TestCountChars_Empty(t *testing.T) {
	got := countChars("")
	if got != 0 {
		t.Errorf("countChars empty: want 0, got %d", got)
	}
}

// ── TestCalcOutCodeLines ─────────────────────────────────────────────────

// TP-24: write_to_file tool_call 计算代码行数
func TestCalcOutCodeLines_WriteToFile(t *testing.T) {
	toolCalls := []struct {
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}{
		{Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{
			Name:      "write_to_file",
			Arguments: `{"path":"a.go","content":"line1\nline2\nline3"}`,
		}},
	}
	got := calcOutCodeLines(toolCalls)
	if got != 3 {
		t.Errorf("calcOutCodeLines write_to_file: want 3, got %d", got)
	}
}

// TP-25: apply_diff tool_call 计算代码行数
func TestCalcOutCodeLines_ApplyDiff(t *testing.T) {
	toolCalls := []struct {
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}{
		{Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{
			Name:      "apply_diff",
			Arguments: `{"path":"b.go","diff":"<<<<<<< SEARCH\nold1\nold2\n=======\nnew1\nnew2\n>>>>>>> REPLACE"}`,
		}},
	}
	got := calcOutCodeLines(toolCalls)
	if got != 2 {
		t.Errorf("calcOutCodeLines apply_diff: want 2, got %d", got)
	}
}

// TP-26: 非写文件 tool_call 不计入
func TestCalcOutCodeLines_OtherToolIgnored(t *testing.T) {
	toolCalls := []struct {
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}{
		{Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{
			Name:      "read_file",
			Arguments: `{"path":"a.go","content":"line1\nline2"}`,
		}},
	}
	got := calcOutCodeLines(toolCalls)
	if got != 0 {
		t.Errorf("calcOutCodeLines other tool: want 0, got %d", got)
	}
}

// TP-27: content 以 \n 结尾时末尾空行不计
func TestCalcOutCodeLines_TrailingNewline(t *testing.T) {
	toolCalls := []struct {
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}{
		{Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{
			Name:      "write_to_file",
			Arguments: `{"path":"a.go","content":"line1\nline2\n"}`,
		}},
	}
	got := calcOutCodeLines(toolCalls)
	if got != 2 {
		t.Errorf("calcOutCodeLines trailing newline: want 2, got %d", got)
	}
}

// TP-28: 多个 tool_call 累加
func TestCalcOutCodeLines_MultipleToolCalls(t *testing.T) {
	toolCalls := []struct {
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}{
		{Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{Name: "write_to_file", Arguments: `{"content":"a\nb"}`}},
		{Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{Name: "apply_diff", Arguments: `{"path":"c.go","diff":"<<<<<<< SEARCH\nold\n=======\nx\ny\nz\n>>>>>>> REPLACE"}`}},
	}
	got := calcOutCodeLines(toolCalls)
	if got != 5 {
		t.Errorf("calcOutCodeLines multiple: want 5, got %d", got)
	}
}

// TP-29: tool_calls 为空切片 → 返回0
func TestCalcOutCodeLines_Empty(t *testing.T) {
	toolCalls := []struct {
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}{}
	got := calcOutCodeLines(toolCalls)
	if got != 0 {
		t.Errorf("calcOutCodeLines empty: want 0, got %d", got)
	}
}

// ── TestCalculateCost ────────────────────────────────────────────────────

// TP-30: 已知模型正常计费
func TestCalculateCost_KnownModel(t *testing.T) {
	prices := defaultPrices()
	// GLM-4.7: in=0.5/M, out=1.0/M
	// 1000000 in_tokens + 500000 out_tokens
	// = 1.0 * 0.5 + 0.5 * 1.0 = 0.5 + 0.5 = 1.0
	got := calculateCost("GLM-4.7", 1_000_000, 500_000, prices)
	want := 1.0
	if got != want {
		t.Errorf("calculateCost GLM-4.7: want %.4f, got %.4f", want, got)
	}
}

// TP-31: 未知模型返回 0
func TestCalculateCost_UnknownModel(t *testing.T) {
	got := calculateCost("Unknown-Model", 1000, 500, defaultPrices())
	if got != 0 {
		t.Errorf("calculateCost unknown: want 0, got %f", got)
	}
}

// TP-32: Auto 模型（in_price=out_price=0）返回 0
func TestCalculateCost_AutoModel(t *testing.T) {
	got := calculateCost("Auto", 100_000, 50_000, defaultPrices())
	if got != 0 {
		t.Errorf("calculateCost Auto: want 0, got %f", got)
	}
}

// TP-33: tokens 为 0 → 费用为 0
func TestCalculateCost_ZeroTokens(t *testing.T) {
	got := calculateCost("GLM-4.7", 0, 0, defaultPrices())
	if got != 0 {
		t.Errorf("calculateCost zero tokens: want 0, got %f", got)
	}
}

// TP-34: api_cost 通过 ParseRawJSON 集成验证
func TestParseRawJSON_APICostIntegration(t *testing.T) {
	// usage: prompt=1_000_000, completion=0
	// GLM-4.7 in=0.5/M → cost = 0.5
	data := buildRawJSON(map[string]interface{}{
		"usage": map[string]interface{}{
			"prompt_tokens":     int64(1_000_000),
			"completion_tokens": int64(0),
		},
	})
	doc, err := ParseRawJSON(data, defaultPrices(), emptyOrgProvider())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := 0.5
	if doc.APICost != want {
		t.Errorf("APICost: want %.4f, got %.4f", want, doc.APICost)
	}
}

// ── TestParseRawJSON_RealWorldData ───────────────────────────────────────

// TP-35: 基于真实数据结构的集成测试（sender=system，user_in_chars 应=0）
func TestParseRawJSON_SenderSystem(t *testing.T) {
	data := buildRawJSON(map[string]interface{}{
		"identity": map[string]interface{}{
			"task_id": "task-sys", "request_id": "req-sys",
			"client_id": "ccba87428a6c8ba28febe7e3db61", "client_ide": "vscode",
			"client_version": "2.5.3", "client_os": "Windows",
			"user_name": "13129925206", "project_path": `d:\zsj\dmp-web`,
			"caller": "chat", "sender": "system",
			"user_info": map[string]interface{}{
				"uuid": "bf3ad850-a390-4192-bccc-c5d1140326ee",
				"phone": "13129925206", "github_name": "", "name": "13129925206",
			},
		},
	})
	doc, err := ParseRawJSON(data, defaultPrices(), emptyOrgProvider())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.UserInChars != 0 {
		t.Errorf("sender=system 时 UserInChars 应=0, got %d", doc.UserInChars)
	}
}

// TP-36: 真实数据 - usage 全为 0（报错场景）时 cost = 0
func TestParseRawJSON_ZeroUsage(t *testing.T) {
	data := buildRawJSON(map[string]interface{}{
		"usage": map[string]interface{}{
			"prompt_tokens":     int64(0),
			"completion_tokens": int64(0),
		},
	})
	doc, err := ParseRawJSON(data, defaultPrices(), emptyOrgProvider())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.APICost != 0 {
		t.Errorf("zero usage 时 APICost 应=0, got %f", doc.APICost)
	}
}
