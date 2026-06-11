package models

import (
	"context"
	"database/sql"
	"fmt"
)

const (
	AdvisoryLockImport      int64 = 0x4b414e42414e0001
	AdvisoryLockAutoMigrate int64 = 0x4b414e42414e0002
)

type AdvisoryLock struct {
	conn *sql.Conn
	key  int64
}

func AcquireAdvisoryLock(ctx context.Context, db *sql.DB, key int64) (*AdvisoryLock, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, key); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return &AdvisoryLock{conn: conn, key: key}, nil
}

func TryAcquireAdvisoryLock(ctx context.Context, db *sql.DB, key int64) (*AdvisoryLock, bool, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, false, err
	}
	var acquired bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, key).Scan(&acquired); err != nil {
		_ = conn.Close()
		return nil, false, err
	}
	if !acquired {
		_ = conn.Close()
		return nil, false, nil
	}
	return &AdvisoryLock{conn: conn, key: key}, true, nil
}

func (l *AdvisoryLock) Release(ctx context.Context) error {
	if l == nil || l.conn == nil {
		return nil
	}
	var unlocked bool
	err := l.conn.QueryRowContext(ctx, `SELECT pg_advisory_unlock($1)`, l.key).Scan(&unlocked)
	closeErr := l.conn.Close()
	l.conn = nil
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if !unlocked {
		return fmt.Errorf("advisory lock %d was not held", l.key)
	}
	return nil
}
