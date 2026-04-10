# import_rawdata.ps1 — 从 rawdata 的 task 聚合文件解析真实数据，通过 v2 API 写入系统
# 用法: pwsh import_rawdata.ps1

$API_BASE = "http://localhost:9990/api/v2"
$RAWDATA_DIR = "D:\My\PubCode\kanban\rawdata"
$MODEL_PRICES = @{
    "GLM-4.7"     = @{ In = 0.5;  Out = 0.5 }
    "GLM-5"       = @{ In = 1.0;  Out = 1.0 }
    "Kimi-K2.5"   = @{ In = 0.8;  Out = 0.8 }
    "MiniMax-M2.1"= @{ In = 0.3;  Out = 0.3 }
    "Auto"        = @{ In = 0.5;  Out = 0.5 }
}
$stats = @{ tasks = 0; conversations = 0; errors = 0; skipped = 0 }

function CalcCost($model, $inTokens, $outTokens) {
    $p = $MODEL_PRICES[$model]
    if (-not $p) { $p = $MODEL_PRICES["Auto"] }
    return [math]::Round(($inTokens / 1000000.0) * $p.In + ($outTokens / 1000000.0) * $p.Out, 6)
}

# ============================================================
# 第一步: 扫描所有 task 聚合文件
# ============================================================
Write-Host "=== 扫描 task 聚合文件 ===" -ForegroundColor Cyan
$taskFiles = @()
foreach ($dayDir in @("2026-03/29", "2026-03/30", "2026-03/31")) {
    $dir = Join-Path $RAWDATA_DIR $dayDir
    if (Test-Path $dir) {
        $files = Get-ChildItem -Path $dir -Filter "task_*.json" -File
        $taskFiles += $files
    }
}
Write-Host "找到 $($taskFiles.Count) 个 task 聚合文件"

# ============================================================
# 第二步: 解析 task 文件并通过 API 写入
# ============================================================
Write-Host "`n=== 解析并写入 task 数据 ===" -ForegroundColor Cyan
foreach ($f in $taskFiles) {
    try {
        $raw = Get-Content -Path $f.FullName -Raw -Encoding utf8
        $task = $raw | ConvertFrom-Json

        if (-not $task.task_id) { $stats.skipped++; continue }

        # 构造 task 写入数据
        $taskData = @{
            task_id     = $task.task_id
            user_id     = if ($task.user_id) { $task.user_id } else { "" }
            user_name   = if ($task.user_name) { $task.user_name } else { "" }
            client_id   = ""
            client_ide  = "vscode"
            caller      = "chat"
            repo_addr   = ""
            repo_branch = ""
            repo_id     = if ($task.project_id) { $task.project_id } else { "" }
            project_path = ""
            project_id  = if ($task.project_id) { $task.project_id } else { "" }
            start_time  = $task.start_time
            end_time    = $task.end_time
            upstream_tokens   = 0
            downstream_tokens = 0
            cost        = 0
            diff_lines  = if ($task.total_code_lines) { [int64]$task.total_code_lines } else { 0 }
            ai_estimated_ancient_days  = if ($task.ai_estimated_days) { $task.ai_estimated_days } else { $null }
            ai_estimated_ancient_reason = if ($task.ai_estimated_reason) { $task.ai_estimated_reason } else { "" }
        }

        # 解析 conversations 聚合 tokens/cost
        $convDataList = @()
        $totalIn = 0; $totalOut = 0; $totalCost = 0.0
        if ($task.conversations) {
            foreach ($conv in $task.conversations) {
                $cData = @{
                    task_id      = $task.task_id
                    request_id   = if ($conv.request_id) { $conv.request_id } else { [guid]::NewGuid().ToString() }
                    sender       = "user"
                    prompt_mode  = "vibe"
                    mode         = "code"
                    model        = "GLM-4.7"
                    start_time   = $conv.timestamp
                    end_time     = $conv.timestamp
                    process_time = 5000
                    process_ttft = 800
                    upstream_tokens   = 5000
                    downstream_tokens = 2000
                    cost         = 0.0035
                    user_input   = if ($conv.user_input) { $conv.user_input } else { "" }
                    diff         = ""
                    diff_lines   = 0
                }

                # 如果有 code_outputs，提取 diff
                if ($conv.code_outputs -and $conv.code_outputs.Count -gt 0) {
                    $diffText = ""
                    $diffLines = 0
                    foreach ($co in $conv.code_outputs) {
                        if ($co.code) {
                            $diffText += "--- $($co.path)`n$($co.code)`n"
                            $diffLines += ($co.code -split "`n").Count
                        }
                    }
                    $cData.diff = $diffText
                    $cData.diff_lines = $diffLines
                    $cData.downstream_tokens = [math]::Max(2000, $diffLines * 20)
                }

                $cData.cost = CalcCost "GLM-4.7" $cData.upstream_tokens $cData.downstream_tokens
                $totalIn += $cData.upstream_tokens
                $totalOut += $cData.downstream_tokens
                $totalCost += $cData.cost
                $convDataList += $cData
            }
        }

        $taskData.upstream_tokens = $totalIn
        $taskData.downstream_tokens = $totalOut
        $taskData.cost = [math]::Round($totalCost, 4)

        # 写入 task
        $body = $taskData | ConvertTo-Json -Depth 5 -Compress
        try {
            $null = Invoke-RestMethod -Uri "$API_BASE/tasks" -Method Post -ContentType "application/json" -Body $body
            $stats.tasks++
        } catch {
            Write-Host "  WARN: task $($task.task_id) 写入失败: $($_.Exception.Message)" -ForegroundColor Yellow
            $stats.errors++
            continue
        }

        # 写入 conversations
        if ($convDataList.Count -gt 0) {
            $convBody = ConvertTo-Json -InputObject @($convDataList) -Depth 5 -Compress
            try {
                $null = Invoke-RestMethod -Uri "$API_BASE/tasks/conversations/batch" -Method Post -ContentType "application/json" -Body $convBody
                $stats.conversations += $convDataList.Count
            } catch {
                Write-Host "  WARN: conversations 写入失败: $($_.Exception.Message)" -ForegroundColor Yellow
                $stats.errors++
            }
        }

        if ($stats.tasks % 10 -eq 0) { Write-Host "  已处理 $($stats.tasks) tasks..." }
    } catch {
        Write-Host "  ERROR: 解析 $($f.Name) 失败: $($_.Exception.Message)" -ForegroundColor Red
        $stats.errors++
    }
}

