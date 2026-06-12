package governance

import (
	"fmt"

	"gorm.io/gorm"
)

// governanceBatchSize 各治理子 pass 分批扫描 commits 表的批大小。
const governanceBatchSize = 500

// ApplyCommitGovernance commit 治理总入口：在 efficiency-v2 主流程开头执行，
// 按治理配置重判 commits 的排除标记（排除=打标记不删数据），改名单→重跑 efficiency-v2 即生效。
// 依次执行凭据脱敏、身份治理、纯文档 commit 排除、commit 规则治理与 merge 治理五个子 pass
// （脱敏改写基表存量值是安全例外，必须最先跑；doc_only 放在 identity 之后，让身份排除优先占位 excluded_reason；
// merge 置 0 优先级最高，必须放在 commitrules 之后兜底）。
// TODO: startDate/endDate（YYYY-MM-DD，空=全量）的识别窗口语义由子 pass 落地时统一实现，
// 当前先透传不裁剪。
func ApplyCommitGovernance(db *gorm.DB, cfg Config, startDate, endDate string) error {
	_ = startDate
	_ = endDate
	if err := applySanitizeRepoAddrs(db); err != nil {
		return fmt.Errorf("repo_addr 凭据脱敏子 pass 失败: %w", err)
	}
	if err := applyIdentityRules(db, cfg); err != nil {
		return fmt.Errorf("identity 治理子 pass 失败: %w", err)
	}
	if err := applyDocOnlyRules(db, cfg); err != nil {
		return fmt.Errorf("纯文档 commit 治理子 pass 失败: %w", err)
	}
	if err := applyCommitRules(db, cfg); err != nil {
		return fmt.Errorf("commit 规则治理子 pass 失败: %w", err)
	}
	if err := applyMergeRules(db); err != nil {
		return fmt.Errorf("merge 治理子 pass 失败: %w", err)
	}
	return nil
}
