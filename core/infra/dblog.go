package infra

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// QueryWithLog 执行查询并记录日志
func QueryWithLog(db *sql.DB, query string, args ...interface{}) (*sql.Rows, error) {
	// 如果Logger未初始化，直接执行查询
	if Logger == nil {
		return db.Query(query, args...)
	}

	fullSQL := replacePlaceholders(query, args)
	Logger.Info("SQL Query:\n%s", fullSQL)

	start := time.Now()
	rows, err := db.Query(query, args...)
	duration := time.Since(start)

	if err != nil {
		Logger.Error("SQL Query Error: %v (duration: %v)", err, duration)
	} else {
		Logger.Info("SQL Query Success (duration: %v)", duration)
	}

	return rows, err
}

// QueryRowWithLog 执行单行查询并记录日志
func QueryRowWithLog(db *sql.DB, query string, args ...interface{}) *sql.Row {
	// 如果Logger未初始化，直接执行查询
	if Logger == nil {
		return db.QueryRow(query, args...)
	}

	fullSQL := replacePlaceholders(query, args)
	Logger.Info("SQL QueryRow:\n%s", fullSQL)

	start := time.Now()
	row := db.QueryRow(query, args...)
	duration := time.Since(start)

	Logger.Info("SQL QueryRow executed (duration: %v)", duration)

	return row
}

// ExecWithLog 执行SQL命令并记录日志
func ExecWithLog(db *sql.DB, query string, args ...interface{}) (sql.Result, error) {
	// 如果Logger未初始化，直接执行查询
	if Logger == nil {
		return db.Exec(query, args...)
	}

	fullSQL := replacePlaceholders(query, args)
	Logger.Info("SQL Exec:\n%s", fullSQL)

	start := time.Now()
	result, err := db.Exec(query, args...)
	duration := time.Since(start)

	if err != nil {
		Logger.Error("SQL Exec Error: %v (duration: %v)", err, duration)
	} else {
		Logger.Info("SQL Exec Success (duration: %v)", duration)
	}

	return result, err
}

// fmtValue 格式化参数值为SQL字符串
func fmtValue(v interface{}) string {
	if v == nil {
		return "NULL"
	}

	rv := reflect.ValueOf(v)

	// 处理字符串类型
	if s, ok := v.(string); ok {
		return fmt.Sprintf("'%s'", s)
	}

	// 处理时间类型
	if t, ok := v.(time.Time); ok {
		return fmt.Sprintf("'%s'", t.Format("2006-01-02"))
	}

	// 处理指针类型
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return "NULL"
		}
		return fmtValue(rv.Elem().Interface())
	}

	// 处理切片/数组类型
	if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
		var parts []string
		for i := 0; i < rv.Len(); i++ {
			elem := rv.Index(i).Interface()
			parts = append(parts, fmt.Sprintf("'%v'", elem))
		}
		return fmt.Sprintf("ARRAY[%s]", strings.Join(parts, ","))
	}

	return fmt.Sprintf("%v", v)
}

// replacePlaceholders 替换SQL中的占位符
func replacePlaceholders(query string, args []interface{}) string {
	result := query
	for i, arg := range args {
		placeholder := fmt.Sprintf("$%d", i+1)
		value := fmtValue(arg)
		result = strings.Replace(result, placeholder, value, 1)
	}
	return result
}
