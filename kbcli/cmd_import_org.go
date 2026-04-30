package main

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"os"
	"strings"

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

func loadUserOrgsFromCSV(csvFile string) ([]UserOrg, error) {
	f, err := os.Open(csvFile)
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

	var userOrgs []UserOrg
	for i, record := range records {
		if i == 0 {
			continue
		}
		if len(record) < 13 {
			return nil, fmt.Errorf("第 %d 行数据列数不足，需要13列", i+1)
		}

		userOrg := UserOrg{
			UserID:       record[0],
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
	logInfof("从CSV文件加载到 %d 条用户组织记录", len(userOrgs))
	return userOrgs, nil
}

func loadUserOrgsFromDB(fromDSN string) ([]UserOrg, error) {
	authDSN := replaceDBName(fromDSN, "auth")
	authDB, err := openSQLDB(authDSN)
	if err != nil {
		return nil, fmt.Errorf("连接 auth 数据库失败: %w", err)
	}
	defer authDB.Close()
	logInfo("auth 数据库连接成功")

	quotaDSN := replaceDBName(fromDSN, "quota_manager")
	quotaDB, err := openSQLDB(quotaDSN)
	if err != nil {
		return nil, fmt.Errorf("连接 quota_manager 数据库失败: %w", err)
	}
	defer quotaDB.Close()
	logInfo("quota_manager 数据库连接成功")

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

	var userOrgs []UserOrg
	for userRows.Next() {
		var userID, userName, gitUserName, gitUserEmail string
		var empNum sql.NullString
		if err := userRows.Scan(&userID, &userName, &gitUserName, &gitUserEmail, &empNum); err != nil {
			return nil, fmt.Errorf("读取用户数据失败: %w", err)
		}

		org := UserOrg{
			UserID:       userID,
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
	logInfof("查询到 %d 条用户组织记录", len(userOrgs))
	return userOrgs, nil
}

func writeOrgCSV(path string, rows []UserOrg) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write([]string{"user_id", "user_name", "org1", "org2", "org3", "org4", "org5", "org6", "org7", "org8", "org9", "git_user_name", "git_user_email"}); err != nil {
		return err
	}

	for _, r := range rows {
		if err := w.Write([]string{r.UserID, r.UserName, r.Org1, r.Org2, r.Org3, r.Org4, r.Org5, r.Org6, r.Org7, r.Org8, r.Org9, r.GitUserName, r.GitUserEmail}); err != nil {
			return err
		}
	}

	return w.Error()
}

func saveUserOrgs(db *gorm.DB, rows []UserOrg) error {
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
				return fmt.Errorf("写入记录失败 [user_id=%s]: %w", r.UserID, result.Error)
			}
		}
		return nil
	})
}

func runImportOrg(fromDSN, fromCSV, toCSV string) error {
	var userOrgs []UserOrg
	var err error

	if fromCSV != "" {
		userOrgs, err = loadUserOrgsFromCSV(fromCSV)
		if err != nil {
			return err
		}
	} else {
		userOrgs, err = loadUserOrgsFromDB(fromDSN)
		if err != nil {
			return err
		}
	}

	if toCSV != "" {
		if err := writeOrgCSV(toCSV, userOrgs); err != nil {
			return fmt.Errorf("写入CSV文件失败: %w", err)
		}
		logInfof("CSV 文件已写入: %s", toCSV)
	}

	gormDB, err := openGormDB(cfg.StatDatabase.DSN())
	if err != nil {
		return fmt.Errorf("连接目标数据库失败: %w", err)
	}
	sqlDB, _ := gormDB.DB()
	defer sqlDB.Close()
	logInfo("目标数据库连接成功")

	if err := saveUserOrgs(gormDB, userOrgs); err != nil {
		return fmt.Errorf("写入user_org表失败: %w", err)
	}
	logInfof("已写入 %d 条记录到 user_org 表", len(userOrgs))

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

		if fromDSN == "" {
			fromDSN = cfg.OrgDSN
		}

		return runImportOrg(fromDSN, fromCSV, toCSV)
	},
}

func init() {
	importOrgCmd.Flags().SortFlags = false
	importOrgCmd.Flags().String("from-db", "", "源数据库DSN")
	importOrgCmd.Flags().String("from-csv", "", "从指定的CSV文件加载UserOrg数据，替代从数据库加载")
	importOrgCmd.Flags().String("to-csv", "", "导出CSV文件路径（可选，不指定则不导出）")
	rootCmd.AddCommand(importOrgCmd)
}
