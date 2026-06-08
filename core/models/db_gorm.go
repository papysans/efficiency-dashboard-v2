package models

import (
	"database/sql"

	_ "github.com/lib/pq" // 注册 postgres 驱动，供 OpenSQLDB 的 sql.Open("postgres") 使用

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func postgresOpener(dsn string) gorm.Dialector {
	return postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true,
	})
}

func OpenSQLDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	return db, nil
}
