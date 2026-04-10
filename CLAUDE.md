# CLAUDE.md

本文件为AI Coding工具在此仓库中工作提供指导规范。

---

# 第一部分：通用行为规范

## 核心原则

**谨慎优先于速度，代码简洁，目标驱动。**

### 1. 先思考后编码
- 明确陈述假设，不确定就问
- 存在多种解释时要说明，不要默默选择
- 有更简单的方法要说出来
- 不清楚就停下来说明困惑点

### 2. 简洁优先
- 只实现要求的功能，不做推测性功能
- 不为单次使用写抽象
- 不要未经请求的"灵活性"或"可配置性"
- 200 行能写成 50 行就重写

### 3. 精准修改
- 只修改必要的内容
- 不"优化"相邻代码、注释或格式
- 不重构没问题的代码
- 匹配现有风格
- 只清理自己产生的孤儿代码

### 4. 代码复用优先
- **写新函数/新前端组件之前，先检查是否有可复用的老函数/老组件**
- 如果有，应该复用；如果位置不合适，移到合适位置
- 尽可能复用代码，减少代码量，提升可维护性

### 5. 测试与验证
- **生成代码后必须测试验证**，使用真实场景和数据
- 除非万不得已，不要用 MOCK 方式虚拟测试数据
- 临时测试文件用完即删除
- 定义可验证的成功标准，多步骤任务说明计划：`[步骤] → 验证：[检查]`

### 6. 持续改进与错误总结
- **每次遇到错误后，必须总结原因并写入本文档**，避免重复犯错
- 每次把用户提出的问题或建议，整理是否应该持久化到本文档
- 反思整个工程其他地方是否还有类似问题，都梳理并修改
- 吃一堑长一智

### AI Accessibility
  Follow the `ai-friendly-web-design` skill for all frontend work.

### 防止改错已有逻辑
1. 根据需求，找到最佳设计，在复用已有逻辑基础上，进行最小化改动。
2. **删除前必须验证**：删除源文件前，用 `rg "被删函数名"` 确认所有调用方已更新，确认无残留引用
3. **逐步验证**：每完成一个任务后立即编译，不要攒多个任务再一起编译
4. **SQL占位符**：使用 `$1`, `$2`（不是 `?`）

### 防止重新造轮子：
1. 新建文件前先搜索：创建任何新函数前，用 rg 搜索项目中是否已有相同功能的实现
2. 以现有实现为基础：任务中标注了"以 X 为基础"的，必须从该文件复用，可以对这个函数等进行重构为公共函数，实在不得已，可以参考已有逻辑实现，不建议从零开始重新发挥


## 常见错误与解决方法

#### Bash/PowerShell 环境变量错误
**错误现象**：执行 psql 命令时报错 `:PGPASSWORD=1: command not found`
**原因**：在 Bash 中误用了 PowerShell 语法，或环境变量设置未生效
**正确用法**：
```bash
export PGPASSWORD='1' && psql -U postgres -d report -c "SQL"
```
**要点**：
1. 使用 `export` 设置环境变量
2. 用 `&&` 连接命令，确保环境变量生效
3. 不要在 PowerShell 中用 `$env:` 语法执行 Bash 脚本

#### 数据问题排查流程
**错误现象**：前端显示某些字段没有数据
**排查步骤**：
1. 先查数据库确认数据是否存在
2. 检查 API 返回是否包含该字段
3. 检查模板配置是否包含该字段
4. 检查前端解析逻辑是否正确
5. 确认数据更新时间（可能是缓存或异步问题）

#### 计算字段未生成
**错误现象**：源字段有数据，但计算字段（如 CAGR）为空
**排查步骤**：
1. 确认源字段数据存在
2. 检查 `field_title` 表的 formula 配置
3. 手动运行 `comdig.exe recalc --company=companyID` 触发重算
4. 检查后端 `initCompanyData` 是否正确触发计算字段重建

**被切割表格的结构特征**：
- PDF 截图转换时，垂直切割导致同一逻辑表格分裂成多个连续表格
- 第一个表格通常是表头+少量数据（1行左右）
- 后续表格第一行可能有空单元格（表头错位）
- 所有片段列数相同（考虑 colspan）

