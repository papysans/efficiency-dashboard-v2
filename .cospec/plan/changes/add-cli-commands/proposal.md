# 变更：扩展 CLI 命令（correct/analyze 维度/export/cat-analysis）

## 原因
a.md 9.3 节定义了完整的 CLI 命令集，当前已实现 reindex/reload-org/validate-org/analyze git，还缺少：analyze 按维度分析（project/repo）、correct 纠错、export 导出报告、cat-analysis 查看分析文件等命令。

## 变更内容
- 扩展 analyze 命令：支持 `--dimension=project|repo` 维度分析（不仅限于 git 子命令）
- 新增 correct 命令：`kbcli correct --dimension=project --id=xxx --field=ai_estimated_days --value=50.5 --reason="..." --by="admin"`
- 新增 cat-analysis 命令：查看分析过程文件内容
- 注册所有新命令到 cmd_root.go

## 影响
- **受影响的代码**：
    - `kbcli/cmd_analyze.go`: 扩展 analyze 支持 dimension 模式
    - `kbcli/cmd_correct.go`: **新增文件**，纠错命令
    - `kbcli/cmd_cat_analysis.go`: **新增文件**，查看分析文件
    - `kbcli/cmd_root.go`: 注册新命令
