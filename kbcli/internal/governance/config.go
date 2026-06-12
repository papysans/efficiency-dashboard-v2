package governance

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// DownweightRule 低价值 comment 降权规则：commit comment 命中 Pattern（Go 正则）时，
// diff_lines 乘以 Factor 折算为 effective_diff_lines。
type DownweightRule struct {
	Pattern string  `yaml:"pattern"`
	Factor  float64 `yaml:"factor"`
}

// IdentityMapConfig user_id → 允许的 git emails 映射。
// Enforce 为 true 且 Users 有条目时，才对映射内 user_id 下使用映射外邮箱的 commit 硬排。
type IdentityMapConfig struct {
	Enforce bool                `yaml:"enforce"`
	Users   map[string][]string `yaml:"users"`
}

// IdentityConfig 身份治理配置：识别并排除测试/机器人身份伪造的交付。
type IdentityConfig struct {
	BuiltinBotPatterns   bool              `yaml:"builtin_bot_patterns"`   // 启用内置邮箱正则（见 identity.go builtinBotEmailRe，词边界锚定）
	BlockedEmails        []string          `yaml:"blocked_emails"`         // 精确匹配的黑名单 git 邮箱
	BlockedEmailPatterns []string          `yaml:"blocked_email_patterns"` // 通配符邮箱模式，如 *@example.com
	BlockedNamePatterns  []string          `yaml:"blocked_name_patterns"`  // git user name 黑名单模式
	BlockedUserIds       []string          `yaml:"blocked_user_ids"`       // 平台 user_id 黑名单：该账号全部活动不参与统计（与 blocked_emails 的 git 身份维度互补）
	IdentityMap          IdentityMapConfig `yaml:"identity_map"`
}

// CommitRulesConfig commit 级治理规则：巨型批量提交软上限、低价值 comment 降权、merge/rebase 重放去重、纯文档 commit 排除。
type CommitRulesConfig struct {
	DiffLinesSoftcap          int64            `yaml:"diff_lines_softcap"`
	DownweightCommentPatterns []DownweightRule `yaml:"downweight_comment_patterns"`
	ReplayDedup               bool             `yaml:"replay_dedup"`
	// DocFileExtensions 文档后缀列表（大小写不敏感后缀匹配）：touched_files 非空且全部命中时
	// 该 commit 整条排除（纯文档 commit，diff 全是文档行，不计入提效）。空列表=禁用本规则。
	// 默认 [.md .mdx .markdown]；故意不含 .txt（requirements.txt/CMakeLists.txt/构建输出多为非文档，会误伤）。
	DocFileExtensions []string `yaml:"doc_file_extensions"`
}

// NormalizationConfig 归一化配置：repo 地址写法分裂的规范化。
type NormalizationConfig struct {
	RepoAddrCanon bool `yaml:"repo_addr_canon"`
}

// Config 治理配置根结构，对应独立治理 YAML 文件（kbcli 主 config 用 governance_file 字段引用，
// 样例见 config/governance.example.yaml）。
type Config struct {
	Identity      IdentityConfig      `yaml:"identity"`
	CommitRules   CommitRulesConfig   `yaml:"commit_rules"`
	Normalization NormalizationConfig `yaml:"normalization"`
}

// DefaultConfig 内置默认治理配置。黑名单内容属于部署配置，不内置（保持空列表），
// 这里只给规则开关与系数的默认值。
func DefaultConfig() Config {
	return Config{
		Identity: IdentityConfig{
			BuiltinBotPatterns: true,
		},
		CommitRules: CommitRulesConfig{
			DiffLinesSoftcap: 3000,
			DownweightCommentPatterns: []DownweightRule{
				// 行首锚定 ^[\[\(]? 容忍 "[Merge]xxx" / "(merge)xxx" 前缀；\b 避免子串误伤（definitely 含 init）。
				// 教训：曾用全文 \b 匹配，"feat: 修复init逻辑" 这类句中词命中致 52% 误伤、9.5 万行真实交付被 ×0.2。
				{Pattern: `(?i)^[\[\(]?(merge|sync|format|style|scaffold|init)\b`, Factor: 0.2},
			},
			ReplayDedup:       true,
			DocFileExtensions: []string{".md", ".mdx", ".markdown"},
		},
		Normalization: NormalizationConfig{
			RepoAddrCanon: true,
		},
	}
}

// Load 加载治理配置：path 为空或文件不存在 → 返回内置默认（不报错）；
// 文件存在则以内置默认为底座做 YAML 覆盖，解析失败才报错。
func Load(path string) (Config, error) {
	if strings.TrimSpace(path) == "" {
		return DefaultConfig(), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return Config{}, fmt.Errorf("读取治理配置 %s 失败: %w", path, err)
	}
	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("解析治理配置 %s 失败: %w", path, err)
	}
	return cfg, nil
}
