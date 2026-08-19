# =============================================================================
# SIMLABKOM - build_old_main.ps1 (host helper, Windows)
#
# Membangun binary web app SIMLab LAMA (branch main) untuk "existing install"
# pada re-test VM (doc 024 R9). Output (GOOS=linux GOARCH=amd64 CGO_ENABLED=0):
#   out/old_main/app-simlab     binary server cmd/server (bukan bundle/refactoring)
#   out/old_main/web/           templates + static — DIBACA DARI DISK oleh app lama
#                               (internal/server/server.go LoadTemplates("web/templates"),
#                               versioner.New("./web/static")), wajib di samping binary.
#   out/old_main_<ts>.zip       distribusi ZIP berisi folder old_main/ (app-simlab + web/).
#                               Nama entry FORWARD SLASH (ZipArchive .NET, BUKAN
#                               Compress-Archive yang memakai backslash) — aman
#                               diekstrak di Linux dengan `unzip`.
#
# Binary dipakai bersama seed_old_install.sh --bin <path> (skenario EXISTING).
# Branch main diambil via git worktree agar tidak menyentuh tree kerja.
#
# Jalankan di PowerShell:
#   .\build_old_main.ps1                 # default: branch main
#   .\build_old_main.ps1 -Ref <ref>      # commit/branch lain (mis. deploy_linux)
#   .\build_old_main.ps1 -OutDir <dir>
# =============================================================================
param(
    [string]$Ref = "main",
    [string]$OutDir = "$PSScriptRoot\out\old_main"
)

$ErrorActionPreference = "Continue"

# ---------------------------------------------------------------- Resolusi path
if (-not [System.IO.Path]::IsPathRooted($OutDir)) { $OutDir = Join-Path $PWD $OutDir }
$OutDir = [System.IO.Path]::GetFullPath($OutDir)
$PocProto = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot "..\.."))

# ---------------------------------------------------------------- 1. Validasi prasyarat
Write-Host "==> Validasi prasyarat"
if (-not (Get-Command go -ErrorAction SilentlyContinue)) { throw "go tidak ditemukan di PATH" }
if (-not (Get-Command git -ErrorAction SilentlyContinue)) { throw "git tidak ditemukan di PATH" }
if (-not (Test-Path -LiteralPath (Join-Path $PocProto ".git"))) { throw "PocProto bukan repo git: $PocProto" }
Write-Host "    OK: go + git + repo tersedia"

# ---------------------------------------------------------------- 2. Resolve ref + worktree
$ts = Get-Date -Format yyyyMMdd_HHmmss
$tmp = Join-Path ([System.IO.Path]::GetTempPath()) "simlab_old_main_$ts"
Write-Host "==> Resolve ref '$Ref' -> commit"
$commit = (& git rev-parse --verify "${Ref}^{commit}").Trim()
if ($LASTEXITCODE -ne 0 -or -not $commit) { throw "ref tidak valid: $Ref" }
Write-Host "    commit: $commit"
Write-Host "==> Worktree (detach $commit) -> $tmp"
Push-Location $PocProto
try {
    & git worktree add --detach $tmp $commit 2>&1 | ForEach-Object { Write-Host "    $_" }
    if ($LASTEXITCODE -ne 0) { throw "git worktree add gagal ($Ref)" }

    # -------------------------------------------------------- 3. Build server linux
    Write-Host "==> Build app-simlab (cmd/server) linux amd64"
    $oldGOOS = $env:GOOS; $oldGOARCH = $env:GOARCH; $oldCGO = $env:CGO_ENABLED
    try {
        $env:GOOS = "linux"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"
        Push-Location $tmp
        try {
            & go build -ldflags="-s -w" -o app-simlab ./cmd/server/main.go 2>&1 |
                ForEach-Object { Write-Host "    $_" }
            if ($LASTEXITCODE -ne 0) { throw "go build app-simlab gagal" }
        } finally { Pop-Location }
    } finally {
        $env:GOOS = $oldGOOS; $env:GOARCH = $oldGOARCH; $env:CGO_ENABLED = $oldCGO
    }

    # -------------------------------------------------------- 4. Salin binary + web/ ke output
    New-Item -ItemType Directory -Path $OutDir -Force | Out-Null
    $binSrc = Join-Path $tmp "app-simlab"
    Copy-Item -LiteralPath $binSrc -Destination (Join-Path $OutDir "app-simlab") -Force
    $webSrc = Join-Path $tmp "web"
    if (-not (Test-Path -LiteralPath (Join-Path $webSrc "templates"))) {
        throw "web/templates tidak ada di main: $webSrc"
    }
    Copy-Item -LiteralPath $webSrc -Destination (Join-Path $OutDir "web") -Recurse -Force

    # -------------------------------------------------------- 5. Verifikasi ELF
    $binOut = Join-Path $OutDir "app-simlab"
    $bytes = [System.IO.File]::ReadAllBytes($binOut)[0..3]
    $isElf = $bytes[0] -eq 0x7F -and [System.Text.Encoding]::ASCII.GetString($bytes[1..3]) -eq "ELF"
    if (-not $isElf) { throw "output bukan ELF linux: $binOut" }
} finally {
    Write-Host "==> Bersihkan worktree"
    Push-Location $PocProto
    try { & git worktree remove $tmp --force 2>&1 | Out-Null } catch {}
    Pop-Location
}

