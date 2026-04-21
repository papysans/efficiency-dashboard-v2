-- ============================================================
-- 测试数据种子脚本 (seed_data.sql)
-- 用途：为 costrict_tasks / costrict_task_conversations / costrict_commits / costrict_projects 填充模拟数据
-- 执行方式：$env:PGPASSWORD='1'; psql -U postgres -d report -f seed_data.sql
-- 幂等：使用 INSERT ... ON CONFLICT DO NOTHING，可重复执行
-- ============================================================

-- === costrict_tasks 测试数据 ===
-- 15 个 task，分布在 3 个仓库和 5 个用户中
-- 仓库: repo-costrict-main, repo-kanban-dev, repo-webapp-master
-- 用户: user-001~005, caller: chat/codereview, ide: vscode/intellij

INSERT INTO costrict_tasks (task_id, user_id, user_name, client_id, ide, version, os, os_version, caller, repo_addr, repo_branch, repo_id, project_path, project_id, start_time, end_time, upstream_tokens, downstream_tokens, cost, diff_lines, ai_estimated_ancient_days, ai_estimated_ancient_reason)
VALUES
  ('task-001', 'user-001', '138xxxx0001', 'client-a1', 'vscode', '1.88.0', 'windows', '10.0.19045', 'chat', 'https://github.com/zgsm-ai/costrict.git', 'main', 'repo-costrict-main', '/home/dev/costrict', 'proj-costrict', '2026-03-05T09:00:00', '2026-03-05T09:45:00', 2500, 3800, 1.25, 120, 2.5, '涉及认证模块重构，逻辑较复杂')
ON CONFLICT (task_id) DO NOTHING;

INSERT INTO costrict_tasks (task_id, user_id, user_name, client_id, ide, version, os, os_version, caller, repo_addr, repo_branch, repo_id, project_path, project_id, start_time, end_time, upstream_tokens, downstream_tokens, cost, diff_lines, ai_estimated_ancient_days, ai_estimated_ancient_reason)
VALUES
  ('task-002', 'user-002', '139xxxx0002', 'client-b1', 'intellij', '2025.1', 'linux', '6.5.0', 'codereview', 'https://github.com/zgsm-ai/costrict.git', 'main', 'repo-costrict-main', '/home/dev/costrict', 'proj-costrict', '2026-03-06T10:30:00', '2026-03-06T11:15:00', 1800, 2900, 0.95, 85, 1.5, '代码审查修改量适中')
ON CONFLICT (task_id) DO NOTHING;

INSERT INTO costrict_tasks (task_id, user_id, user_name, client_id, ide, version, os, os_version, caller, repo_addr, repo_branch, repo_id, project_path, project_id, start_time, end_time, upstream_tokens, downstream_tokens, cost, diff_lines, ai_estimated_ancient_days, ai_estimated_ancient_reason)
VALUES
  ('task-003', 'user-003', 'zhangsan-gh', 'client-c1', 'vscode', '1.88.0', 'macos', '14.3', 'chat', 'https://github.com/zgsm-ai/costrict.git', 'main', 'repo-costrict-main', '/Users/zhangsan/costrict', 'proj-costrict', '2026-03-08T14:00:00', '2026-03-08T15:30:00', 4200, 6100, 2.10, 230, 3.0, '新增功能模块，涉及多文件改动')
ON CONFLICT (task_id) DO NOTHING;

INSERT INTO costrict_tasks (task_id, user_id, user_name, client_id, ide, version, os, os_version, caller, repo_addr, repo_branch, repo_id, project_path, project_id, start_time, end_time, upstream_tokens, downstream_tokens, cost, diff_lines, ai_estimated_ancient_days, ai_estimated_ancient_reason)
VALUES
  ('task-004', 'user-004', 'lisi-dev', 'client-d1', 'intellij', '2025.1', 'windows', '11.0.22631', 'codereview', 'https://github.com/zgsm-ai/costrict.git', 'main', 'repo-costrict-main', 'D:/projects/costrict', 'proj-costrict', '2026-03-10T08:00:00', '2026-03-10T08:30:00', 900, 1500, 0.48, 45, 0.5, '小范围代码审查')
ON CONFLICT (task_id) DO NOTHING;

INSERT INTO costrict_tasks (task_id, user_id, user_name, client_id, ide, version, os, os_version, caller, repo_addr, repo_branch, repo_id, project_path, project_id, start_time, end_time, upstream_tokens, downstream_tokens, cost, diff_lines, ai_estimated_ancient_days, ai_estimated_ancient_reason)
VALUES
  ('task-005', 'user-005', 'wangwu2025', 'client-e1', 'vscode', '1.89.0', 'linux', '6.8.0', 'chat', 'https://github.com/zgsm-ai/costrict.git', 'main', 'repo-costrict-main', '/home/wangwu/costrict', 'proj-costrict', '2026-03-12T16:00:00', '2026-03-12T17:20:00', 3100, 4700, 1.58, 175, 2.0, '数据库查询优化，涉及索引调整')
ON CONFLICT (task_id) DO NOTHING;

INSERT INTO costrict_tasks (task_id, user_id, user_name, client_id, ide, version, os, os_version, caller, repo_addr, repo_branch, repo_id, project_path, project_id, start_time, end_time, upstream_tokens, downstream_tokens, cost, diff_lines, ai_estimated_ancient_days, ai_estimated_ancient_reason)
VALUES
  ('task-006', 'user-001', '138xxxx0001', 'client-a2', 'vscode', '1.88.0', 'windows', '10.0.19045', 'chat', 'https://github.com/zgsm-ai/kanban.git', 'dev', 'repo-kanban-dev', '/home/dev/kanban', 'proj-kanban', '2026-03-14T09:30:00', '2026-03-14T10:45:00', 2800, 4200, 1.40, 150, 2.0, '前端看板组件开发')
ON CONFLICT (task_id) DO NOTHING;

