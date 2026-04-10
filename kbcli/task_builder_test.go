package main

import (
	"testing"
	"time"
)

// TestBuildTaskDocs_Normal 多条相同 task_id 的记录正确聚合
func TestBuildTaskDocs_Normal(t *testing.T) {
	now := time.Now().UTC()
	docs := []RawDoc{
		{
			TaskID: "task-001", Caller: "chat",
			ClientID: "cid1", UserID: "u1", UserName: "张三",
			ProjectPath: "/proj", ProjectID: "p1",
			ClientIDE: "vscode", ClientVersion: "1.0", ClientOS: "Windows",
			PromptMode: "vibe", Mode: "code",
			Org1: "公司A", Org2: "部门1", Org3: "组1", Org4: "团队1",
			UserInChars: 10, AssistantOutCodeLines: 5,
			SystemTokens: 100, UserTokens: 50,
			APIProcessTime: 1000, APITtft: 200,
			APIInTokens: 300, APIOutTokens: 150,
			APICost:        0.5,
			APIRequestTime: now,
			APIEndTime:     now.Add(1 * time.Second),
		},
		{
			TaskID: "task-001", Caller: "chat",
			ClientID: "cid1", UserID: "u1", UserName: "张三",
			ProjectPath: "/proj", ProjectID: "p1",
			ClientIDE: "vscode", ClientVersion: "1.0", ClientOS: "Windows",
			PromptMode: "vibe", Mode: "code",
			Org1: "公司A", Org2: "部门1", Org3: "组1", Org4: "团队1",
			UserInChars: 20, AssistantOutCodeLines: 10,
			SystemTokens: 200, UserTokens: 100,
			APIProcessTime: 2000, APITtft: 300,
			APIInTokens: 600, APIOutTokens: 300,
			APICost:        1.0,
			APIRequestTime: now.Add(-1 * time.Second),
			APIEndTime:     now.Add(3 * time.Second),
		},
	}

	results := BuildTaskDocs(docs)
	if len(results) != 1 {
		t.Fatalf("期望1个TaskDoc, 得到 %d", len(results))
	}
	td := results[0]

	if td.TaskID != "task-001" {
		t.Errorf("TaskID: want task-001, got %s", td.TaskID)
	}
	if td.APICount != 2 {
		t.Errorf("APICount: want 2, got %d", td.APICount)
	}
	if td.UserInChars != 30 {
		t.Errorf("UserInChars: want 30, got %d", td.UserInChars)
	}
	if td.AssistantOutCodeLines != 15 {
		t.Errorf("AssistantOutCodeLines: want 15, got %d", td.AssistantOutCodeLines)
	}
	if td.SystemTokens != 300 {
		t.Errorf("SystemTokens: want 300, got %d", td.SystemTokens)
	}
	if td.UserTokens != 150 {
		t.Errorf("UserTokens: want 150, got %d", td.UserTokens)
	}
	if td.APIProcessTime != 3000 {
		t.Errorf("APIProcessTime: want 3000, got %d", td.APIProcessTime)
	}
	if td.APIInTokens != 900 {
		t.Errorf("APIInTokens: want 900, got %d", td.APIInTokens)
	}
	if td.APIOutTokens != 450 {
		t.Errorf("APIOutTokens: want 450, got %d", td.APIOutTokens)
	}
	if td.APICost != 1.5 {
		t.Errorf("APICost: want 1.5, got %f", td.APICost)
	}
	// APIRequestTime 取 min
	if !td.APIRequestTime.Equal(now.Add(-1 * time.Second)) {
		t.Errorf("APIRequestTime 应取最小值")
	}
	// APIEndTime 取 max
	if !td.APIEndTime.Equal(now.Add(3 * time.Second)) {
		t.Errorf("APIEndTime 应取最大值")
	}
	// 标识字段取第一条
	if td.UserName != "张三" {
		t.Errorf("UserName: want 张三, got %s", td.UserName)
	}
	if td.Org1 != "公司A" {
		t.Errorf("Org1: want 公司A, got %s", td.Org1)
	}
}

// TestBuildTaskDocs_SingleRequest 单请求 task
func TestBuildTaskDocs_SingleRequest(t *testing.T) {
	now := time.Now().UTC()
	docs := []RawDoc{
		{
			TaskID: "task-single", Caller: "chat",
			ClientID: "cid1", UserID: "u1", UserName: "李四",
			APIRequestTime: now, APIEndTime: now.Add(1 * time.Second),
			UserInChars: 5, APIInTokens: 100, APIOutTokens: 50,
		},
	}

	results := BuildTaskDocs(docs)
	if len(results) != 1 {
		t.Fatalf("期望1个TaskDoc, 得到 %d", len(results))
	}
	if results[0].APICount != 1 {
		t.Errorf("APICount: want 1, got %d", results[0].APICount)
	}
	if results[0].UserInChars != 5 {
		t.Errorf("UserInChars: want 5, got %d", results[0].UserInChars)
	}
}

// TestBuildTaskDocs_FilterNonChat caller != "chat" 的记录不参与聚合
func TestBuildTaskDocs_FilterNonChat(t *testing.T) {
	now := time.Now().UTC()
	docs := []RawDoc{
		{
			TaskID: "task-mixed", Caller: "chat",
			APIRequestTime: now, APIEndTime: now.Add(1 * time.Second),
			UserInChars: 10,
		},
		{
			TaskID: "task-mixed", Caller: "completion",
			APIRequestTime: now, APIEndTime: now.Add(1 * time.Second),
			UserInChars: 999,
		},
	}

	results := BuildTaskDocs(docs)
	if len(results) != 1 {
		t.Fatalf("期望1个TaskDoc, 得到 %d", len(results))
	}
	if results[0].APICount != 1 {
		t.Errorf("APICount: want 1 (只计 chat), got %d", results[0].APICount)
	}
	if results[0].UserInChars != 10 {
		t.Errorf("UserInChars: want 10, got %d", results[0].UserInChars)
	}
}

// TestBuildTaskDocs_EmptyInput nil 和空 slice 返回空 slice
func TestBuildTaskDocs_EmptyInput(t *testing.T) {
	results := BuildTaskDocs(nil)
	if len(results) != 0 {
		t.Errorf("nil 输入: 期望空 slice, 得到 %d 条", len(results))
	}

	results = BuildTaskDocs([]RawDoc{})
	if len(results) != 0 {
		t.Errorf("空 slice 输入: 期望空 slice, 得到 %d 条", len(results))
	}
}

// TestBuildTaskDocs_EmptyTaskID task_id 为空的记录被跳过
func TestBuildTaskDocs_EmptyTaskID(t *testing.T) {
	now := time.Now().UTC()
	docs := []RawDoc{
		{
			TaskID: "", Caller: "chat",
			APIRequestTime: now, APIEndTime: now.Add(1 * time.Second),
		},
		{
			TaskID: "task-valid", Caller: "chat",
			APIRequestTime: now, APIEndTime: now.Add(1 * time.Second),
			UserInChars: 5,
		},
	}

	results := BuildTaskDocs(docs)
	if len(results) != 1 {
		t.Fatalf("期望1个TaskDoc (空task_id被跳过), 得到 %d", len(results))
	}
	if results[0].TaskID != "task-valid" {
		t.Errorf("TaskID: want task-valid, got %s", results[0].TaskID)
	}
}
