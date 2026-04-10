package fields

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
)

var FieldTitleMapping = map[string]string{}

type FieldTitleManager struct {
	db               *sql.DB
	fieldTitleMap    map[string]string // 内存缓存 field -> title
	titleFieldMap    map[string]string // 内存缓存 title -> field（反向查找）
	fieldUnitMap     map[string]string // 内存缓存 field -> unit
	fieldUnitTypeMap map[string]string // 内存缓存 field -> unit_type
	pendingInsert    []FieldTitlePair  // 待插入的field-title对
	mu               sync.RWMutex      // 读写锁
}

// FieldTitlePair field和title的键值对
type FieldTitlePair struct {
	Field string
	Title string
}

var (
	fieldTitleManager *FieldTitleManager
	managerOnce       sync.Once
)

// InitFieldTitleManager 初始化FieldTitleManager单例
func InitFieldTitleManager(db *sql.DB) error {
	var err error
	managerOnce.Do(func() {
		fieldTitleManager = &FieldTitleManager{
			db:               db,
			fieldTitleMap:    make(map[string]string),
			titleFieldMap:    make(map[string]string),
			fieldUnitMap:     make(map[string]string),
			fieldUnitTypeMap: make(map[string]string),
			pendingInsert:    make([]FieldTitlePair, 0),
		}
	})
	return err
}

// GetFieldTitleManager 获取FieldTitleManager单例
func GetFieldTitleManager() *FieldTitleManager {
	return fieldTitleManager
}