INSERT INTO costrict_tasks (task_id, user_id, user_name, client_id, ide, version, os, os_version, caller, repo_addr, repo_branch, repo_id, project_path, project_id, start_time, end_time, upstream_tokens, downstream_tokens, cost, diff_lines, ai_estimated_ancient_days, ai_estimated_ancient_reason)
VALUES
  ('task-007', 'user-002', '139xxxx0002', 'client-b2', 'vscode', '1.89.0', 'linux', '6.5.0', 'codereview', 'https://github.com/zgsm-ai/kanban.git', 'dev', 'repo-kanban-dev', '/home/dev/kanban', 'proj-kanban', '2026-03-16T11:00:00', '2026-03-16T11:40:00', 1200, 2000, 0.64, 60, 1.0, 'API接口审查')
ON CONFLICT (task_id) DO NOTHING;

INSERT INTO costrict_tasks (task_id, user_id, user_name, client_id, ide, version, os, os_version, caller, repo_addr, repo_branch, repo_id, project_path, project_id, start_time, end_time, upstream_tokens, downstream_tokens, cost, diff_lines, ai_estimated_ancient_days, ai_estimated_ancient_reason)
VALUES
  ('task-008', 'user-003', 'zhangsan-gh', 'client-c2', 'intellij', '2025.1', 'macos', '14.3', 'chat', 'https://github.com/zgsm-ai/kanban.git', 'dev', 'repo-kanban-dev', '/Users/zhangsan/kanban', 'proj-kanban', '2026-03-18T13:00:00', '2026-03-18T14:30:00', 3500, 5200, 1.74, 195, 2.5, '图表数据聚合逻辑实现')
ON CONFLICT (task_id) DO NOTHING;

INSERT INTO costrict_tasks (task_id, user_id, user_name, client_id, ide, version, os, os_version, caller, repo_addr, repo_branch, repo_id, project_path, project_id, start_time, end_time, upstream_tokens, downstream_tokens, cost, diff_lines, ai_estimated_ancient_days, ai_estimated_ancient_reason)
VALUES
  ('task-009', 'user-004', 'lisi-dev', 'client-d2', 'vscode', '1.88.0', 'windows', '11.0.22631', 'codereview', 'https://github.com/zgsm-ai/kanban.git', 'dev', 'repo-kanban-dev', 'D:/projects/kanban', 'proj-kanban', '2026-03-20T15:00:00', '2026-03-20T15:25:00', 800, 1300, 0.42, 35, 0.5, '前端样式微调审查')
ON CONFLICT (task_id) DO NOTHING;

INSERT INTO costrict_tasks (task_id, user_id, user_name, client_id, ide, version, os, os_version, caller, repo_addr, repo_branch, repo_id, project_path, project_id, start_time, end_time, upstream_tokens, downstream_tokens, cost, diff_lines, ai_estimated_ancient_days, ai_estimated_ancient_reason)
VALUES
  ('task-010', 'user-005', 'wangwu2025', 'client-e2', 'intellij', '2025.1', 'linux', '6.8.0', 'chat', 'https://github.com/zgsm-ai/kanban.git', 'dev', 'repo-kanban-dev', '/home/wangwu/kanban', 'proj-kanban', '2026-03-22T10:00:00', '2026-03-22T11:30:00', 3800, 5600, 1.88, 210, 3.0, '权限控制模块全新实现')
ON CONFLICT (task_id) DO NOTHING;

INSERT INTO costrict_tasks (task_id, user_id, user_name, client_id, ide, version, os, os_version, caller, repo_addr, repo_branch, repo_id, project_path, project_id, start_time, end_time, upstream_tokens, downstream_tokens, cost, diff_lines, ai_estimated_ancient_days, ai_estimated_ancient_reason)
VALUES
  ('task-011', 'user-001', '138xxxx0001', 'client-a3', 'vscode', '1.89.0', 'windows', '10.0.19045', 'chat', 'https://gitee.com/example/webapp.git', 'master', 'repo-webapp-master', '/home/dev/webapp', 'proj-webapp', '2026-03-24T08:30:00', '2026-03-24T09:45:00', 2200, 3400, 1.12, 105, 1.5, '用户注册流程实现')
ON CONFLICT (task_id) DO NOTHING;

INSERT INTO costrict_tasks (task_id, user_id, user_name, client_id, ide, version, os, os_version, caller, repo_addr, repo_branch, repo_id, project_path, project_id, start_time, end_time, upstream_tokens, downstream_tokens, cost, diff_lines, ai_estimated_ancient_days, ai_estimated_ancient_reason)
VALUES
  ('task-012', 'user-002', '139xxxx0002', 'client-b3', 'intellij', '2025.1', 'linux', '6.5.0', 'codereview', 'https://gitee.com/example/webapp.git', 'master', 'repo-webapp-master', '/home/dev/webapp', 'proj-webapp', '2026-03-26T14:00:00', '2026-03-26T14:50:00', 1600, 2600, 0.84, 75, 1.0, '表单验证逻辑审查')
ON CONFLICT (task_id) DO NOTHING;

INSERT INTO costrict_tasks (task_id, user_id, user_name, client_id, ide, version, os, os_version, caller, repo_addr, repo_branch, repo_id, project_path, project_id, start_time, end_time, upstream_tokens, downstream_tokens, cost, diff_lines, ai_estimated_ancient_days, ai_estimated_ancient_reason)
VALUES
  ('task-013', 'user-003', 'zhangsan-gh', 'client-c3', 'vscode', '1.88.0', 'macos', '14.3', 'chat', 'https://gitee.com/example/webapp.git', 'master', 'repo-webapp-master', '/Users/zhangsan/webapp', 'proj-webapp', '2026-03-28T10:00:00', '2026-03-28T11:40:00', 3900, 5800, 1.94, 240, 3.5, '支付模块集成，涉及第三方SDK')
ON CONFLICT (task_id) DO NOTHING;

