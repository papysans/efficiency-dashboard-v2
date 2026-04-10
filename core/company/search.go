package company

import (
	"database/sql"
	"log"
	"strings"
)

// CompanyResult 公司搜索结果
type CompanyResult struct {
	ID       string `json:"id"`
	Market   string `json:"market"`
	Code     string `json:"code"`
	ViewName string `json:"view_name"`
	Pinyin   string `json:"pinyin"`
	HasData  bool   `json:"has_data"`
}

// containsChinese 检查字符串是否包含中文字符（内部函数）
func containsChinese(s string) bool {
	for _, r := range s {
		if r >= '\u4e00' && r <= '\u9fff' {
			return true
		}
	}
	return false
}

// isNumeric 检查字符串是否为纯数字（内部函数）
func isNumeric(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// GetCompaniesWithData 获取所有有财务数据的公司ID列表
func GetCompaniesWithData(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`SELECT DISTINCT company_id FROM fin`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// SearchCompanies 根据关键词搜索公司（支持代码、名称、拼音搜索、完整id搜索）
func SearchCompanies(db *sql.DB, keyword string) ([]CompanyResult, error) {
	q := strings.ToLower(strings.TrimSpace(keyword))
	if q == "" {
		return []CompanyResult{}, nil
	}

	log.Printf("DEBUG: SearchCompanies called with keyword: '%s'", q)

	isChinese := containsChinese(q)
	isNumber := isNumeric(q)

	log.Printf("DEBUG: Input detection - Chinese: %v, Number: %v", isChinese, isNumber)

	companyIDs, err := GetCompaniesWithData(db)
	if err != nil {
		log.Printf("ERROR: Failed to get companies with data: %v", err)
		return nil, err
	}

	companyIDsWithData := make(map[string]bool)
	for _, companyID := range companyIDs {
		companyIDsWithData[companyID] = true
	}

	results := make([]CompanyResult, 0)

	// 优先尝试完整 id 匹配（如 sz300454, sh600519 等）
	if !isChinese && !isNumber {
		exactQuery := `
			SELECT id, market, code, view_name, pinyin
			FROM companies
			WHERE LOWER(id) = $1
			LIMIT 1
		`
		exactRows, exactErr := db.Query(exactQuery, q)
		if exactErr == nil {
			defer exactRows.Close()
			for exactRows.Next() {
				var company CompanyResult
				if err := exactRows.Scan(&company.ID, &company.Market, &company.Code, &company.ViewName, &company.Pinyin); err != nil {
					continue
				}
				company.ID = strings.TrimSpace(company.ID)
				company.Market = strings.TrimSpace(company.Market)
				company.Code = strings.TrimSpace(company.Code)
				company.ViewName = strings.TrimSpace(company.ViewName)
				company.Pinyin = strings.TrimSpace(company.Pinyin)
				company.HasData = companyIDsWithData[company.ID]
				results = append(results, company)
			}
			exactRows.Close()
			if len(results) > 0 {
				log.Printf("DEBUG: Found exact id match for '%s'", q)
				return results, nil
			}
		}
	}

	// 没有精确匹配，使用模糊搜索
	var query string
	var args []interface{}

	if isNumber {
		query = `
			SELECT id, market, code, view_name, pinyin
			FROM companies
			WHERE code LIKE $1
			ORDER BY code
			LIMIT 5
		`
		args = []interface{}{q + "%"}
		log.Printf("DEBUG: Querying by code with q: '%s'", q)
	} else if isChinese {
		query = `
			SELECT id, market, code, view_name, pinyin
			FROM companies
			WHERE local_name LIKE $1 OR view_name LIKE $1
			ORDER BY code
			LIMIT 5
		`
		args = []interface{}{"%" + q + "%"}
		log.Printf("DEBUG: Querying by Chinese name with q: '%s'", q)
	} else {
		// 拼音或代码前缀搜索
		query = `
			SELECT id, market, code, view_name, pinyin
			FROM companies
			WHERE pinyin LIKE $1 OR code LIKE $1
			ORDER BY code
			LIMIT 5
		`
		args = []interface{}{q + "%"}
		log.Printf("DEBUG: Querying by pinyin/code with q: '%s'", q)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("ERROR: Query failed: %v", err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var company CompanyResult
		if err := rows.Scan(&company.ID, &company.Market, &company.Code, &company.ViewName, &company.Pinyin); err != nil {
			log.Printf("ERROR: Scan failed for row: %v", err)
			continue
		}

		company.ID = strings.TrimSpace(company.ID)
		company.Market = strings.TrimSpace(company.Market)
		company.Code = strings.TrimSpace(company.Code)
		company.ViewName = strings.TrimSpace(company.ViewName)
		company.Pinyin = strings.TrimSpace(company.Pinyin)
		company.HasData = companyIDsWithData[company.ID]

		results = append(results, company)
	}

	log.Printf("DEBUG: Final query returned %d companies", len(results))
	return results, nil
}
