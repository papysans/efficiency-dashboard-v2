# fix-task-detail-display: Task 详情页两个问题修复

## 阶段 1：数据修复

- [x] 1.1 SQL 修复：从 reason 文本提取人天数值回写 task_ancient_minutes
     执行 SQL 从 task_ancient_minutes_reason 中正则提取 "X.X 人天" 格式数值，乘以 480 转为分钟，回写到 task_ancient_minutes 字段
     仅更新 task_ancient_minutes IS NULL 且 reason 中包含"人天"的记录

## 阶段 2：前端改造——人工调整后的删除线 + 双 tooltip 对比显示

- [x] 2.1 TaskDetailV2.vue：元信息区和提效卡片的删除线 + 双 tooltip 改造
     当存在 _manual 值时：正常显示 manual 值 + 橙色 ? tooltip 显示 manual reason，旁边删除线显示系统值 + 灰色小 ? tooltip 显示系统 reason
     涉及字段：古法预估(task_ancient_minutes)、实际耗时(task_real_minutes)
     涉及区域：el-descriptions 元信息区 + 提效统计摘要卡片区

- [x] 2.2 CommitDetailV2.vue：元信息区和提效卡片的删除线 + 双 tooltip 改造
     同样模式应用到 commit_ancient_minutes 和 commit_real_minutes 字段
     涉及区域：el-descriptions 元信息区 + 提效指标卡片区

## 阶段 3：用户反馈修复

- [x] 3.1 提效比去掉小数点 + 卡片排版改为一行 + 系统值显示修复
     TaskDetailV2.vue 和 CommitDetailV2.vue：
     a) 提效比卡片 .toFixed(1) 改为 Math.round()
     b) 卡片区删除线部分从 div 改为 span inline 显示，与主值同行
     c) 当系统值为 NULL 但 reason 存在时，仍显示 reason tooltip（已实现），删除线区域不显示值文本（正确行为）
