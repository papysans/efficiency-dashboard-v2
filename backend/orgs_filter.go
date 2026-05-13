package main

import (
	"fmt"
	"reflect"
	"strings"

	"kanban/core/models"

	"gorm.io/gorm"
)

type OrgsFilter struct {
	Org1 string
	Org2 string
	Org3 string
	Org4 string
	Org5 string
	Org6 string
	Org7 string
	Org8 string
	Org9 string
}

func (f *OrgsFilter) HasOrgsFilter() bool {
	return f.Org1 != "" || f.Org2 != "" || f.Org3 != "" || f.Org4 != "" ||
		f.Org5 != "" || f.Org6 != "" || f.Org7 != "" || f.Org8 != "" || f.Org9 != ""
}

func (f *OrgsFilter) SetOrgPath(orgPath string) {
	f.Org1, f.Org2, f.Org3, f.Org4, f.Org5, f.Org6, f.Org7, f.Org8, f.Org9 = "", "", "", "", "", "", "", "", ""
	if orgPath == "" {
		return
	}
	parts := strings.Split(orgPath, "/")
	vf := reflect.ValueOf(f).Elem()
	for i, v := range parts {
		if i >= 9 {
			break
		}
		if field := vf.FieldByName(fmt.Sprintf("Org%d", i+1)); field.IsValid() {
			field.SetString(v)
		}
	}
}

func (f *OrgsFilter) SetOrgs(orgs []string) {
	f.Org1, f.Org2, f.Org3, f.Org4, f.Org5, f.Org6, f.Org7, f.Org8, f.Org9 = "", "", "", "", "", "", "", "", ""
	vf := reflect.ValueOf(f).Elem()
	for i, v := range orgs {
		if i >= 9 {
			break
		}
		if field := vf.FieldByName(fmt.Sprintf("Org%d", i+1)); field.IsValid() {
			field.SetString(v)
		}
	}
}

func (f *OrgsFilter) GetUsers() []models.UserOrg {
	var result []models.UserOrg
	for _, m := range orgMappings {
		if f.matchOrgFields(m) {
			result = append(result, *m)
		}
	}
	return result
}

func (f *OrgsFilter) GetUserIds() []string {
	var result []string
	for _, m := range orgMappings {
		if f.matchOrgFields(m) {
			result = append(result, m.UserId)
		}
	}
	return result
}

func (f *OrgsFilter) GetOrgPath() string {
	parts := []string{}
	vf := reflect.ValueOf(f).Elem()
	for i := 1; i <= 9; i++ {
		if field := vf.FieldByName(fmt.Sprintf("Org%d", i)); field.IsValid() && field.String() != "" {
			parts = append(parts, field.String())
		}
	}
	return strings.Join(parts, "/")
}

func (f *OrgsFilter) MatchOrg(userID string) (*models.UserOrg, bool) {
	om, ok := orgMappings[userID]
	if !ok {
		if !f.HasOrgsFilter() {
			return &models.UserOrg{}, true
		}
		return nil, false
	}
	if !f.matchOrgFields(om) {
		return nil, false
	}
	return om, true
}

func (f *OrgsFilter) matchOrgFields(om *models.UserOrg) bool {
	vf := reflect.ValueOf(f).Elem()
	vo := reflect.ValueOf(om).Elem()

	for i := 1; i <= 9; i++ {
		ff := vf.FieldByName(fmt.Sprintf("Org%d", i))
		of := vo.FieldByName(fmt.Sprintf("Org%d", i))
		if !ff.IsValid() || !of.IsValid() {
			continue
		}
		if filterVal := ff.String(); filterVal != "" && filterVal != of.String() {
			return false
		}
	}
	return true
}

func (f *OrgsFilter) GetFilter() []string {
	if !f.HasOrgsFilter() {
		return nil
	}
	userIds := []string{}
	for uid, m := range orgMappings {
		if uid == "" {
			continue
		}
		if f.matchOrgFields(m) {
			userIds = append(userIds, uid)
		}
	}
	return userIds
}

func (f *OrgsFilter) ApplyOrgsToQuery(q *gorm.DB) *gorm.DB {
	userIds := f.GetFilter()
	if userIds != nil {
		if len(userIds) > 0 {
			q = q.Where("user_id IN ?", userIds)
		} else {
			q = q.Where("1 = 0")
		}
	}
	return q
}
