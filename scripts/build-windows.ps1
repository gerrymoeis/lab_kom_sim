# Build script for Windows (PowerShell)
# Usage: .\scripts\build-windows.ps1
# No C compiler needed — SQLite via modernc.org/sqlite (pure Go)

$ErrorActionPreference = "Stop"
Push-Location "$PSScriptRoot\.."

Write-Host "Building app-simlab.exe for Windows..." -ForegroundColor Cyan
$env:CGO_ENABLED = "0"
go build -ldflags="-s -w" -o app-simlab.exe .\cmd\server\main.go

if (Test-Path "app-simlab.exe") {
    $size = (Get-Item "app-simlab.exe").Length / 1MB
    Write-Host "Build selesai: .\app-simlab.exe ($('{0:N1}' -f $size) MB)" -ForegroundColor Green
} else {
    Write-Host "Build gagal" -ForegroundColor Red
    Pop-Location
    exit 1
}

Pop-Location
