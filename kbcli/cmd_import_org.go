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

func runImportOrg(fromDSN, csvFile string) error {
	authDSN := replaceDBName(fromDSN, "auth")
	authDB, err := openSQLDB(authDSN)
	if err != nil {
		return fmt.Errorf("连接 auth 数据库失败: %w", err)
	}
	defer authDB.Close()
	fmt.Println("auth 数据库连接成功")

	quotaDSN := replaceDBName(fromDSN, "quota_manager")
	quotaDB, err := openSQLDB(quotaDSN)
	if err != nil {
		return fmt.Errorf("连接 quota_manager 数据库失败: %w", err)
	}
	defer quotaDB.Close()
	fmt.Println("quota_manager 数据库连接成功")

	userRows, err := authDB.Query(`
		SELECT id, name, github_name, email, employee_number
		FROM auth_users
		WHERE employee_number IS NOT NULL AND employee_number != ''
		ORDER BY name
	`)
	if err != nil {
		return fmt.Errorf("查询 auth_users 失败: %w", err)
	}
	defer userRows.Close()

	deptRows, err := quotaDB.Query(`SELECT employee_number, dept_full_level_names FROM employee_department`)
	if err != nil {
		return fmt.Errorf("查询 employee_department 失败: %w", err)
	}
	defer deptRows.Close()

	deptMap := make(map[string]string)
	for deptRows.Next() {
		var empNum, deptFullLevelNames sql.NullString
		if err := deptRows.Scan(&empNum, &deptFullLevelNames); err != nil {
			return fmt.Errorf("读取部门数据失败: %w", err)
		}
		if empNum.Valid {
			deptMap[empNum.String] = deptFullLevelNames.String
		}
	}
	if err := deptRows.Err(); err != nil {
		return fmt.Errorf("遍历部门数据失败: %w", err)
	}

	var userOrgs []UserOrg
	for userRows.Next() {
		var userID, userName, gitUserName, gitUserEmail string
		var empNum sql.NullString
		if err := userRows.Scan(&userID, &userName, &gitUserName, &gitUserEmail, &empNum); err != nil {
			return fmt.Errorf("读取用户数据失败: %w", err)
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
		return fmt.Errorf("遍历用户数据失败: %w", err)
	}
	fmt.Printf("查询到 %d 条用户组织记录\n", len(userOrgs))

	if err := writeOrgCSV(csvFile, userOrgs); err != nil {
		return fmt.Errorf("写入CSV文件失败: %w", err)
	}
	fmt.Printf("CSV 文件已写入: %s\n", csvFile)

	gormDB, err := openGormDB(cfg.StatDatabase.DSN())
	if err != nil {
		return fmt.Errorf("连接目标数据库失败: %w", err)
	}
	sqlDB, _ := gormDB.DB()
	defer sqlDB.Close()
	fmt.Println("目标数据库连接成功")

	if err := saveUserOrgsGorm(gormDB, userOrgs); err != nil {
		return fmt.Errorf("写入user_org表失败: %w", err)
	}
	fmt.Printf("已写入 %d 条记录到 user_org 表\n", len(userOrgs))

	return nil
}

var importOrgCmd = &cobra.Command{
	Use:   "import-org",
	Short: "从源数据库导入用户组织信息到 costrict_stat.user_org 表及 CSV 文件",
	Long: `从源数据库的 auth.auth_users 和 quota_manager.employee_department 表读取数据，
	按 employee_number 关联，将 dept_full_level_names 拆分为 org1~org9，
	写入目标数据库的 user_org 表并导出 CSV 文件。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fromDSN, _ := cmd.Flags().GetString("from-db")
		csvFile, _ := cmd.Flags().GetString("csv-file")

		if fromDSN == "" {
			fromDSN = cfg.IndicatorDSN
		}
		if csvFile == "" {
			csvFile = cfg.OrgCSVFile
		}

		return runImportOrg(fromDSN, csvFile)
	},
}

func init() {
	importOrgCmd.Flags().String("from-db", "", "源数据库DSN")
	importOrgCmd.Flags().String("csv-file", "", "导出CSV文件路径")
	rootCmd.AddCommand(importOrgCmd)
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

func saveUserOrgsGorm(db *gorm.DB, rows []UserOrg) error {
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
