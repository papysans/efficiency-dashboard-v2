## 实施

- [x] 4.1 扩展 analyze 命令支持维度分析
     【目标对象】`kbcli/cmd_analyze.go`
     【修改目的】支持 `kbcli analyze --dimension=project|repo --id=xxx --start-date=... --end-date=...` 按维度触发提效分析
     【修改方式】修改 runAnalyze 函数，新增 dimension 子命令路径
     【相关依赖】`kbcli/es_client.go` ES 查询；后端 backend API `/api/analysis/efficiency/calculate`
     【修改内容】
        - runAnalyze 路由扩展：当第一个参数不是 "git" 时，检查 --dimension 参数
        - 实现 runAnalyzeDimension(config, args)：
          * 解析 --dimension(project/repo), --id, --start-date, --end-date, --all, --force
          * --all 模式：从 ES 获取所有 project_id 或 repo_id 列表，逐个分析
          * 单个分析：调用后端 `/api/analysis/efficiency/calculate` API 触发计算
          * 打印分析结果摘要

- [x] 4.2 新增 correct 纠错命令
     【目标对象】`kbcli/cmd_correct.go`（新增文件）
     【修改目的】支持 `kbcli correct --dimension=project --id=xxx --field=ai_estimated_days --value=50.5 --reason="..." --by="admin"`
     【修改方式】新增文件
     【相关依赖】后端 `/api/analysis/efficiency/correct` API
     【修改内容】
        - 实现 runCorrect(config, args) 函数
        - 解析参数：--dimension, --id, --field, --value, --reason, --by
        - 调用后端 PUT `/api/analysis/efficiency/correct` API
        - 打印纠错结果

- [x] 4.3 新增 cat-analysis 查看分析文件命令
     【目标对象】`kbcli/cmd_cat_analysis.go`（新增文件）
     【修改目的】支持 `kbcli cat-analysis --dimension=project --id=xxx --date=20260331`
     【修改方式】新增文件
     【相关依赖】rawdata 目录下的 analysis JSON 文件
     【修改内容】
        - 实现 runCatAnalysis(config, args) 函数
        - 解析参数：--dimension, --id, --date
        - 查找对应的分析文件：rawdata/YYYY-MM/analysis/{dimension}_{safeID}_{date}.json
        - 格式化输出 JSON 内容

- [x] 4.4 注册所有新命令到 cmd_root.go
     【目标对象】`kbcli/cmd_root.go`
     【修改目的】注册 correct 和 cat-analysis 命令
     【修改方式】修改 switch 和 printUsage
     【修改内容】
        - switch 新增 case "correct": runCorrect(config, subArgs)
        - switch 新增 case "cat-analysis": runCatAnalysis(config, subArgs)
        - printUsage 更新帮助信息

- [x] 4.5 编译验证
     【目标对象】`kbcli/`
     【修改内容】
        - go build ./... 通过
        - go test ./... 通过
