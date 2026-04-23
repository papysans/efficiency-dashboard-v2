package main

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"os"
	"strings"

	_ "github.com/lib/pq"

	"github.com/spf13/cobra"
)

type userOrgRow struct {
	UserID       string
	UserName     string
	Org1         string
	Org2         string
	Org3         string
	Org4         string
	Org5         string
	Org6         string
	Org7         string
	Org8         string
	Org9         string
	GitUserName  string
	GitUserEmail string
}

// replaceDBName 替换 PostgreSQL DSN 中的数据库名
func replaceDBName(dsn, newDBName string) string {
	// DSN 格式示例: host=x port=x user=x password=x dbname=x sslmode=x
	// 我们需要替换 dbname 的值
	result := dsn
	if !strings.Contains(result, "dbname=") {
		// 如果没有 dbname，添加它
		result += " dbname=" + newDBName
	} else {
		// 替换现有的 dbname
		result = strings.ReplaceAll(result,
			strings.Join([]string{"dbname=", extractDBName(dsn)}, ""),
			"dbname="+newDBName)
	}
	return result
}

// extractDBName 从 DSN 中提取数据库名
func extractDBName(dsn string) string {
	parts := strings.Split(dsn, " ")
	for _, part := range parts {
		if strings.HasPrefix(part, "dbname=") {
			dbname := strings.TrimPrefix(part, "dbname=")
			// 如果后面还有其他参数（如 sslmode），只取 dbname 的值
			if idx := strings.Index(dbname, " "); idx != -1 {
				dbname = dbname[:idx]
			}
			return dbname
		}
	}
	return ""
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
			return fmt.Errorf("必须指定 --from-db 参数")
		}

		// 连接 auth 数据库
		authDSN := replaceDBName(fromDSN, "auth")
		authConn, err := sql.Open("postgres", authDSN)
		if err != nil {
			return fmt.Errorf("连接 auth 数据库失败: %w", err)
		}
		defer authConn.Close()

		if err := authConn.Ping(); err != nil {
			return fmt.Errorf("auth 数据库连接测试失败: %w", err)
		}
		fmt.Println("auth 数据库连接成功")

		// 连接 quota_manager 数据库
		quotaDSN := replaceDBName(fromDSN, "quota_manager")
		quotaConn, err := sql.Open("postgres", quotaDSN)
		if err != nil {
			return fmt.Errorf("连接 quota_manager 数据库失败: %w", err)
		}
		defer quotaConn.Close()

		if err := quotaConn.Ping(); err != nil {
			return fmt.Errorf("quota_manager 数据库连接测试失败: %w", err)
		}
		fmt.Println("quota_manager 数据库连接成功")

		// 从 auth 数据库读取用户信息
		userRows, err := authConn.Query(`
			SELECT
				id,
				name,
				github_name,
				email,
				employee_number
			FROM auth_users
			WHERE employee_number IS NOT NULL AND employee_number != ''
			ORDER BY name
		`)
		if err != nil {
			return fmt.Errorf("查询 auth_users 失败: %w", err)
		}
		defer userRows.Close()

		// 从 quota_manager 数据库读取部门信息
		deptRows, err := quotaConn.Query(`
			SELECT
				employee_number,
				dept_full_level_names
			FROM employee_department
		`)
		if err != nil {
			return fmt.Errorf("查询 employee_department 失败: %w", err)
		}
		defer deptRows.Close()

		// 构建部门信息映射
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

		// 合并用户和部门信息
		var userOrgs []userOrgRow
		for userRows.Next() {
			var r userOrgRow
			var empNum sql.NullString
			if err := userRows.Scan(&r.UserID, &r.UserName, &r.GitUserName, &r.GitUserEmail, &empNum); err != nil {
				return fmt.Errorf("读取用户数据失败: %w", err)
			}

			// 通过 employee_number 关联部门信息
			if empNum.Valid {
				if deptFullLevelNames, ok := deptMap[empNum.String]; ok {
					parts := strings.Split(deptFullLevelNames, ",")
					orgFields := []*string{&r.Org1, &r.Org2, &r.Org3, &r.Org4, &r.Org5, &r.Org6, &r.Org7, &r.Org8, &r.Org9}
					for i, p := range parts {
						if i >= len(orgFields) {
							break
						}
						*orgFields[i] = strings.TrimSpace(p)
					}
				}
			}
			userOrgs = append(userOrgs, r)
		}
		if err := userRows.Err(); err != nil {
			return fmt.Errorf("遍历用户数据失败: %w", err)
		}
		fmt.Printf("查询到 %d 条用户组织记录\n", len(userOrgs))

		if err := writeOrgCSV(csvFile, userOrgs); err != nil {
			return fmt.Errorf("写入CSV文件失败: %w", err)
		}
		fmt.Printf("CSV 文件已写入: %s\n", csvFile)

		toConn, err := sql.Open("postgres", cfg.StatDatabase.DSN())
		if err != nil {
			return fmt.Errorf("连接目标数据库失败: %w", err)
		}
		defer toConn.Close()

		if err := toConn.Ping(); err != nil {
			return fmt.Errorf("目标数据库连接测试失败: %w", err)
		}
		fmt.Println("目标数据库连接成功")

		if err := ensureUserOrgTable(toConn); err != nil {
			return fmt.Errorf("创建user_org表失败: %w", err)
		}

		if err := saveUserOrgs(toConn, userOrgs); err != nil {
			return fmt.Errorf("写入user_org表失败: %w", err)
		}
		fmt.Printf("已写入 %d 条记录到 user_org 表\n", len(userOrgs))

		return nil
	},
}