# -------------------------------------------------------- 6. Buat zip distribusi
function New-SimlabZip {
    # Zip folder $SourceDir -> $ZipPath dengan root $RootName dan nama entry
    # FORWARD SLASH (aman utk unzip di Linux). Compress-Archive PS5.1 memakai
    # backslash di nama entry -> TIDAK aman lintas-platform, tidak dipakai.
    param([string]$SourceDir, [string]$ZipPath, [string]$RootName)
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    if (Test-Path -LiteralPath $ZipPath) { Remove-Item -LiteralPath $ZipPath -Force }
    $zip = [System.IO.Compression.ZipFile]::Open($ZipPath, 'Create')
    try {
        Get-ChildItem -LiteralPath $SourceDir -Recurse -File | ForEach-Object {
            $rel = $_.FullName.Substring($SourceDir.Length).TrimStart('\', '/')
            $entry = $zip.CreateEntry(("$RootName/$rel" -replace '\\', '/'), 'Optimal')
            $es = $entry.Open()
            try {
                $fs = [System.IO.File]::OpenRead($_.FullName)
                try { $fs.CopyTo($es) } finally { $fs.Dispose() }
            } finally { $es.Dispose() }
        }
    } finally { $zip.Dispose() }
}

$zip = Join-Path (Split-Path -Parent $OutDir) "old_main_$ts.zip"
Write-Host "==> Buat zip distribusi: $zip"
New-SimlabZip -SourceDir $OutDir -ZipPath $zip -RootName "old_main"
Add-Type -AssemblyName System.IO.Compression.FileSystem
$zr = [System.IO.Compression.ZipFile]::OpenRead($zip)
try {
    $entries = @($zr.Entries | ForEach-Object { $_.FullName })
    $okBin = $entries -contains "old_main/app-simlab"
    $okTpl = @($entries | Where-Object { $_ -like "old_main/web/templates/*" }).Count -gt 0
    $okSta = @($entries | Where-Object { $_ -like "old_main/web/static/*" }).Count -gt 0
    if (-not ($okBin -and $okTpl -and $okSta)) {
        throw "isi zip tidak lengkap (butuh old_main/app-simlab + web/templates/* + web/static/*)"
    }
} finally { $zr.Dispose() }
Write-Host "    OK: zip berisi old_main/app-simlab + web/templates + web/static (forward slash)"

$binOut = Join-Path $OutDir "app-simlab"
Write-Host ""
Write-Host "OLD MAIN BUILD OK:"
Write-Host "    binary : $binOut ($([math]::Round((Get-Item -LiteralPath $binOut).Length/1MB,1)) MB, ELF linux amd64)"
Write-Host "    web/   : $(Join-Path $OutDir 'web')"
Write-Host "    zip    : $zip"
Write-Host ""
Write-Host "Langkah berikut (skenario EXISTING):"
Write-Host "    1. scp/copy $zip ke VM (mis. ~/Unduhan/)"
Write-Host "    2. extract di VM: cd ~/Unduhan && unzip -q old_main_$ts.zip"
Write-Host "       (bila unzip belum ada: sudo apt-get install -y unzip)"
Write-Host "    3. ./seed_old_install.sh /opt/simlab v1 --service --replace --bin ~/Unduhan/old_main/app-simlab"