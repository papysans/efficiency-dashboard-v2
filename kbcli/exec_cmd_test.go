package main

import (
	"strings"
	"testing"
)

func TestGetStringParam(t *testing.T) {
	tests := []struct {
		name       string
		params     map[string]interface{}
		key        string
		defaultVal string
		want       string
	}{
		{
			name:       "key exists, value is non-empty string",
			params:     map[string]interface{}{"k": "hello"},
			key:        "k",
			defaultVal: "default",
			want:       "hello",
		},
		{
			name:       "key exists, value is empty string",
			params:     map[string]interface{}{"k": ""},
			key:        "k",
			defaultVal: "default",
			want:       "default",
		},
		{
			name:       "key exists, value is non-string (int)",
			params:     map[string]interface{}{"k": 123},
			key:        "k",
			defaultVal: "default",
			want:       "default",
		},
		{
			name:       "key missing",
			params:     map[string]interface{}{},
			key:        "k",
			defaultVal: "default",
			want:       "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getStringParam(tt.params, tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("getStringParam() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetBoolParam(t *testing.T) {
	tests := []struct {
		name       string
		params     map[string]interface{}
		key        string
		defaultVal bool
		want       bool
	}{
		{
			name:       "key exists, value is true bool",
			params:     map[string]interface{}{"k": true},
			key:        "k",
			defaultVal: false,
			want:       true,
		},
		{
			name:       "key exists, value is false bool",
			params:     map[string]interface{}{"k": false},
			key:        "k",
			defaultVal: true,
			want:       false,
		},
		{
			name:       "key exists, value is true string",
			params:     map[string]interface{}{"k": "true"},
			key:        "k",
			defaultVal: false,
			want:       true,
		},
		{
			name:       "key exists, value is 1 string",
			params:     map[string]interface{}{"k": "1"},
			key:        "k",
			defaultVal: false,
			want:       true,
		},
		{
			name:       "key exists, value is false string",
			params:     map[string]interface{}{"k": "false"},
			key:        "k",
			defaultVal: true,
			want:       false,
		},
		{
			name:       "key exists, value is 1 int",
			params:     map[string]interface{}{"k": 1},
			key:        "k",
			defaultVal: false,
			want:       true,
		},
		{
			name:       "key exists, value is 0 int",
			params:     map[string]interface{}{"k": 0},
			key:        "k",
			defaultVal: true,
			want:       false,
		},
		{
			name:       "key exists, value is 1.0 float64",
			params:     map[string]interface{}{"k": 1.0},
			key:        "k",
			defaultVal: false,
			want:       true,
		},
		{
			name:       "key exists, value is 0.0 float64",
			params:     map[string]interface{}{"k": 0.0},
			key:        "k",
			defaultVal: true,
			want:       false,
		},
		{
			name:       "key missing",
			params:     map[string]interface{}{},
			key:        "k",
			defaultVal: true,
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getBoolParam(tt.params, tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("getBoolParam() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetIntParam(t *testing.T) {
	tests := []struct {
		name       string
		params     map[string]interface{}
		key        string
		defaultVal int
		want       int
	}{
		{
			name:       "key exists, value is int",
			params:     map[string]interface{}{"k": 123},
			key:        "k",
			defaultVal: 0,
			want:       123,
		},
		{
			name:       "key exists, value is 0 int",
			params:     map[string]interface{}{"k": 0},
			key:        "k",
			defaultVal: 99,
			want:       99,
		},
		{
			name:       "key exists, value is float64",
			params:     map[string]interface{}{"k": 45.0},
			key:        "k",
			defaultVal: 0,
			want:       45,
		},
		{
			name:       "key exists, value is 0.0 float64",
			params:     map[string]interface{}{"k": 0.0},
			key:        "k",
			defaultVal: 99,
			want:       99,
		},
		{
			name:       "key exists, value is numeric string",
			params:     map[string]interface{}{"k": "123"},
			key:        "k",
			defaultVal: 0,
			want:       123,
		},
		{
			name:       "key exists, value is empty string",
			params:     map[string]interface{}{"k": ""},
			key:        "k",
			defaultVal: 99,
			want:       99,
		},
		{
			name:       "key exists, value is non-numeric string",
			params:     map[string]interface{}{"k": "abc"},
			key:        "k",
			defaultVal: 99,
			want:       0,
		},
		{
			name:       "key missing",
			params:     map[string]interface{}{},
			key:        "k",
			defaultVal: 99,
			want:       99,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getIntParam(tt.params, tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("getIntParam() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateTaskExecutor(t *testing.T) {
	tests := []struct {
		name    string
		typ     string
		wantNil bool
		wantErr bool
	}{
		{"import", "import", false, false},
		{"import-conv", "import-conv", false, false},
		{"import-repo", "import-repo", false, false},
		{"import-org", "import-org", false, false},
		{"silica", "silica", false, false},
		{"efficiency", "efficiency", false, false},
		{"unknown", "unknown", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn, err := createTaskExecutor(tt.typ, map[string]interface{}{})
			if (err != nil) != tt.wantErr {
				t.Errorf("createTaskExecutor() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (fn == nil) != tt.wantNil {
				t.Errorf("createTaskExecutor() fn == nil = %v, wantNil %v", fn == nil, tt.wantNil)
			}
			if tt.wantErr && err != nil {
				if !strings.Contains(err.Error(), "未知任务类型") {
					t.Errorf("createTaskExecutor() error = %v, want contain '未知任务类型'", err)
				}
			}
		})
	}
}
