//go:build integration

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"kanban/core/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 端到端复现截图场景（招商局成员花名册）：走真实 HTTP → handler → aggregateUsersV2 →
// 真库 SQL，验证「AI 代码占比 '-' 但含硅量有值」这一核心诉求是否成立。
//
// 跑法：
//
//	SILICA_TEST_DSN="host=127.0.0.1 port=5442 user=postgres password=1 dbname=v2_silica_e2e sslmode=disable" \
//	  go test ./backend/ -tags integration -run E2E -v

func resetE2EDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := models.OpenGormDB(silicaTestDSN())
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.Exec("TRUNCATE TABLE commits, needs, user_productivity_v2").Error; err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return db
}

func weekStart(s string) time.Time {
	v, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return v
}

// seedE2EScreenshotScenario 构造三类成员，对应截图里能观察到的三种情况。
func seedE2EScreenshotScenario(t *testing.T, db *gorm.DB) {
	t.Helper()

	// 周表：三个人都有看板活动。u-dengbin 故意保留 need 反推得到的 Commit/代码行 0，
	// 用于验证用户汇总会改取 commits 直聚的 3/240。
	upv := []models.UserProductivityV2{
		// 邓彬型：合并需求 1，但 need 侧配不上 commit（见下方 needs 只给 u-full）。
		{UserProductivityV2Id: "w-dengbin", WeekStart: weekStart("2026-07-13"), UserId: "u-dengbin", UserName: "邓彬",
			MergedNeedCount: 1, CommitCount: 0, CommitDiffLines: 0},
		// 真没用 AI 型：有提交，但 commit 指纹一行没中。
		{UserProductivityV2Id: "w-noai", WeekStart: weekStart("2026-07-13"), UserId: "u-noai", UserName: "手写型",
			MergedNeedCount: 1, CommitCount: 2, CommitDiffLines: 180},
		// 正常型：need 侧也配得上，两个指标都该有值。
		{UserProductivityV2Id: "w-full", WeekStart: weekStart("2026-07-13"), UserId: "u-full", UserName: "正常型",
			MergedNeedCount: 1, CommitCount: 1, CommitDiffLines: 100},
	}
	if err := db.Create(&upv).Error; err != nil {
		t.Fatalf("seed user_productivity_v2: %v", err)
	}

	// needs：只有 u-full 有一条可计入的 need。
	// 注意 needSoftwareUserCaliberSQL 要求"该 user 至少有一个带 session_ids 的 need"，
	// 所以这条 need 必须带 session_ids，否则连 u-full 也会被人级闸滤掉。
	devEnd := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	needs := []models.Need{{
		NeedId: "n-full", PrimaryUserId: "u-full", Status: "merged",
		RepoAddr: "git@example.com/x.git", RepoBranch: "feature/x",
		SessionIds: models.StringJSON(`["s-1"]`),
		DevEndTs:   &devEnd,
		ChangedLoc: 100, AICoveredLoc: 60,
		OutlierFlag: false, CoverageEligible: true,
	}}
	if err := db.Create(&needs).Error; err != nil {
		t.Fatalf("seed needs: %v", err)
	}

	// commits：含硅量与用户提交指标的唯一数据源，不经 need 边界。
	commits := []models.Commit{
		// 邓彬：3 个 commit 共 240 行，其中 150 行指纹命中 → 150/240 = 0.625
		{CommitId: "c-d1", UserId: "u-dengbin", CommitTime: ts("2026-07-14T10:00:00Z"), DiffLines: 100, Silica: 0.8}, // 80
		{CommitId: "c-d2", UserId: "u-dengbin", CommitTime: ts("2026-07-14T14:00:00Z"), DiffLines: 100, Silica: 0.6}, // 60
		{CommitId: "c-d3", UserId: "u-dengbin", CommitTime: ts("2026-07-15T09:00:00Z"), DiffLines: 40, Silica: 0.25}, // 10
		// 手写型：有行数，零命中 → 0（不是 nil）
		{CommitId: "c-n1", UserId: "u-noai", CommitTime: ts("2026-07-14T10:00:00Z"), DiffLines: 80, Silica: 0},
		{CommitId: "c-n2", UserId: "u-noai", CommitTime: ts("2026-07-14T11:00:00Z"), DiffLines: 100, Silica: 0},
		// 正常型
		{CommitId: "c-f1", UserId: "u-full", CommitTime: ts("2026-07-14T10:00:00Z"), DiffLines: 100, Silica: 0.42},
	}
	if err := db.Create(&commits).Error; err != nil {
		t.Fatalf("seed commits: %v", err)
	}
}

