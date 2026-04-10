package screen

import (
	"database/sql"
	"fmt"
	"sort"
)

// CalculateSanhuScore 计算单个公司的散户乙综合得分（0-100分）
// 散户乙选股理念：高ROE、高分红、高成长、低负债、现金流健康、估值合理
func CalculateSanhuScore(data *SanhuScreenResult) float64 {
	if data == nil {
		return 0
	}

	// 辅助函数：安全解引用指针，nil时返回0
	pval := func(p *float64) float64 {
		if p == nil {
			return 0
		}
		return *p
	}

	// 1. ROE得分：ROE与15%比较，线性映射到0-100（15%得60分，30%得100分，0%得0分）
	roeScore := linearMap(data.ROE, 0, 30, 0, 100)
	if data.ROE >= 15 && data.ROE <= 30 {
		// 15%对应60分，30%对应100分
		roeScore = linearMap(data.ROE, 15, 30, 60, 100)
	} else if data.ROE > 30 {
		roeScore = 100
	} else if data.ROE < 0 {
		roeScore = 0
	}

	// 2. ROA接近度得分：计算ROE与ROA差值，差值<1%得100分，差值>10%得0分，线性插值
	// ROE与ROA接近说明公司杠杆使用合理，利润主要来自资产经营而非负债
	roaDiff := data.ROE - data.ROA
	if roaDiff < 0 {
		roaDiff = 0 // ROA大于ROE是异常情况，给满分（按0差值处理）
	}
	roaProxScore := linearMap(roaDiff, 1, 10, 100, 0)
	if roaDiff < 1 {
		roaProxScore = 100
	} else if roaDiff > 10 {
		roaProxScore = 0
	}

	// 3. 股息率得分：股息率与3%比较，线性映射（3%得60分，6%得100分）
	// 无数据时给0分
	divYield := pval(data.DividendYield)
	dividendScore := linearMap(divYield, 0, 6, 0, 100)
	if divYield >= 3 && divYield <= 6 {
		dividendScore = linearMap(divYield, 3, 6, 60, 100)
	} else if divYield > 6 {
		dividendScore = 100
	}

	// 4. 负债率得分：负债率<30%得100分，负债率>60%得0分，线性插值
	// 低负债意味着财务稳健，抗风险能力强
	debtScore := linearMap(data.DebtRatio, 30, 60, 100, 0)
	if data.DebtRatio < 30 {
		debtScore = 100
	} else if data.DebtRatio > 60 {
		debtScore = 0
	}

	// 5. 成长性得分：CAGR5与10%比较，线性映射（10%得60分，30%得100分）
	// 无数据时给中性分30分
	cagr5 := pval(data.CAGR5)
	growthScore := linearMap(cagr5, 0, 30, 0, 100)
	if data.CAGR5 == nil {
		growthScore = 30 // 无数据给中性分
	} else if cagr5 >= 10 && cagr5 <= 30 {
		growthScore = linearMap(cagr5, 10, 30, 60, 100)
	} else if cagr5 > 30 {
		growthScore = 100
	}

	// 6. 现金占资产比例得分：货币资金/总资产（%），比例越高越好
	// >30%得100分，<5%得0分，线性插值；无数据时给中性分30分
	cashRatio := pval(data.CashFlowRatio)
	cashFlowScore := linearMap(cashRatio, 5, 30, 0, 100)
	if data.CashFlowRatio == nil {
		cashFlowScore = 30 // 无数据给中性分
	} else if cashRatio >= 30 {
		cashFlowScore = 100
	} else if cashRatio < 5 {
		cashFlowScore = 0
	}

	// 7. PB得分：PB<1得100分，PB>4得0分，线性插值
	// 低PB代表安全边际高；无数据时给中性分50分
	pb := pval(data.PB)
	pbScore := linearMap(pb, 1, 4, 100, 0)
	if data.PB == nil || pb <= 0 {
		pbScore = 50 // 无数据给中性分
	} else if pb < 1 {
		pbScore = 100
	} else if pb > 4 {
		pbScore = 0
	}

	// 综合得分 = 各单项得分 × 对应权重后求和
	score := roeScore*WeightROE +
		roaProxScore*WeightROAProx +
		dividendScore*WeightDividend +
		debtScore*WeightDebt +
		growthScore*WeightGrowth +
		cashFlowScore*WeightCashFlow +
		pbScore*WeightPB

	// 边界处理
	if score < 0 {
		score = 0
	} else if score > 100 {
		score = 100
	}

	return score
}

