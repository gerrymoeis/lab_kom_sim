# =============================================================================
# SIMLABKOM - E2E Production Test - prepare_e2e.ps1 (host helper, opsional)
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
    [string]$BundlePath = "$PSScriptRoot",
    [string]$Repo = "$PSScriptRoot\..\tools\migrate_single_to_multi"
)

$ErrorActionPreference = "Stop"

# ---------------------------------------------------------------- Helper: baca file .env
function Read-EnvFile {
    param([string]$Path, [hashtable]$Into)
    $Into.Clear()
    if (-not (Test-Path -LiteralPath $Path)) {
        throw "file env tidak ada: $Path"
    }
    foreach ($line in Get-Content -LiteralPath $Path) {
        $t = $line.Trim()
        if ($t -eq "" -or $t.StartsWith("#")) { continue }
        if ($t -match '^([A-Za-z_][A-Za-z0-9_]*)=(.*)$') {
            $name = $Matches[1]
            $val  = $Matches[2]
            # Buang komentar inline: spasi lalu '#' (hanya bila # didahului spasi,
            # agar URL/token yang sah tidak terpotong).
            if ($val -match '^(.*?)[ \t]+#') {
                $val = $Matches[1]
            }
            $val = $val.Trim()
            if ($val.Length -ge 2 -and (($val[0] -eq '"' -and $val[-1] -eq '"') -or ($val[0] -eq "'" -and $val[-1] -eq "'"))) {
                $val = $val.Substring(1, $val.Length - 2)
            }
            $Into[$name] = $val
        }
    }
}

# Resolusi path relatif terhadap $PWD (lokasi PowerShell), BUKAN CWD .NET -
# supaya bundle/etl selalu lahir di folder yang dimaksud user.
if (-not [System.IO.Path]::IsPathRooted($BundlePath)) {
    $BundlePath = Join-Path $PWD $BundlePath
}
$BundlePath = [System.IO.Path]::GetFullPath($BundlePath)

# ---------------------------------------------------------------- 1. Build ETL linux
$etlBin = [System.IO.Path]::GetFullPath((Join-Path $BundlePath "etl"))
Write-Host "==> Build ETL (linux/amd64) dari $Repo"
$oldGOOS = $env:GOOS; $oldGOARCH = $env:GOARCH; $oldCGO = $env:CGO_ENABLED
Push-Location $Repo
try {
    $env:GOOS = "linux"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"
    & go build -o $etlBin ./...
    if ($LASTEXITCODE -ne 0) { throw "go build ETL gagal (exit $LASTEXITCODE)" }
} finally {
    $env:GOOS = $oldGOOS; $env:GOARCH = $oldGOARCH; $env:CGO_ENABLED = $oldCGO
    Pop-Location
}
Write-Host "    OK: $etlBin (linux/amd64, CGO=0)"

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
    if (-not (Test-Path -LiteralPath $src)) {
        throw "item bundle tidak ada: $src"
    }
    Copy-Item -Recurse -Force $src $staging
}
if (-not (Test-Path -LiteralPath (Join-Path $staging "etl"))) {
    throw "binary etl tidak tersalin ke staging - build ETL gagal/terlewat"
}

# keys.env: OTOMATIS diisi penuh dari poc_prototype/scripts/build_linux_release/.env.config
# (sumber nilai asli). Tidak perlu copy-paste manual di VM.
$envConfigPath = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot "..\poc_prototype\scripts\build_linux_release\.env.config"))
$cfg = @{}
Read-EnvFile -Path $envConfigPath -Into $cfg

# GITHUB_TOKEN: main (versi lama) pakai nama GITHUB_TOKEN; refactoring (versi baru) pakai
# PC_PHOTO_TOKEN. Keduanya PAT GitHub yang SAMA (GITHUB_ reserved prefix GitHub Actions).
$cfg["GITHUB_TOKEN"] = $cfg["PC_PHOTO_TOKEN"]

