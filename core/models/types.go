package models

import "database/sql/driver"

// StringJSON 用于数据库 JSONB 列的自定义类型
type StringJSON string

// Scan 实现 sql.Scanner 接口
func (j *StringJSON) Scan(value interface{}) error {
	if value == nil {
		*j = ""
		return nil
	}
	switch v := value.(type) {
	case []byte:
		*j = StringJSON(v)
	case string:
		*j = StringJSON(v)
	}
	return nil
}

// Value 实现 driver.Valuer 接口
func (j StringJSON) Value() (driver.Value, error) {
	if j == "" {
		return "[]", nil
	}
	return string(j), nil
}
