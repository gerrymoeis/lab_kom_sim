# Deploy script for Windows (PowerShell)
# Usage: .\scripts\deploy-windows.ps1
# Build + run as background process

$ErrorActionPreference = "Stop"
Push-Location "$PSScriptRoot\.."

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  Deploy - Inventaris Lab Komputer      " -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# 1. Build
Write-Host "[1/3] Building binary..." -ForegroundColor Yellow
$env:CGO_ENABLED = "0"
go build -ldflags="-s -w" -o app-simlab.exe .\cmd\server\main.go
if (-not (Test-Path "app-simlab.exe")) {
    Write-Host "Build gagal" -ForegroundColor Red
    Pop-Location
    exit 1
}
$size = (Get-Item "app-simlab.exe").Length / 1MB
Write-Host "  Build selesai: app-simlab.exe ($('{0:N1}' -f $size) MB)" -ForegroundColor Green

# 2. Stop existing server
Write-Host "[2/3] Stopping existing server..." -ForegroundColor Yellow
$existing = Get-Process -Name "app-simlab" -ErrorAction SilentlyContinue
if ($existing) {
    $existing | Stop-Process -Force
    Write-Host "  Server lama dihentikan" -ForegroundColor Green
    Start-Sleep -Seconds 1
} else {
    Write-Host "  Tidak ada server yang berjalan" -ForegroundColor Gray
}

# 3. Start new server
Write-Host "[3/3] Starting new server..." -ForegroundColor Yellow
$envObj = @{}
Get-Content ".env" | Where-Object { $_ -match "^\s*([^#]\w+)\s*=\s*(.+)\s*$" } | ForEach-Object {
    $key, $value = $_ -split "=", 2
    $envObj[$key.Trim()] = $value.Trim()
}
$logFile = "server.log"
$psi = New-Object System.Diagnostics.ProcessStartInfo
$psi.FileName = (Get-Item "app-simlab.exe").FullName
$psi.WorkingDirectory = (Get-Location).Path
$psi.RedirectStandardOutput = $true
$psi.RedirectStandardError = $true
$psi.UseShellExecute = $false
$psi.CreateNoWindow = $true
$envObj.GetEnumerator() | ForEach-Object { $psi.EnvironmentVariables[$_.Key] = $_.Value }
$proc = [System.Diagnostics.Process]::Start($psi)
Start-Sleep -Seconds 2

if (-not $proc.HasExited) {
    Write-Host "  Server berjalan dengan PID $($proc.Id)" -ForegroundColor Green
    Write-Host "  Log: $((Get-Location).Path)\$logFile" -ForegroundColor Green
    Write-Host ""
    Write-Host "  Akses: http://localhost:8080" -ForegroundColor Cyan
} else {
    Write-Host "  Server gagal berjalan" -ForegroundColor Red
}

Write-Host "========================================" -ForegroundColor Cyan
Pop-Location
