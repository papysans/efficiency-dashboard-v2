package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestCSV(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "org.csv")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("写入测试CSV失败: %v", err)
	}
	return path
}

// TestNewOrgProvider_Normal 正常加载 CSV
func TestNewOrgProvider_Normal(t *testing.T) {
	csv := "user_id,user_name,org1,org2,org3,org4\nuser-001,alice,公司A,部门1,组1,团队1\nuser-003,charlie,公司C,部门3,组3,团队3\n"
	path := writeTestCSV(t, csv)

	p, err := NewOrgProvider(path)
	if err != nil {
		t.Fatalf("NewOrgProvider 返回错误: %v", err)
	}
	if p.Count() != 2 {
		t.Errorf("Count: want 2, got %d", p.Count())
	}
}

// TestGetOrgInfo_ByUserID user_id 精确匹配
func TestGetOrgInfo_ByUserID(t *testing.T) {
	csv := "user_id,user_name,org1,org2,org3,org4\nuser-001,alice,公司A,部门1,组1,团队1\n"
	path := writeTestCSV(t, csv)

	p, err := NewOrgProvider(path)
	if err != nil {
		t.Fatalf("NewOrgProvider 返回错误: %v", err)
	}

	info := p.GetOrgInfo("user-001", "")
	if info.Org1 != "公司A" {
		t.Errorf("Org1: want 公司A, got %s", info.Org1)
	}
	if info.Org2 != "部门1" {
		t.Errorf("Org2: want 部门1, got %s", info.Org2)
	}
	if info.Org3 != "组1" {
		t.Errorf("Org3: want 组1, got %s", info.Org3)
	}
	if info.Org4 != "团队1" {
		t.Errorf("Org4: want 团队1, got %s", info.Org4)
	}
}

// TestGetOrgInfo_ByUserName user_id 未命中时 fallback 到 user_name
func TestGetOrgInfo_ByUserName(t *testing.T) {
	csv := "user_id,user_name,org1,org2,org3,org4\nuser-001,alice,公司A,部门1,组1,团队1\n"
	path := writeTestCSV(t, csv)

	p, err := NewOrgProvider(path)
	if err != nil {
		t.Fatalf("NewOrgProvider 返回错误: %v", err)
	}

	info := p.GetOrgInfo("no-match", "alice")
	if info.Org1 != "公司A" {
		t.Errorf("Org1: want 公司A, got %s", info.Org1)
	}
}

// TestGetOrgInfo_NotFound 都未匹配返回空 OrgInfo
func TestGetOrgInfo_NotFound(t *testing.T) {
	csv := "user_id,user_name,org1,org2,org3,org4\nuser-001,alice,公司A,部门1,组1,团队1\n"
	path := writeTestCSV(t, csv)

	p, err := NewOrgProvider(path)
	if err != nil {
		t.Fatalf("NewOrgProvider 返回错误: %v", err)
	}

	info := p.GetOrgInfo("no-match", "no-match")
	if info.Org1 != "" || info.Org2 != "" || info.Org3 != "" || info.Org4 != "" {
		t.Errorf("未匹配时应返回空 OrgInfo, got %+v", info)
	}
}

// TestNewOrgProvider_EmptyUserID CSV 中 user_id 为空但 user_name 有值
func TestNewOrgProvider_EmptyUserID(t *testing.T) {
	csv := "user_id,user_name,org1,org2,org3,org4\n,bob,公司B,部门2,组2,团队2\n"
	path := writeTestCSV(t, csv)

	p, err := NewOrgProvider(path)
	if err != nil {
		t.Fatalf("NewOrgProvider 返回错误: %v", err)
	}
	// user_id 为空不计入 userIDMap
	if p.Count() != 0 {
		t.Errorf("Count: want 0 (user_id为空), got %d", p.Count())
	}
	// 但 user_name 仍可查询
	info := p.GetOrgInfo("", "bob")
	if info.Org1 != "公司B" {
		t.Errorf("Org1: want 公司B, got %s", info.Org1)
	}
}

// TestNewOrgProvider_FileNotExist CSV 文件不存在返回 error
func TestNewOrgProvider_FileNotExist(t *testing.T) {
	_, err := NewOrgProvider("/nonexistent/path/org.csv")
	if err == nil {
		t.Error("期望返回 error，但未返回")
	}
}

// TestCount Count() 方法返回正确的条目数
func TestCount(t *testing.T) {
	csv := "user_id,user_name,org1,org2,org3,org4\nuser-001,alice,公司A,部门1,组1,团队1\n,bob,公司B,部门2,组2,团队2\nuser-003,charlie,公司C,部门3,组3,团队3\n"
	path := writeTestCSV(t, csv)

	p, err := NewOrgProvider(path)
	if err != nil {
		t.Fatalf("NewOrgProvider 返回错误: %v", err)
	}
	// 只有 user-001 和 user-003 有 user_id
	if p.Count() != 2 {
		t.Errorf("Count: want 2, got %d", p.Count())
	}
}

// TestLookupByGitAuthor_WithGitColumns CSV 包含 git 列时能正确映射
func TestLookupByGitAuthor_WithGitColumns(t *testing.T) {
	csv := "user_id,user_name,org1,org2,org3,org4,git_user_name,git_user_email\nuser-001,alice,公司A,部门1,组1,团队1,alice-git,alice@git.com\nuser-002,bob,公司B,部门2,组2,团队2,bob-git,bob@git.com\n"
	path := writeTestCSV(t, csv)

	p, err := NewOrgProvider(path)
	if err != nil {
		t.Fatalf("NewOrgProvider 返回错误: %v", err)
	}

	// 按 email 查找
	uid, found := p.LookupByGitAuthor("", "alice@git.com")
	if !found {
		t.Fatal("LookupByGitAuthor by email 期望找到")
	}
	if uid != "user-001" {
		t.Errorf("期望 user-001, 实际 %s", uid)
	}

	// 按 name 查找
	uid, found = p.LookupByGitAuthor("bob-git", "")
	if !found {
		t.Fatal("LookupByGitAuthor by name 期望找到")
	}
	if uid != "user-002" {
		t.Errorf("期望 user-002, 实际 %s", uid)
	}

	// email 优先于 name：给 alice 的 email 但 bob 的 name
	uid, found = p.LookupByGitAuthor("bob-git", "alice@git.com")
	if !found {
		t.Fatal("LookupByGitAuthor 期望找到")
	}
	if uid != "user-001" {
		t.Errorf("email 应优先: 期望 user-001, 实际 %s", uid)
	}

	// 不存在的 git 用户
	_, found = p.LookupByGitAuthor("unknown", "unknown@other.com")
	if found {
		t.Error("不存在的 git 用户应返回 found=false")
	}
}

// TestLookupByGitAuthor_WithoutGitColumns CSV 无 git 列时返回 found=false
func TestLookupByGitAuthor_WithoutGitColumns(t *testing.T) {
	csv := "user_id,user_name,org1,org2,org3,org4\nuser-001,alice,公司A,部门1,组1,团队1\n"
	path := writeTestCSV(t, csv)

	p, err := NewOrgProvider(path)
	if err != nil {
		t.Fatalf("NewOrgProvider 返回错误: %v", err)
	}

	_, found := p.LookupByGitAuthor("alice", "alice@example.com")
	if found {
		t.Error("CSV 无 git 列时 LookupByGitAuthor 应返回 found=false")
	}
}
