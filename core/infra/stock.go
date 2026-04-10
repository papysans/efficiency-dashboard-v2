package infra

import (
	"database/sql"
	"fmt"
	"strings"
	"unicode"
)

// Stock GP信息
type Stock struct {
	Code   string `yaml:"code" json:"code"`
	Market string `yaml:"market" json:"market"`
	Name   string `yaml:"name" json:"name"`
}

// ParseStockParam 解析GP参数，支持以下格式：
// 1. sh600760!中航沈飞 (交易所+代码!名称)
// 2. sh600760 (交易所+代码)
// 3. 600760 (仅代码)
// 4. !中航沈飞 (仅名称)
// 5. 中航沈飞 (仅名称，无感叹号)
func ParseStockParam(param string) (*Stock, error) {
	// 检查是否包含感叹号分隔符
	parts := strings.Split(param, "!")

	var codeWithMarket, name string

	if len(parts) == 2 {
		// 格式: 交易所+代码!名称 或 !名称
		codeWithMarket = strings.TrimSpace(parts[0])
		name = strings.TrimSpace(parts[1])
	} else if len(parts) == 1 {
		// 格式: 交易所+代码 或 代码 或 名称
		trimmed := strings.TrimSpace(parts[0])
		// 检测是否为纯中文名称（不含数字，且包含中文）
		hasChinese := false
		hasDigit := false
		for _, r := range trimmed {
			if isChineseChar(r) {
				hasChinese = true
			}
			if unicode.IsDigit(r) {
				hasDigit = true
			}
		}
		// 如果是纯中文（不含数字），则识别为名称
		if hasChinese && !hasDigit {
			codeWithMarket = ""
			name = trimmed
		} else {
			codeWithMarket = trimmed
			name = ""
		}
	} else {
		return nil, fmt.Errorf("无效的GP参数格式: %s", param)
	}

	// 如果代码和名称都为空，返回错误
	if codeWithMarket == "" && name == "" {
		return nil, fmt.Errorf("GP代码和名称不能同时为空")
	}

	var stock Stock

	// 解析代码部分（可能包含交易所前缀）
	if codeWithMarket != "" {
		// 检查是否有交易所前缀
		market := ""
		code := codeWithMarket

		if len(codeWithMarket) >= 2 {
			prefix := strings.ToLower(codeWithMarket[:2])
			if prefix == "sh" || prefix == "sz" || prefix == "bj" {
				market = prefix
				if len(codeWithMarket) > 2 {
					code = codeWithMarket[2:]
				} else {
					code = ""
				}
			}
		}

		// 如果没有明确的市场前缀，根据代码规则推断
		if market == "" && len(code) == 6 {
			switch code[0] {
			case '6':
				market = "sh" // sh主板
			case '0':
				market = "sz" // sz主板
			case '3':
				market = "sz" // 创业板
			case '8':
				market = "bj" // bj
			default:
				// 科创板688开头
				if len(code) >= 3 && code[:3] == "688" {
					market = "sh"
				}
			}
		}

		stock.Code = code
		stock.Market = market
	}

	stock.Name = name

	// 验证：代码和名称至少有一个
	if stock.Code == "" && stock.Name == "" {
		return nil, fmt.Errorf("GP代码和名称至少需要提供一个")
	}

	return &stock, nil
}

// isChineseChar 检查字符是否为中文字符
func isChineseChar(r rune) bool {
	return unicode.Is(unicode.Han, r)
}

// isAllDigits 检查字符串是否全为数字
func isAllDigits(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return len(s) > 0
}

// SearchCompanyInDB 在 companies 表中搜索公司
// 根据输入类型分别查询：
// - 纯数字：按代码查询
// - 包含中文：按 view_name 或 local_name 查询
// - 纯英文字母：按拼音查询（在 pinyin 字段中查找）
func SearchCompanyInDB(db *sql.DB, input string) (*Company, error) {
	if input == "" {
		return nil, fmt.Errorf("搜索关键词不能为空")
	}

	// 判断输入类型
	hasChinese := false
	for _, r := range input {
		if isChineseChar(r) {
			hasChinese = true
			break
		}
	}

	var company Company
	var err error

	// 情况1：纯数字 - 按代码查询
	if isAllDigits(input) {
		// 如果是纯数字（如6位代码）
		if len(input) == 6 {
			// 先尝试精确匹配 code 字段
			err = db.QueryRow(`
				SELECT id, market, code, view_name, local_name, listing_date, industry
				FROM companies
				WHERE code = $1
				LIMIT 1
			`, input).Scan(&company.ID, &company.Market, &company.Code, &company.ViewName,
				&company.LocalName, &company.ListingDate, &company.Industry)

			if err == nil {
				return &company, nil
			}
			if err != sql.ErrNoRows {
				return nil, fmt.Errorf("按代码查询公司失败: %w", err)
			}
		}
	} else if hasChinese {
		// 情况2：包含中文 - 按 view_name 或 local_name 查询
		// 优先精确匹配 view_name
		err = db.QueryRow(`
			SELECT id, market, code, view_name, local_name, listing_date, industry
			FROM companies
			WHERE view_name = $1
			LIMIT 1
		`, input).Scan(&company.ID, &company.Market, &company.Code, &company.ViewName,
			&company.LocalName, &company.ListingDate, &company.Industry)

		if err == nil {
			return &company, nil
		}
		if err != sql.ErrNoRows {
			return nil, fmt.Errorf("按名称查询公司失败: %w", err)
		}

		// 如果没有精确匹配，尝试模糊匹配 view_name 或 local_name
		err = db.QueryRow(`
			SELECT id, market, code, view_name, local_name, listing_date, industry
			FROM companies
			WHERE view_name LIKE $1 OR local_name LIKE $1
			ORDER BY view_name
			LIMIT 1
		`, "%"+input+"%").Scan(&company.ID, &company.Market, &company.Code, &company.ViewName,
			&company.LocalName, &company.ListingDate, &company.Industry)

		if err == nil {
			return &company, nil
		}
		if err != sql.ErrNoRows {
			return nil, fmt.Errorf("模糊查询公司失败: %w", err)
		}
	} else {
		// 情况3：纯英文字母（拼音）- 按 pinyin 字段查询
		// 注意：这需要 companies 表中有 pinyin 字段
		err = db.QueryRow(`
			SELECT id, market, code, view_name, local_name, listing_date, industry
			FROM companies
			WHERE pinyin LIKE $1 OR pinyin = $2
			ORDER BY
				CASE WHEN pinyin = $2 THEN 0 ELSE 1 END,
				view_name
			LIMIT 1
		`, "%"+strings.ToLower(input)+"%", strings.ToLower(input)).Scan(&company.ID, &company.Market, &company.Code, &company.ViewName,
			&company.LocalName, &company.ListingDate, &company.Industry)

		if err == nil {
			return &company, nil
		}
		if err != sql.ErrNoRows {
			// pinyin 字段可能不存在，忽略这个错误继续后续查询
			Logger.Debug("按拼音查询公司失败(可能pinyin字段不存在): %v", err)
		}
	}

	return nil, sql.ErrNoRows
}
