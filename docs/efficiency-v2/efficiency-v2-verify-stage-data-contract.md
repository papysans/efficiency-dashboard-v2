# 验证(verify)阶段数据采集对接规格

> 目的：说明效率看板 V2「思考 / 执行 / **验证**」三段中，**验证段为何恒为 0**，以及采集端（AI coding 客户端 / SDK / 上报链路）需要补哪些**结构化字段**才能让验证段（以及更准确的执行段）真正工作。
>
> 适用：`feat/efficiency-v2-pipeline`。本文是 [数据格式与字段说明_上环境.md](数据格式与字段说明_上环境.md) §1.2 的**增量补充**，只聚焦工具调用 / 命令执行事件这一块。

---

## 1. 现象与根因（已用内网数据坐实）

### 1.1 现象
- 看板「验证」栏 / `needs.total_verify_min` / `session_stage_metrics.verify_active_min` **全部为 0**。
- 「思考」「执行」有数据，但思考偏高（兜底箩筐）、执行偏粗。

### 1.2 内网实测（库 `costrict_stat`）
| 检查 | 结果 |
|---|---|
| `conversation_events` 来源分布 | 只有 `synthetic/message`（≈73 万）+ `conversation_diff/edit`（≈14.6 万），**全部 parse_quality=degraded**；无 `raw_tool`、无 `read`、无 `verify` |
| `command_text` 非空数 | **0 / 876254** |
| `session_stage_metrics` | `verify_event_count=0`、`verify_active_min=0.0`；think≈113 万分、exec≈74 万分 |
| `conversations.request/response_content` | 全是**自然语言对话**，非结构化工具 JSON |

### 1.3 根因（结构性，非 import bug）
后端判定验证段的唯一依据（`kbcli/efficiency_v2_stage.go:59`）：

```
事件 class==verify  ⇔  command_text 非空  且  命中验证命令白名单（go test/go build/tsc/eslint…，见 config.go:183）
```

而 `command_text` 只在 **raw_tool 路径**（`efficiency_v2_events.go:67 extractEfficiencyV2RawToolEvent`）才会被填充——该路径要求 `request_content`/`response_content` 本身是带 `tool_name`/`command`/`event_kind` 字段的 JSON。

**但采集端契约（§1.2 conversation jsonl）压根没有这些字段**：每条记录只有 `request_content`/`response_content`/`user_input`/`diff`/`diff_lines`。上游记录的是「用户↔AI 一问一答」的**对话轮**，不是 agent 内部「每次工具调用 / bash 执行」的**步骤**。工具调用（含跑测试 / 编译 / lint）发生在 agent 黑盒内，从未被拆成结构化事件上报。

→ 结论：`command_text` 注定恒空 ⇒ verify 类永不产生 ⇒ 验证段恒 0。**这不是 import 的 bug，是数据源里没有这个信息**，本仓 import 无法无中生有。

> 附带影响：因为没有 read/工具事件，`read` 类、`raw_tool/exact` 质量也全缺失；执行段只能靠 `diff_lines>0` 粗略兜底；思考段成为"第一个 diff 之前 + 全程无 diff"的兜底箩筐，系统性偏高。

---

## 2. 目标

让一次 agent 会话的**工具调用 / 命令执行**以结构化事件上报，使后端能：
1. 产出真实的 **验证段**（识别 test/build/lint/typecheck 等命令）；
2. 用真实工具事件（edit/read/command）替代 diff 兜底，得到更准的**执行段**与更瘦的**思考段**；
3. 把 `stage_confidence` 从 `low/degraded` 提升到 `high/medium`。

---

## 3. 采集端需要补的数据（二选一，推荐方案 A）

后端解析逻辑（`extractEfficiencyV2RawToolEvent` / `ClassifyEfficiencyV2Event` / `InferEfficiencyV2EventDurationSec`）已经**预留好**对 `tool_name`/`command`/`event_kind`/`touched_files`/`event_start_ts`/`event_end_ts` 的支持，缺的只是采集端把数据喂进来。

### 方案 A（推荐）：新增独立的工具事件流 `task/tool_event/YYYY/MM/DD/<task_id>.jsonl`

每个 agent 工具调用一行 JSONL，粒度 = agent 步骤（不是对话轮）。这样一轮对话里的多次工具调用能各自独立计时与分类。

