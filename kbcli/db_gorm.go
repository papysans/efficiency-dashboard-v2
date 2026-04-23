package main

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func isPostgresUndefinedColumn(err error) bool {
	if pqErr, ok := err.(*pq.Error); ok {
		return pqErr.Code == "42703"
	}
	return false
}

func postgresOpener(dsn string) gorm.Dialector {
	return postgres.Open(dsn)
}

func openSQLDB(dsn string) (*sql.DB, error) {
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

func execDDLIgnoreError(db *gorm.DB, ddl string) {
	if err := db.Exec(ddl).Error; err != nil {
		fmt.Fprintf(os.Stderr, "警告: 执行DDL失败(可忽略): %v\n", err)
	}
}

func isPostgresUndefinedColumnGorm(err error) bool {
	if err == nil {
		return false
	}
	return isPostgresUndefinedColumn(err)
}
