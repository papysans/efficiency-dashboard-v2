package main

import (
	"context"
	"fmt"
	"kanban/core/models"
	"kanban/kbcli/internal/appconfig"
	"kanban/kbcli/internal/logx"
)

func withImportAdvisoryLock(command string, fn func() error) error {
	ctx := context.Background()
	db, err := models.OpenSQLDB(appconfig.Cfg.StatDatabase.DSN())
	if err != nil {
		return fmt.Errorf("连接统计数据库获取导入锁失败: %w", err)
	}
	defer db.Close()

	lock, ok, err := models.TryAcquireAdvisoryLock(ctx, db, models.AdvisoryLockImport)
	if err != nil {
		return fmt.Errorf("获取导入互斥锁失败: %w", err)
	}
	if !ok {
		logx.Warnf("[import-lock] 已有导入任务正在运行，跳过当前任务: %s", command)
		return nil
	}
	defer func() {
		if err := lock.Release(ctx); err != nil {
			logx.Warnf("[import-lock] 释放导入互斥锁失败: %v", err)
		}
	}()

	logx.Infof("[import-lock] 已获得导入互斥锁: %s", command)
	return fn()
}
