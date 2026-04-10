package main

import (
	"encoding/csv"
	"fmt"
	"os"
)

// OrgInfo 组织信息
type OrgInfo struct {
	UserID   string
	UserName string
	Org1     string
	Org2     string
	Org3     string
	Org4     string
}

// OrgProvider 基于 CSV 文件的组织信息提供者
type OrgProvider struct {
	userIDMap   map[string]OrgInfo
	userNameMap map[string]OrgInfo
	gitNameMap  map[string]string // git_user_name → user_id
	gitEmailMap map[string]string // git_user_email → user_id
	csvFile     string
}

// NewOrgProvider 从 CSV 文件创建 OrgProvider
func NewOrgProvider(csvFile string) (*OrgProvider, error) {
	p := &OrgProvider{
		userIDMap:   make(map[string]OrgInfo),
		userNameMap: make(map[string]OrgInfo),
		gitNameMap:  make(map[string]string),
		gitEmailMap: make(map[string]string),
		csvFile:     csvFile,
	}
	if err := p.load(); err != nil {
		return nil, err
	}
	return p, nil
}

// GetOrgInfo 根据 userID 或 userName 查询组织信息
func (p *OrgProvider) GetOrgInfo(userID, userName string) OrgInfo {
	if userID != "" {
		if info, ok := p.userIDMap[userID]; ok {
			return info
		}
	}
	if userName != "" {
		if info, ok := p.userNameMap[userName]; ok {
			return info
		}
	}
	return OrgInfo{}
}

// Count 返回已加载的组织信息条目数（userIDMap 的条目数）
func (p *OrgProvider) Count() int {
	return len(p.userIDMap)
}

// LookupByGitAuthor 根据 git 作者信息查找系统用户 ID
// 优先按 email 查询（更精确），未命中再按 name 查询
func (p *OrgProvider) LookupByGitAuthor(gitName, gitEmail string) (userID string, found bool) {
	if gitEmail != "" {
		if uid, ok := p.gitEmailMap[gitEmail]; ok {
			return uid, true
		}
	}
	if gitName != "" {
		if uid, ok := p.gitNameMap[gitName]; ok {
			return uid, true
		}
	}
	return "", false
}

// Reload 重新读取 CSV 文件，替换内部 map
func (p *OrgProvider) Reload() error {
	return p.load()
}

func (p *OrgProvider) load() error {
	f, err := os.Open(p.csvFile)
	if err != nil {
		return fmt.Errorf("打开CSV文件失败: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("解析CSV文件失败: %w", err)
	}
	if len(records) < 1 {
		return fmt.Errorf("CSV文件为空")
	}

	// 解析表头，确定列索引
	header := records[0]
	colIdx := make(map[string]int)
	for i, col := range header {
		colIdx[col] = i
	}
	requiredCols := []string{"user_id", "user_name", "org1", "org2", "org3", "org4"}
	for _, col := range requiredCols {
		if _, ok := colIdx[col]; !ok {
			return fmt.Errorf("CSV缺少必要列: %s", col)
		}
	}

	userIDMap := make(map[string]OrgInfo)
	userNameMap := make(map[string]OrgInfo)
	gitNameMap := make(map[string]string)
	gitEmailMap := make(map[string]string)

	for _, row := range records[1:] {
		if len(row) < len(header) {
			continue
		}
		uid := row[colIdx["user_id"]]
		uname := row[colIdx["user_name"]]
		info := OrgInfo{
			UserID:   uid,
			UserName: uname,
			Org1:     row[colIdx["org1"]],
			Org2:     row[colIdx["org2"]],
			Org3:     row[colIdx["org3"]],
			Org4:     row[colIdx["org4"]],
		}
		if uid != "" {
			userIDMap[uid] = info
		}
		if uname != "" {
			userNameMap[uname] = info
		}

		// git 身份映射（可选列）
		if idx, ok := colIdx["git_user_name"]; ok && uid != "" {
			gitName := row[idx]
			if gitName != "" {
				gitNameMap[gitName] = uid
			}
		}
		if idx, ok := colIdx["git_user_email"]; ok && uid != "" {
			gitEmail := row[idx]
			if gitEmail != "" {
				gitEmailMap[gitEmail] = uid
			}
		}
	}

	p.userIDMap = userIDMap
	p.userNameMap = userNameMap
	p.gitNameMap = gitNameMap
	p.gitEmailMap = gitEmailMap
	return nil
}
