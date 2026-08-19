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

# -------------------------------------------------------- 6. Buat zip distribusi (mode-aware)
function New-SimlabZip {
    # Zip folder $SourceDir -> $ZipPath dengan root $RootName, nama entry FORWARD
    # SLASH (aman utk unzip di Linux; Compress-Archive PS5.1 memakai backslash,
    # tidak dipakai) + external attribute Unix (0755 binary/*.sh, 0644 lainnya)
    # agar unzip Linux menerapkan mode saat extract (tanpa chmod manual).
    param([string]$SourceDir, [string]$ZipPath, [string]$RootName)
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    if (Test-Path -LiteralPath $ZipPath) { Remove-Item -LiteralPath $ZipPath -Force }
    $zip = [System.IO.Compression.ZipFile]::Open($ZipPath, 'Create')
    try {
        Get-ChildItem -LiteralPath $SourceDir -Recurse -File | ForEach-Object {
            $rel = $_.FullName.Substring($SourceDir.Length).TrimStart('\', '/')
            $arc = ("$RootName/$rel" -replace '\\', '/')
            $entry = $zip.CreateEntry($arc, 'Optimal')
            $isExec = ($arc -eq "$RootName/app-simlab") -or $arc.EndsWith('.sh')
            $entry.ExternalAttributes = if ($isExec) { (0x81ED -shl 16) } else { (0x81A4 -shl 16) }
            $es = $entry.Open()
            try {
                $fs = [System.IO.File]::OpenRead($_.FullName)
                try { $fs.CopyTo($es) } finally { $fs.Dispose() }
            } finally { $es.Dispose() }
        }
    } finally { $zip.Dispose() }
}

function Set-ZipUnixHost {
    # .NET menulis "version made by" host = 0 (DOS) -> unzip Linux mengabaikan
    # mode unix. Patch byte host (central directory, offset+5) menjadi 3 (Unix)
    # utk SEMUA entry agar unzip Linux menerapkan external attribute (0755/0644).
    param([string]$ZipPath)
    $bytes = [System.IO.File]::ReadAllBytes($ZipPath)
    $eocd = -1
    for ($i = $bytes.Length - 22; $i -ge 0; $i--) {
        if ($bytes[$i] -eq 0x50 -and $bytes[$i + 1] -eq 0x4b -and $bytes[$i + 2] -eq 0x05 -and $bytes[$i + 3] -eq 0x06) { $eocd = $i; break }
    }
    if ($eocd -lt 0) { throw "EOCD tidak ditemukan: $ZipPath" }
    $cdOffset = [BitConverter]::ToInt32($bytes, $eocd + 16)
    $count = [BitConverter]::ToUInt16($bytes, $eocd + 10)
    $p = $cdOffset
    for ($n = 0; $n -lt $count; $n++) {
        if ($bytes[$p] -ne 0x50 -or $bytes[$p + 1] -ne 0x4b -or $bytes[$p + 2] -ne 0x01 -or $bytes[$p + 3] -ne 0x02) {
            throw "Central directory rusak di offset ${p}: $ZipPath"
        }
        $bytes[$p + 5] = 3
        $p += 46 + [BitConverter]::ToUInt16($bytes, $p + 28) + [BitConverter]::ToUInt16($bytes, $p + 30) + [BitConverter]::ToUInt16($bytes, $p + 32)
    }
    [System.IO.File]::WriteAllBytes($ZipPath, $bytes)
}

$zip = Join-Path (Split-Path -Parent $OutDir) "old_main_$ts.zip"
Write-Host "==> Buat zip distribusi (mode Unix 0755 binary / 0644 web): $zip"
New-SimlabZip -SourceDir $OutDir -ZipPath $zip -RootName "old_main"
Set-ZipUnixHost -ZipPath $zip
Add-Type -AssemblyName System.IO.Compression.FileSystem
$zr = [System.IO.Compression.ZipFile]::OpenRead($zip)
try {
    $entries = @($zr.Entries | ForEach-Object { $_.FullName })
    $okBin = $entries -contains "old_main/app-simlab"
    $okTpl = @($entries | Where-Object { $_ -like "old_main/web/templates/*" }).Count -gt 0
    $okSta = @($entries | Where-Object { $_ -like "old_main/web/static/*" }).Count -gt 0
    $binAttr = ($zr.Entries | Where-Object { $_.FullName -eq "old_main/app-simlab" }).ExternalAttributes
    if (-not ($okBin -and $okTpl -and $okSta)) {
        throw "isi zip tidak lengkap (butuh old_main/app-simlab + web/templates/* + web/static/*)"
    }
    if ($binAttr -ne (0x81ED -shl 16)) {
        throw "app-simlab di zip tidak bermode 0755 (attr=0x$($binAttr.ToString('X8')))"
    }
} finally { $zr.Dispose() }
Write-Host "    OK: zip berisi old_main/app-simlab (0755) + web/templates + web/static (forward slash)"

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
Write-Host "       app-simlab SUDAH 0755 dari zip (tanpa chmod manual)"
Write-Host "    3. ./seed_old_install.sh /opt/simlab v1 --service --replace --bin ~/Unduhan/old_main/app-simlab"