**示例合并**（深信服2018anna "募集资金承诺项目情况"）：
- 表格1：12列，1行表头
- 表格2：13列（2空+11内容），多行
- 修复：移除表格2第一行的2个空单元格 → 变成11列
- 结果：12列+11列成功合并（列数差1，在允许范围内）

## 输入处理规范
- **所有搜索/过滤输入框，传给后端或用于前端过滤前，必须先 `.trim()` 去除两侧空格**
- 防止用户粘贴时带入多余空格导致搜不到数据

---

# 第二部分：Shell 命令规范

## PowerShell 规范
- 路径有空格必须用双引号：`mkdir "/path/with spaces"`
- 删除文件：`Remove-Item -Path "file.txt" -Force`（不用 `rm`）
- 内容搜索用 `rg`，不用 `Get-Content | Select-Object`
- 文件搜索用 `fd`，不用 `Get-ChildItem -Recurse`

## Git 操作
- **必须加 `-c core.autocrlf=false` 参数**
- 示例：`git -c core.autocrlf=false add .`

## 交互式命令处理
生成命令自动执行时，不要出现交互式命令。如 psql 需要密码：
- **bash**: `export PGPASSWORD='1' && psql -U postgres -d report`
- **PowerShell**: `$env:PGPASSWORD='1'; psql -U postgres -d report`

## 常用工具

### durl（网页获取，支持渲染）
现代的网页多少动态渲染的，所以，尽量少用curl，多用durl
```powershell
# 从雪球获取财报详情
durl.exe -l css -s ".stock-info-content" -o a.md https://xueqiu.com/snowman/S/SZ300454/detail#/GSLRB

# 从 bing.com 检索
durl --site bing "keyword" -f markdown

# Search Baidu and get results
durl --site baidu "golang tutorial" -f json
```

### fd（文件查找）
```powershell
fd -H --glob "github*"  # 以glob方式查找包含隐藏目录在内的名字以github开头的文件或目录
fd -e js                # 查找所有 .js 文件
fd main.go              # 查找 main.go 文件
fd -t d src             # 查找目录
```

### rg（内容搜索）
```powershell
rg "function main"      # 搜索内容
rg -i "error" log/      # 忽略大小写搜索
rg -l "import"          # 只显示文件名
rg -C 3 "TODO" src/     # 显示上下 3 行
rg -g "*.go" "package"  # 在 .go 文件中搜索
```
**注意**：过滤文件类型用 `-g "*.go"`，不是 `--include`
比如不能这样：
$ rg "PageLayout" frontend/src --include="*.vue" -l
会报错：
rg: unrecognized flag --include
similar flags that are available: --include-zero
而应该改为：
$ rg "PageLayout" frontend/src -g "*.vue" -l


---

# 第三部分：工程规范

## 项目概述

这是一个AI Coding的指标看板程序。整体设计都是参考并复用自 D:\My\PubCode\comdigger 中的已有代码。

**技术栈**：Go (Gin) + PostgreSQL + Vue 3 + Element Plus

## 架构与目录结构




```
kanban/
├── kbcli/           # CLI 工具（kbcli.exe），数据获取
├── rawdata/          # 这个目录对你来说是readonly的，不能修改，而且这个路径D:\My\PubCode\kanban\rawdata前缀应该是一个变量，里面的目录结构是: 年-月/日/用户ID/某一次请求.json
├── backend/          # Go REST API 服务
├── frontend/         # Vue 3 前端
├── core/             # 共享 Go 库（数据库、日志、字段管理）
│   ├── infra/        # DB 连接、日志、配置
│   ├── fields/       # 字段元数据管理
│   ├── company/      # 公司信息
│   ├── query/        # 财务数据查询
│   └── time/         # 时间工具
├── tools/            # 一次性工具，每个工具一个子目录
├── temp/             # 临时文件目录，用完删除
└── init_db.sql       # 数据库初始化脚本
```


### 连接信息
- 数据库：`report`
- 用户：`postgres`
- 密码：`1`

### 执行 SQL
```powershell
# 简单 SQL
$env:PGPASSWORD='1'; psql -U postgres -d report -f file.sql

# 含中文或 $() 的 SQL，必须用单引号 here-string
@'
SELECT * FROM companies WHERE local_name = '深信服';
'@ | Out-File -Encoding UTF8 temp.sql
$env:PGPASSWORD='1'; psql -U postgres -d report -f temp.sql
Remove-Item temp.sql
```

### 重要约束
- 

