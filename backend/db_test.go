package main

import (
	"sort"
	"testing"
)

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