INSERT INTO costrict_tasks (task_id, user_id, user_name, client_id, ide, version, os, os_version, caller, repo_addr, repo_branch, repo_id, project_path, project_id, start_time, end_time, upstream_tokens, downstream_tokens, cost, diff_lines, ai_estimated_ancient_days, ai_estimated_ancient_reason)
VALUES
  ('task-014', 'user-004', 'lisi-dev', 'client-d3', 'intellij', '2025.1', 'windows', '11.0.22631', 'chat', 'https://gitee.com/example/webapp.git', 'master', 'repo-webapp-master', 'D:/projects/webapp', 'proj-webapp', '2026-03-30T09:00:00', '2026-03-30T10:15:00', 2600, 3900, 1.30, 140, 2.0, '日志系统重构')
ON CONFLICT (task_id) DO NOTHING;

INSERT INTO costrict_tasks (task_id, user_id, user_name, client_id, ide, version, os, os_version, caller, repo_addr, repo_branch, repo_id, project_path, project_id, start_time, end_time, upstream_tokens, downstream_tokens, cost, diff_lines, ai_estimated_ancient_days, ai_estimated_ancient_reason)
VALUES
  ('task-015', 'user-005', 'wangwu2025', 'client-e3', 'vscode', '1.89.0', 'linux', '6.8.0', 'codereview', 'https://gitee.com/example/webapp.git', 'master', 'repo-webapp-master', '/home/wangwu/webapp', 'proj-webapp', '2026-04-01T16:00:00', '2026-04-01T16:35:00', 1100, 1800, 0.58, 50, 0.5, '工具函数审查，改动较小')
ON CONFLICT (task_id) DO NOTHING;


-- === costrict_task_conversations 测试数据 ===
-- 每个 task 3~5 条 conversation，共约 55 条
-- model: GLM-4.7, Kimi-K2.5, MiniMax-M2.1
-- sender: user, assistant

-- task-001 conversations (5条)
INSERT INTO costrict_task_conversations (task_id, request_id, sender, prompt_mode, mode, model, start_time, end_time, process_time, process_ttft, upstream_tokens, downstream_tokens, cost, request_content, response_content, user_input, diff, diff_lines, error_code, error_reason)
VALUES
  ('task-001', 'req-001-1', 'user', 'normal', 'chat', 'GLM-4.7', '2026-03-05T09:00:00', '2026-03-05T09:02:00', 1200, 350, 500, 800, 0.26, '帮我实现登录功能', NULL, '帮我实现登录功能', NULL, NULL, NULL, NULL),
  ('task-001', 'req-001-2', 'assistant', 'normal', 'chat', 'GLM-4.7', '2026-03-05T09:02:00', '2026-03-05T09:05:00', 2800, 420, 800, 1200, 0.40, NULL, '好的，我来为你实现一个基于JWT的登录功能...', NULL, '+func Login(ctx *gin.Context) {\n+  // validate credentials\n+}', 35, NULL, NULL),
  ('task-001', 'req-001-3', 'user', 'normal', 'chat', 'GLM-4.7', '2026-03-05T09:10:00', '2026-03-05T09:12:00', 1500, 380, 600, 900, 0.30, '添加密码加密处理', NULL, '添加密码加密处理', NULL, NULL, NULL, NULL),
  ('task-001', 'req-001-4', 'assistant', 'normal', 'chat', 'GLM-4.7', '2026-03-05T09:12:00', '2026-03-05T09:16:00', 3200, 450, 900, 1400, 0.46, NULL, '使用bcrypt进行密码哈希处理...', NULL, '+import "golang.org/x/crypto/bcrypt"\n+func HashPassword(pwd string) ...', 50, NULL, NULL),
  ('task-001', 'req-001-5', 'user', 'normal', 'chat', 'Kimi-K2.5', '2026-03-05T09:20:00', '2026-03-05T09:22:00', 1100, 300, 400, 700, 0.22, '添加单元测试', NULL, '添加单元测试', NULL, NULL, NULL, NULL)
ON CONFLICT (task_id, request_id) DO NOTHING;

-- task-002 conversations (3条)
INSERT INTO costrict_task_conversations (task_id, request_id, sender, prompt_mode, mode, model, start_time, end_time, process_time, process_ttft, upstream_tokens, downstream_tokens, cost, request_content, response_content, user_input, diff, diff_lines, error_code, error_reason)
VALUES
  ('task-002', 'req-002-1', 'user', 'review', 'codereview', 'Kimi-K2.5', '2026-03-06T10:30:00', '2026-03-06T10:33:00', 1800, 400, 600, 1000, 0.32, '帮我review这段代码', NULL, '帮我review这段代码', NULL, NULL, NULL, NULL),
  ('task-002', 'req-002-2', 'assistant', 'review', 'codereview', 'Kimi-K2.5', '2026-03-06T10:33:00', '2026-03-06T10:38:00', 3500, 500, 800, 1300, 0.42, NULL, '发现几个问题：1. 缺少错误处理 2. 变量命名不规范...', NULL, '-var x = getData()\n+result, err := getData()\n+if err != nil { return err }', 40, NULL, NULL),
  ('task-002', 'req-002-3', 'user', 'review', 'codereview', 'Kimi-K2.5', '2026-03-06T10:40:00', '2026-03-06T10:42:00', 1400, 360, 400, 600, 0.20, '修复这个bug', NULL, '修复这个bug', NULL, NULL, NULL, NULL)
ON CONFLICT (task_id, request_id) DO NOTHING;

