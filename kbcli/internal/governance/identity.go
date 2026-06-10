package governance

import "gorm.io/gorm"

// applyIdentityRules 身份治理子 pass：按内置 bot 正则、blocked_emails/blocked_email_patterns、
// blocked_name_patterns 与 identity_map（enforce 时）识别测试/机器人/伪造身份的 commit，
// 命中者写 excluded_flag=true + excluded_reason（打标记不删数据）。
// TODO(W1): 由并行流 W1 实现，当前为桩。
func applyIdentityRules(db *gorm.DB, cfg Config) error {
	return nil
}
