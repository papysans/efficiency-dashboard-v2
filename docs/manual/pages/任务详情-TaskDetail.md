# 页面说明书 · 任务详情（TaskDetail）

> 路由 `/kanban/task/:taskId`
> 组件 `frontend-react/src/pages/tasks/TaskDetail.tsx`
> 数据接口 `GET /api/v2/tasks/{id}`（`endpoints.ts:193`），人工调整 `PUT /v2/tasks/{id}/manual`

---

## 0. 这个页面是什么

单个 Task 的端到端明细页：① 基础信息 → ② 度量信息 → ③ 对话历史。响应含 `{task, conversations}`。

> 与列表一致，**详情页也不展示提效比**（后端虽返回 `efficiency_ratio` 但前端不渲染——Task 维度故意不接提效口径）。

---

## 1. 字段速查表

### ① 基础信息（`TaskDetail.tsx:112-154`）
Task ID / 任务描述 / 用户（真名，可点）/ 仓库（`repo_addr#repo_branch`，可点）/ 工作目录（可点）/ 开始·结束时间 / 系统（OS+版本）/ 客户端（IDE+版本）/ 模式（caller）。空值均显 `-`。

### ② 度量信息（`TaskDetail.tsx:156-189`）
| 显示 | 含义 | 口径 | 空态 |
|---|---|---|---|
| 生成代码量 | diff 行数 + "查看详情"文件链 | 🔢 原始行 | null→`-` |
| 实际耗时 | manual 优先工时 | 🧑 | manual 删除线原值 null→"(AI未出值)" |
| 传统开发时长预估 | manual 优先古法预估 | 🧑 古法（→白皮书§12.4） | 同上 |
| API 请求次数 | conversation 条数（前端聚合） | 🔢 | 0→`-` |
| 总 Tokens | 前端累加上下行 | 🔢 | ≤0→`-` |
| 费用 | 任务费用（带"元"），回退 conversations 累加 | 🔢 | 皆 0→`-` |

> **manual 优先展示**（`ManualValue`，`TaskDetail.tsx:283-312`）：有人工调整值时主显人工值（黑）+ 黄(?)人工理由 + 删除线原 AI 值（灰）+ 灰(?)原理由；无则只显原值。

### ③ 对话历史（`TaskDetail.tsx:191-274`）
每条 conversation 一节点（纯线性时间线，按 `start_time` 升序，后端固定）。逐条显：时间 / 提问·系统消息·Agent 轮三态正文 / model·mode / 耗时 / 上下行 token / 费用 / 代码行 / 错误标签 / "查看原始数据"外链。
- **正文三态**：用户提问（剥去 harness 注入块后非空）/ 系统消息（剥完为空）/ Agent 自动轮（`user_input` 为空，可展开请求内容）。
- 耗时 `process_time`：**0→`-`**（采集常缺失，被烤成 0）。

---

## 2. 筛选 / 排序 / 分页

详情页无筛选/排序/分页（单实体）。对话顺序由后端固定 `ORDER BY start_time ASC`。Token/费用聚合在前端做。

---

## 3. 人工调整

标题栏"人工调整"按钮 → 4 字段弹窗（实际耗时值+理由、传统预估值+理由）→ `PUT /v2/tasks/{id}/manual`。人工值在显示中优先于 AI 原值。空字符串清除人工值。

> 说明：人工调整是对古法工时口径（V1）的运营兜底，Task 与 Commit 详情保留此能力（项目详情已移除）。

---

## 4. 边界与常见质疑应答

**Q：对话里"Agent 自动轮次（无用户输入）"是什么？**
Agent 循环里只有人真正敲字那一轮才有 `user_input`，其余自动轮无用户输入，归为此类，可展开看请求内容。

**Q：耗时显示 `-` 是没耗时吗？**
对话级 `process_time` 采集常缺失（被存成 0），0 显示为 `-`，不代表真的零耗时。

**Q：为什么这里看不到提效比？**
Task 维度故意不接提效口径（单任务算不出可信提效）。提效看《需求 / 用户 / 组织》维度。

---

*行号基于 `feat/needs-user-search-multi-caliber` 分支当前快照。*
