package main

import "testing"

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