-- task-003 conversations (4条)
INSERT INTO costrict_task_conversations (task_id, request_id, sender, prompt_mode, mode, model, start_time, end_time, process_time, process_ttft, upstream_tokens, downstream_tokens, cost, request_content, response_content, user_input, diff, diff_lines, error_code, error_reason)
VALUES
  ('task-003', 'req-003-1', 'user', 'normal', 'chat', 'MiniMax-M2.1', '2026-03-08T14:00:00', '2026-03-08T14:03:00', 2000, 420, 800, 1200, 0.40, '重构用户模块', NULL, '重构用户模块', NULL, NULL, NULL, NULL),
  ('task-003', 'req-003-2', 'assistant', 'normal', 'chat', 'MiniMax-M2.1', '2026-03-08T14:03:00', '2026-03-08T14:10:00', 4500, 550, 1200, 2000, 0.66, NULL, '我来帮你重构用户模块，将其拆分为service和repository两层...', NULL, '+type UserService struct {\n+  repo UserRepository\n+}', 80, NULL, NULL),
  ('task-003', 'req-003-3', 'user', 'normal', 'chat', 'MiniMax-M2.1', '2026-03-08T14:15:00', '2026-03-08T14:18:00', 2200, 430, 700, 1100, 0.36, '添加接口定义', NULL, '添加接口定义', NULL, NULL, NULL, NULL),
  ('task-003', 'req-003-4', 'assistant', 'normal', 'chat', 'GLM-4.7', '2026-03-08T14:18:00', '2026-03-08T14:25:00', 4000, 500, 1100, 1800, 0.60, NULL, '定义UserRepository接口...', NULL, '+type UserRepository interface {\n+  FindByID(id string) (*User, error)\n+}', 70, NULL, NULL)
ON CONFLICT (task_id, request_id) DO NOTHING;

-- task-004 conversations (3条)
INSERT INTO costrict_task_conversations (task_id, request_id, sender, prompt_mode, mode, model, start_time, end_time, process_time, process_ttft, upstream_tokens, downstream_tokens, cost, request_content, response_content, user_input, diff, diff_lines, error_code, error_reason)
VALUES
  ('task-004', 'req-004-1', 'user', 'review', 'codereview', 'GLM-4.7', '2026-03-10T08:00:00', '2026-03-10T08:03:00', 1600, 370, 300, 500, 0.16, '帮我review这段代码', NULL, '帮我review这段代码', NULL, NULL, NULL, NULL),
  ('task-004', 'req-004-2', 'assistant', 'review', 'codereview', 'GLM-4.7', '2026-03-10T08:03:00', '2026-03-10T08:08:00', 2800, 440, 400, 700, 0.22, NULL, '代码整体结构良好，建议优化错误处理方式', NULL, NULL, NULL, NULL, NULL),
  ('task-004', 'req-004-3', 'user', 'review', 'codereview', 'GLM-4.7', '2026-03-10T08:10:00', '2026-03-10T08:12:00', 1200, 320, 200, 300, 0.10, '好的，已修改', NULL, '好的，已修改', NULL, NULL, NULL, NULL)
ON CONFLICT (task_id, request_id) DO NOTHING;

-- task-005 conversations (4条)
INSERT INTO costrict_task_conversations (task_id, request_id, sender, prompt_mode, mode, model, start_time, end_time, process_time, process_ttft, upstream_tokens, downstream_tokens, cost, request_content, response_content, user_input, diff, diff_lines, error_code, error_reason)
VALUES
  ('task-005', 'req-005-1', 'user', 'normal', 'chat', 'Kimi-K2.5', '2026-03-12T16:00:00', '2026-03-12T16:03:00', 1800, 390, 600, 900, 0.30, '优化数据库查询性能', NULL, '优化数据库查询性能', NULL, NULL, NULL, NULL),
  ('task-005', 'req-005-2', 'assistant', 'normal', 'chat', 'Kimi-K2.5', '2026-03-12T16:03:00', '2026-03-12T16:10:00', 4200, 520, 1000, 1600, 0.52, NULL, '分析慢查询日志后，建议添加复合索引并优化JOIN语句...', NULL, '+CREATE INDEX idx_tasks_user_time ON tasks(user_id, start_time);', 55, NULL, NULL),
  ('task-005', 'req-005-3', 'user', 'normal', 'chat', 'Kimi-K2.5', '2026-03-12T16:15:00', '2026-03-12T16:17:00', 1500, 360, 500, 800, 0.26, '再优化一下N+1查询问题', NULL, '再优化一下N+1查询问题', NULL, NULL, NULL, NULL),
  ('task-005', 'req-005-4', 'assistant', 'normal', 'chat', 'MiniMax-M2.1', '2026-03-12T16:17:00', '2026-03-12T16:25:00', 4800, 560, 1100, 1700, 0.56, NULL, '使用预加载替代循环查询...', NULL, '+func PreloadTasks(ids []string) ([]Task, error) {...}', 65, NULL, NULL)
ON CONFLICT (task_id, request_id) DO NOTHING;

-- task-006 conversations (4条)
INSERT INTO costrict_task_conversations (task_id, request_id, sender, prompt_mode, mode, model, start_time, end_time, process_time, process_ttft, upstream_tokens, downstream_tokens, cost, request_content, response_content, user_input, diff, diff_lines, error_code, error_reason)
VALUES
  ('task-006', 'req-006-1', 'user', 'normal', 'chat', 'GLM-4.7', '2026-03-14T09:30:00', '2026-03-14T09:33:00', 1900, 400, 550, 850, 0.28, '帮我实现看板拖拽功能', NULL, '帮我实现看板拖拽功能', NULL, NULL, NULL, NULL),
  ('task-006', 'req-006-2', 'assistant', 'normal', 'chat', 'GLM-4.7', '2026-03-14T09:33:00', '2026-03-14T09:40:00', 4100, 510, 950, 1500, 0.50, NULL, '使用vuedraggable实现看板拖拽...', NULL, '+<draggable v-model="tasks" @end="onDragEnd">', 45, NULL, NULL),
  ('task-006', 'req-006-3', 'user', 'normal', 'chat', 'Kimi-K2.5', '2026-03-14T09:45:00', '2026-03-14T09:48:00', 1600, 370, 500, 800, 0.26, '添加拖拽动画效果', NULL, '添加拖拽动画效果', NULL, NULL, NULL, NULL),
  ('task-006', 'req-006-4', 'assistant', 'normal', 'chat', 'Kimi-K2.5', '2026-03-14T09:48:00', '2026-03-14T09:55:00', 3800, 480, 850, 1350, 0.44, NULL, '添加CSS transition实现平滑拖拽效果...', NULL, '+.drag-item { transition: transform 0.3s ease; }', 60, NULL, NULL)
