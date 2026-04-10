# 变更：在 raw 和 stat 索引中添加组织维度字段和统计

## 原因
需要按组织层级（公司/体系/部门/团队）统计 AI 编码指标，支持多层级的数据分析和报表。

## 变更内容
- 在 raw 层文档中添加 org1/org2/org3/org4 字段（通过 user_uuid 查询获取）
- 在 stat 层添加 4 个组织维度的聚合统计（org1/org2/org3/org4）
- 实现组织信息查询接口（当前 mock 实现，预留 SQL 查询扩展点）

## 组织层级定义
- **org1**: 公司名称（或子公司）
- **org2**: 体系或 BG 名称
- **org3**: 部门名称
- **org4**: 团队名称

层级关系：org1 → org2 → org3 → org4（父子递进关系）
允许为空：任何层级都可能为空字符串

## 影响
- **受影响的规范**：数据采集与 ES 写入
- **受影响的代码**：
  - `kbcli/org_provider.go`：组织信息查询接口（新增）
  - `kbcli/raw_parser.go`：添加 org 字段提取逻辑
  - `kbcli/stat_builder.go`：添加 4 个组织维度聚合
  - `kbcli/es_mappings.go`：更新 mapping 定义
  - `config.yaml`：添加 org_mappings 配置段（mock 数据）
