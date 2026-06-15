package main

import (
	"os"
	"sort"
	"strings"
	"testing"

	"kanban/core/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestNeedSoftwareUserCaliberSQL_Shape 无库回归：人级软件用户谓词常量形态断言（CI 可跑）。
// 防误删/改坏导致 commit-only(非软件用户)need 重新涌入首页/列表/项目/分布/AI比。
func TestNeedSoftwareUserCaliberSQL_Shape(t *testing.T) {
	for _, want := range []string{
		"primary_user_id",
		"SELECT DISTINCT primary_user_id FROM needs",
		"session_ids IS NOT NULL",
		"NOT IN ('[]','null','')",
	} {
		if !strings.Contains(needSoftwareUserCaliberSQL, want) {
			t.Fatalf("needSoftwareUserCaliberSQL 缺少片段 %q\n常量: %s", want, needSoftwareUserCaliberSQL)
		}
	}
}

// TestApplyNeedCaliberFilter_PersonLevelPredicate 用 DryRun ToSQL 断言看板口径 SQL 含人级软件用户谓词。
// 回归保护：防止误删该谓词导致没用软件的人(commit-only need)重新涌入首页/列表/项目/分布/AI比。
// 需可连测试库渲染 SQL；CI 无库时 t.Skip（gorm postgres dialector 即使 DryRun 也在 Open 时连库）。
func TestApplyNeedCaliberFilter_PersonLevelPredicate(t *testing.T) {
	dsn := os.Getenv("TEST_PG_DSN")
	if dsn == "" {
		dsn = "host=127.0.0.1 port=5434 user=postgres password=1 dbname=costrict_stat sslmode=disable"
	}
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true}), &gorm.Config{DryRun: true})
	if err != nil {
		t.Skipf("无测试数据库，跳过 SQL 渲染断言: %v", err)
	}
	sqlStr := strings.ToLower(db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		var rows []models.Need
		return applyNeedCaliberFilter(tx.Model(&models.Need{})).Find(&rows)
	}))
	// 人级软件用户口径必须落进 SQL。
	for _, want := range []string{
		"primary_user_id",
		"session_ids",
		"select distinct primary_user_id from needs",
	} {
		if !strings.Contains(sqlStr, want) {
			t.Fatalf("applyNeedCaliberFilter SQL 缺少人级谓词片段 %q\nSQL: %s", want, sqlStr)
		}
	}
	// 既有口径(已交付 + 非主干分支)仍在，未被破坏。
	if !strings.Contains(sqlStr, "status") || !strings.Contains(sqlStr, "repo_branch") {
		t.Fatalf("applyNeedCaliberFilter 丢失既有口径(status/repo_branch)\nSQL: %s", sqlStr)
	}
}

func TestIntersectUserIdFilter(t *testing.T) {
	tests := []struct {
		name       string
		orgUserIds []string
		userId     string
		userIds    []string
		wantNil    bool
		want       []string
	}{
		{
			name:    "all nil/no filter",
			wantNil: true,
		},
		{
			name:       "only org, nil",
			orgUserIds: nil,
			wantNil:    true,
		},
		{
			name:       "only org, empty slice",
			orgUserIds: []string{},
			want:       []string{},
		},
		{
			name:       "only org, has values",
			orgUserIds: []string{"a", "b"},
			want:       []string{"a", "b"},
		},
		{
			name:    "only userId, empty",
			userId:  "",
			wantNil: true,
		},
		{
			name:   "only userId, has value",
			userId: "a",
			want:   []string{"a"},
		},
		{
			name:    "only userIds, nil",
			userIds: nil,
			wantNil: true,
		},
		{
			name:    "only userIds, empty slice",
			userIds: []string{},
			want:    []string{},
		},
		{
			name:    "only userIds, has values",
			userIds: []string{"a", "b"},
			want:    []string{"a", "b"},
		},
		{
			name:       "org + userId, intersect match",
			orgUserIds: []string{"a", "b"},
			userId:     "a",
			want:       []string{"a"},
		},
		{
			name:       "org + userId, intersect miss",
			orgUserIds: []string{"a", "b"},
			userId:     "c",
			want:       []string{},
		},
		{
			name:       "org + userId, org empty",
			orgUserIds: []string{},
			userId:     "a",
			want:       []string{},
		},
		{
			name:    "userId + userIds, intersect match single",
			userId:  "a",
			userIds: []string{"a", "b"},
			want:    []string{"a"},
		},
		{
			name:    "userId + userIds, intersect miss",
			userId:  "c",
			userIds: []string{"a", "b"},
			want:    []string{},
		},
		{
			name:    "userId + userIds, userIds empty",
			userId:  "a",
			userIds: []string{},
			want:    []string{},
		},
		{
			name:       "org + userIds, intersect match",
			orgUserIds: []string{"a", "b", "c"},
			userIds:    []string{"b", "c", "d"},
			want:       []string{"b", "c"},
		},
		{
			name:       "org + userIds, intersect empty",
			orgUserIds: []string{"a"},
			userIds:    []string{"b"},
			want:       []string{},
		},
		{
			name:       "org + userIds, org empty",
			orgUserIds: []string{},
			userIds:    []string{"a", "b"},
			want:       []string{},
		},
		{
			name:       "org + userIds, userIds empty",
			orgUserIds: []string{"a", "b"},
			userIds:    []string{},
			want:       []string{},
		},
		{
			name:       "all three, intersect single",
			orgUserIds: []string{"a", "b"},
			userId:     "a",
			userIds:    []string{"a", "c"},
			want:       []string{"a"},
		},
		{
			name:       "all three, userId not in org",
			orgUserIds: []string{"a", "b"},
			userId:     "c",
			userIds:    []string{"a", "c"},
			want:       []string{},
		},
		{
			name:       "all three, userId in org but not in userIds",
			orgUserIds: []string{"a", "b"},
			userId:     "b",
			userIds:    []string{"a", "c"},
			want:       []string{},
		},
		{
			name:       "all three, org empty",
			orgUserIds: []string{},
			userId:     "a",
			userIds:    []string{"a"},
			want:       []string{},
		},
		{
			name:       "all three, userIds empty",
			orgUserIds: []string{"a"},
			userId:     "a",
			userIds:    []string{},
			want:       []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := intersectUserIdFilter(tt.orgUserIds, tt.userId, tt.userIds)
			if tt.wantNil {
				if got != nil {
					t.Errorf("intersectUserIdFilter() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Errorf("intersectUserIdFilter() = nil, want %v", tt.want)
				return
			}
			sort.Strings(got)
			want := append([]string(nil), tt.want...)
			sort.Strings(want)
			if len(got) != len(want) {
				t.Errorf("intersectUserIdFilter() = %v, want %v", got, want)
				return
			}
			for i := range got {
				if got[i] != want[i] {
					t.Errorf("intersectUserIdFilter() = %v, want %v", got, want)
					return
				}
			}
		})
	}
}
