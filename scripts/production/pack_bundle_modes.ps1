# =============================================================================
# SIMLABKOM - Production Deploy - pack_bundle_modes.ps1 (host helper, Windows)
#
# Membangun bundle .tar.gz dari folder staging DENGAN MODE (permission) yang
# benar sehingga langsung bisa dipakai di Linux TANPA chmod manual (requirement
# user 19 Agu 2026). Windows tidak punya executable-bit, maka mode di-embed ke
# tar lewat GNU tar (Git for Windows, --mode per-pass):
#   pass 1 (0755): script *.sh, lib/*.sh, bin/*, test-runner/test-bin/*.test + direktori
#   pass 2 (0644): file data lain
#   pass 3 (0600): config/.env.config (sekret API key)
# Member diberi prefix $RootName (mis. deploy_production_<ts>) sehingga extract
# menghasilkan folder, sama seperti perilaku tar lama. Owner/group di-embed 0:0
# (deterministik). Ekstraksi sebagai non-root tetap menghasilkan file milik user
# extractor; mode ikut terekstrak.
#
# Fallback bila GNU tar (Git) tidak ditemukan: pakai `tar`/7z bawaan (tanpa
# executable bit) + peringatan.
#
# Jalankan:
#   .\pack_bundle_modes.ps1 -Staging <dir bundle staging> -OutTarGz <file.tar.gz>
# =============================================================================
param(
    [Parameter(Mandatory = $true)]
    [string]$Staging,
    [Parameter(Mandatory = $true)]
    [string]$OutTarGz
)

$ErrorActionPreference = "Stop"

# ---------------------------------------------------------------- Resolusi path
$Staging = [System.IO.Path]::GetFullPath($Staging)
$OutTarGz = [System.IO.Path]::GetFullPath($OutTarGz)
$RootName = Split-Path -Leaf $Staging
$OutDir = Split-Path -Parent $OutTarGz
if (-not (Test-Path -LiteralPath $Staging)) { throw "Staging tidak ada: $Staging" }
if (-not (Test-Path -LiteralPath $OutDir)) { New-Item -ItemType Directory -Path $OutDir -Force | Out-Null }

# ---------------------------------------------------------------- Cari GNU tar (Git for Windows)
$gitExe = (Get-Command git -ErrorAction SilentlyContinue).Source
$gnuTar = $null; $gnuGzip = $null
if ($gitExe) {
    $gitRoot = Split-Path (Split-Path $gitExe -Parent) -Parent
    $candTar = Join-Path $gitRoot "usr\bin\tar.exe"
    $candGzip = Join-Path $gitRoot "usr\bin\gzip.exe"
    if ((Test-Path -LiteralPath $candTar) -and (Test-Path -LiteralPath $candGzip)) {
        $gnuTar = $candTar; $gnuGzip = $candGzip
    }
}

function ConvertTo-PosixPath([string]$p) {
    $p = $p.Replace('\', '/')
    return ($p -replace '^([A-Za-z]):/', '/$1/')
}
function Get-RelPath([string]$full, [string]$base) {
    return $full.Substring($base.Length).TrimStart('\', '/').Replace('\', '/')
}

if (-not $gnuTar) {
    Write-Host "    WARN: GNU tar (Git for Windows) tidak ditemukan - fallback tar bawaan (executable bit TIDAK di-embed)."
    if (Test-Path -LiteralPath $OutTarGz) { Remove-Item -LiteralPath $OutTarGz -Force }
    Push-Location $OutDir
    try {
        if (Get-Command tar -ErrorAction SilentlyContinue) {
            & tar -czf $OutTarGz $RootName
        } else {
            & 7z a -ttar $OutTarGz $RootName
            & 7z a -tgzip $OutTarGz
        }
        if ($LASTEXITCODE -ne 0) { throw "tar bundle gagal (exit $LASTEXITCODE)" }
    } finally { Pop-Location }
    Write-Host "    OK (fallback): $OutTarGz"
    return
}

# ---------------------------------------------------------------- Klasifikasi file
$stagingFiles = Get-ChildItem -LiteralPath $Staging -Recurse -File
$stagingDirs = Get-ChildItem -LiteralPath $Staging -Recurse -Directory
$execList = @(); $dataList = @(); $envList = @()
foreach ($f in $stagingFiles) {
    $r = Get-RelPath $f.FullName $Staging
    if ($r -match '^(deploy_production\.sh|run_deploy\.sh|seed_old_install\.sh)$' -or
        $r -match '^lib/[^/]+\.sh$' -or $r -match '^bin/[^/]+$' -or $r -match '^test-runner/test-bin/[^/]+\.test$') {
        $execList += $r
    } elseif ($r -eq 'config/.env.config') {
        $envList += $r
    } else {
        $dataList += $r
    }
}
$dirList = @($stagingDirs | ForEach-Object { Get-RelPath $_.FullName $Staging })

# ---------------------------------------------------------------- Tulis daftar member (LF, prefix RootName)
$utf8 = New-Object System.Text.UTF8Encoding($false)
$l1 = Join-Path $OutDir "$RootName.list1.txt"
$l2 = Join-Path $OutDir "$RootName.list2.txt"
$l3 = Join-Path $OutDir "$RootName.list3.txt"
[System.IO.File]::WriteAllText($l1, (($dirList + $execList | ForEach-Object { "$RootName/$_" }) -join "`n") + "`n", $utf8)
[System.IO.File]::WriteAllText($l2, (($dataList | ForEach-Object { "$RootName/$_" }) -join "`n") + "`n", $utf8)
[System.IO.File]::WriteAllText($l3, (($envList | ForEach-Object { "$RootName/$_" }) -join "`n") + "`n", $utf8)

# ---------------------------------------------------------------- Bangun tar (3 pass mode) + gzip
$tarPath = $OutTarGz -replace '\.tar\.gz$', '.tar'
if (Test-Path -LiteralPath $tarPath) { Remove-Item -LiteralPath $tarPath -Force }
$pxOut = ConvertTo-PosixPath (Split-Path -Parent $Staging)
$pxTar = ConvertTo-PosixPath $tarPath
$common = @('--no-recursion', '--owner=0', '--group=0', '-C', $pxOut)
& $gnuTar -cf $pxTar @common --mode=755 -T (ConvertTo-PosixPath $l1)
if ($LASTEXITCODE -ne 0) { throw "tar pass 1 gagal (exit $LASTEXITCODE)" }
if ($dataList.Count -gt 0) { & $gnuTar -rf $pxTar @common --mode=644 -T (ConvertTo-PosixPath $l2) }
if ($LASTEXITCODE -ne 0) { throw "tar pass 2 gagal (exit $LASTEXITCODE)" }
if ($envList.Count -gt 0) { & $gnuTar -rf $pxTar @common --mode=600 -T (ConvertTo-PosixPath $l3) }
if ($LASTEXITCODE -ne 0) { throw "tar pass 3 gagal (exit $LASTEXITCODE)" }
if (-not (Test-Path -LiteralPath $tarPath)) { throw "tar tidak dihasilkan: $tarPath" }
& $gnuGzip -9 -f $pxTar
if ($LASTEXITCODE -ne 0) { throw "gzip gagal (exit $LASTEXITCODE)" }
Remove-Item -LiteralPath $l1, $l2, $l3 -Force -ErrorAction SilentlyContinue
Write-Host "    OK: $OutTarGz (script/bin 0755, data 0644, config/.env.config 0600, owner 0:0)"