package fields

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

// FormulaCalc 基于内存数据的公式计算器
// values: field -> value 的映射（已从数据库查出的原始数据）
// formulas: field -> formula 的映射（从 field_title 加载）
type FormulaCalc struct {
	formulas map[string]string  // field -> formula
	values   map[string]float64 // field -> value（内存缓存）
}

// GetFormulaFields 返回所有有公式的字段名列表（排除函数型公式）
func GetFormulaFields(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`SELECT field FROM field_title WHERE formula IS NOT NULL AND formula != '' AND formula NOT SIMILAR TO '[A-Za-z][A-Za-z0-9_]*\(.*'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var fields []string
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err == nil {
			fields = append(fields, f)
		}
	}
	return fields, nil
}

// NewFormulaCalc 从数据库加载所有公式，创建计算器
func NewFormulaCalc(db *sql.DB) (*FormulaCalc, error) {
	rows, err := db.Query(`SELECT field, formula FROM field_title WHERE formula IS NOT NULL AND formula != '' AND formula NOT SIMILAR TO '[A-Za-z][A-Za-z0-9_]*\(.*'`)
	if err != nil {
		return nil, fmt.Errorf("加载公式失败: %w", err)
	}
	defer rows.Close()

	formulas := make(map[string]string)
	for rows.Next() {
		var field, formula string
		if err := rows.Scan(&field, &formula); err != nil {
			continue
		}
		formulas[field] = formula
	}
	return &FormulaCalc{
		formulas: formulas,
		values:   make(map[string]float64),
	}, nil
}

// SetValues 设置原始字段值（每次计算前调用，传入某一期的所有字段值）
func (c *FormulaCalc) SetValues(values map[string]float64) {
	c.values = values
}

// Calc 计算指定字段的值，返回 nil 表示无法计算（依赖字段缺失）
func (c *FormulaCalc) Calc(field string) *float64 {
	// 先查内存
	if v, ok := c.values[field]; ok {
		return &v
	}

	formula, ok := c.formulas[field]
	if !ok {
		return nil
	}

	result, err := c.evalFormula(formula, 0)
	if err != nil {
		return nil
	}
	c.values[field] = result
	return &result
}

var fieldRefRe = regexp.MustCompile(`\$\(([A-Za-z][A-Za-z0-9_]*)\)`)

func (c *FormulaCalc) evalFormula(formula string, depth int) (float64, error) {
	if depth > 10 {
		return 0, fmt.Errorf("公式嵌套过深")
	}

	expr := formula
	matches := fieldRefRe.FindAllStringSubmatch(formula, -1)
	for _, m := range matches {
		refField := m[1]
		var val float64
		if v, ok := c.values[refField]; ok {
			val = v
		} else if subFormula, ok := c.formulas[refField]; ok {
			v2, err := c.evalFormula(subFormula, depth+1)
			if err != nil {
				return 0, fmt.Errorf("计算依赖字段 %s 失败: %w", refField, err)
			}
			c.values[refField] = v2
			val = v2
		} else {
			return 0, fmt.Errorf("依赖字段 %s 无数据", refField)
		}
		expr = strings.ReplaceAll(expr, m[0], fmt.Sprintf("%f", val))
	}

	return evalExpr(expr)
}

// EvalExpr 计算简单四则运算表达式（含括号），供外部使用
func EvalExpr(expr string) (float64, error) {
	return evalExpr(expr)
}

// evalExpr 计算简单四则运算表达式（含括号）
func evalExpr(expr string) (float64, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return 0, fmt.Errorf("空表达式")
	}
	tokens, err := tokenize(expr)
	if err != nil {
		return 0, err
	}
	val, _, err := parseExpr(tokens, 0)
	return val, err
}

type token struct {
	kind string // "num", "op", "lparen", "rparen"
	val  string
}

func tokenize(expr string) ([]token, error) {
	var tokens []token
	i := 0
	for i < len(expr) {
		ch := expr[i]
		if ch == ' ' {
			i++
			continue
		}
		if ch == '(' {
			tokens = append(tokens, token{"lparen", "("})
			i++
		} else if ch == ')' {
			tokens = append(tokens, token{"rparen", ")"})
			i++
		} else if ch == '+' || ch == '*' || ch == '/' {
			tokens = append(tokens, token{"op", string(ch)})
			i++
		} else if ch == '-' {
			// 判断是负号还是减号
			if len(tokens) == 0 || tokens[len(tokens)-1].kind == "op" || tokens[len(tokens)-1].kind == "lparen" {
				// 负号：读取后面的数字
				i++
				j := i
				for j < len(expr) && (expr[j] >= '0' && expr[j] <= '9' || expr[j] == '.') {
					j++
				}
				if j == i {
					return nil, fmt.Errorf("负号后无数字")
				}
				tokens = append(tokens, token{"num", "-" + expr[i:j]})
				i = j
			} else {
				tokens = append(tokens, token{"op", "-"})
				i++
			}
		} else if (ch >= '0' && ch <= '9') || ch == '.' {
			j := i
			for j < len(expr) && (expr[j] >= '0' && expr[j] <= '9' || expr[j] == '.') {
				j++
			}
			tokens = append(tokens, token{"num", expr[i:j]})
			i = j
		} else {
			return nil, fmt.Errorf("未知字符: %c", ch)
		}
	}
	return tokens, nil
}

func parseExpr(tokens []token, pos int) (float64, int, error) {
	left, pos, err := parseTerm(tokens, pos)
	if err != nil {
		return 0, pos, err
	}
	for pos < len(tokens) && tokens[pos].kind == "op" && (tokens[pos].val == "+" || tokens[pos].val == "-") {
		op := tokens[pos].val
		pos++
		right, newPos, err := parseTerm(tokens, pos)
		if err != nil {
			return 0, newPos, err
		}
		pos = newPos
		if op == "+" {
			left += right
		} else {
			left -= right
		}
	}
	return left, pos, nil
}

func parseTerm(tokens []token, pos int) (float64, int, error) {
	left, pos, err := parseFactor(tokens, pos)
	if err != nil {
		return 0, pos, err
	}
	for pos < len(tokens) && tokens[pos].kind == "op" && (tokens[pos].val == "*" || tokens[pos].val == "/") {
		op := tokens[pos].val
		pos++
		right, newPos, err := parseFactor(tokens, pos)
		if err != nil {
			return 0, newPos, err
		}
		pos = newPos
		if op == "*" {
			left *= right
		} else {
			if right == 0 {
				return 0, newPos, fmt.Errorf("除以零")
			}
			left /= right
		}
	}
	return left, pos, nil
}

func parseFactor(tokens []token, pos int) (float64, int, error) {
	if pos >= len(tokens) {
		return 0, pos, fmt.Errorf("表达式不完整")
	}
	t := tokens[pos]
	if t.kind == "num" {
		var v float64
		fmt.Sscanf(t.val, "%f", &v)
		return v, pos + 1, nil
	}
	if t.kind == "lparen" {
		val, newPos, err := parseExpr(tokens, pos+1)
		if err != nil {
			return 0, newPos, err
		}
		if newPos >= len(tokens) || tokens[newPos].kind != "rparen" {
			return 0, newPos, fmt.Errorf("缺少右括号")
		}
		return val, newPos + 1, nil
	}
	return 0, pos, fmt.Errorf("意外的token: %v", t)
}
