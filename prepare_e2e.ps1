# =============================================================================
# SIMLABKOM — E2E Production Test — prepare_e2e.ps1 (host helper, opsional)
#
# Menyiapkan bundle yang di-upload ke VM Linux untuk menjalankan run_e2e.sh:
#   1. Build binary ETL versi LINUX dari tools/migrate_single_to_multi
#   2. Build archive bundle (run_e2e.sh, lib/, config/, assets/, etl binary,
#      keys.env) untuk di-scp ke VM
#   3. (Opsional) Jalankan run_e2e.sh di VM via SSH jika VM_SSH diisi
#
# Catatan: script utama tetap bash (run_e2e.sh) yang berjalan di VM Linux.
# Helper ini hanya mempermudah host Windows. Jalankan di PowerShell:
#   .\prepare_e2e.ps1                 # bundle saja
#   .\prepare_e2e.ps1 -SSH ubuntu@1.2.3.4 -Deploy   # build + scp + ssh run
# =============================================================================
param(
    [string]$SSH = "",
    [switch]$Deploy,
    [string]$BundlePath = ".",
    [string]$Repo = "$PSScriptRoot\..\tools\migrate_single_to_multi"
)

$ErrorActionPreference = "Stop"

# ---------------------------------------------------------------- 1. Build ETL linux
Write-Host "==> Build ETL (linux/amd64) dari $Repo"
$etlBin = Join-Path $BundlePath "etl"
Push-Location $Repo
try {
    & go build -o $etlBin ./...
    if ($LASTEXITCODE -ne 0) { throw "go build ETL gagal (exit $LASTEXITCODE)" }
} finally {
    Pop-Location
}
Write-Host "    OK: $etlBin"

# ---------------------------------------------------------------- 2. Siapkan bundle
Write-Host "==> Susun bundle staging"
$staging = Join-Path $env:TEMP "e2e_bundle_$(Get-Date -Format yyyyMMdd_HHmmss)"
New-Item -ItemType Directory -Path $staging -Force | Out-Null

# Struktur bundle sama dengan E2E_ROOT di VM (tetapi dijalankan di HOME di VM).
$copyItems = @(
    (Join-Path $PSScriptRoot "run_e2e.sh"),
    (Join-Path $PSScriptRoot "lib"),
    (Join-Path $PSScriptRoot "config"),
    (Join-Path $PSScriptRoot "assets"),
    $etlBin
)
foreach ($src in $copyItems) {
    if (Test-Path -LiteralPath $src) {
        Copy-Item -Recurse -Force $src $staging
    }
}

# keys.env: dari keys.env.example lalu isi nilai asli (JANGAN commit keys.env).
$keysExample = Join-Path $PSScriptRoot "config\keys.env.example"
$keysDest = Join-Path $staging "keys.env"
Copy-Item $keysExample $keysDest
Write-Host "    PERHATIAN: isi $keysDest dengan API key ASLI sebelum upload"

# Arsitek zip (posix path untuk scp).
$zipName = "e2e_bundle_$(Get-Date -Format yyyyMMdd_HHmmss).tar.gz"
$zipPath = Join-Path $BundlePath $zipName
Push-Location $staging
try {
    if (Get-Command tar -ErrorAction SilentlyContinue) {
        & tar -czf $zipPath run_e2e.sh lib config assets etl keys.env
    } else {
        # fallback: gunakan 7z jika ada
        & 7z a -ttar $zipPath run_e2e.sh lib config assets etl keys.env
        & 7z a -tgzip $zipPath
    }
} finally {
    Pop-Location
}
Write-Host "    OK: $zipPath"
Write-Host "    Upload: scp $zipPath $SSH:/tmp/"

# ---------------------------------------------------------------- 3. Deploy + run di VM
if ($Deploy -and $SSH -ne "") {
    Write-Host "==> Deploy ke $SSH"
    & scp $zipPath "${SSH}:/tmp/"
    if ($LASTEXITCODE -ne 0) { throw "scp gagal" }

    $remoteCmd = "sudo apt-get install -y sqlite3 >/dev/null 2>&1; " +
                 "mkdir -p ~/e2e_test && tar -xzf /tmp/$zipName -C ~/e2e_test; " +
                 "cd ~/e2e_test && bash run_e2e.sh"
    Write-Host "==> SSH run run_e2e.sh (interaktif, butuh keys.env terisi di VM)"
    & ssh $SSH $remoteCmd
    if ($LASTEXITCODE -ne 0) { throw "ssh run gagal" }
}

Write-Host "==> Selesai. Di VM: cd ~/e2e_test && bash run_e2e.sh"