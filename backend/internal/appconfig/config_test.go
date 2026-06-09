package appconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigDashboardTitlePrefix(t *testing.T) {
	dir := t.TempDir()

	defaultCfg, err := loadConfig(filepath.Join(dir, "missing.yaml"))
	if err != nil {
		t.Fatalf("load missing config: %v", err)
	}
	if defaultCfg.DashboardTitlePrefix != "" {
		t.Fatalf("default DashboardTitlePrefix = %q, want empty", defaultCfg.DashboardTitlePrefix)
	}

	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("dashboard_title_prefix: \"  Costrict  \"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.DashboardTitlePrefix != "Costrict" {
		t.Fatalf("DashboardTitlePrefix = %q, want %q", cfg.DashboardTitlePrefix, "Costrict")
	}
}
