## 实施

- [x] 1.1 数据库新增字段迁移
     【目标对象】`init_db.sql`
     【修改目的】为 costrict_commits 表新增 commit_real_ai_minutes 和 commit_real_ancient_minutes 两个字段
     【修改方式】在文件末尾（L464 `END $$;` 之后）新增一个 DO $$ BEGIN...END $$ 迁移块
     【相关依赖】无
     【修改内容】
        - 新建 DO $$ BEGIN...END $$ 块，添加迁移注释说明用途
        - 使用 IF NOT EXISTS 幂等模式（与 L440-L463 已有迁移风格一致）：
          - ALTER TABLE costrict_commits ADD COLUMN commit_real_ai_minutes DECIMAL(10,2)
          - ALTER TABLE costrict_commits ADD COLUMN commit_real_ancient_minutes DECIMAL(10,2)

- [x] 1.2 Go struct 新增字段
     【目标对象】`backend/db.go` L214-241 CostrictCommit struct
     【修改目的】新增两个计算字段的 Go 结构体映射，使数据库字段可被正确读写
     【修改方式】在 TaskIDsSilica（L234）和 CommitRealMinutes（L235）之间插入两个新字段
     【相关依赖】任务 1.1 的数据库字段
     【修改内容】
        - 新增 CommitRealAIMinutes *float64 `json:"commit_real_ai_minutes"` 字段
        - 新增 CommitRealAncientMinutes *float64 `json:"commit_real_ancient_minutes"` 字段
        - 字段类型为 *float64（与 CommitRealMinutes 一致，支持 nil 表示未计算）

- [x] 1.3 SELECT 列名和 scan 函数更新
     【目标对象】`backend/db.go` L467-500 costrictCommitSelectColumns 变量和 scanCostrictCommit 函数
     【修改目的】让数据库查询和结果扫描包含新增的两个字段
     【修改方式】修改 costrictCommitSelectColumns 字符串和 scanCostrictCommit 函数的 Scan 调用
     【相关依赖】任务 1.2 新增的 struct 字段
     【修改内容】
        - costrictCommitSelectColumns（L469-476）：在 `task_ids, task_ids_silica,`（L473）之后、`commit_real_minutes`（L474）之前插入 `commit_real_ai_minutes, commit_real_ancient_minutes,`
        - scanCostrictCommit（L478-500）：在 `&taskIDs, &taskIDsSilica,`（L485）之后、`&m.CommitRealMinutes`（L486）之前插入 `&m.CommitRealAIMinutes, &m.CommitRealAncientMinutes,`（直接用指针字段 scan，无需额外中间变量）

- [x] 1.4 UpdateCostrictCommitTaskAssoc 函数更新
     【目标对象】`backend/db.go` L1519-1542 UpdateCostrictCommitTaskAssoc 函数
     【修改目的】使异步写回时能同时写入两个新计算字段
     【修改方式】修改函数签名、SQL UPDATE SET 子句和 Exec 参数列表
     【相关依赖】任务 1.2 新增的 struct 字段；任务 1.5 的异步调用
     【修改内容】
        - 函数签名增加两个参数：realAIMinutes *float64, realAncientMinutes *float64（放在 realMinutes 参数之后）
        - SQL UPDATE SET 子句中增加两个字段赋值：`commit_real_ai_minutes = $7, commit_real_ancient_minutes = $8`（当前占位符为 $1-$6，新增 $7 和 $8）
        - db.Exec 参数列表末尾追加 realAIMinutes, realAncientMinutes
        - 注意：当前该函数无外部调用方，仅在任务 1.5 中首次被调用，无需更新其他调用点

