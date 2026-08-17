# =============================================================================
# SIMLABKOM - upload_vm.ps1 (host helper, Windows) - Fase F (doc 018)
#
# SATU COMMAND dari Windows:
#   1) Membangun bundle  (prepare_production.ps1, F10 640/0/0)
#   2) Upload tar.gz ke VM Linux via scp  (default 127.0.0.1:2222)
#   3) Extract + chmod di VM  (ssh)
#
# Username SSH ditanyakan interaktif (Enter = simlab); password diminta manual
# oleh scp/ssh bila perlu.
#
# Jalankan (dari mana saja):
#   powershell -ExecutionPolicy Bypass -File .\upload_vm.ps1
#   powershell -ExecutionPolicy Bypass -File .\upload_vm.ps1 -User simlab -SkipBuild
# =============================================================================
param(
    [string]$User = "",
    [string]$VMHost = "127.0.0.1",
    [int]$VMPort = 2222,
    [string]$RemoteDir = "/tmp",
    [switch]$SkipBuild
)

$ErrorActionPreference = "Stop"
$here = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $here

if (-not $SkipBuild) {
    Write-Host "==> [1/3] Build bundle"
    & powershell -ExecutionPolicy Bypass -File .\prepare_production.ps1
    if ($LASTEXITCODE -ne 0) { throw "prepare_production.ps1 gagal (exit $LASTEXITCODE)" }
}

$bundle = Get-ChildItem -Path (Join-Path $here "out\deploy_production_*.tar.gz") |
    Sort-Object LastWriteTime -Descending | Select-Object -First 1
if (-not $bundle) { throw "bundle tidak ditemukan di out/" }

if (-not $User) {
    $User = Read-Host "VM username (Enter = simlab)"
    if (-not $User) { $User = "simlab" }
}

$dest = "$User@$VMHost"
$name = $bundle.Name -replace '\.tar\.gz$', ''

Write-Host "==> [2/3] Upload $($bundle.Name) -> ${dest}:${RemoteDir}/"
& scp -P $VMPort $bundle.FullName "${dest}:${RemoteDir}/"
if ($LASTEXITCODE -ne 0) { throw "scp gagal - cek username/password/port-forward 127.0.0.1:${VMPort}" }

$remote = "cd $RemoteDir && rm -rf $name && tar xzf $bundle.Name && " +
    "chmod 600 $name/config/.env.config && " +
    "chmod +x $name/bin/* $name/deploy_production.sh $name/cleanup_production.sh $name/seed_old_install.sh $name/lib/*.sh && " +
    "echo EXTRACT_OK:$name"
Write-Host "==> [3/3] Extract + chmod di VM"
& ssh -p $VMPort $dest $remote
if ($LASTEXITCODE -ne 0) { throw "ssh extract gagal (exit $LASTEXITCODE)" }

Write-Host ""
Write-Host "SELESAI. Di VM Linux:"
Write-Host "  bundle : $RemoteDir/$name/"
Write-Host "  deploy : cd $RemoteDir/$name && sudo bash deploy_production.sh [--no-color]"
Write-Host "  seed   : cd $RemoteDir/$name && sudo bash seed_old_install.sh <LOC> [v2|v1] [--service|--start]"