ON CONFLICT (task_id, request_id) DO NOTHING;

-- task-007 conversations (3条)
INSERT INTO costrict_task_conversations (task_id, request_id, sender, prompt_mode, mode, model, start_time, end_time, process_time, process_ttft, upstream_tokens, downstream_tokens, cost, request_content, response_content, user_input, diff, diff_lines, error_code, error_reason)
VALUES
  ('task-007', 'req-007-1', 'user', 'review', 'codereview', 'MiniMax-M2.1', '2026-03-16T11:00:00', '2026-03-16T11:03:00', 1700, 380, 400, 650, 0.21, '帮我review这段API代码', NULL, '帮我review这段API代码', NULL, NULL, NULL, NULL),
  ('task-007', 'req-007-2', 'assistant', 'review', 'codereview', 'MiniMax-M2.1', '2026-03-16T11:03:00', '2026-03-16T11:08:00', 3200, 470, 500, 900, 0.29, NULL, 'API路由设计合理，但缺少参数校验中间件...', NULL, '+func ValidateParams() gin.HandlerFunc {...}', 30, NULL, NULL),
  ('task-007', 'req-007-3', 'user', 'review', 'codereview', 'MiniMax-M2.1', '2026-03-16T11:10:00', '2026-03-16T11:12:00', 1300, 340, 300, 450, 0.14, '已按建议修改', NULL, '已按建议修改', NULL, NULL, NULL, NULL)
ON CONFLICT (task_id, request_id) DO NOTHING;

-- task-008 conversations (4条)
INSERT INTO costrict_task_conversations (task_id, request_id, sender, prompt_mode, mode, model, start_time, end_time, process_time, process_ttft, upstream_tokens, downstream_tokens, cost, request_content, response_content, user_input, diff, diff_lines, error_code, error_reason)
VALUES
  ('task-008', 'req-008-1', 'user', 'normal', 'chat', 'GLM-4.7', '2026-03-18T13:00:00', '2026-03-18T13:03:00', 2100, 410, 700, 1050, 0.35, '实现数据聚合查询接口', NULL, '实现数据聚合查询接口', NULL, NULL, NULL, NULL),
  ('task-008', 'req-008-2', 'assistant', 'normal', 'chat', 'GLM-4.7', '2026-03-18T13:03:00', '2026-03-18T13:10:00', 4400, 540, 1100, 1800, 0.60, NULL, '实现按时间维度聚合的查询接口...', NULL, '+func AggregateByPeriod(period string) ([]Metric, error) {...}', 65, NULL, NULL),
  ('task-008', 'req-008-3', 'user', 'normal', 'chat', 'Kimi-K2.5', '2026-03-18T13:15:00', '2026-03-18T13:18:00', 1800, 390, 600, 950, 0.31, '添加缓存层减少数据库压力', NULL, '添加缓存层减少数据库压力', NULL, NULL, NULL, NULL),
  ('task-008', 'req-008-4', 'assistant', 'normal', 'chat', 'Kimi-K2.5', '2026-03-18T13:18:00', '2026-03-18T13:26:00', 4600, 530, 1050, 1700, 0.56, NULL, '使用Redis缓存聚合结果，TTL设为5分钟...', NULL, '+func CachedAggregate(key string) ([]Metric, error) {...}', 70, NULL, NULL)
ON CONFLICT (task_id, request_id) DO NOTHING;

-- task-009 conversations (3条)
INSERT INTO costrict_task_conversations (task_id, request_id, sender, prompt_mode, mode, model, start_time, end_time, process_time, process_ttft, upstream_tokens, downstream_tokens, cost, request_content, response_content, user_input, diff, diff_lines, error_code, error_reason)
VALUES
  ('task-009', 'req-009-1', 'user', 'review', 'codereview', 'MiniMax-M2.1', '2026-03-20T15:00:00', '2026-03-20T15:02:00', 1100, 310, 250, 400, 0.13, '帮我review前端样式代码', NULL, '帮我review前端样式代码', NULL, NULL, NULL, NULL),
  ('task-009', 'req-009-2', 'assistant', 'review', 'codereview', 'MiniMax-M2.1', '2026-03-20T15:02:00', '2026-03-20T15:06:00', 2400, 400, 350, 600, 0.19, NULL, '样式基本OK，建议统一使用CSS变量管理颜色...', NULL, NULL, NULL, NULL, NULL),
  ('task-009', 'req-009-3', 'user', 'review', 'codereview', 'MiniMax-M2.1', '2026-03-20T15:08:00', '2026-03-20T15:10:00', 1000, 280, 200, 300, 0.10, '收到，已统一CSS变量', NULL, '收到，已统一CSS变量', NULL, NULL, NULL, NULL)
ON CONFLICT (task_id, request_id) DO NOTHING;

