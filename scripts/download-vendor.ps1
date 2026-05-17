# Download vendor static assets for offline deployment (PowerShell)
# Run from project root: .\scripts\download-vendor.ps1

$VENDOR = "web\static\vendor"
New-Item -ItemType Directory -Path "$VENDOR\bootstrap\css" -Force | Out-Null
New-Item -ItemType Directory -Path "$VENDOR\bootstrap\js" -Force | Out-Null
New-Item -ItemType Directory -Path "$VENDOR\bootstrap-icons\fonts" -Force | Out-Null

Write-Host "Downloading Bootstrap CSS..." -ForegroundColor Cyan
Invoke-WebRequest -Uri "https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css" -OutFile "$VENDOR\bootstrap\css\bootstrap.min.css"

Write-Host "Downloading Bootstrap JS..." -ForegroundColor Cyan
Invoke-WebRequest -Uri "https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/js/bootstrap.bundle.min.js" -OutFile "$VENDOR\bootstrap\js\bootstrap.bundle.min.js"

Write-Host "Downloading Bootstrap Icons..." -ForegroundColor Cyan
Invoke-WebRequest -Uri "https://cdn.jsdelivr.net/npm/bootstrap-icons@1.11.0/font/bootstrap-icons.min.css" -OutFile "$VENDOR\bootstrap-icons\bootstrap-icons.min.css"
Invoke-WebRequest -Uri "https://cdn.jsdelivr.net/npm/bootstrap-icons@1.11.0/font/fonts/bootstrap-icons.woff2" -OutFile "$VENDOR\bootstrap-icons\fonts\bootstrap-icons.woff2"
Invoke-WebRequest -Uri "https://cdn.jsdelivr.net/npm/bootstrap-icons@1.11.0/font/fonts/bootstrap-icons.woff" -OutFile "$VENDOR\bootstrap-icons\fonts\bootstrap-icons.woff"

Write-Host "Downloading heic-to..." -ForegroundColor Cyan
Invoke-WebRequest -Uri "https://cdn.jsdelivr.net/npm/heic-to@1.4.2/dist/iife/heic-to.js" -OutFile "$VENDOR\heic-to.js"

Write-Host "Done. Vendor files downloaded to $VENDOR\" -ForegroundColor Green
Get-ChildItem -Path "$VENDOR" -Recurse -File | ForEach-Object { Write-Host "  $($_.FullName) ($('{0:N1}' -f ($_.Length/1KB)) KB)" }