# ============================================================
# 第三步: 从原始请求补充更多细节（抽样处理几个用户）
# ============================================================
Write-Host "`n=== 扫描原始请求文件（抽样）===" -ForegroundColor Cyan
$requestDir = Join-Path $RAWDATA_DIR "request\2026-03\31"
if (Test-Path $requestDir) {
    $userDirs = Get-ChildItem -Path $requestDir -Directory | Select-Object -First 5
    foreach ($uDir in $userDirs) {
        $reqFiles = Get-ChildItem -Path $uDir.FullName -Filter "*.json" -File | Select-Object -First 10
        $taskGroups = @{}
        foreach ($rf in $reqFiles) {
            try {
                $raw = Get-Content -Path $rf.FullName -Raw -Encoding utf8
                $req = $raw | ConvertFrom-Json
                $tid = $req.identity.task_id
                if (-not $tid) { continue }
                if (-not $taskGroups.ContainsKey($tid)) { $taskGroups[$tid] = @() }
                $taskGroups[$tid] += $req
            } catch { continue }
        }

        foreach ($tid in $taskGroups.Keys) {
            $reqs = $taskGroups[$tid]
            $first = $reqs[0]
            $userId = if ($first.identity.user_info.uuid) { $first.identity.user_info.uuid } else { $first.identity.user_name }
            $userName = $first.identity.user_name
            $projectPath = $first.identity.project_path

            # 构造 task
            $startTime = $first.timestamp
            $endTime = $reqs[-1].timestamp
            $totalIn = 0; $totalOut = 0; $totalCost = 0.0
            $convList = @()
            foreach ($r in $reqs) {
                $model = if ($r.params.llm_params.model) { $r.params.llm_params.model } else { $r.params.model }
                if (-not $model) { $model = "Auto" }
                $inTok = if ($r.usage.prompt_tokens) { [int64]$r.usage.prompt_tokens } else { 0 }
                $outTok = if ($r.usage.completion_tokens) { [int64]$r.usage.completion_tokens } else { 0 }
                $cost = CalcCost $model $inTok $outTok
                $totalIn += $inTok; $totalOut += $outTok; $totalCost += $cost
                $mode = if ($r.params.llm_params.extra_body.mode) { $r.params.llm_params.extra_body.mode } else { "code" }
                $ttft = if ($r.latency.first_token_latency_ms) { [int64]$r.latency.first_token_latency_ms } else { 0 }
                $procTime = if ($r.latency.total_latency_ms) { [int64]$r.latency.total_latency_ms } else { 0 }

                $cData = @{
                    task_id      = $tid
                    request_id   = $r.identity.request_id
                    sender       = if ($r.identity.sender) { $r.identity.sender } else { "user" }
                    prompt_mode  = "vibe"
                    mode         = $mode
                    model        = $model
                    start_time   = $r.timestamp
                    end_time     = $r.timestamp
                    process_time = $procTime
                    process_ttft = $ttft
                    upstream_tokens   = $inTok
                    downstream_tokens = $outTok
                    cost         = $cost
                    user_input   = ""
                    diff         = ""
                    diff_lines   = 0
                }
                $convList += $cData
            }

            $taskData = @{
                task_id     = $tid
                user_id     = $userId
                user_name   = $userName
                client_id   = $first.identity.client_id
                client_ide  = if ($first.identity.client_ide) { $first.identity.client_ide } else { "vscode" }
                client_version = if ($first.identity.client_version) { $first.identity.client_version } else { "" }
                client_os   = if ($first.identity.client_os) { $first.identity.client_os } else { "" }
                caller      = if ($first.identity.caller) { $first.identity.caller } else { "chat" }
                repo_addr   = ""
                repo_branch = ""
                repo_id     = ""
                project_path = $projectPath
                project_id  = if ($first.identity.client_id) { $first.identity.client_id.Substring(0, [math]::Min(10, $first.identity.client_id.Length)) + ":" + $projectPath } else { $projectPath }
                start_time  = $startTime
                end_time    = $endTime
                upstream_tokens   = $totalIn
                downstream_tokens = $totalOut
                cost        = [math]::Round($totalCost, 4)
                diff_lines  = 0
            }

            $body = $taskData | ConvertTo-Json -Depth 5 -Compress
            try {
                $null = Invoke-RestMethod -Uri "$API_BASE/tasks" -Method Post -ContentType "application/json" -Body $body
                $stats.tasks++
            } catch { $stats.errors++; continue }

            if ($convList.Count -gt 0) {
                $convBody = ConvertTo-Json -InputObject @($convList) -Depth 5 -Compress
                try {
                    $null = Invoke-RestMethod -Uri "$API_BASE/tasks/conversations/batch" -Method Post -ContentType "application/json" -Body $convBody
                    $stats.conversations += $convList.Count
                } catch { $stats.errors++ }
            }
        }
        Write-Host "  用户 $($uDir.Name): $($taskGroups.Count) tasks"
    }
}