- [x] 1.5 在 getCommitDetailV2 中实现计算逻辑
     【目标对象】`backend/commit_handler_v2.go` L93-171 getCommitDetailV2 函数
     【修改目的】在获取 commit 详情时实时计算 commit_real_ai_minutes、commit_real_ancient_minutes、commit_real_minutes，并异步写回数据库
     【修改方式】在 L136-152 的 relatedTasks 遍历循环中添加累加计算，循环结束后赋值到 commit 对象并异步写回
     【相关依赖】`backend/db.go` 的 CostrictTask 结构体（TaskRealMinutes、TaskAncientMinutes 字段）；`backend/db.go` 的 UpdateCostrictCommitTaskAssoc 函数（任务 1.4 更新后）
     【修改内容】
        - 在 L136 循环之前声明两个累加变量：var aiMinutes, ancientMinutes float64
        - 在 L136-152 遍历循环中，对每个 task 累加计算：
          - 取 silica 值：若 i < len(silicaList) 则 silica = silicaList[i]，否则 silica = 0（边界保护）
          - AI 耗时：若 task.TaskRealMinutes != nil，则 aiMinutes += (*task.TaskRealMinutes) * silica
          - 古法耗时：若 task.TaskAncientMinutes != nil，则 ancientMinutes += (*task.TaskAncientMinutes) * (1 - silica)
          - 若对应字段为 nil 则跳过该项累加（视为 0）
        - 循环结束后处理空 task_ids 情况：
          - 若 len(taskIDs) == 0：aiMinutes = 0，ancientMinutes 取 commit.CommitAncientMinutes 的值（若 nil 则为 0）
        - 计算 commit_real_minutes = aiMinutes + ancientMinutes
        - 赋值到 commit 对象的三个字段（CommitRealAIMinutes、CommitRealAncientMinutes、CommitRealMinutes）
        - 异步写回数据库：使用 go func 闭包（与 task_handler_v2.go L257-262 的模式一致），在 goroutine 中调用 UpdateCostrictCommitTaskAssoc，传入 commitID、repoID、taskIDs、taskIDsSilica 和三个计算结果
        - 错误处理：goroutine 内捕获错误并 log.Printf 打印（与已有模式一致）
        - 计算逻辑放在 L153（efficiency_ratio 计算）之前，确保 commit.CommitRealMinutes 已更新后再用于 efficiency_ratio 计算

- [x] 1.6 更新 seed_data.sql 测试数据
     【目标对象】`seed_data.sql`
     【修改目的】为 task 补充 task_ancient_minutes 和 task_real_minutes 数据，为 commit 添加 task_ids 和 task_ids_silica 关联数据，使计算逻辑可通过真实数据验证
     【修改方式】在文件末尾追加 UPDATE 语句（不修改已有 INSERT 语句，保持幂等性）
     【相关依赖】任务 1.1 的数据库新字段；`seed_data.sql` 中已有的 task-001~015 和 commit-001~012 数据
     【修改内容】
        - 追加注释块说明："=== 补充 task 实际耗时数据和 commit-task 关联（add-commit-real-minutes-calc）==="
        - UPDATE 语句为以下 task 补充 task_ancient_minutes 和 task_real_minutes：
          - task-001: task_real_minutes=22, task_ancient_minutes=120
          - task-003: task_real_minutes=45, task_ancient_minutes=180
          - task-005: task_real_minutes=30, task_ancient_minutes=90
          - task-006: task_real_minutes=35, task_ancient_minutes=120
          - task-008: task_real_minutes=40, task_ancient_minutes=150
        - UPDATE 语句为以下 commit 设置 task_ids 和 task_ids_silica（JSONB 格式为字符串数组和数字数组）：
          - commit-001 (repo_id='repo-costrict-main'): task_ids='["task-001","task-003"]', task_ids_silica='[0.8,0.6]'
          - commit-003 (repo_id='repo-costrict-main'): task_ids='["task-005"]', task_ids_silica='[0.9]'
          - commit-005 (repo_id='repo-kanban-dev'): task_ids='["task-006","task-008"]', task_ids_silica='[0.75,0.85]'
          - commit-008 (repo_id='repo-kanban-dev'): 不设置 task_ids（测试空 task_ids 场景，使用默认值）
        - 附注释说明各 commit 的预期计算结果，方便人工验证，例如：
          - commit-001: ai=22*0.8+45*0.6=44.6, ancient=120*0.2+180*0.4=96.0, real=140.6
          - commit-003: ai=30*0.9=27.0, ancient=90*0.1=9.0, real=36.0
          - commit-005: ai=35*0.75+40*0.85=60.25, ancient=120*0.25+150*0.15=52.5, real=112.75
          - commit-008: ai=0, ancient=commit_ancient_minutes（空 task 回退场景）