| 字段 | 类型 | 必填 | 含义 / 取值 |
|---|---|---|---|
| `task_id` | string | ✓ | 所属 session（= summary 的 task_id），用于归属与排序 |
| `request_id` | string | ✓ | 触发该工具调用的对话轮 request_id（关联回 conversation）|
| `event_id` | string | 建议 | 工具事件唯一 ID；缺省时后端按 (session,request,start,kind,tool) 生成 |
| `event_kind` | string | ✓ | 事件类型，**小写**：`edit` / `write` / `read` / `grep` / `command` / `message` / `other`（决定 think/exec/verify 分类）|
| `tool_name` | string | ✓ | 工具名，**小写**：`edit`/`write`/`multiedit`/`read`/`grep`/`glob`/`bash`/`shell`/… |
| `command` | string | 命令类必填 | **完整命令行文本**（如 `go test ./...`、`npm run build`、`eslint src`）。验证段识别**全靠它**——必须是真实执行的命令，不是 AI 的叙述 |
| `start_time` | string ISO8601 | ✓ | 工具调用开始时间 |
| `end_time` | string ISO8601 | 强烈建议 | 工具调用结束时间。**有它才有真实时长**；缺失时后端退化为「到下一个事件的间隔（封顶 5 分钟）→ 类默认值」估算 |
| `touched_files` | string[] | 可选 | 该次涉及的文件路径（edit/read）|
| `exit_code` | number | 可选 | 命令退出码（未来可用于"验证是否通过"）|
| `repo_addr`/`repo_branch`/`work_dir` | string | 可选 | 与 summary 冗余即可 |

### 方案 B（轻量）：在现有 conversation jsonl 每行加 `tool_calls`

不新增文件，给 §1.2 每条对话记录加一个数组字段，列出本轮 agent 执行的工具/命令：

```jsonc
{
  "request_id": "...", "sender": "agent", "start_time": "...", "end_time": "...",
  "request_content": "...", "response_content": "...", "diff_lines": 12,
  "tool_calls": [
    { "event_kind": "edit",    "tool_name": "edit", "touched_files": ["a.go"], "start_time": "...", "end_time": "..." },
    { "event_kind": "command", "tool_name": "bash", "command": "go test ./...", "start_time": "...", "end_time": "...", "exit_code": 0 }
  ]
}
```

字段含义同方案 A。**代价**：本仓需新增解析（把一行里的 `tool_calls` 展开成多条 `ConversationEvent`）；时长仍以子事件的 start/end 为准。方案 B 不如 A 干净（一行多事件、时间易错位），但改采集端最小。

> 不推荐"纯文本正则兜底"：从 `response_content` 抓"编译成功 / go build"等只是 AI 叙述，不等于真执行、无时间戳、无法定位时段，会产生不可信的验证数字，只能作为弱布尔信号、**绝不进时间三分法**。

---

## 4. 本仓对接改造点（采集端就绪后）

| 方案 | 本仓改动 | 文件 |
|---|---|---|
| A | 新增 import 路径读 `tool_event/` jsonl → 直接落 `conversation_events`（字段已天然对齐，几乎零映射）| 新增 `kbcli/cmd_import_tool_event.go`，复用 `buildEfficiencyV2Event` |
| B | 扩展 conversation 解析，把 `tool_calls[]` 展开为多条事件 | `kbcli/efficiency_v2_events.go:buildEfficiencyV2ConversationEvent` |

分类 / 时长 / 阶段切分 / 验证白名单**均无需改动**——`ClassifyEfficiencyV2Event`、`InferEfficiencyV2EventDurationSec`、`BuildEfficiencyV2SessionStageMetric`、`defaultEfficiencyV2VerificationCommandPatterns`（`config.go:183`）已就绪。验证白名单可在 `kbcli-config.yaml` 的 `efficiency_v2.verification_command_patterns` 增补团队特有命令。

---

## 5. 验收标准

采集端补齐后，重跑 `kbcli import-conv`（或新增的 `import-tool-event`）+ `kbcli efficiency-v2`，应满足：
- `SELECT COUNT(*) FILTER (WHERE command_text<>'') FROM conversation_events` **> 0**；
- `conversation_events.source` 出现 `raw_tool`、`parse_quality=exact`；出现 `event_kind=command/read`；
- `SUM(verify_active_min) FROM session_stage_metrics` **> 0**，`verify_event_count > 0`；
- `stage_confidence` 中 `high/medium` 占比显著上升；
- 前端「验证」栏从「采集未覆盖」恢复为真实数值。

---

## 6. 过渡期（采集端上线前）

前端已做**口径止血**：实际采集维度的验证时长为 0 时显示「—（采集未覆盖）」并加 tooltip，思考/执行标注为粗略估算口径，避免看板被误读为"团队不做验证"。基线组成表里 algo/llm 估算的 verify 是模型计算值，不受影响、照常显示。
