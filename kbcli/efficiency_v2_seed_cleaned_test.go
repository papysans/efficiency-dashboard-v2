//go:build seedclean

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"kanban/core/models"

	"gorm.io/gorm/clause"
)

// TestEfficiencyV2SeedCleanedData seeds the cleaned downstream data into the
// configured database. Wipe + reseed.
// gate: SEED_CLEAN_DSN=host=... port=... user=... password=... dbname=... sslmode=disable
// data root: SEED_CLEAN_ROOT=./.local/cleaned
func TestEfficiencyV2SeedCleanedData(t *testing.T) {
	dsn := os.Getenv("SEED_CLEAN_DSN")
	if dsn == "" {
		t.Skip("SEED_CLEAN_DSN not set")
	}
	root := os.Getenv("SEED_CLEAN_ROOT")
	if root == "" {
		root = filepath.Join("..", ".local", "cleaned")
	}
	db, err := models.OpenGormDB(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	// ---- 1. Wipe v2 + base tables ----
	tables := []string{
		"user_productivity_v2", "needs", "session_stage_metrics", "conversation_events",
		"user_productivity", "tasks", "conversations", "commits", "sessions", "user_org",
		"task_manual_ground_truth",
	}
	for _, tbl := range tables {
		if err := db.Exec("TRUNCATE TABLE " + tbl + " CASCADE").Error; err != nil {
			t.Logf("truncate %s: %v (ignoring)", tbl, err)
		}
	}
	t.Logf("step1 wipe done")

	// ---- 2. Load commits + build user_org map ----
	type rawCommit struct {
		CommitId     string   `json:"commit_id"`
		CommitTime   string   `json:"commit_time"`
		RepoAddr     string   `json:"repo_addr"`
		RepoBranch   string   `json:"repo_branch"`
		GitUserName  string   `json:"git_user_name"`
		GitUserEmail string   `json:"git_user_email"`
		UserId       string   `json:"user_id"`
		UserName     string   `json:"user_name"`
		ClientId     string   `json:"client_id"`
		ClientIde    string   `json:"client_ide"`
		WorkDir      string   `json:"work_dir"`
		WorkPath     string   `json:"work_path"`
		DiffLines    int      `json:"diff_lines"`
		Comment      string   `json:"comment"`
		Files        []string `json:"files"`
		Branches     []string `json:"branches"`
	}
	commitRows := []models.Commit{}
	userOrgMap := map[string]string{}        // user_id → user_name
	workDirToUser := map[string]string{}     // work_dir → user_id（用来回推 conversation 的 user_id）

	scanJSONL(t, filepath.Join(root, "cleaned_commits.jsonl"), func(line []byte) {
		var rc rawCommit
		if err := json.Unmarshal(line, &rc); err != nil {
			t.Logf("bad commit: %v", err); return
		}
		ct := parseSeedTime(rc.CommitTime)
		workDir := rc.WorkDir
		if workDir == "" {
			workDir = rc.WorkPath
		}
		commitRows = append(commitRows, models.Commit{
			CommitId:     rc.CommitId,
			CommitTime:   ct,
			RepoAddr:     rc.RepoAddr,
			RepoBranch:   rc.RepoBranch,
			GitUserName:  rc.GitUserName,
			GitUserEmail: rc.GitUserEmail,
			UserId:       rc.UserId,
			UserName:     rc.UserName,
			ClientId:     rc.ClientId,
			WorkDir:      workDir,
			WorkDirId:    workDirToID(rc.ClientId, workDir),
			DiffLines:    rc.DiffLines,
			Comment:      rc.Comment,
		})
		if rc.UserId != "" {
			if name := rc.UserName; name != "" {
				userOrgMap[rc.UserId] = name
			} else {
				userOrgMap[rc.UserId] = rc.GitUserName
			}
		}
		if rc.UserId != "" && workDir != "" {
			workDirToUser[workDir] = rc.UserId
		}
	})
	t.Logf("step2 loaded %d commits, %d unique users, %d workdir→user mappings", len(commitRows), len(userOrgMap), len(workDirToUser))

	// ---- 3. Insert user_org ----
	userRows := make([]models.UserOrg, 0, len(userOrgMap))
	for uid, uname := range userOrgMap {
		userRows = append(userRows, models.UserOrg{UserId: uid, UserName: uname, Org1: "cleaned-import"})
	}
	if err := db.CreateInBatches(&userRows, 200).Error; err != nil {
		t.Fatalf("seed user_org: %v", err)
	}
	t.Logf("step3 seeded %d user_org rows", len(userRows))

	// ---- 4. Insert commits ----
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "commit_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"commit_time", "repo_addr", "repo_branch", "user_id", "user_name", "diff_lines", "comment"}),
	}).CreateInBatches(&commitRows, 200).Error; err != nil {
		t.Fatalf("seed commits: %v", err)
	}
	t.Logf("step4 seeded %d commits", len(commitRows))

	// ---- 5. Load conversations + group by task_id → build sessions ----
	type rawConv struct {
		TaskId           string   `json:"task_id"`
		RequestId        string   `json:"request_id"`
		PromptMode       string   `json:"prompt_mode"`
		Mode             string   `json:"mode"`
		Model            string   `json:"model"`
		StartTime        string   `json:"start_time"`
		EndTime          string   `json:"end_time"`
		ProcessTime      *int64   `json:"process_time"`
		ProcessTtft      *int64   `json:"process_ttft"`
		UpstreamTokens   *int64   `json:"upstream_tokens"`
		DownstreamTokens *int64   `json:"downstream_tokens"`
		Cost             *float64 `json:"cost"`
		Sender           string   `json:"sender"`
		RequestContent   string   `json:"request_content"`
		ResponseContent  string   `json:"response_content"`
		UserInput        string   `json:"user_input"`
		Diff             string   `json:"diff"`
		DiffLines        *int64   `json:"diff_lines"`
		RepoAddr         string   `json:"repo_addr"`
		RepoBranch       string   `json:"repo_branch"`
		WorkDir          string   `json:"work_dir"`
	}
	deref := func(p *int64) int64 { if p == nil { return 0 }; return *p }
	derefF := func(p *float64) float64 { if p == nil { return 0 }; return *p }

	convRows := []models.Conversation{}
	sessionInfo := map[string]*models.Session{} // task_id → session
	dedupConv := map[string]bool{}              // (task_id, request_id)

	scanJSONL(t, filepath.Join(root, "cleaned_conversations.jsonl"), func(line []byte) {
		var rc rawConv
		if err := json.Unmarshal(line, &rc); err != nil {
			return
		}
		if rc.TaskId == "" || rc.RequestId == "" {
			return
		}
		key := rc.TaskId + "|" + rc.RequestId
		if dedupConv[key] {
			return
		}
		dedupConv[key] = true

		st := parseSeedTime(rc.StartTime)
		et := parseSeedTime(rc.EndTime)
		userID := workDirToUser[rc.WorkDir] // 反查；查不到留空

		convRows = append(convRows, models.Conversation{
			SessionId:        rc.TaskId,
			RequestId:        rc.RequestId,
			TaskId:           rc.TaskId,
			Sender:           sanitizePGString(rc.Sender),
			PromptMode:       sanitizePGString(rc.PromptMode),
			Mode:             sanitizePGString(rc.Mode),
			Model:            sanitizePGString(rc.Model),
			StartTime:        st,
			EndTime:          et,
			ProcessTime:      deref(rc.ProcessTime),
			ProcessTtft:      deref(rc.ProcessTtft),
			UpstreamTokens:   deref(rc.UpstreamTokens),
			DownstreamTokens: deref(rc.DownstreamTokens),
			Cost:             derefF(rc.Cost),
			DiffLines:        deref(rc.DiffLines),
			RepoAddr:         sanitizePGString(rc.RepoAddr),
			RepoBranch:       sanitizePGString(rc.RepoBranch),
			WorkDir:          sanitizePGString(rc.WorkDir),
			WorkDirId:        workDirToID("", rc.WorkDir),
			UserInput:        truncateStr(rc.UserInput, 8000),
			RequestContent:   truncateStr(rc.RequestContent, 8000),
			ResponseContent:  truncateStr(rc.ResponseContent, 8000),
		})

		// session: 每个 task 一个 session，CreateTime = 最早的 conversation start_time
		s, ok := sessionInfo[rc.TaskId]
		if !ok {
			s = &models.Session{
				SessionId:        rc.TaskId,
				CreateTime:       st,
				UserId:           userID,
				UserName:         userOrgMap[userID],
				ClientIde:        "cleaned",
				SessionDate:      st.Format("2006/01/02"),
				ConversationDate: st.Format("2006/01/02"),
			}
			sessionInfo[rc.TaskId] = s
		} else {
			if !st.IsZero() && (s.CreateTime.IsZero() || st.Before(s.CreateTime)) {
				s.CreateTime = st
			}
			if s.UserId == "" && userID != "" {
				s.UserId = userID
				s.UserName = userOrgMap[userID]
			}
		}
	})

	sessionRows := make([]models.Session, 0, len(sessionInfo))
	for _, s := range sessionInfo {
		sessionRows = append(sessionRows, *s)
	}
	sort.SliceStable(sessionRows, func(i, j int) bool { return sessionRows[i].SessionId < sessionRows[j].SessionId })

	// ---- 6. Insert sessions ----
	if err := db.CreateInBatches(&sessionRows, 500).Error; err != nil {
		t.Fatalf("seed sessions: %v", err)
	}
	t.Logf("step6 seeded %d sessions", len(sessionRows))

	// ---- 7. Insert conversations in chunks ----
	if err := db.CreateInBatches(&convRows, 500).Error; err != nil {
		t.Fatalf("seed conversations: %v", err)
	}
	t.Logf("step7 seeded %d conversations", len(convRows))

	// ---- 8. Insert tasks (so LLM prompt can pick titles) ----
	// 我们这次没 title 信息（cleaned data 里 task 没 title 字段），跳过 tasks 表
	// efficiency-v2 不强依赖 tasks 表，跳过没问题

	// ---- 9. Quick verification ----
	var counts struct {
		Sessions      int64
		Conversations int64
		Commits       int64
		Users         int64
	}
	db.Raw("SELECT (SELECT COUNT(*) FROM sessions) AS sessions, (SELECT COUNT(*) FROM conversations) AS conversations, (SELECT COUNT(*) FROM commits) AS commits, (SELECT COUNT(*) FROM user_org) AS users").Scan(&counts)
	t.Logf("seed verify: sessions=%d conversations=%d commits=%d users=%d",
		counts.Sessions, counts.Conversations, counts.Commits, counts.Users)

	t.Logf("\n下一步：")
	t.Logf("  .local/bin/kbcli --config .local/kbcli-config.yaml efficiency-v2 \\")
	t.Logf("    --start-date 20260518 --end-date 20260522")
}

// helpers ────────────────────────────────────────────────

func scanJSONL(t *testing.T, path string, fn func([]byte)) {
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 64*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		fn(line)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan %s: %v", path, err)
	}
}

func parseSeedTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05.999Z",
		"2006-01-02T15:04:05.999-07:00",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func workDirToID(clientID, workDir string) string {
	wd := strings.TrimSpace(workDir)
	if wd == "" {
		return ""
	}
	return strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(clientID+"-"+wd), "/", "-"), " ", "-")[:min(200, len(clientID+wd)+1)]
}

func truncateStr(s string, max int) string {
	// First strip null bytes (Postgres TEXT 拒绝) + invalid UTF-8
	s = strings.ReplaceAll(s, "\x00", "")
	s = strings.ToValidUTF8(s, "")
	if len(s) <= max {
		return s
	}
	// 截断后再 validate 一次（截断可能砍在多字节字符中间）
	return strings.ToValidUTF8(s[:max], "") + "…[truncated]"
}

func sanitizePGString(s string) string {
	s = strings.ReplaceAll(s, "\x00", "")
	return strings.ToValidUTF8(s, "")
}

func min(a, b int) int { if a < b { return a }; return b }

var _ = fmt.Sprintf
