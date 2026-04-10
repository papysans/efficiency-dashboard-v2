# Set console encoding to UTF-8 (code page 65001)
$null = & chcp 65001 2>&1
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8

$pidFile = Join-Path $PSScriptRoot ".restart.pids"

# Save current directory
$originalDir = Get-Location
try {

# Kill previous backend/frontend windows by saved PIDs
if (Test-Path $pidFile) {
    $savedPids = Get-Content $pidFile | Where-Object { $_ -match '^\d+$' }
    foreach ($savedPid in $savedPids) {
        $proc = Get-Process -Id $savedPid -ErrorAction SilentlyContinue
        if ($proc) {
            Stop-Process -Id $savedPid -Force -ErrorAction SilentlyContinue
            Write-Host "Closed previous window (PID: $savedPid)" -ForegroundColor Green
        }
    }
    Remove-Item $pidFile -Force
    Start-Sleep -Seconds 1
}

# Read backend go.mod to get module name
$goModPath = "backend/go.mod"
$moduleLine = Get-Content $goModPath | Select-String "^module" | Select-Object -First 1
$moduleName = ($moduleLine -split "\s+")[1]
$exeName = ($moduleName -split "/")[-1] + ".exe"

Write-Host "Module Name: $moduleName" -ForegroundColor Green
Write-Host "Executable File: $exeName" -ForegroundColor Green

# Terminate existing backend process
Write-Host "`nTerminating existing process..." -ForegroundColor Yellow
$existingProcess = Get-Process -Name ($exeName -replace "\.exe$", "") -ErrorAction SilentlyContinue
if ($existingProcess) {
    taskkill /F /T /IM $exeName 2>&1 | Out-Null
    Start-Sleep -Seconds 2
    Write-Host "Process terminated" -ForegroundColor Green
} else {
    Write-Host "No running process found" -ForegroundColor Yellow
}

# Build backend
Write-Host "`nBuilding backend..." -ForegroundColor Yellow
$backendDir = "backend"
Set-Location $backendDir
go build -o $exeName 2>&1
Set-Location ".."

if (Test-Path "$backendDir\$exeName") {
    Write-Host "Build successful: $exeName" -ForegroundColor Green
} else {
    Write-Host "Build failed" -ForegroundColor Red
    exit 1
}

# Start backend in separate window, record PID
Write-Host "`nStarting backend in new window..." -ForegroundColor Yellow
$backendProc = Start-Process powershell.exe -ArgumentList "-Command", "Set-Location '$(Get-Location)\$backendDir'; .\$exeName" -WindowStyle Normal -PassThru
Write-Host "Backend started in new window (PID: $($backendProc.Id))" -ForegroundColor Green

# Wait for backend to start
Start-Sleep -Seconds 3

Write-Host "`n-------------------------------------------------------------------" -ForegroundColor Yellow

# Terminate frontend processes
Write-Host "`nTerminating frontend processes..." -ForegroundColor Yellow
$frontendProcesses = Get-Process | Where-Object { $_.ProcessName -eq "node" -or $_.ProcessName -eq "vite" } -ErrorAction SilentlyContinue
if ($frontendProcesses) {
    foreach ($process in $frontendProcesses) {
        try {
            $processInfo = Get-WmiObject Win32_Process -Filter "ProcessId=$($process.Id)" | Select-Object CommandLine
            if ($processInfo.CommandLine -like "*vite*" -or $processInfo.CommandLine -like "*frontend*") {
                taskkill /F /PID $($process.Id) 2>&1 | Out-Null
                Write-Host "Terminated: $($process.ProcessName) (PID: $($process.Id))" -ForegroundColor Green
            }
        } catch {
            # Ignore errors
        }
    }
    Start-Sleep -Seconds 2
}

# Check if port 8880 is in use
$port = 8880
try {
    $portInUse = Get-NetTCPConnection -LocalPort $port -ErrorAction SilentlyContinue
    if ($portInUse) {
        Write-Host "Port $port is in use, terminating..." -ForegroundColor Yellow
        foreach ($connection in $portInUse) {
            $process = Get-Process -Id $connection.OwningProcess -ErrorAction SilentlyContinue
            if ($process) {
                Stop-Process -Id $connection.OwningProcess -Force 2>&1 | Out-Null
                Write-Host "Terminated: $($process.ProcessName)" -ForegroundColor Green
            }
        }
        Start-Sleep -Seconds 2
    }
} catch {
    # Ignore Get-NetTCPConnection errors
}

    # Start frontend in separate window, record PID
    Write-Host "`nStarting frontend in new window..." -ForegroundColor Yellow
    $frontendDir = "frontend"
    $frontendProc = Start-Process powershell.exe -ArgumentList "-Command", "Set-Location '$(Get-Location)\$frontendDir'; npm run dev" -WindowStyle Normal -PassThru
    Write-Host "Frontend started in new window (PID: $($frontendProc.Id))" -ForegroundColor Green

    # Save PIDs for next restart
    @($backendProc.Id, $frontendProc.Id) | Set-Content $pidFile

} finally {
    # Restore original directory
    Set-Location $originalDir -ErrorAction SilentlyContinue
    Write-Host "`nRestored working directory: $originalDir" -ForegroundColor Green
}
