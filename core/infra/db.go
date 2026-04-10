package infra

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"sslmode"`
}

// GetDSN 获取数据库连接字符串
func (c *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode)
}

// InitDB 初始化数据库连接
func InitDB(config DatabaseConfig) (*sql.DB, error) {
	db, err := sql.Open("postgres", config.GetDSN())
	if err != nil {
		return nil, fmt.Errorf("打开数据库连接失败: %w", err)
	}

	// 测试连接
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("数据库连接测试失败: %w", err)
	}

	// 设置连接池参数
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	return db, nil
}

// Company 公司模型
type Company struct {
	ID          string  `json:"id"`
	Market      string  `json:"market"`
	Code        string  `json:"code"`
	ViewName    string  `json:"view_name"`    // 根据UI视图而修改的名称
	LocalName   *string `json:"local_name"`   // 公司本地名称
	ListingDate *string `json:"listing_date"` // 上市日期
	Industry    *string `json:"industry"`     // 所属行业
}

// Fin 财务数据模型（合并后的表）
type Fin struct {
	ID              string   `json:"id"`                // ID格式: company_id + report_date + item_field
	CompanyID       string   `json:"company_id"`        // 公司ID
	ReportDate      string   `json:"report_date"`       // 报告日期（DATE格式）
	ReportType      string   `json:"report_type"`       // 报表类型（fzb/lrb/llb）
	ItemField       string   `json:"item_field"`        // 字段名
	ItemValue       *float64 `json:"item_value"`        // 项目值（人民币，单位元）
	ItemDisplayType int      `json:"item_display_type"` // 显示类型
	ItemGroupNo     int      `json:"item_group_no"`     // 分组编号
	ItemTongbi      *float64 `json:"item_tongbi"`       // 同比增长率
	UpdatedAt       string   `json:"updated_at"`        // 更新时间
}

// UpsertFin 插入或更新 fin 表记录
// 返回 (isNew bool, error)
func UpsertFin(db *sql.DB, id, companyID string, reportDate interface{}, reportType, itemField string, itemValue *float64, itemDisplayType, itemGroupNo *int, itemTongbi *float64) (bool, error) {
	var existingID string
	err := db.QueryRow(`
		SELECT id FROM fin
		WHERE company_id = $1 AND report_date = $2 AND item_field = $3
	`, companyID, reportDate, itemField).Scan(&existingID)

	if err == sql.ErrNoRows {
		_, err = db.Exec(`
			INSERT INTO fin (
				id, company_id, report_date, report_type, item_field,
				item_value, item_display_type, item_group_no, item_tongbi
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, id, companyID, reportDate, reportType, itemField,
			itemValue, itemDisplayType, itemGroupNo, itemTongbi)
		if err != nil {
			return false, fmt.Errorf("插入财务数据失败: %w", err)
		}
		return true, nil
	} else if err != nil {
		return false, fmt.Errorf("查询财务数据失败: %w", err)
	}

	_, err = db.Exec(`
		UPDATE fin SET
			item_value = $1,
			item_display_type = $2,
			item_group_no = $3,
			item_tongbi = $4,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $5
	`, itemValue, itemDisplayType, itemGroupNo, itemTongbi, existingID)
	if err != nil {
		return false, fmt.Errorf("更新财务数据失败: %w", err)
	}
	return false, nil
}