func init() {
	importOrgCmd.Flags().String("from-db", "", "源数据库DSN（host=127.0.0.1 port=5432 user=xxx password=xxx sslmode=disable）")
	importOrgCmd.Flags().String("csv-file", "./org_mapping.csv", "导出CSV文件路径")
	if err := importOrgCmd.MarkFlagRequired("from-db"); err != nil {
		panic(err)
	}
	rootCmd.AddCommand(importOrgCmd)
}

func writeOrgCSV(path string, rows []userOrgRow) error {
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

func ensureUserOrgTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS user_org (
		user_id VARCHAR(255) PRIMARY KEY,
		user_name VARCHAR(255),
		org1 VARCHAR(255),
		org2 VARCHAR(255),
		org3 VARCHAR(255),
		org4 VARCHAR(255),
		org5 VARCHAR(255),
		org6 VARCHAR(255),
		org7 VARCHAR(255),
		org8 VARCHAR(255),
		org9 VARCHAR(255),
		git_user_name VARCHAR(255),
		git_user_email VARCHAR(255),
		created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return err
	}

	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_user_org_user_name ON user_org(user_name)`,
		`CREATE INDEX IF NOT EXISTS idx_user_org_git_user_name ON user_org(git_user_name)`,
		`CREATE INDEX IF NOT EXISTS idx_user_org_git_user_email ON user_org(git_user_email)`,
	}
	for _, idx := range indexes {
		if _, err := db.Exec(idx); err != nil {
			fmt.Fprintf(os.Stderr, "警告: 创建索引失败(可忽略): %v\n", err)
		}
	}

	return nil
}

func saveUserOrgs(db *sql.DB, rows []userOrgRow) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}

	stmt, err := tx.Prepare(`
		INSERT INTO user_org (user_id, user_name, org1, org2, org3, org4, org5, org6, org7, org8, org9, git_user_name, git_user_email, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, CURRENT_TIMESTAMP)
		ON CONFLICT (user_id) DO UPDATE SET
			user_name = EXCLUDED.user_name,
			org1 = EXCLUDED.org1,
			org2 = EXCLUDED.org2,
			org3 = EXCLUDED.org3,
			org4 = EXCLUDED.org4,
			org5 = EXCLUDED.org5,
			org6 = EXCLUDED.org6,
			org7 = EXCLUDED.org7,
			org8 = EXCLUDED.org8,
			org9 = EXCLUDED.org9,
			git_user_name = EXCLUDED.git_user_name,
			git_user_email = EXCLUDED.git_user_email,
			updated_at = CURRENT_TIMESTAMP
	`)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("预处理语句失败: %w", err)
	}
	defer stmt.Close()

	for _, r := range rows {
		if _, err := stmt.Exec(r.UserID, r.UserName, r.Org1, r.Org2, r.Org3, r.Org4, r.Org5, r.Org6, r.Org7, r.Org8, r.Org9, r.GitUserName, r.GitUserEmail); err != nil {
			tx.Rollback()
			return fmt.Errorf("写入记录失败 [user_id=%s]: %w", r.UserID, err)
		}
	}

	return tx.Commit()
}