-- task-010 conversations (5条)
INSERT INTO costrict_task_conversations (task_id, request_id, sender, prompt_mode, mode, model, start_time, end_time, process_time, process_ttft, upstream_tokens, downstream_tokens, cost, request_content, response_content, user_input, diff, diff_lines, error_code, error_reason)
VALUES
  ('task-010', 'req-010-1', 'user', 'normal', 'chat', 'Kimi-K2.5', '2026-03-22T10:00:00', '2026-03-22T10:03:00', 2000, 410, 750, 1100, 0.36, '实现RBAC权限控制', NULL, '实现RBAC权限控制', NULL, NULL, NULL, NULL),
  ('task-010', 'req-010-2', 'assistant', 'normal', 'chat', 'Kimi-K2.5', '2026-03-22T10:03:00', '2026-03-22T10:12:00', 5200, 580, 1300, 2100, 0.70, NULL, '设计角色-权限模型，包含role、permission、user_role三张表...', NULL, '+CREATE TABLE roles (\n+  id SERIAL PRIMARY KEY,\n+  name VARCHAR(50)\n+);', 75, NULL, NULL),
  ('task-010', 'req-010-3', 'user', 'normal', 'chat', 'GLM-4.7', '2026-03-22T10:15:00', '2026-03-22T10:18:00', 1700, 380, 600, 950, 0.31, '添加权限校验中间件', NULL, '添加权限校验中间件', NULL, NULL, NULL, NULL),
  ('task-010', 'req-010-4', 'assistant', 'normal', 'chat', 'GLM-4.7', '2026-03-22T10:18:00', '2026-03-22T10:26:00', 4800, 550, 1100, 1800, 0.60, NULL, '实现Gin中间件进行权限校验...', NULL, '+func RequirePermission(perm string) gin.HandlerFunc {...}', 60, NULL, NULL),
  ('task-010', 'req-010-5', 'user', 'normal', 'chat', 'GLM-4.7', '2026-03-22T10:30:00', '2026-03-22T10:32:00', 1300, 340, 450, 700, 0.23, '添加单元测试', NULL, '添加单元测试', NULL, NULL, NULL, NULL)
ON CONFLICT (task_id, request_id) DO NOTHING;

-- task-011 conversations (3条)
INSERT INTO costrict_task_conversations (task_id, request_id, sender, prompt_mode, mode, model, start_time, end_time, process_time, process_ttft, upstream_tokens, downstream_tokens, cost, request_content, response_content, user_input, diff, diff_lines, error_code, error_reason)
VALUES
  ('task-011', 'req-011-1', 'user', 'normal', 'chat', 'MiniMax-M2.1', '2026-03-24T08:30:00', '2026-03-24T08:33:00', 1900, 400, 500, 750, 0.25, '帮我实现用户注册功能', NULL, '帮我实现用户注册功能', NULL, NULL, NULL, NULL),
  ('task-011', 'req-011-2', 'assistant', 'normal', 'chat', 'MiniMax-M2.1', '2026-03-24T08:33:00', '2026-03-24T08:40:00', 4000, 500, 850, 1400, 0.46, NULL, '实现包含邮箱验证的注册流程...', NULL, '+func Register(ctx *gin.Context) {\n+  // validate & create user\n+}', 55, NULL, NULL),
  ('task-011', 'req-011-3', 'user', 'normal', 'chat', 'MiniMax-M2.1', '2026-03-24T08:45:00', '2026-03-24T08:48:00', 1600, 370, 450, 700, 0.23, '添加邮箱验证码发送', NULL, '添加邮箱验证码发送', NULL, NULL, NULL, NULL)
ON CONFLICT (task_id, request_id) DO NOTHING;

-- task-012 conversations (3条)
INSERT INTO costrict_task_conversations (task_id, request_id, sender, prompt_mode, mode, model, start_time, end_time, process_time, process_ttft, upstream_tokens, downstream_tokens, cost, request_content, response_content, user_input, diff, diff_lines, error_code, error_reason)
VALUES
  ('task-012', 'req-012-1', 'user', 'review', 'codereview', 'GLM-4.7', '2026-03-26T14:00:00', '2026-03-26T14:03:00', 1700, 380, 500, 800, 0.26, '帮我review表单验证逻辑', NULL, '帮我review表单验证逻辑', NULL, NULL, NULL, NULL),
  ('task-012', 'req-012-2', 'assistant', 'review', 'codereview', 'GLM-4.7', '2026-03-26T14:03:00', '2026-03-26T14:08:00', 3100, 460, 700, 1100, 0.36, NULL, '建议使用validator库统一验证规则...', NULL, '+binding:"required,email"', 35, NULL, NULL),
  ('task-012', 'req-012-3', 'user', 'review', 'codereview', 'Kimi-K2.5', '2026-03-26T14:12:00', '2026-03-26T14:14:00', 1200, 330, 300, 500, 0.16, '已按建议修改验证逻辑', NULL, '已按建议修改验证逻辑', NULL, NULL, NULL, NULL)
ON CONFLICT (task_id, request_id) DO NOTHING;

-- task-013 conversations (5条)
INSERT INTO costrict_task_conversations (task_id, request_id, sender, prompt_mode, mode, model, start_time, end_time, process_time, process_ttft, upstream_tokens, downstream_tokens, cost, request_content, response_content, user_input, diff, diff_lines, error_code, error_reason)
VALUES
  ('task-013', 'req-013-1', 'user', 'normal', 'chat', 'Kimi-K2.5', '2026-03-28T10:00:00', '2026-03-28T10:03:00', 2100, 420, 800, 1200, 0.40, '集成支付宝支付SDK', NULL, '集成支付宝支付SDK', NULL, NULL, NULL, NULL),
  ('task-013', 'req-013-2', 'assistant', 'normal', 'chat', 'Kimi-K2.5', '2026-03-28T10:03:00', '2026-03-28T10:12:00', 5500, 600, 1400, 2200, 0.74, NULL, '封装支付宝SDK调用，实现统一支付接口...', NULL, '+type PaymentService struct {\n+  client *alipay.Client\n+}', 85, NULL, NULL),
  ('task-013', 'req-013-3', 'user', 'normal', 'chat', 'MiniMax-M2.1', '2026-03-28T10:15:00', '2026-03-28T10:18:00', 1900, 400, 650, 1000, 0.33, '添加支付回调处理', NULL, '添加支付回调处理', NULL, NULL, NULL, NULL),
  ('task-013', 'req-013-4', 'assistant', 'normal', 'chat', 'MiniMax-M2.1', '2026-03-28T10:18:00', '2026-03-28T10:27:00', 5100, 570, 1200, 1900, 0.64, NULL, '实现异步通知回调验签和订单状态更新...', NULL, '+func HandlePayNotify(ctx *gin.Context) {...}', 70, NULL, NULL),
  ('task-013', 'req-013-5', 'user', 'normal', 'chat', 'GLM-4.7', '2026-03-28T10:30:00', '2026-03-28T10:32:00', 1400, 350, 500, 800, 0.26, '添加支付失败重试机制', NULL, '添加支付失败重试机制', NULL, NULL, NULL, NULL)