$requiredKeys = @("GEMINI_API_KEY", "OPENROUTER_API_KEY", "PC_PHOTO_RELEASE_URL", "PC_PHOTO_TOKEN", "GITHUB_TOKEN")
$missing = @($requiredKeys | Where-Object { -not $cfg.ContainsKey($_) -or [string]::IsNullOrWhiteSpace($cfg[$_]) })
if ($missing.Count -gt 0) {
    throw "key kosong/tidak ada di $envConfigPath : $($missing -join ', ')"
}
$keysContent = @(
    "# Auto-generated oleh prepare_e2e.ps1 dari .env.config - JANGAN commit ke git",
    "GEMINI_API_KEY=$($cfg['GEMINI_API_KEY'])",
    "OPENROUTER_API_KEY=$($cfg['OPENROUTER_API_KEY'])",
    "PC_PHOTO_RELEASE_URL=$($cfg['PC_PHOTO_RELEASE_URL'])",
    "PC_PHOTO_TOKEN=$($cfg['PC_PHOTO_TOKEN'])",
    "GITHUB_TOKEN=$($cfg['GITHUB_TOKEN'])"
) -join "`n"
$keysPath = Join-Path $staging "keys.env"
[System.IO.File]::WriteAllText($keysPath, $keysContent, (New-Object System.Text.UTF8Encoding($false)))
Write-Host "    keys.env diisi OTOMATIS dari $envConfigPath (5 key lengkap)"

# Archive bundle (path ABSOLUT supaya tar tidak menulis ke dalam staging).
$zipName = "e2e_bundle_$(Get-Date -Format yyyyMMdd_HHmmss).tar.gz"
$zipPath = [System.IO.Path]::GetFullPath((Join-Path $BundlePath $zipName))
Push-Location $staging
try {
    if (Get-Command tar -ErrorAction SilentlyContinue) {
        & tar -czf $zipPath run_e2e.sh lib config assets etl keys.env
    } else {
        # fallback: gunakan 7z jika ada
        & 7z a -ttar $zipPath run_e2e.sh lib config assets etl keys.env
        & 7z a -tgzip $zipPath
    }
    if ($LASTEXITCODE -ne 0) { throw "tar bundle gagal (exit $LASTEXITCODE)" }
} finally {
    Pop-Location
}
Write-Host "    OK: $zipPath"
if ($SSH -ne "") {
    Write-Host "    Upload: scp $zipPath ${SSH}:/tmp/"
}

# ---------------------------------------------------------------- 3. Deploy + run di VM
if ($Deploy -and $SSH -ne "") {
    Write-Host "==> Deploy ke $SSH"
    & scp $zipPath "${SSH}:/tmp/"
    if ($LASTEXITCODE -ne 0) { throw "scp gagal" }

    # Distro-agnostic: deteksi package manager (Arch= pacman, Debian/Ubuntu= apt).
    # Toolchain (git/go/curl/sqlite3/tar/pkill) wajib ada - F0 di run_e2e.sh juga memeriksa.
    $remoteCmd = "if command -v pacman >/dev/null 2>&1; then " +
                 "sudo pacman -S --noconfirm --needed git go curl sqlite tar procps-ng >/dev/null 2>&1; " +
                 "elif command -v apt-get >/dev/null 2>&1; then " +
                 "sudo apt-get install -y git golang-go curl sqlite3 tar procps >/dev/null 2>&1; fi; " +
                 "mkdir -p ~/e2e_test && tar -xzf /tmp/$zipName -C ~/e2e_test; " +
                 "cd ~/e2e_test && bash run_e2e.sh"
    Write-Host "==> SSH run run_e2e.sh (keys.env sudah terisi otomatis di bundle)"
    & ssh $SSH $remoteCmd
    if ($LASTEXITCODE -ne 0) { throw "ssh run gagal" }
}

Write-Host "==> Selesai. Di VM: cd ~/e2e_test && bash run_e2e.sh (keys.env sudah terisi)"