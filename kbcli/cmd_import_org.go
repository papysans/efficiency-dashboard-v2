package main

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"fmt"
	"kanban/core/models"
	"kanban/core/storage"
	"kanban/kbcli/internal/appconfig"
	"kanban/kbcli/internal/logx"
	"kanban/kbcli/internal/util"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/spf13/cobra"
)

func replaceDBName(dsn, newDBName string) string {
	result := dsn
	if !strings.Contains(result, "dbname=") {
		result += " dbname=" + newDBName
	} else {
		result = strings.ReplaceAll(result,
			strings.Join([]string{"dbname=", extractDBName(dsn)}, ""),
			"dbname="+newDBName)
	}
	return result
}

func extractDBName(dsn string) string {
	parts := strings.Split(dsn, " ")
	for _, part := range parts {
		if strings.HasPrefix(part, "dbname=") {
			dbname := strings.TrimPrefix(part, "dbname=")
			if idx := strings.Index(dbname, " "); idx != -1 {
				dbname = dbname[:idx]
			}
			return dbname
		}
	}
	return ""
}

func loadUserOrgsFromCSV(csvFile string) ([]models.UserOrg, error) {
	f, err := storage.Open(csvFile)
	if err != nil {
		return nil, fmt.Errorf("打开CSV文件失败: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("读取CSV文件失败: %w", err)
	}

	if len(records) < 1 {
		return nil, fmt.Errorf("CSV文件为空")
	}

	var userOrgs []models.UserOrg
	for i, record := range records {
		if i == 0 {
			continue
		}
		if len(record) < 13 {
			return nil, fmt.Errorf("第 %d 行数据列数不足，需要13列", i+1)
		}

		userOrg := models.UserOrg{
			UserId:       record[0],
			UserName:     record[1],
			Org1:         record[2],
			Org2:         record[3],
			Org3:         record[4],
			Org4:         record[5],
			Org5:         record[6],
			Org6:         record[7],
			Org7:         record[8],
			Org8:         record[9],
			Org9:         record[10],
			GitUserName:  record[11],
			GitUserEmail: record[12],
		}
		userOrgs = append(userOrgs, userOrg)
	}
	logx.Infof("从CSV文件加载到 %d 条用户组织记录", len(userOrgs))
	return userOrgs, nil
}

func loadUserOrgsFromDB(fromDSN string) ([]models.UserOrg, error) {
	authDSN := replaceDBName(fromDSN, "auth")
	authDB, err := models.OpenSQLDB(authDSN)
	if err != nil {
		return nil, fmt.Errorf("连接 auth 数据库失败: %w", err)
	}
	defer authDB.Close()
	logx.Info("auth 数据库连接成功")

	quotaDSN := replaceDBName(fromDSN, "quota_manager")
	quotaDB, err := models.OpenSQLDB(quotaDSN)
	if err != nil {
		return nil, fmt.Errorf("连接 quota_manager 数据库失败: %w", err)
	}
	defer quotaDB.Close()
	logx.Info("quota_manager 数据库连接成功")

	userRows, err := authDB.Query(`
		SELECT id, name, github_name, email, employee_number
		FROM auth_users
		WHERE employee_number IS NOT NULL AND employee_number != ''
		ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("查询 auth_users 失败: %w", err)
	}
	defer userRows.Close()

	deptRows, err := quotaDB.Query(`SELECT employee_number, dept_full_level_names FROM employee_department`)
	if err != nil {
		return nil, fmt.Errorf("查询 employee_department 失败: %w", err)
	}
	defer deptRows.Close()

	deptMap := make(map[string]string)
	for deptRows.Next() {
		var empNum, deptFullLevelNames sql.NullString
		if err := deptRows.Scan(&empNum, &deptFullLevelNames); err != nil {
			return nil, fmt.Errorf("读取部门数据失败: %w", err)
		}
		if empNum.Valid {
			deptMap[empNum.String] = deptFullLevelNames.String
		}
	}
	if err := deptRows.Err(); err != nil {
		return nil, fmt.Errorf("遍历部门数据失败: %w", err)
	}

	var userOrgs []models.UserOrg
	for userRows.Next() {
		var userID, userName, gitUserName, gitUserEmail string
		var empNum sql.NullString
		if err := userRows.Scan(&userID, &userName, &gitUserName, &gitUserEmail, &empNum); err != nil {
			return nil, fmt.Errorf("读取用户数据失败: %w", err)
		}

		org := models.UserOrg{
			UserId:       userID,
			UserName:     userName,
			GitUserName:  gitUserName,
			GitUserEmail: gitUserEmail,
		}

		if empNum.Valid {
			if deptFullLevelNames, ok := deptMap[empNum.String]; ok {
				parts := strings.Split(deptFullLevelNames, ",")
				orgFields := []*string{&org.Org1, &org.Org2, &org.Org3, &org.Org4, &org.Org5, &org.Org6, &org.Org7, &org.Org8, &org.Org9}
				for i, p := range parts {
					if i >= len(orgFields) {
						break
					}
					*orgFields[i] = strings.TrimSpace(p)
				}
			}
		}
		userOrgs = append(userOrgs, org)
	}
	if err := userRows.Err(); err != nil {
		return nil, fmt.Errorf("遍历用户数据失败: %w", err)
	}
	logx.Infof("查询到 %d 条用户组织记录", len(userOrgs))
	return userOrgs, nil
}

func loadDefaultUserOrgsFromLocalData(db *gorm.DB) ([]models.UserOrg, error) {
	type userRow struct {
		UserId       string
		UserName     string
		GitUserName  string
		GitUserEmail string
	}

	rowsByUser := make(map[string]userRow)
	merge := func(rows []userRow) {
		for _, row := range rows {
			row.UserId = strings.TrimSpace(row.UserId)
			if row.UserId == "" {
				continue
			}
			existing := rowsByUser[row.UserId]
			if existing.UserName == "" {
				existing.UserName = strings.TrimSpace(row.UserName)
			}
			if existing.GitUserName == "" {
				existing.GitUserName = strings.TrimSpace(row.GitUserName)
			}
			if existing.GitUserEmail == "" {
				existing.GitUserEmail = strings.TrimSpace(row.GitUserEmail)
			}
			rowsByUser[row.UserId] = existing
		}
	}

	var taskRows []userRow
	if err := db.Raw(`
		SELECT DISTINCT user_id, user_name, '' AS git_user_name, '' AS git_user_email
		FROM tasks
		WHERE user_id IS NOT NULL AND user_id != ''
	`).Scan(&taskRows).Error; err != nil {
		return nil, fmt.Errorf("从 tasks 收集用户失败: %w", err)
	}
	merge(taskRows)

	var commitRows []userRow
	if err := db.Raw(`
		SELECT DISTINCT user_id, user_name, git_user_name, git_user_email
		FROM commits
		WHERE user_id IS NOT NULL AND user_id != ''
	`).Scan(&commitRows).Error; err != nil {
		return nil, fmt.Errorf("从 commits 收集用户失败: %w", err)
	}
	merge(commitRows)

	var sessionRows []userRow
	if err := db.Raw(`
		SELECT DISTINCT user_id, user_name, '' AS git_user_name, '' AS git_user_email
		FROM sessions
		WHERE user_id IS NOT NULL AND user_id != ''
	`).Scan(&sessionRows).Error; err != nil {
		return nil, fmt.Errorf("从 sessions 收集用户失败: %w", err)
	}
	merge(sessionRows)

	userIDs := make([]string, 0, len(rowsByUser))
	for userID := range rowsByUser {
		userIDs = append(userIDs, userID)
	}
	sort.Strings(userIDs)

	userOrgs := make([]models.UserOrg, 0, len(userIDs))
	for _, userID := range userIDs {
		row := rowsByUser[userID]
		userOrgs = append(userOrgs, models.UserOrg{
			UserId:       userID,
			UserName:     row.UserName,
			Org1:         "临时组织",
			GitUserName:  row.GitUserName,
			GitUserEmail: row.GitUserEmail,
		})
	}
	return userOrgs, nil
}

func writeOrgCSV(path string, rows []models.UserOrg) error {
	// 先写入内存缓冲，再经 storage 一次性落盘/上传（兼容 s3 后端）
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	if err := w.Write([]string{"user_id", "user_name", "org1", "org2", "org3", "org4", "org5", "org6", "org7", "org8", "org9", "git_user_name", "git_user_email"}); err != nil {
		return err
	}

	for _, r := range rows {
		if err := w.Write([]string{r.UserId, r.UserName, r.Org1, r.Org2, r.Org3, r.Org4, r.Org5, r.Org6, r.Org7, r.Org8, r.Org9, r.GitUserName, r.GitUserEmail}); err != nil {
			return err
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}
	return storage.WriteFile(path, buf.Bytes())
}

func saveUserOrgs(db *gorm.DB, rows []models.UserOrg) error {
	return db.Transaction(func(tx *gorm.DB) error {
		for _, r := range rows {
			result := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "user_id"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"user_name", "org1", "org2", "org3", "org4", "org5", "org6", "org7", "org8", "org9",
					"git_user_name", "git_user_email", "updated_at",
				}),
			}).Create(&r)
			if result.Error != nil {
				return fmt.Errorf("写入记录失败 [user_id=%s]: %w", r.UserId, result.Error)
			}
		}
		return nil
	})
}

// saveUserOrgsInsertOnly 只插入新用户、绝不更新已有行（ON CONFLICT DO NOTHING）。
// 专供"临时组织"占位兜底路径使用：定时 import 再也不会把 import-dept / 人工 SQL
// 写入的真实 org 覆盖成"临时组织"，但全新用户仍会得到"临时组织"占位。
func saveUserOrgsInsertOnly(db *gorm.DB, rows []models.UserOrg) error {
	return db.Transaction(func(tx *gorm.DB) error {
		for _, r := range rows {
			result := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "user_id"}},
				DoNothing: true,
			}).Create(&r)
			if result.Error != nil {
				return fmt.Errorf("写入记录失败 [user_id=%s]: %w", r.UserId, result.Error)
			}
		}
		return nil
	})
}

func runImportOrg(fromDSN, fromCSV, toCSV string) error {
	startTime := time.Now()
	var userOrgs []models.UserOrg
	var err error
	var gormDB *gorm.DB

	if fromCSV != "" {
		userOrgs, err = loadUserOrgsFromCSV(fromCSV)
		if err != nil {
			util.RecordCommandRun("import-org", startTime, 0, 0, 0, err)
			return err
		}
	} else {
		userOrgs, err = loadUserOrgsFromDB(fromDSN)
		if err != nil {
			logx.Warnf("从 auth/quota 导入组织失败，改用本地已导入用户生成临时组织: %v", err)
		}
	}

	gormDB, err = models.OpenGormDB(appconfig.Cfg.StatDatabase.DSN())
	if err != nil {
		util.RecordCommandRun("import-org", startTime, 0, 0, 0, err)
		return fmt.Errorf("连接目标数据库失败: %w", err)
	}
	sqlDB, _ := gormDB.DB()
	defer sqlDB.Close()
	logx.Info("目标数据库连接成功")

	if userOrgs == nil {
		// 先尝试用配置的 org_csv_file 加载完整 org1~9 映射
		if appconfig.Cfg.OrgCSVFile != "" {
			csvOrgs, csvErr := loadUserOrgsFromCSV(appconfig.Cfg.OrgCSVFile)
			if csvErr == nil && len(csvOrgs) > 0 {
				logx.Infof("DB 不可用，已从配置 org_csv_file(%s)加载 %d 条完整组织记录", appconfig.Cfg.OrgCSVFile, len(csvOrgs))
				userOrgs = csvOrgs
			} else if csvErr != nil {
				logx.Warnf("org_csv_file(%s) 读取失败，继续回落到本地任务数据: %v", appconfig.Cfg.OrgCSVFile, csvErr)
			}
		}
	}

	// fromLocalFallback 标记 userOrgs 来自"临时组织"占位兜底（本地任务数据），
	// 此路径必须用非破坏性的 InsertOnly 写法，避免覆盖 import-dept/人工写入的真实 org。
	fromLocalFallback := false
	if userOrgs == nil {
		// 最终兜底：从本地 tasks/commits/sessions 生成临时组织
		localOrgs, localErr := loadDefaultUserOrgsFromLocalData(gormDB)
		if localErr != nil {
			util.RecordCommandRun("import-org", startTime, 0, 0, 0, localErr)
			return localErr
		}
		userOrgs = localOrgs
		fromLocalFallback = true
		logx.Warnf("已生成 %d 条临时 user_org 记录，全部归入 org1=临时组织（org_csv_file 不可用或为空）", len(userOrgs))
	}

	if toCSV != "" {
		if err := writeOrgCSV(toCSV, userOrgs); err != nil {
			util.RecordCommandRun("import-org", startTime, 0, 0, 0, err)
			return fmt.Errorf("写入CSV文件失败: %w", err)
		}
		logx.Infof("CSV 文件已写入: %s", toCSV)
	}

	// 兜底"临时组织"占位走 InsertOnly（只插新用户、不覆盖已有真实 org）；
	// CSV / 真实 DB 路径仍用 saveUserOrgs（UPSERT）覆盖更新。
	saveFn := saveUserOrgs
	if fromLocalFallback {
		saveFn = saveUserOrgsInsertOnly
	}
	if err := saveFn(gormDB, userOrgs); err != nil {
		util.RecordCommandRun("import-org", startTime, 0, 0, 0, err)
		return fmt.Errorf("写入user_org表失败: %w", err)
	}
	logx.Infof("已写入 %d 条记录到 user_org 表", len(userOrgs))
	util.RecordCommandRun("import-org", startTime, len(userOrgs), 0, 0, nil)
	return nil
}

var importOrgCmd = &cobra.Command{
	Use:   "import-org",
	Short: "从源数据库或CSV文件导入用户组织信息到 costrict_stat.user_org 表，可选择导出到 CSV 文件",
	Long: `从源数据库的 auth.auth_users 和 quota_manager.employee_department 表读取数据，
	按 employee_number 关联，将 dept_full_level_names 拆分为 org1~org9，
	或直接从CSV文件加载UserOrg数据，
	写入目标数据库的 user_org 表。
	如需导出CSV文件，请使用 --to-csv 选项。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fromDSN, _ := cmd.Flags().GetString("from-db")
		fromCSV, _ := cmd.Flags().GetString("from-csv")
		toCSV, _ := cmd.Flags().GetString("to-csv")
		remote, _ := cmd.Flags().GetString("remote")

		if remote != "" {
			return util.SendToRemote(remote, "import-org", map[string]interface{}{
				"from_db":  fromDSN,
				"from_csv": fromCSV,
				"to_csv":   toCSV,
			})
		}

		if fromDSN == "" {
			fromDSN = appconfig.Cfg.OrgDSN
		}

		return withImportAdvisoryLock("import-org", func() error {
			return runImportOrg(fromDSN, fromCSV, toCSV)
		})
	},
}

func init() {
	importOrgCmd.Flags().SortFlags = false
	importOrgCmd.Flags().String("from-db", "", "源数据库DSN")
	importOrgCmd.Flags().String("from-csv", "", "从指定的CSV文件加载UserOrg数据，替代从数据库加载")
	importOrgCmd.Flags().String("to-csv", "", "导出CSV文件路径（可选，不指定则不导出）")
	importOrgCmd.Flags().String("remote", "", "远程kbcli服务地址（如 http://127.0.0.1:8080），指定后命令将发送到远程执行")
	rootCmd.AddCommand(importOrgCmd)
}