func newE2ERouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v2/users", listUsersV2Native)
	return r
}

func TestE2E_UsersAPI_SilicaFillsWhereAICodeRatioIsDash(t *testing.T) {
	db := resetE2EDB(t)
	seedE2EScreenshotScenario(t, db)

	prev := statDB
	statDB = db
	defer func() { statDB = prev }()

	req := httptest.NewRequest(http.MethodGet, "/api/v2/users?startDate=2026-07-01&endDate=2026-07-31", nil)
	w := httptest.NewRecorder()
	newE2ERouter().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Total int `json:"total"`
		Data  []struct {
			UserId          string   `json:"user_id"`
			UserName        string   `json:"user_name"`
			CommitCount     int64    `json:"commit_count"`
			CommitDiffLines int64    `json:"commit_diff_lines"`
			AICodeRatio     *float64 `json:"ai_code_ratio"`
			Silica          *float64 `json:"silica"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v — body=%s", err, w.Body.String())
	}

	byUser := map[string]int{}
	for i, row := range resp.Data {
		byUser[row.UserId] = i
	}
	get := func(uid string) (*float64, *float64, int64, int64) {
		t.Helper()
		i, ok := byUser[uid]
		if !ok {
			t.Fatalf("%s 不在返回里；body=%s", uid, w.Body.String())
		}
		r := resp.Data[i]
		return r.AICodeRatio, r.Silica, r.CommitCount, r.CommitDiffLines
	}

	// ① 邓彬型——本次改动的核心目标：
	//    AI 代码占比 nil（前端 '-'，与截图一致），提交指标与含硅量均来自 commits 直聚。
	air, sil, cc, cdl := get("u-dengbin")
	if air != nil {
		t.Fatalf("u-dengbin 的 ai_code_ratio 应为 nil（need 侧配不上），got %v", *air)
	}
	if cc != 3 || cdl != 240 {
		t.Fatalf("u-dengbin commits 直聚应为 commit=3/lines=240，got %d/%d", cc, cdl)
	}
	if sil == nil {
		t.Fatal("u-dengbin 的 silica 不应为 nil —— 换成含硅量后本行应该出数（本次改动的核心诉求）")
	}
	// (100*0.8 + 100*0.6 + 40*0.25) / 240 = 150/240
	if want := 150.0 / 240.0; *sil < want-1e-9 || *sil > want+1e-9 {
		t.Fatalf("u-dengbin 含硅量 got %v, want %v", *sil, want)
	}

	// ② 真没用 AI——silica 不是万能药，该 0 就 0（且必须区别于 nil）。
	air, sil, _, _ = get("u-noai")
	if air != nil {
		t.Fatalf("u-noai 的 ai_code_ratio 应为 nil，got %v", *air)
	}
	if sil == nil {
		t.Fatal("u-noai 的 silica 应为 0 而非 nil（有提交有行数，只是零命中）")
	}
	if *sil != 0 {
		t.Fatalf("u-noai 含硅量应为 0，got %v", *sil)
	}

	// ③ 正常型：两个口径都出数，且互相独立（0.6 vs 0.42）。
	air, sil, _, _ = get("u-full")
	if air == nil {
		t.Fatal("u-full 的 ai_code_ratio 不应为 nil")
	}
	if want := 60.0 / 100.0; *air < want-1e-9 || *air > want+1e-9 {
		t.Fatalf("u-full ai_code_ratio got %v, want %v", *air, want)
	}
	if sil == nil {
		t.Fatal("u-full 的 silica 不应为 nil")
	}
	if *sil < 0.42-1e-9 || *sil > 0.42+1e-9 {
		t.Fatalf("u-full 含硅量 got %v, want 0.42", *sil)
	}
}

// 日期窗口必须同时作用在含硅量上——否则切换面板日期时 AI 代码占比变了、含硅量不变，
// 两列口径会错位。
func TestE2E_UsersAPI_SilicaRespectsDateWindow(t *testing.T) {
	db := resetE2EDB(t)
	seedE2EScreenshotScenario(t, db)

	prev := statDB
	statDB = db
	defer func() { statDB = prev }()

	// 窗口收到 07-13~07-14：邓彬的 c-d3（07-15，40 行/0.25）应被排除
	// → (100*0.8 + 100*0.6) / 200 = 0.7
	//
	// ⚠️ 起点必须盖住 week_start(07-13)：用户行本身来自 user_productivity_v2 的**周**锚点，
	// 而含硅量按 commit_time 的**天**过滤——三套日期口径（周表 week_start / need dev_end_ts /
	// commit commit_time）并存，窗口不含周锚点时整行都不返回，不是含硅量的问题。
	req := httptest.NewRequest(http.MethodGet, "/api/v2/users?startDate=2026-07-13&endDate=2026-07-14", nil)
	w := httptest.NewRecorder()
	newE2ERouter().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data []struct {
			UserId          string   `json:"user_id"`
			CommitCount     int64    `json:"commit_count"`
			CommitDiffLines int64    `json:"commit_diff_lines"`
			Silica          *float64 `json:"silica"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, row := range resp.Data {
		if row.UserId != "u-dengbin" {
			continue
		}
		if row.CommitCount != 2 || row.CommitDiffLines != 200 {
			t.Fatalf("窗口收窄后 commit 聚合 got %d/%d, want 2/200（c-d3 应被日期过滤排除）",
				row.CommitCount, row.CommitDiffLines)
		}
		if row.Silica == nil {
			t.Fatal("u-dengbin 在窗口内应有含硅量")
		}
		if *row.Silica < 0.7-1e-9 || *row.Silica > 0.7+1e-9 {
			t.Fatalf("窗口收窄后含硅量 got %v, want 0.7（c-d3 应被日期过滤排除）", *row.Silica)
		}
		return
	}
	t.Fatal("返回里没有 u-dengbin")
}

// 用户详情页的「最近 Commit」列表必须与顶部「Commit / 代码行」卡同口径——
// 否则会复现「下面列了 commit、上面卡显示 0」的自相矛盾（治理排除 / 0 行 commit 正是此类）。
func TestE2E_UserDetail_CommitListMatchesCardCaliber(t *testing.T) {
	db := resetE2EDB(t)

	if err := db.Create(&[]models.UserProductivityV2{{
		UserProductivityV2Id: "w-z", WeekStart: weekStart("2026-07-13"),
		UserId: "u-zang", UserName: "臧某", MergedNeedCount: 0,
	}}).Error; err != nil {
		t.Fatalf("seed upv: %v", err)
	}
	// 一条正常 commit + 一条治理排除 + 一条 0 行：后两条既不该进卡、也不该进列表。
	if err := db.Create(&[]models.Commit{
		{CommitId: "c-ok", UserId: "u-zang", CommitTime: ts("2026-07-14T10:00:00Z"), DiffLines: 39, Silica: 0.641},
		{CommitId: "c-excl", UserId: "u-zang", CommitTime: ts("2026-07-14T11:00:00Z"), DiffLines: 500, Silica: 1.0, ExcludedFlag: true},
		{CommitId: "c-zero", UserId: "u-zang", CommitTime: ts("2026-07-14T12:00:00Z"), DiffLines: 0, Silica: 0},
	}).Error; err != nil {
		t.Fatalf("seed commits: %v", err)
	}

	prev := statDB
	statDB = db
	defer func() { statDB = prev }()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v2/users/:userId", getUserV2DetailNative)
	req := httptest.NewRequest(http.MethodGet, "/api/v2/users/u-zang?startDate=2026-07-01&endDate=2026-07-31", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Summary struct {
			CommitCount     int64 `json:"commit_count"`
			CommitDiffLines int64 `json:"commit_diff_lines"`
		} `json:"summary"`
		Commits []struct {
			CommitId string `json:"commit_id"`
		} `json:"commits"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v — %s", err, w.Body.String())
	}

	// 卡：只计正常那条
	if resp.Summary.CommitCount != 1 || resp.Summary.CommitDiffLines != 39 {
		t.Fatalf("汇总卡应为 1/39，got %d/%d", resp.Summary.CommitCount, resp.Summary.CommitDiffLines)
	}
	// 列表：条数必须与卡一致，且不含被排除/0 行的
	if len(resp.Commits) != int(resp.Summary.CommitCount) {
		ids := make([]string, 0, len(resp.Commits))
		for _, c := range resp.Commits {
			ids = append(ids, c.CommitId)
		}
		t.Fatalf("列表条数 %v 与卡 %d 不一致（口径漂移）", ids, resp.Summary.CommitCount)
	}
	if resp.Commits[0].CommitId != "c-ok" {
		t.Fatalf("列表应只含 c-ok，got %s", resp.Commits[0].CommitId)
	}
}
