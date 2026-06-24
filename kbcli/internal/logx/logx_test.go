package logx

import "testing"

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected LogLevel
	}{
		{"lowercase debug", "debug", LogDebug},
		{"lowercase info", "info", LogInfo},
		{"lowercase warn", "warn", LogWarn},
		{"lowercase error", "error", LogError},
		{"uppercase DEBUG", "DEBUG", LogDebug},
		{"mixed case Info", "Info", LogInfo},
		{"mixed case WaRn", "WaRn", LogWarn},
		{"mixed case ErRoR", "ErRoR", LogError},
		{"empty string defaults to info", "", LogInfo},
		{"unknown string defaults to info", "unknown", LogInfo},
		{"warning is not warn, defaults to info", "warning", LogInfo},
		{"uppercase INFO", "INFO", LogInfo},
		{"uppercase WARN", "WARN", LogWarn},
		{"uppercase ERROR", "ERROR", LogError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLogLevel(tt.input)
			if got != tt.expected {
				t.Errorf("parseLogLevel(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestLogLevelString(t *testing.T) {
	tests := []struct {
		name     string
		level    LogLevel
		expected string
	}{
		{"LogDebug", LogDebug, "DEBUG"},
		{"LogInfo", LogInfo, "INFO"},
		{"LogWarn", LogWarn, "WARN"},
		{"LogError", LogError, "ERROR"},
		{"zero value", LogLevel(0), "UNKNOWN"},
		{"arbitrary value", LogLevel(99), "UNKNOWN"},
		{"negative value", LogLevel(-1), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.level.String()
			if got != tt.expected {
				t.Errorf("LogLevel(%v).String() = %q, want %q", tt.level, got, tt.expected)
			}
		})
	}
}