ON CONFLICT (task_id, request_id) DO NOTHING;

-- task-014 conversations (4条)
INSERT INTO costrict_task_conversations (task_id, request_id, sender, prompt_mode, mode, model, start_time, end_time, process_time, process_ttft, upstream_tokens, downstream_tokens, cost, request_content, response_content, user_input, diff, diff_lines, error_code, error_reason)
VALUES
  ('task-014', 'req-014-1', 'user', 'normal', 'chat', 'MiniMax-M2.1', '2026-03-30T09:00:00', '2026-03-30T09:03:00', 1800, 390, 550, 850, 0.28, '重构日志系统，支持结构化日志', NULL, '重构日志系统，支持结构化日志', NULL, NULL, NULL, NULL),
  ('task-014', 'req-014-2', 'assistant', 'normal', 'chat', 'MiniMax-M2.1', '2026-03-30T09:03:00', '2026-03-30T09:10:00', 4200, 520, 1000, 1600, 0.54, NULL, '使用zap替换标准log库，实现结构化日志输出...', NULL, '+var Logger *zap.Logger\n+func InitLogger() {...}', 50, NULL, NULL),
  ('task-014', 'req-014-3', 'user', 'normal', 'chat', 'GLM-4.7', '2026-03-30T09:15:00', '2026-03-30T09:18:00', 1600, 370, 500, 800, 0.26, '添加日志轮转配置', NULL, '添加日志轮转配置', NULL, NULL, NULL, NULL),
  ('task-014', 'req-014-4', 'assistant', 'normal', 'chat', 'GLM-4.7', '2026-03-30T09:18:00', '2026-03-30T09:24:00', 3600, 470, 850, 1350, 0.44, NULL, '配置lumberjack实现日志轮转...', NULL, '+MaxSize: 100, MaxBackups: 3, MaxAge: 28', 40, NULL, NULL)
ON CONFLICT (task_id, request_id) DO NOTHING;

-- task-015 conversations (3条)
INSERT INTO costrict_task_conversations (task_id, request_id, sender, prompt_mode, mode, model, start_time, end_time, process_time, process_ttft, upstream_tokens, downstream_tokens, cost, request_content, response_content, user_input, diff, diff_lines, error_code, error_reason)
VALUES
  ('task-015', 'req-015-1', 'user', 'review', 'codereview', 'Kimi-K2.5', '2026-04-01T16:00:00', '2026-04-01T16:03:00', 1500, 360, 350, 550, 0.18, '帮我review工具函数', NULL, '帮我review工具函数', NULL, NULL, NULL, NULL),
  ('task-015', 'req-015-2', 'assistant', 'review', 'codereview', 'Kimi-K2.5', '2026-04-01T16:03:00', '2026-04-01T16:07:00', 2600, 420, 450, 750, 0.24, NULL, '工具函数整体简洁，建议补充边界条件处理...', NULL, '+if len(input) == 0 { return "", ErrEmptyInput }', 25, NULL, NULL),
  ('task-015', 'req-015-3', 'user', 'review', 'codereview', 'Kimi-K2.5', '2026-04-01T16:10:00', '2026-04-01T16:12:00', 1100, 300, 250, 400, 0.13, '已添加边界检查', NULL, '已添加边界检查', NULL, NULL, NULL, NULL)
ON CONFLICT (task_id, request_id) DO NOTHING;


-- === costrict_commits 测试数据 ===
-- 12 个 commit，分布在 3 个仓库中（每仓库 4 个）

-- repo-costrict-main 的 commits
INSERT INTO costrict_commits (commit_id, commit_time, repo_addr, repo_branch, repo_id, git_user_name, git_user_email, user_id, user_name, client_id, project_path, diff_lines, ai_estimated_ancient_days, ai_estimated_ancient_reason)
VALUES
  ('commit-001', '2026-03-05T10:00:00', 'https://github.com/zgsm-ai/costrict.git', 'main', 'repo-costrict-main', 'zhangsan', 'zhangsan@example.com', 'user-003', 'zhangsan-gh', 'client-c1', '/Users/zhangsan/costrict', 135, 2.0, '认证模块新增，包含JWT和bcrypt集成'),
  ('commit-002', '2026-03-08T16:00:00', 'https://github.com/zgsm-ai/costrict.git', 'main', 'repo-costrict-main', 'lisi', 'lisi@example.com', 'user-004', 'lisi-dev', 'client-d1', 'D:/projects/costrict', 250, 3.0, '用户模块重构，拆分service和repository'),
  ('commit-003', '2026-03-12T18:00:00', 'https://github.com/zgsm-ai/costrict.git', 'main', 'repo-costrict-main', 'wangwu', 'wangwu@example.com', 'user-005', 'wangwu2025', 'client-e1', '/home/wangwu/costrict', 180, 2.5, '数据库查询优化，添加复合索引'),
  ('commit-004', '2026-03-15T11:00:00', 'https://github.com/zgsm-ai/costrict.git', 'main', 'repo-costrict-main', 'zhangsan', 'zhangsan@example.com', 'user-003', 'zhangsan-gh', 'client-c1', '/Users/zhangsan/costrict', 95, 1.0, '接口定义补充和文档更新')
ON CONFLICT (commit_id, repo_id) DO NOTHING;

