package main

import (
	"os"
	"testing"
)

// TP-53: 正常加载配置文件
func TestLoadConfig_Normal(t *testing.T) {
	// 使用项目根目录的真实 config.yaml（kbcli 运行时通常从上层目录加载）
	// 这里写一个临时配置文件来测试
	yaml := `
elasticsearch:
  url: "https://127.0.0.1:9200"
  username: "testuser"
  password: "testpass"
model_prices:
  GLM-4.7:
    in_price: 0.5
    out_price: 1.0
  Auto:
    in_price: 0.0
    out_price: 0.0
rawdata_dir: "../rawdata"
`
	f, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	defer os.Remove(f.Name())
	f.WriteString(yaml)
	f.Close()

	cfg, err := LoadConfig(f.Name())
	if err != nil {
		t.Fatalf("LoadConfig 返回错误: %v", err)
	}

	if cfg.Elasticsearch.URL != "https://127.0.0.1:9200" {
		t.Errorf("ES URL: want https://127.0.0.1:9200, got %s", cfg.Elasticsearch.URL)
	}
	if cfg.Elasticsearch.Username != "testuser" {
		t.Errorf("ES Username: want testuser, got %s", cfg.Elasticsearch.Username)
	}
	if cfg.Elasticsearch.Password != "testpass" {
		t.Errorf("ES Password: want testpass, got %s", cfg.Elasticsearch.Password)
	}
	if len(cfg.ModelPrices) != 2 {
		t.Errorf("ModelPrices count: want 2, got %d", len(cfg.ModelPrices))
	}
	glm, ok := cfg.ModelPrices["GLM-4.7"]
	if !ok {
		t.Error("ModelPrices 应包含 GLM-4.7")
	} else {
		if glm.InPrice != 0.5 {
			t.Errorf("GLM-4.7 InPrice: want 0.5, got %f", glm.InPrice)
		}
		if glm.OutPrice != 1.0 {
			t.Errorf("GLM-4.7 OutPrice: want 1.0, got %f", glm.OutPrice)
		}
	}
	if cfg.RawDataDir != "../rawdata" {
		t.Errorf("RawDataDir: want ../rawdata, got %s", cfg.RawDataDir)
	}
}

// TP-54: rawdata_dir 为空时使用默认值
func TestLoadConfig_DefaultRawDataDir(t *testing.T) {
	yaml := `
elasticsearch:
  url: "https://127.0.0.1:9200"
  username: ""
  password: ""
model_prices: {}
`
	f, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	defer os.Remove(f.Name())
	f.WriteString(yaml)
	f.Close()

	cfg, err := LoadConfig(f.Name())
	if err != nil {
		t.Fatalf("LoadConfig 返回错误: %v", err)
	}
	if cfg.RawDataDir != "../rawdata" {
		t.Errorf("默认 RawDataDir: want ../rawdata, got %s", cfg.RawDataDir)
	}
}

// TP-55: 文件不存在 → 返回 error
func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("期望返回 error，但未返回")
	}
}

// TP-56: YAML 格式不合法 → 返回 error
func TestLoadConfig_InvalidYAML(t *testing.T) {
	f, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	defer os.Remove(f.Name())
	f.WriteString("invalid: yaml: content: [unclosed")
	f.Close()

	_, err = LoadConfig(f.Name())
	if err == nil {
		t.Error("期望返回 error，但未返回")
	}
}

// TP-57: model_prices 为空 map 时 map 非 nil
func TestLoadConfig_EmptyModelPrices(t *testing.T) {
	yaml := `
elasticsearch:
  url: "https://127.0.0.1:9200"
  username: ""
  password: ""
model_prices: {}
rawdata_dir: "../rawdata"
`
	f, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	defer os.Remove(f.Name())
	f.WriteString(yaml)
	f.Close()

	cfg, err := LoadConfig(f.Name())
	if err != nil {
		t.Fatalf("LoadConfig 返回错误: %v", err)
	}
	if cfg.ModelPrices == nil {
		t.Error("空 model_prices 应返回非 nil map")
	}
	if len(cfg.ModelPrices) != 0 {
		t.Errorf("空 model_prices 应长度为0, got %d", len(cfg.ModelPrices))
	}
}

// TP-58: 多个模型价格正确加载
func TestLoadConfig_MultipleModelPrices(t *testing.T) {
	yaml := `
elasticsearch:
  url: "https://127.0.0.1:9200"
  username: ""
  password: ""
model_prices:
  GLM-4.7:
    in_price: 0.5
    out_price: 1.0
  GLM-5:
    in_price: 1.0
    out_price: 2.0
  Kimi-K2.5-Moonshot:
    in_price: 1.0
    out_price: 2.0
  Auto:
    in_price: 0.0
    out_price: 0.0
rawdata_dir: "../rawdata"
`
	f, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	defer os.Remove(f.Name())
	f.WriteString(yaml)
	f.Close()

	cfg, err := LoadConfig(f.Name())
	if err != nil {
		t.Fatalf("LoadConfig 返回错误: %v", err)
	}
	if len(cfg.ModelPrices) != 4 {
		t.Errorf("ModelPrices count: want 4, got %d", len(cfg.ModelPrices))
	}
	kimi, ok := cfg.ModelPrices["Kimi-K2.5-Moonshot"]
	if !ok {
		t.Error("应包含 Kimi-K2.5-Moonshot")
	} else if kimi.OutPrice != 2.0 {
		t.Errorf("Kimi OutPrice: want 2.0, got %f", kimi.OutPrice)
	}
}
