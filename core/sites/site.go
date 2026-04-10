package sites

import (
	"context"
	"database/sql"
	"time"
)

// FetchOptions 抓取选项
type FetchOptions struct {
	DB        *sql.DB
	CompanyID string
	StockCode string // 大写雪球格式，如 SZ300454
	ProxyURL  string
	Timeout   time.Duration
	Force     bool // 强制重新下载，忽略已有文件
	From      string // 起始年份（格式：2018）
	To        string // 结束年份（格式：2024）
}

// Site 数据源插件接口
type Site interface {
	Name() string
	Fetch(ctx context.Context, opts FetchOptions) error
}

// registry 全局注册表
var registry = map[string]Site{}

// Register 注册一个 Site 插件（以 s.Name() 为 key）
func Register(s Site) {
	registry[s.Name()] = s
}

// Get 获取指定名称的 Site 插件
func Get(name string) (Site, bool) {
	s, ok := registry[name]
	return s, ok
}