-- repo-kanban-dev 的 commits
INSERT INTO costrict_commits (commit_id, commit_time, repo_addr, repo_branch, repo_id, git_user_name, git_user_email, user_id, user_name, client_id, project_path, diff_lines, ai_estimated_ancient_days, ai_estimated_ancient_reason)
VALUES
  ('commit-005', '2026-03-14T11:00:00', 'https://github.com/zgsm-ai/kanban.git', 'dev', 'repo-kanban-dev', 'zhangsan', 'zhangsan@example.com', 'user-001', '138xxxx0001', 'client-a2', '/home/dev/kanban', 160, 2.0, '看板拖拽功能实现'),
  ('commit-006', '2026-03-18T15:00:00', 'https://github.com/zgsm-ai/kanban.git', 'dev', 'repo-kanban-dev', 'zhangsan', 'zhangsan@example.com', 'user-003', 'zhangsan-gh', 'client-c2', '/Users/zhangsan/kanban', 200, 2.5, '数据聚合接口和缓存层'),
  ('commit-007', '2026-03-22T12:00:00', 'https://github.com/zgsm-ai/kanban.git', 'dev', 'repo-kanban-dev', 'wangwu', 'wangwu@example.com', 'user-005', 'wangwu2025', 'client-e2', '/home/wangwu/kanban', 220, 3.0, 'RBAC权限控制完整实现'),
  ('commit-008', '2026-03-25T09:00:00', 'https://github.com/zgsm-ai/kanban.git', 'dev', 'repo-kanban-dev', 'lisi', 'lisi@example.com', 'user-004', 'lisi-dev', 'client-d2', 'D:/projects/kanban', 40, 0.5, '前端样式微调')
ON CONFLICT (commit_id, repo_id) DO NOTHING;

-- repo-webapp-master 的 commits
INSERT INTO costrict_commits (commit_id, commit_time, repo_addr, repo_branch, repo_id, git_user_name, git_user_email, user_id, user_name, client_id, project_path, diff_lines, ai_estimated_ancient_days, ai_estimated_ancient_reason)
VALUES
  ('commit-009', '2026-03-24T10:00:00', 'https://gitee.com/example/webapp.git', 'master', 'repo-webapp-master', 'zhangsan', 'zhangsan@example.com', 'user-001', '138xxxx0001', 'client-a3', '/home/dev/webapp', 110, 1.5, '用户注册流程实现'),
  ('commit-010', '2026-03-28T12:00:00', 'https://gitee.com/example/webapp.git', 'master', 'repo-webapp-master', 'zhangsan', 'zhangsan@example.com', 'user-003', 'zhangsan-gh', 'client-c3', '/Users/zhangsan/webapp', 260, 3.5, '支付模块集成，含回调处理'),
  ('commit-011', '2026-03-30T11:00:00', 'https://gitee.com/example/webapp.git', 'master', 'repo-webapp-master', 'lisi', 'lisi@example.com', 'user-004', 'lisi-dev', 'client-d3', 'D:/projects/webapp', 145, 2.0, '日志系统重构为结构化日志'),
  ('commit-012', '2026-04-01T17:00:00', 'https://gitee.com/example/webapp.git', 'master', 'repo-webapp-master', 'wangwu', 'wangwu@example.com', 'user-005', 'wangwu2025', 'client-e3', '/home/wangwu/webapp', 55, 0.5, '工具函数边界条件修复')
ON CONFLICT (commit_id, repo_id) DO NOTHING;


-- === 补充 task 实际耗时数据和 commit-task 关联（add-commit-real-minutes-calc）===

-- 补充 task 的 ancient_minutes 和 real_minutes 数据
UPDATE costrict_tasks SET task_ancient_minutes = 120, task_real_minutes = 22 WHERE task_id = 'task-001';
UPDATE costrict_tasks SET task_ancient_minutes = 180, task_real_minutes = 45 WHERE task_id = 'task-003';
UPDATE costrict_tasks SET task_ancient_minutes = 90, task_real_minutes = 30 WHERE task_id = 'task-005';
UPDATE costrict_tasks SET task_ancient_minutes = 120, task_real_minutes = 35 WHERE task_id = 'task-006';
UPDATE costrict_tasks SET task_ancient_minutes = 150, task_real_minutes = 40 WHERE task_id = 'task-008';

-- commit-001 关联 task-001(silica=0.8), task-003(silica=0.6)
-- 预期: ai = 22*0.8 + 45*0.6 = 17.6 + 27 = 44.6
-- 预期: ancient = 120*0.2 + 180*0.4 = 24 + 72 = 96
-- 预期: real = 44.6 + 96 = 140.6
UPDATE costrict_commits SET task_ids = '["task-001","task-003"]'::jsonb, task_ids_silica = '[0.8, 0.6]'::jsonb WHERE commit_id = 'commit-001' AND repo_id = 'repo-costrict-main';

-- commit-003 关联 task-005(silica=0.9)
-- 预期: ai = 30*0.9 = 27
-- 预期: ancient = 90*0.1 = 9
-- 预期: real = 27 + 9 = 36
UPDATE costrict_commits SET task_ids = '["task-005"]'::jsonb, task_ids_silica = '[0.9]'::jsonb WHERE commit_id = 'commit-003' AND repo_id = 'repo-costrict-main';

-- commit-005 关联 task-006(silica=0.75), task-008(silica=0.85)
-- 预期: ai = 35*0.75 + 40*0.85 = 26.25 + 34 = 60.25
-- 预期: ancient = 120*0.25 + 150*0.15 = 30 + 22.5 = 52.5
-- 预期: real = 60.25 + 52.5 = 112.75
UPDATE costrict_commits SET task_ids = '["task-006","task-008"]'::jsonb, task_ids_silica = '[0.75, 0.85]'::jsonb WHERE commit_id = 'commit-005' AND repo_id = 'repo-kanban-dev';

-- commit-008 无关联 task（测试空 task_ids 场景）
-- 预期: ai = 0, ancient = commit_ancient_minutes 的值