# ============================================================
# 第四步: 触发关联分析
# ============================================================
Write-Host "`n=== 触发 Project 关联分析 ===" -ForegroundColor Cyan
try {
    $r = Invoke-RestMethod -Uri "$API_BASE/projects/associate" -Method Post
    Write-Host "关联分析完成: $($r.count) projects"
} catch {
    Write-Host "关联分析失败: $($_.Exception.Message)" -ForegroundColor Yellow
}

# ============================================================
# 汇总
# ============================================================
Write-Host "`n=== 导入完成 ===" -ForegroundColor Green
Write-Host "  Tasks:         $($stats.tasks)"
Write-Host "  Conversations: $($stats.conversations)"
Write-Host "  Errors:        $($stats.errors)"
Write-Host "  Skipped:       $($stats.skipped)"

# 验证
Write-Host "`n=== 数据验证 ===" -ForegroundColor Cyan
try {
    $dash = Invoke-RestMethod "$API_BASE/dashboard/summary"
    Write-Host "  总Tasks:    $($dash.total_tasks)"
    Write-Host "  总Users:    $($dash.total_users)"
    Write-Host "  总Commits:  $($dash.total_commits)"
    Write-Host "  总Projects: $($dash.total_projects)"
    Write-Host "  总Tokens:   $($dash.total_tokens)"
    Write-Host "  总Cost:     $($dash.total_cost)"
} catch {
    Write-Host "  验证API调用失败: $($_.Exception.Message)" -ForegroundColor Yellow
}
