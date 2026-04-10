package fields

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
)

type FieldAliasManager struct {
	db       *sql.DB
	aliasMap map[string]map[string]string // source -> alias_field -> canonical_field
	mu       sync.RWMutex
}

var (
	fieldAliasManager *FieldAliasManager
	aliasManagerOnce  sync.Once
)

func InitFieldAliasManager(db *sql.DB) error {
	aliasManagerOnce.Do(func() {
		fieldAliasManager = &FieldAliasManager{
			db:       db,
			aliasMap: make(map[string]map[string]string),
		}
	})
	return nil
}

func GetFieldAliasManager() *FieldAliasManager {
	return fieldAliasManager
}

func FieldAliasLoad(db *sql.DB) error {
	if err := InitFieldAliasManager(db); err != nil {
		return err
	}

	mgr := GetFieldAliasManager()
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	mgr.aliasMap = make(map[string]map[string]string)

	rows, err := db.Query(`SELECT source, alias_field, canonical_field FROM field_alias`)
	if err != nil {
		return fmt.Errorf("查询field_alias表失败: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var source, alias, canonical string
		if err := rows.Scan(&source, &alias, &canonical); err != nil {
			return fmt.Errorf("扫描field_alias记录失败: %w", err)
		}
		if mgr.aliasMap[source] == nil {
			mgr.aliasMap[source] = make(map[string]string)
		}
		mgr.aliasMap[source][alias] = canonical
		count++
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("遍历field_alias记录失败: %w", err)
	}

	log.Printf("从数据库加载了 %d 个字段别名映射", count)
	return nil
}

// FieldAliasResolve 将 alias_field 解析为 canonical_field，找不到则返回原值
func FieldAliasResolve(source, field string) string {
	mgr := GetFieldAliasManager()
	if mgr == nil {
		return field
	}

	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	if srcMap, ok := mgr.aliasMap[source]; ok {
		if canonical, ok := srcMap[field]; ok {
			return canonical
		}
	}
	return field
}

// FieldAliasAdd 在运行时添加一条别名映射（仅写入数据库，不影响内存缓存直到下次 Load）
func FieldAliasAdd(db *sql.DB, source, aliasField, canonicalField, remark string) error {
	_, err := db.Exec(`
		INSERT INTO field_alias (alias_field, canonical_field, source, remark)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (alias_field, source) DO UPDATE SET
			canonical_field = EXCLUDED.canonical_field,
			remark = EXCLUDED.remark
	`, aliasField, canonicalField, source, remark)
	if err != nil {
		return fmt.Errorf("添加字段别名失败: %w", err)
	}

	mgr := GetFieldAliasManager()
	if mgr != nil {
		mgr.mu.Lock()
		if mgr.aliasMap[source] == nil {
			mgr.aliasMap[source] = make(map[string]string)
		}
		mgr.aliasMap[source][aliasField] = canonicalField
		mgr.mu.Unlock()
	}
	return nil
}
