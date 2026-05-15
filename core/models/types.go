package models

import (
	"database/sql/driver"
	"encoding/json"
)

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

// MarshalJSON 实现 json.Marshaler 接口，原样输出 JSON 内容
func (j StringJSON) MarshalJSON() ([]byte, error) {
	if j == "" || j == "null" {
		return []byte("[]"), nil
	}
	return []byte(j), nil
}

// UnmarshalJSON 实现 json.Unmarshaler 接口
func (j *StringJSON) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*j = "[]"
		return nil
	}
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*j = StringJSON(s)
		return nil
	}
	*j = StringJSON(data)
	return nil
}
