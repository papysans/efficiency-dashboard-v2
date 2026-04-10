# 变更：更新项目详情页度量指标展示

## 原因
项目详情页（ProjectDetailV2.vue）当前展示的度量字段不够清晰，缺少关键效能指标，且 tooltip 描述不充分，导致用户难以理解数据含义和来源。

## 变更内容
- **移除** 基础信息中的"创建时间"和"更新时间"两个字段
- **优化** "传统开发预估"和"实际处理耗时"的 tooltip，改为说明计算来源（汇聚了哪些 Task 和 Commit 数据）
- **新增** "总Tokens"字段，替换"总上行Tokens"和"总下行Tokens"，tooltip 说明是两者之和及各自数值
- **新增** "生成代码量"字段：累加项目内所有 commit 的 diff_lines 之和
- **新增** "实际耗时"字段：等同于"实际处理耗时"，单位改为人天显示，tooltip 说明来源
- **替换** "提效比"为四个新字段：
  1. 实际人天代码量 = 生成代码量 / 实际耗时（行/人天）
  2. 传统开发人天代码量 = 生成代码量 / 传统开发预估（行/人天），tooltip 说明与企业基准值的对比
  3. 开发提效比 = 传统开发预估 / 实际耗时（倍数）
  4. 端到端提效比 = 传统开发预估 / 项目周期（倍数）
- **新增** config.yaml 全局配置项 `traditional_dev_lines_per_day`（传统开发人天代码量基准值，默认100）
- **新增** 后端 config 读取 + API 端点 GET /api/v2/config 返回前端所需全局配置
- **新增** 前端读取全局配置并在 tooltip 中展示与基准值的对比

## 影响
- **受影响的规范**：项目详情页度量展示
- **受影响的代码**：
    - `frontend/src/views/ProjectDetailV2.vue`: 修改基础信息和度量信息卡片，增删字段，更新 tooltip，新增计算 computed 属性
    - `backend/main.go`: 新增 `/api/v2/config` 路由
    - `backend/constants.go`: 新增 `DefaultTraditionalDevLinesPerDay` 常量
    - `config.yaml`: 新增 `traditional_dev_lines_per_day` 配置项
    - `backend/main.go` 或新文件 `backend/config_handler.go`: 新增 config 查询 handler，读取 traditional_dev_lines_per_day
    - `frontend/src/api/es.js`: 新增 getGlobalConfig API 调用
