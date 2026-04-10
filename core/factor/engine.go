package factor

import (
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// FactorEngine 因子计算引擎
type FactorEngine struct {
	data map[string]*FactorData // key=companyID
}

// NewFactorEngine 创建新的因子引擎
func NewFactorEngine() *FactorEngine {
	return &FactorEngine{
		data: make(map[string]*FactorData),
	}
}

// LoadFromDB 从 fin 表加载历史财务数据
// fields: 需要加载的字段名列表（如 ["ROE", "NETPROFIT"]）
// years: 加载最近多少年的年报数据
func (e *FactorEngine) LoadFromDB(db *sql.DB, fields []string, years int) error {
	if len(fields) == 0 {
		return fmt.Errorf("fields 不能为空")
	}

	// 构建 IN 子句
	placeholders := make([]string, len(fields))
	args := make([]interface{}, len(fields)+1)
	for i, f := range fields {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = strings.ToUpper(f)
	}
	// 年份过滤：只取年报（12月31日），最近 years 年
	minYear := time.Now().Year() - years
	args[len(fields)] = fmt.Sprintf("%d-12-31", minYear)

	query := fmt.Sprintf(`
		SELECT company_id, report_date::text, item_field, item_value
		FROM fin
		WHERE item_field IN (%s)
		  AND EXTRACT(MONTH FROM report_date) = 12
		  AND EXTRACT(DAY FROM report_date) = 31
		  AND report_date >= $%d::date
		  AND item_value IS NOT NULL
		ORDER BY company_id, report_date ASC`,
		strings.Join(placeholders, ","),
		len(fields)+1,
	)

	rows, err := db.Query(query, args...)
	if err != nil {
		return fmt.Errorf("查询财务数据失败: %w", err)
	}
	defer rows.Close()

	// 临时存储：companyID → reportDate → fieldName → value
	type rowKey struct {
		companyID  string
		reportDate string
	}
	rawData := make(map[rowKey]map[string]float64)
	datesByCompany := make(map[string][]string)

	for rows.Next() {
		var companyID, reportDate, itemField string
		var itemValue sql.NullFloat64
		if err := rows.Scan(&companyID, &reportDate, &itemField, &itemValue); err != nil {
			continue
		}
		if !itemValue.Valid {
			continue
		}
		// report_date is cast to text as "YYYY-MM-DD", convert to "YYYYMMDD" for consistency
		reportDate = strings.ReplaceAll(reportDate, "-", "")

		key := rowKey{companyID, reportDate}
		if rawData[key] == nil {
			rawData[key] = make(map[string]float64)
			datesByCompany[companyID] = append(datesByCompany[companyID], reportDate)
		}
		rawData[key][strings.ToUpper(itemField)] = itemValue.Float64
	}

	// 去重并排序日期
	for companyID, dates := range datesByCompany {
		// 去重
		seen := make(map[string]bool)
		var uniqueDates []string
		for _, d := range dates {
			if !seen[d] {
				seen[d] = true
				uniqueDates = append(uniqueDates, d)
			}
		}
		sort.Strings(uniqueDates)
		datesByCompany[companyID] = uniqueDates
	}

	// 构建 FactorData
	for companyID, dates := range datesByCompany {
		fd := &FactorData{
			CompanyID: companyID,
			Dates:     make([]time.Time, 0, len(dates)),
			Values:    make(map[string][]float64),
		}
		for _, f := range fields {
			fd.Values[strings.ToUpper(f)] = make([]float64, 0, len(dates))
		}

		for _, dateStr := range dates {
			// 解析日期：20231231
			t, err := time.Parse("20060102", dateStr)
			if err != nil {
				continue
			}
			fd.Dates = append(fd.Dates, t)

			key := rowKey{companyID, dateStr}
			for _, f := range fields {
				fname := strings.ToUpper(f)
				v := rawData[key][fname]
				fd.Values[fname] = append(fd.Values[fname], v)
			}
		}

		if len(fd.Dates) > 0 {
			e.data[companyID] = fd
		}
	}

	return nil
}

// CsRank 全市场截面百分位排名（0-1），myValue 在 allValues 中的升序百分位
func CsRank(allValues []float64, myValue float64) float64 {
	if len(allValues) == 0 {
		return 0
	}
	count := 0
	for _, v := range allValues {
		if myValue > v {
			count++
		}
	}
	return float64(count) / float64(len(allValues))
}

// CsZscore 截面 Z-score 标准化：(myValue - mean) / std，std=0 时返回 0
func CsZscore(allValues []float64, myValue float64) float64 {
	if len(allValues) == 0 {
		return 0
	}
	mean := 0.0
	for _, v := range allValues {
		mean += v
	}
	mean /= float64(len(allValues))

	variance := 0.0
	for _, v := range allValues {
		d := v - mean
		variance += d * d
	}
	variance /= float64(len(allValues)) // 总体标准差（除以 n）
	std := math.Sqrt(variance)
	if std == 0 {
		return 0
	}
	return (myValue - mean) / std
}

// CsWinsorize 截面去极值：将 myValue 截断到 [mean-n*std, mean+n*std]
func CsWinsorize(allValues []float64, myValue float64, n float64) float64 {
	if len(allValues) == 0 {
		return myValue
	}
	mean := 0.0
	for _, v := range allValues {
		mean += v
	}
	mean /= float64(len(allValues))

	variance := 0.0
	for _, v := range allValues {
		d := v - mean
		variance += d * d
	}
	variance /= float64(len(allValues))
	std := math.Sqrt(variance)

	lo := mean - n*std
	hi := mean + n*std
	if myValue < lo {
		return lo
	}
	if myValue > hi {
		return hi
	}
	return myValue
}

// parseExpr 解析因子表达式，返回 (outerFunc, innerFunc, fieldName, n, isNested)
// 支持：
//   - "TsMean(ROE,3)" → outerFunc="TsMean", field="ROE", n=3, isNested=false
//   - "Rank(TsMean(ROE,3))" → outerFunc="Rank", innerFunc="TsMean", field="ROE", n=3, isNested=true
func parseExpr(expr string) (outerFunc, innerFunc, fieldName string, n int, isNested bool, err error) {
	expr = strings.TrimSpace(expr)

	// 检查是否是 Rank(...) 嵌套
	if strings.HasPrefix(strings.ToLower(expr), "rank(") {
		inner := expr[5 : len(expr)-1] // 去掉 "Rank(" 和 ")"
		innerFunc2, _, field2, n2, nested2, err2 := parseExpr(inner)
		if err2 != nil {
			return "", "", "", 0, false, err2
		}
		if nested2 {
			return "", "", "", 0, false, fmt.Errorf("不支持超过两层嵌套")
		}
		return "Rank", innerFunc2, field2, n2, true, nil
	}

	// 解析 FuncName(FIELD,N) 格式
	parenIdx := strings.Index(expr, "(")
	if parenIdx < 0 {
		// 纯字段名，无函数
		return "", "", strings.ToUpper(expr), 0, false, nil
	}

	outerFunc = expr[:parenIdx]
	inner := expr[parenIdx+1 : len(expr)-1]
	parts := strings.SplitN(inner, ",", 2)
	if len(parts) < 1 {
		return "", "", "", 0, false, fmt.Errorf("表达式格式错误: %s", expr)
	}
	fieldName = strings.TrimSpace(strings.ToUpper(parts[0]))
	if len(parts) == 2 {
		nStr := strings.TrimSpace(parts[1])
		n, err = strconv.Atoi(nStr)
		if err != nil {
			return "", "", "", 0, false, fmt.Errorf("参数 N 解析失败: %s", nStr)
		}
	}
	return outerFunc, "", fieldName, n, false, nil
}

// evalSingleFunc 对单个时序数据执行函数，返回最新时间点的值
func evalSingleFunc(funcName, fieldName string, n int, fd *FactorData) float64 {
	values, ok := fd.Values[fieldName]
	if !ok || len(values) == 0 {
		return 0
	}
	i := len(values) - 1 // 最新时间点

	switch strings.ToLower(funcName) {
	case "tsmean":
		return TsMean(values, i, n)
	case "tsstd":
		return TsStd(values, i, n)
	case "delay":
		return Delay(values, i, n)
	case "delta":
		return Delta(values, i, n)
	case "tsmax":
		return TsMax(values, i, n)
	case "tsmin":
		return TsMin(values, i, n)
	case "tsrank":
		return TsRank(values, i, n)
	case "log":
		return LogVal(values[i])
	case "abs":
		return AbsVal(values[i])
	case "sign":
		return SignVal(values[i])
	default:
		// 无函数，直接返回最新值
		return values[i]
	}
}

// findTopLevelOp 从右向左扫描，找到括号深度为0的最低优先级顶层运算符
// 优先级：先找 +/- （最低），再找 */÷
// 返回 (op, left, right, found)
func findTopLevelOp(expr string) (op, left, right string, found bool) {
	expr = strings.TrimSpace(expr)
	depth := 0
	// 先从右向左找 +/- （跳过括号内）
	for i := len(expr) - 1; i >= 0; i-- {
		ch := expr[i]
		if ch == ')' {
			depth++
		} else if ch == '(' {
			depth--
		}
		if depth == 0 && (ch == '+' || ch == '-') {
			// 避免把负号当减号（如果左边是空或运算符，跳过）
			if i == 0 {
				continue
			}
			leftStr := strings.TrimSpace(expr[:i])
			rightStr := strings.TrimSpace(expr[i+1:])
			if leftStr == "" || rightStr == "" {
				continue
			}
			return string(ch), leftStr, rightStr, true
		}
	}
	// 再从右向左找 */÷
	depth = 0
	for i := len(expr) - 1; i >= 0; i-- {
		ch := expr[i]
		if ch == ')' {
			depth++
		} else if ch == '(' {
			depth--
		}
		if depth == 0 && (ch == '*' || ch == '/') {
			leftStr := strings.TrimSpace(expr[:i])
			rightStr := strings.TrimSpace(expr[i+1:])
			if leftStr == "" || rightStr == "" {
				continue
			}
			return string(ch), leftStr, rightStr, true
		}
	}
	return "", "", "", false
}

// EvalExpr 解析并执行因子表达式，返回按 FactorValue 降序排列的结果
// 支持：
//   - 纯字段名：ROE
//   - 单函数：TsMean(ROE,3)
//   - 嵌套：Rank(TsMean(ROE,3))、Zscore(TsMean(ROE,3))、Winsorize(ROE)
//   - 双字段：TsCorr(ROE,NETPROFIT,5)
//   - 算术组合：TsMean(ROE,3) * 0.5 + Rank(NETPROFIT) * 0.5
func (e *FactorEngine) EvalExpr(expr string) ([]FactorResult, error) {
	expr = strings.TrimSpace(expr)

	// ── 第一步：检测顶层算术运算符 ──
	if op, leftExpr, rightExpr, found := findTopLevelOp(expr); found {
		leftResults, err := e.EvalExpr(leftExpr)
		if err != nil {
			return nil, fmt.Errorf("左子表达式 %q 计算失败: %w", leftExpr, err)
		}
		rightResults, err := e.EvalExpr(rightExpr)
		if err != nil {
			return nil, fmt.Errorf("右子表达式 %q 计算失败: %w", rightExpr, err)
		}

		// 构建 companyID → value 的 map
		leftMap := make(map[string]float64, len(leftResults))
		leftDate := make(map[string]time.Time, len(leftResults))
		for _, r := range leftResults {
			leftMap[r.CompanyID] = r.FactorValue
			leftDate[r.CompanyID] = r.Date
		}
		rightMap := make(map[string]float64, len(rightResults))
		for _, r := range rightResults {
			rightMap[r.CompanyID] = r.FactorValue
		}

		// 检查右侧是否是纯数字（标量）
		rightScalar, isScalar := strconv.ParseFloat(rightExpr, 64)
		leftScalar, isLeftScalar := strconv.ParseFloat(leftExpr, 64)

		var combined []FactorResult
		if isScalar == nil {
			// 右侧是标量：左侧每个公司值与标量运算
			for cid, lv := range leftMap {
				var val float64
				switch op {
				case "+":
					val = lv + rightScalar
				case "-":
					val = lv - rightScalar
				case "*":
					val = lv * rightScalar
				case "/":
					if rightScalar == 0 {
						val = 0
					} else {
						val = lv / rightScalar
					}
				}
				combined = append(combined, FactorResult{
					CompanyID:   cid,
					Date:        leftDate[cid],
					FactorValue: val,
				})
			}
		} else if isLeftScalar == nil {
			// 左侧是标量：右侧每个公司值与标量运算
			rightDate := make(map[string]time.Time, len(rightResults))
			for _, r := range rightResults {
				rightDate[r.CompanyID] = r.Date
			}
			for cid, rv := range rightMap {
				var val float64
				switch op {
				case "+":
					val = leftScalar + rv
				case "-":
					val = leftScalar - rv
				case "*":
					val = leftScalar * rv
				case "/":
					if rv == 0 {
						val = 0
					} else {
						val = leftScalar / rv
					}
				}
				combined = append(combined, FactorResult{
					CompanyID:   cid,
					Date:        rightDate[cid],
					FactorValue: val,
				})
			}
		} else {
			// 两侧都是表达式：按 companyID 对齐
			for cid, lv := range leftMap {
				rv, ok := rightMap[cid]
				if !ok {
					continue
				}
				var val float64
				switch op {
				case "+":
					val = lv + rv
				case "-":
					val = lv - rv
				case "*":
					val = lv * rv
				case "/":
					if rv == 0 {
						val = 0
					} else {
						val = lv / rv
					}
				}
				combined = append(combined, FactorResult{
					CompanyID:   cid,
					Date:        leftDate[cid],
					FactorValue: val,
				})
			}
		}

		// 重新计算截面 Rank
		allVals := make([]float64, len(combined))
		for i, r := range combined {
			allVals[i] = r.FactorValue
		}
		for i := range combined {
			combined[i].Rank = CsRank(allVals, combined[i].FactorValue)
		}

		sort.Slice(combined, func(i, j int) bool {
			return combined[i].FactorValue > combined[j].FactorValue
		})
		return combined, nil
	}

	// ── 第二步：检测特殊算子（TsCorr / Zscore / Winsorize） ──
	exprLower := strings.ToLower(expr)

	// TsCorr(field1, field2, n)
	if strings.HasPrefix(exprLower, "tscorr(") {
		inner := expr[7 : len(expr)-1] // 去掉 "TsCorr(" 和 ")"
		parts := strings.SplitN(inner, ",", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("TsCorr 需要3个参数: TsCorr(field1,field2,n)，当前: %s", expr)
		}
		field1 := strings.TrimSpace(strings.ToUpper(parts[0]))
		field2 := strings.TrimSpace(strings.ToUpper(parts[1]))
		nStr := strings.TrimSpace(parts[2])
		nVal, err := strconv.Atoi(nStr)
		if err != nil {
			return nil, fmt.Errorf("TsCorr 参数 n 解析失败: %s", nStr)
		}

		var results []FactorResult
		for companyID, fd := range e.data {
			if len(fd.Dates) == 0 {
				continue
			}
			vals1, ok1 := fd.Values[field1]
			vals2, ok2 := fd.Values[field2]
			if !ok1 || !ok2 || len(vals1) == 0 || len(vals2) == 0 {
				continue
			}
			i := len(vals1) - 1
			if len(vals2)-1 < i {
				i = len(vals2) - 1
			}
			val := TsCorr(vals1, vals2, i, nVal)
			results = append(results, FactorResult{
				CompanyID:   companyID,
				Date:        fd.Dates[len(fd.Dates)-1],
				FactorValue: val,
			})
		}
		allVals := make([]float64, len(results))
		for i, r := range results {
			allVals[i] = r.FactorValue
		}
		for i := range results {
			results[i].Rank = CsRank(allVals, results[i].FactorValue)
		}
		sort.Slice(results, func(i, j int) bool {
			return results[i].FactorValue > results[j].FactorValue
		})
		return results, nil
	}

	// Zscore(innerExpr)
	if strings.HasPrefix(exprLower, "zscore(") {
		innerExpr := expr[7 : len(expr)-1] // 去掉 "Zscore(" 和 ")"
		innerResults, err := e.EvalExpr(innerExpr)
		if err != nil {
			return nil, fmt.Errorf("Zscore 内层表达式计算失败: %w", err)
		}
		allVals := make([]float64, len(innerResults))
		for i, r := range innerResults {
			allVals[i] = r.FactorValue
		}
		var results []FactorResult
		for _, r := range innerResults {
			zscore := CsZscore(allVals, r.FactorValue)
			results = append(results, FactorResult{
				CompanyID:   r.CompanyID,
				Date:        r.Date,
				FactorValue: zscore,
			})
		}
		// 重新计算截面 Rank（基于 zscore 值）
		allZscores := make([]float64, len(results))
		for i, r := range results {
			allZscores[i] = r.FactorValue
		}
		for i := range results {
			results[i].Rank = CsRank(allZscores, results[i].FactorValue)
		}
		sort.Slice(results, func(i, j int) bool {
			return results[i].FactorValue > results[j].FactorValue
		})
		return results, nil
	}

	// Winsorize(innerExpr) 或 Winsorize(innerExpr, n)
	if strings.HasPrefix(exprLower, "winsorize(") {
		inner := expr[10 : len(expr)-1] // 去掉 "Winsorize(" 和 ")"
		// 检查是否有第二个参数 n（顶层逗号，不在括号内）
		winN := 3.0
		innerExpr := inner
		// 从右向左找顶层逗号
		depth := 0
		commaIdx := -1
		for i := len(inner) - 1; i >= 0; i-- {
			ch := inner[i]
			if ch == ')' {
				depth++
			} else if ch == '(' {
				depth--
			}
			if depth == 0 && ch == ',' {
				commaIdx = i
				break
			}
		}
		if commaIdx >= 0 {
			nStr := strings.TrimSpace(inner[commaIdx+1:])
			if nf, err := strconv.ParseFloat(nStr, 64); err == nil {
				winN = nf
				innerExpr = strings.TrimSpace(inner[:commaIdx])
			}
		}

		innerResults, err := e.EvalExpr(innerExpr)
		if err != nil {
			return nil, fmt.Errorf("Winsorize 内层表达式计算失败: %w", err)
		}
		allVals := make([]float64, len(innerResults))
		for i, r := range innerResults {
			allVals[i] = r.FactorValue
		}
		var results []FactorResult
		for _, r := range innerResults {
			winVal := CsWinsorize(allVals, r.FactorValue, winN)
			results = append(results, FactorResult{
				CompanyID:   r.CompanyID,
				Date:        r.Date,
				FactorValue: winVal,
			})
		}
		// 重新计算截面 Rank
		allWin := make([]float64, len(results))
		for i, r := range results {
			allWin[i] = r.FactorValue
		}
		for i := range results {
			results[i].Rank = CsRank(allWin, results[i].FactorValue)
		}
		sort.Slice(results, func(i, j int) bool {
			return results[i].FactorValue > results[j].FactorValue
		})
		return results, nil
	}

	// ── 第三步：原有逻辑（Rank嵌套 / 单函数 / 纯字段名） ──
	outerFunc, innerFunc, fieldName, n, isNested, err := parseExpr(expr)
	if err != nil {
		return nil, fmt.Errorf("表达式解析失败: %w", err)
	}

	var results []FactorResult

	if isNested && outerFunc == "Rank" {
		// 先计算所有公司的内层因子值
		type companyVal struct {
			companyID string
			date      time.Time
			val       float64
		}
		var companyVals []companyVal
		for companyID, fd := range e.data {
			if len(fd.Dates) == 0 {
				continue
			}
			val := evalSingleFunc(innerFunc, fieldName, n, fd)
			companyVals = append(companyVals, companyVal{
				companyID: companyID,
				date:      fd.Dates[len(fd.Dates)-1],
				val:       val,
			})
		}

		// 提取所有值用于截面排名
		allVals := make([]float64, len(companyVals))
		for i, cv := range companyVals {
			allVals[i] = cv.val
		}

		// 计算截面排名
		for _, cv := range companyVals {
			rank := CsRank(allVals, cv.val)
			results = append(results, FactorResult{
				CompanyID:   cv.companyID,
				Date:        cv.date,
				FactorValue: cv.val,
				Rank:        rank,
			})
		}
	} else {
		// 非嵌套：直接计算
		for companyID, fd := range e.data {
			if len(fd.Dates) == 0 {
				continue
			}
			var val float64
			if outerFunc == "" {
				// 纯字段名
				vals := fd.Values[fieldName]
				if len(vals) > 0 {
					val = vals[len(vals)-1]
				}
			} else {
				val = evalSingleFunc(outerFunc, fieldName, n, fd)
			}
			results = append(results, FactorResult{
				CompanyID:   companyID,
				Date:        fd.Dates[len(fd.Dates)-1],
				FactorValue: val,
				Rank:        0,
			})
		}

		// 计算截面排名
		allVals := make([]float64, len(results))
		for i, r := range results {
			allVals[i] = r.FactorValue
		}
		for i := range results {
			results[i].Rank = CsRank(allVals, results[i].FactorValue)
		}
	}

	// 按 FactorValue 降序排列
	sort.Slice(results, func(i, j int) bool {
		return results[i].FactorValue > results[j].FactorValue
	})

	return results, nil
}