// linearMap 线性映射函数，将值从 [inMin, inMax] 映射到 [outMin, outMax]
func linearMap(val, inMin, inMax, outMin, outMax float64) float64 {
	if inMax <= inMin {
		return (outMin + outMax) / 2
	}
	ratio := (val - inMin) / (inMax - inMin)
	result := outMin + ratio*(outMax-outMin)
	return result
}

// FilterSanhuCandidates 遍历全市场公司，应用阈值筛选，返回符合条件的公司列表
// 从fin表中获取最新年报数据，计算各项指标后筛选并评分
func FilterSanhuCandidates(db *sql.DB, params SanhuScreenParams) ([]SanhuScreenResult, error) {
	// 构建基础查询SQL
	// 获取各公司最新年报数据（不同字段存储在不同的report_type中）
	// 使用子查询方式获取各类型最新数据
	query := `
		SELECT 
			c.id as company_id,
			c.local_name as company_name,
			-- 从各report_type获取最新数据
			(SELECT item_value FROM fin f 
			 WHERE f.company_id = c.id AND f.report_type = 'calculated' AND f.item_field = 'ROE'
			 ORDER BY f.report_date DESC LIMIT 1) as roe,
			(SELECT item_value FROM fin f 
			 WHERE f.company_id = c.id AND f.report_type = 'calculated' AND f.item_field = 'DEDUNETPROFIT_CAGR5'
			 AND item_value IS NOT NULL
			 ORDER BY f.report_date DESC LIMIT 1) as cagr5,
			(SELECT item_value FROM fin f 
			 WHERE f.company_id = c.id AND f.report_type = 'calculated' AND f.item_field = 'PB_MRQ'
			 AND item_value IS NOT NULL
			 ORDER BY f.report_date DESC LIMIT 1) as pb_mrq,
			(SELECT item_value FROM fin f 
			 WHERE f.company_id = c.id AND f.report_type = 'lrb' AND f.item_field = 'NETPROFIT'
			 ORDER BY f.report_date DESC LIMIT 1) as netprofit,
			(SELECT item_value FROM fin f 
			 WHERE f.company_id = c.id AND f.report_type = 'fzb' AND f.item_field = 'TOTASSET'
			 ORDER BY f.report_date DESC LIMIT 1) as totasset,
			(SELECT item_value FROM fin f 
			 WHERE f.company_id = c.id AND f.report_type = 'fzb' AND f.item_field = 'TOTLIAB'
			 ORDER BY f.report_date DESC LIMIT 1) as totliab,
			(SELECT item_value FROM fin f 
			 WHERE f.company_id = c.id AND f.report_type = 'calculated' AND f.item_field = 'DIVIDEND_YIELD'
			 AND item_value IS NOT NULL
			 ORDER BY f.report_date DESC LIMIT 1) as dividend_yield,
			(SELECT item_value FROM fin f 
			 WHERE f.company_id = c.id AND f.report_type = 'calculated' AND f.item_field = 'DIVIDEND_YIELD_TTM'
			 AND item_value IS NOT NULL
			 ORDER BY f.report_date DESC LIMIT 1) as dividend_yield_ttm,
			(SELECT item_value FROM fin f 
			 WHERE f.company_id = c.id AND f.report_type = 'fzb' AND f.item_field = 'CURFDS'
			 ORDER BY f.report_date DESC LIMIT 1) as curfds
		FROM companies c
		WHERE c.confirmed = true
		ORDER BY c.id
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("查询公司数据失败: %w", err)
	}
	defer rows.Close()

	var results []SanhuScreenResult

	for rows.Next() {
		var (
			companyID        string
			companyName      sql.NullString
			roe              sql.NullFloat64
			cagr5            sql.NullFloat64
			pbMrq            sql.NullFloat64
			netprofit        sql.NullFloat64
			totasset         sql.NullFloat64
			totliab          sql.NullFloat64
			dividendYield    sql.NullFloat64
			dividendYieldTTM sql.NullFloat64
			curfds           sql.NullFloat64
		)

		err := rows.Scan(
			&companyID, &companyName, &roe, &cagr5, &pbMrq, &netprofit, &totasset,
			&totliab, &dividendYield, &dividendYieldTTM, &curfds,
		)
		if err != nil {
			continue // 跳过解析失败的数据
		}

		// 数据完整性检查：核心字段缺失时不跳过，以零值参与筛选和评分
		// 当用户移除所有筛选条件(none)时，无数据公司也应出现在结果中

		// 计算衍生指标
		// ROA = 净利润 / 总资产 * 100
		var roa float64
		if totasset.Valid && totasset.Float64 > 0 && netprofit.Valid {
			roa = netprofit.Float64 / totasset.Float64 * 100
		}

		// 负债率 = 总负债 / 总资产 * 100
		var debtRatio float64
		if totasset.Valid && totasset.Float64 > 0 && totliab.Valid {
			debtRatio = totliab.Float64 / totasset.Float64 * 100
		}

		// 现金占资产比例 = 货币资金 / 总资产 * 100
		var cashFlowRatioPtr *float64
		if totasset.Valid && totasset.Float64 > 0 && curfds.Valid {
			v := curfds.Float64 / totasset.Float64 * 100
			cashFlowRatioPtr = &v
		}

		// PB值（指针，无数据时为nil）
		var pbPtr *float64
		if pbMrq.Valid && pbMrq.Float64 > 0 {
			v := pbMrq.Float64
			pbPtr = &v
		}

		// 股息率指针（无数据时为nil）
		var divYieldPtr *float64
		if dividendYield.Valid && dividendYield.Float64 > 0 {
			v := dividendYield.Float64
			divYieldPtr = &v
		}
		var divYieldTTMPtr *float64
		if dividendYieldTTM.Valid && dividendYieldTTM.Float64 > 0 {
			v := dividendYieldTTM.Float64
			divYieldTTMPtr = &v
		}

		// CAGR5指针（无数据时为nil）
		var cagr5Ptr *float64
		if cagr5.Valid {
			v := cagr5.Float64
			cagr5Ptr = &v
		}

		// 构建结果结构
		result := SanhuScreenResult{
			CompanyID:        companyID,
			CompanyName:      companyName.String,
			ROE:              roe.Float64,
			ROA:              roa,
			DividendYield:    divYieldPtr,
			DividendYieldTTM: divYieldTTMPtr,
			DebtRatio:        debtRatio,
			CAGR5:            cagr5Ptr,
			CashFlowRatio:    cashFlowRatioPtr,
			PB:               pbPtr,
		}

		// 应用阈值筛选
		if !passThresholdFilter(result, params) {
			continue
		}

		// 计算综合得分
		result.Score = CalculateSanhuScore(&result)

		results = append(results, result)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历查询结果时出错: %w", err)
	}

	// 按得分降序排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// 限制返回数量
	if params.TopN > 0 && len(results) > params.TopN {
		results = results[:params.TopN]
	}

	return results, nil
}

// passThresholdFilter 检查公司是否满足阈值筛选条件
func passThresholdFilter(result SanhuScreenResult, params SanhuScreenParams) bool {
	// ROE ≥ MinROE
	if params.MinROE > 0 && result.ROE < params.MinROE {
		return false
	}

	// 股息率（年度）≥ MinDividendYield；无数据(nil)时视为0，不满足有值要求
	if params.MinDividendYield > 0 {
		if result.DividendYield == nil || *result.DividendYield < params.MinDividendYield {
			return false
		}
	}

	// 负债率 ≤ MaxDebtRatio
	if params.MaxDebtRatio > 0 && result.DebtRatio > params.MaxDebtRatio {
		return false
	}

	// CAGR5 ≥ MinCAGR5；无数据(nil)时视为不满足
	if params.MinCAGR5 > 0 {
		if result.CAGR5 == nil || *result.CAGR5 < params.MinCAGR5 {
			return false
		}
	}

	// 现金占资产比例 ≥ MinCashFlowRatio；无数据(nil)时视为不满足
	if params.MinCashFlowRatio > 0 {
		if result.CashFlowRatio == nil || *result.CashFlowRatio < params.MinCashFlowRatio {
			return false
		}
	}

	// PB ≤ MaxPB；无数据(nil)时不过滤
	if params.MaxPB > 0 && result.PB != nil && *result.PB > params.MaxPB {
		return false
	}

	return true
}

// GetDefaultSanhuParams 返回默认的散户乙筛选参数
func GetDefaultSanhuParams() SanhuScreenParams {
	return SanhuScreenParams{
		MinROE:           DefaultSanhuMinROE,
		MinDividendYield: DefaultSanhuMinDividendYield,
		MaxDebtRatio:     DefaultSanhuMaxDebtRatio,
		MinCAGR5:         DefaultSanhuMinCAGR5,
		MinCashFlowRatio: DefaultSanhuMinCashFlowRatio,
		MaxPB:            DefaultSanhuMaxPB,
		TopN:             DefaultSanhuTopN,
	}
}