// FieldTitleLoad 从数据库中加载Field和title的映射关系为map
func FieldTitleLoad(db *sql.DB) error {
	if err := InitFieldTitleManager(db); err != nil {
		return err
	}

	mgr := GetFieldTitleManager()
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	// 清空现有缓存
	mgr.fieldTitleMap = make(map[string]string)
	mgr.titleFieldMap = make(map[string]string)
	mgr.fieldUnitMap = make(map[string]string)
	mgr.fieldUnitTypeMap = make(map[string]string)

	// 从数据库加载
	rows, err := db.Query(`SELECT field, title, COALESCE(unit, ''), COALESCE(unit_type, 'amount') FROM field_title`)
	if err != nil {
		return fmt.Errorf("查询field_title表失败: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var field, title, unit, unitType string
		if err := rows.Scan(&field, &title, &unit, &unitType); err != nil {
			return fmt.Errorf("扫描field_title记录失败: %w", err)
		}
		mgr.fieldTitleMap[field] = title
		mgr.titleFieldMap[title] = field
		if unit != "" {
			mgr.fieldUnitMap[field] = unit
		}
		mgr.fieldUnitTypeMap[field] = unitType
		count++
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("遍历field_title记录失败: %w", err)
	}

	log.Printf("从数据库加载了 %d 个field-title映射", count)
	return nil
}

// FieldTitleExists 判断某个Field是否存在
func FieldTitleExists(field string) bool {
	mgr := GetFieldTitleManager()
	if mgr == nil {
		return false
	}

	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	_, exists := mgr.fieldTitleMap[field]
	return exists
}

// FieldTitleGet 基于field来查找title，找不到返回field
func FieldTitleGet(field string) string {
	mgr := GetFieldTitleManager()
	if mgr == nil {
		return field
	}

	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	if title, ok := mgr.fieldTitleMap[field]; ok {
		return title
	}
	return field
}

// FieldUnitGet 基于field来查找unit，找不到返回空字符串
func FieldUnitGet(field string) string {
	mgr := GetFieldTitleManager()
	if mgr == nil {
		return ""
	}

	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	return mgr.fieldUnitMap[field]
}

// FieldUnitTypeGet 基于field来查找unit_type，找不到返回 "amount"
func FieldUnitTypeGet(field string) string {
	mgr := GetFieldTitleManager()
	if mgr == nil {
		return "amount"
	}

	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	if ut, ok := mgr.fieldUnitTypeMap[field]; ok {
		return ut
	}
	return "amount"
}

// FieldTitleUpdateCache 更新内存缓存中的field对应的title、unit、unit_type
func FieldTitleUpdateCache(field, title, unit, unitType string) {
	mgr := GetFieldTitleManager()
	if mgr == nil {
		return
	}

	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if oldTitle, ok := mgr.fieldTitleMap[field]; ok {
		delete(mgr.titleFieldMap, oldTitle)
	}
	mgr.fieldTitleMap[field] = title
	mgr.titleFieldMap[title] = field
	if unit != "" {
		mgr.fieldUnitMap[field] = unit
	} else {
		delete(mgr.fieldUnitMap, field)
	}
	mgr.fieldUnitTypeMap[field] = unitType
}

// FieldTitleInsert 将新的field title对插入到map中，并记录到待插入数组
func FieldTitleInsert(field, title string) {
	mgr := GetFieldTitleManager()
	if mgr == nil {
		return
	}

	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	// 检查是否已存在
	if _, exists := mgr.fieldTitleMap[field]; exists {
		return
	}

	// 添加到内存缓存（双向映射）
	mgr.fieldTitleMap[field] = title
	mgr.titleFieldMap[title] = field

	// 记录到待插入数组
	mgr.pendingInsert = append(mgr.pendingInsert, FieldTitlePair{
		Field: field,
		Title: title,
	})
}

// FieldTitleGetByTitle 基于title来查找field，找不到返回空字符串
func FieldTitleGetByTitle(title string) string {
	mgr := GetFieldTitleManager()
	if mgr == nil {
		return ""
	}

	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	if field, ok := mgr.titleFieldMap[title]; ok {
		return field
	}
	return ""
}

// FieldTitleSave 将待插入数组中的field title对更新到数据库，并清空待插入数组
func FieldTitleSave() error {
	mgr := GetFieldTitleManager()
	if mgr == nil {
		return fmt.Errorf("FieldTitleManager未初始化")
	}

	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if len(mgr.pendingInsert) == 0 {
		log.Printf("没有需要保存的field-title映射")
		return nil
	}

	// 开始事务
	tx, err := mgr.db.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	// 准备插入语句
	stmt, err := tx.Prepare(`
		INSERT INTO field_title (field, title, created_at, updated_at)
		VALUES ($1, $2, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT (field) DO UPDATE SET
			title = EXCLUDED.title,
			updated_at = CURRENT_TIMESTAMP
	`)
	if err != nil {
		return fmt.Errorf("准备插入语句失败: %w", err)
	}
	defer stmt.Close()

	// 批量插入
	insertCount := 0
	for _, pair := range mgr.pendingInsert {
		if _, err := stmt.Exec(pair.Field, pair.Title); err != nil {
			log.Printf("插入field-title失败 [%s -> %s]: %v", pair.Field, pair.Title, err)
			continue
		}
		insertCount++
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	log.Printf("成功保存 %d 个新的field-title映射到数据库", insertCount)

	// 清空待插入数组
	mgr.pendingInsert = make([]FieldTitlePair, 0)

	return nil
}

// FieldTitleInitFromMapping 从FieldTitleMapping初始化数据库（仅在首次使用时）
func FieldTitleInitFromMapping(db *sql.DB) error {
	// 检查表是否为空
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM field_title`).Scan(&count)
	if err != nil {
		return fmt.Errorf("查询field_title表失败: %w", err)
	}

	if count > 0 {
		log.Printf("field_title表已有数据，跳过初始化")
		return nil
	}

	// 表为空，从FieldTitleMapping初始化
	log.Printf("开始从FieldTitleMapping初始化field_title表...")

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO field_title (field, title, created_at, updated_at)
		VALUES ($1, $2, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		return fmt.Errorf("准备插入语句失败: %w", err)
	}
	defer stmt.Close()

	insertCount := 0
	for field, title := range FieldTitleMapping {
		if _, err := stmt.Exec(field, title); err != nil {
			log.Printf("插入field-title失败 [%s -> %s]: %v", field, title, err)
			continue
		}
		insertCount++
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	log.Printf("成功从FieldTitleMapping初始化 %d 个field-title映射到数据库", insertCount)
	return nil
}
