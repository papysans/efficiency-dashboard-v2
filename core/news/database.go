package news

import (
	"database/sql"
	"fmt"
	"time"
)

// SaveNewsRecords 将新闻数据 Upsert 到 news_flow_records 表
func SaveNewsRecords(db *sql.DB, items []NewsItem, platform string) error {
	query := `
		INSERT INTO news_flow_records (platform, title, url, content, publish_time, score, rank)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (platform, title, publish_time) DO UPDATE SET
			score = EXCLUDED.score,
			rank  = EXCLUDED.rank,
			url   = EXCLUDED.url
	`

	var lastErr error
	count := 0
	for _, item := range items {
		// 解析发布时间，格式为 "2006-01-02 15:04:05"
		var publishTime interface{}
		if item.PublishTime != "" {
			t, err := time.Parse("2006-01-02 15:04:05", item.PublishTime)
			if err == nil {
				publishTime = t
			} else {
				publishTime = nil
			}
		}

		_, err := db.Exec(query,
			platform,
			item.Title,
			item.URL,
			item.Content,
			publishTime,
			item.Score,
			item.Rank,
		)
		if err != nil {
			lastErr = fmt.Errorf("写入新闻记录失败 [%s]: %w", item.Title[:min(20, len(item.Title))], err)
			continue
		}
		count++
	}

	if lastErr != nil {
		return fmt.Errorf("部分写入失败（成功%d条）: %w", count, lastErr)
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// LoadNews 从数据库读取新闻列表，支持分页
func LoadNews(db *sql.DB, limit, offset int) ([]NewsItem, error) {
	query := `
		SELECT platform, title, url, content, 
		       COALESCE(TO_CHAR(publish_time, 'YYYY-MM-DD HH24:MI:SS'), '') as publish_time,
		       COALESCE(score, 0), COALESCE(rank, 0)
		FROM news_flow_records
		ORDER BY publish_time DESC NULLS LAST, score DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := db.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("查询新闻失败: %w", err)
	}
	defer rows.Close()

	var items []NewsItem
	for rows.Next() {
		var item NewsItem
		err := rows.Scan(&item.Source, &item.Title, &item.URL, &item.Content,
			&item.PublishTime, &item.Score, &item.Rank)
		if err != nil {
			return nil, fmt.Errorf("扫描新闻记录失败: %w", err)
		}
		items = append(items, item)
	}

	if items == nil {
		items = []NewsItem{}
	}
	return items, nil
}
