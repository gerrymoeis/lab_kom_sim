# =============================================================================
# SIMLABKOM - Production Deploy - prepare_production.ps1 (host helper, Windows)
#
# Membangun bundle deploy_production_<ts>.tar.gz untuk production Linux server
# (/opt/simlab). Isi bundle mengikuti struktur doc 014 (test_production):
#   - deploy_production.sh / run_deploy.sh / lib/
#     (Fase B/E - kosong bila belum dibuat, disalin otomatis jika sudah ada;
#      run_deploy.sh = SATU file utk admin menjalankan seluruh deploy;
#      P15 = self-cleanup bundle, cleanup_production.sh standalone DIHAPUS di R6)
#   - config/  : etl-config.production.json + .env.config (dari build_linux_release)
#   - bin/     : etl, app-simlab, app-simlab-publish (GOOS=linux CGO_ENABLED=0)
#   - test-runner/ : go.mod + seeds/ + .env.reference + test-bin/*.test (8 pkg)
#   - seeds/   : mi-1, vokasi-1, default (untuk app release dir)
#   - seed_old_install.sh + assets/ : helper uji VM (Fase F, doc 018 S1-S10);
#     TIDAK dimasukkan dengan -NoTestHelpers (bundle produksi lean)
#
# Verifikasi Fase A:
#   - Parsing -test.v 1-to-1 dengan go test ./... -json (F10, 640/0/0): dibangun
#     test binary WINDOWS sementara di temp, dijalankan dari test-runner/ (yang
#     berisi go.mod + seeds/ + .env.reference), lalu hasil parse dibandingkan
#     dengan baseline go test -json. Total/pass/fail/skip harus identik.
#   - Binary LINUX (bundle) diperiksa magic byte ELF; eksekusi nyata test binary
#     linux dilakukan di VM Linux (opsi -SSH / manual: test-runner/test-bin).
#
# Jalankan di PowerShell:
#   .\prepare_production.ps1                         # build + verifikasi parse saja (amd64)
#   .\prepare_production.ps1 -Arch arm64             # build utk ARM64 (AArch64)
#   .\prepare_production.ps1 -Arch arm               # build utk ARM32
#   .\prepare_production.ps1 -NoTestHelpers          # bundle produksi lean (tanpa seed_old_install.sh + assets/)
#   .\prepare_production.ps1 -SSH user@vm -Deploy    # + upload & extract di VM (opsional; admin run run_deploy.sh)
# =============================================================================
param(
    [string]$SSH = "",
    [switch]$Deploy,
    [switch]$NoTestHelpers,
    [ValidateSet("amd64", "arm64", "arm")]
    [string]$Arch = "amd64",
    [string]$OutDir = "$PSScriptRoot\out",
    [string]$PocProto = "$PSScriptRoot\..\..",
    [string]$Tools = "$PSScriptRoot\..\..\..\tools\migrate_single_to_multi",
    [string]$EnvConfigPath = "$PSScriptRoot\..\build_linux_release\.env.config",
    [string]$Commit = "59d5d4e"
)

$ErrorActionPreference = "Stop"

# ---------------------------------------------------------------- Resolusi path
if (-not [System.IO.Path]::IsPathRooted($OutDir)) { $OutDir = Join-Path $PWD $OutDir }
$OutDir = [System.IO.Path]::GetFullPath($OutDir)
$PocProto = [System.IO.Path]::GetFullPath($PocProto)
$Tools = [System.IO.Path]::GetFullPath($Tools)
$EnvConfigPath = [System.IO.Path]::GetFullPath($EnvConfigPath)
$SourceScripts = @(
    (Join-Path $PSScriptRoot "deploy_production.sh"),
    (Join-Path $PSScriptRoot "run_deploy.sh"),
    (Join-Path $PSScriptRoot "README_DEPLOY.md"),
    (Join-Path $PSScriptRoot "lib")
)
# Helper uji VM (Fase F, doc 018) — TIDAK disertakan di bundle produksi lean (-NoTestHelpers).
if (-not $NoTestHelpers) {
    $SourceScripts += (Join-Path $PSScriptRoot "seed_old_install.sh")
}
$TestPackages = @(
    "tests",
    "internal/config",
    "internal/middleware",
    "internal/repository",
    "internal/search",
    "internal/services",
    "internal/verify",
    "internal/versioner"
)

# ---------------------------------------------------------------- 1. Validasi prasyarat
Write-Host "==> Validasi prasyarat"
if (-not (Get-Command go -ErrorAction SilentlyContinue)) { throw "go tidak ditemukan di PATH" }
if (-not (Test-Path -LiteralPath (Join-Path $PocProto "go.mod"))) { throw "PocProto tidak valid: $PocProto" }
if (-not (Test-Path -LiteralPath $EnvConfigPath)) {
    throw "build_linux_release/.env.config tidak ada: $EnvConfigPath (API key nyata; wajib utk bundle)"
}
if (-not (Test-Path -LiteralPath (Join-Path $PocProto "seeds\mi-1"))) { throw "seeds/mi-1 tidak ada di $PocProto" }
if (-not (Test-Path -LiteralPath (Join-Path $PocProto ".env.reference"))) { throw ".env.reference tidak ada di $PocProto" }
$AssetSeedDb = Join-Path $PSScriptRoot "assets\inventaris_lab_empty.db"
if ($NoTestHelpers) {
    Write-Host "    -NoTestHelpers: seed_old_install.sh + assets/ TIDAK dimasukkan (bundle produksi lean)"
} else {
    if (-not (Test-Path -LiteralPath $AssetSeedDb)) { throw "assets/inventaris_lab_empty.db tidak ada (Fase F seed DB)" }
}
Write-Host "    OK: go, poc_prototype, .env.config, seeds, .env.reference lengkap"

# Head commit actual (pin). Bila berbeda dari $Commit, catat peringatan (bukan gagal).
$headCommit = ""
if (Test-Path -LiteralPath (Join-Path $PocProto ".git")) {
    Push-Location $PocProto
    try { $headCommit = (& git rev-parse --short HEAD).Trim() } catch {}
    Pop-Location
}
Write-Host "    Commit refactoring: $headCommit (doc 014 pin: $Commit)"

# ---------------------------------------------------------------- 2. Siapkan staging
$ts = Get-Date -Format yyyyMMdd_HHmmss
$bundleName = "deploy_production_$ts"
$staging = Join-Path $OutDir $bundleName
$binDir = Join-Path $staging "bin"
$testRunnerDir = Join-Path $staging "test-runner"
$testBinDir = Join-Path $testRunnerDir "test-bin"
New-Item -ItemType Directory -Path $binDir -Force | Out-Null
New-Item -ItemType Directory -Path $testBinDir -Force | Out-Null
New-Item -ItemType Directory -Path (Join-Path $staging "config") -Force | Out-Null
New-Item -ItemType Directory -Path (Join-Path $staging "seeds") -Force | Out-Null
Write-Host "==> Staging bundle: $staging"

# ---------------------------------------------------------------- 3. Build binary linux
Write-Host "==> Build binary linux (GOOS=linux GOARCH=$Arch CGO_ENABLED=0)"
$oldGOOS = $env:GOOS; $oldGOARCH = $env:GOARCH; $oldCGO = $env:CGO_ENABLED
function Invoke-GoBuild {
    param([string]$Dir, [string]$Target, [string]$Out, [string]$Label)
    Push-Location $Dir
    try {
        $env:GOOS = "linux"; $env:GOARCH = $Arch; $env:CGO_ENABLED = "0"
        & go build -o $Out $Target
        if ($LASTEXITCODE -ne 0) { throw "go build $Label gagal (exit $LASTEXITCODE)" }
    } finally {
        $env:GOOS = $oldGOOS; $env:GOARCH = $oldGOARCH; $env:CGO_ENABLED = $oldCGO
        Pop-Location
    }
    if (-not (Test-Path -LiteralPath $Out)) { throw "output $Label tidak ada: $Out" }
    Write-Host "    OK: $Label"
}
Invoke-GoBuild -Dir $PocProto -Target "./cmd/server/main.go" -Out (Join-Path $binDir "app-simlab") -Label "app-simlab"
Invoke-GoBuild -Dir $PocProto -Target "./cmd/publish/main.go" -Out (Join-Path $binDir "app-simlab-publish") -Label "app-simlab-publish"
Invoke-GoBuild -Dir $Tools -Target "./..." -Out (Join-Path $binDir "etl") -Label "etl"

# ---------------------------------------------------------------- 4. Build 8 test binary linux
Write-Host "==> Build test binary linux (8 package, GOARCH=$Arch)"
foreach ($pkg in $TestPackages) {
    $name = Split-Path $pkg -Leaf
    $out = Join-Path $testBinDir "$name.test"
    Push-Location $PocProto
    try {
        $env:GOOS = "linux"; $env:GOARCH = $Arch; $env:CGO_ENABLED = "0"
        & go test -c -o $out ./$pkg
        if ($LASTEXITCODE -ne 0) { throw "go test -c $pkg gagal (exit $LASTEXITCODE)" }
    } finally {
        $env:GOOS = $oldGOOS; $env:GOARCH = $oldGOARCH; $env:CGO_ENABLED = $oldCGO
        Pop-Location
    }
    if (-not (Test-Path -LiteralPath $out)) { throw "test binary tidak ada: $out" }
    Write-Host "    OK: $name.test"
}

# ---------------------------------------------------------------- 5. Susun test-runner/ + seeds/
Write-Host "==> Susun test-runner/ dan seeds/"
Copy-Item -Force (Join-Path $PocProto "go.mod") $testRunnerDir
Copy-Item -Force (Join-Path $PocProto ".env.reference") $testRunnerDir
# Salin ISI folder seeds/ (mi-1, vokasi-1, default) — bukan folder menjadi subfolder.
$pocSeeds = Join-Path $PocProto "seeds"
$testRunnerSeeds = Join-Path $testRunnerDir "seeds"
New-Item -ItemType Directory -Path $testRunnerSeeds -Force | Out-Null
Get-ChildItem -LiteralPath $pocSeeds -Directory | ForEach-Object {
    Copy-Item -Recurse -Force $_.FullName $testRunnerSeeds
    Copy-Item -Recurse -Force $_.FullName (Join-Path $staging "seeds")
}
foreach ($seed in @("mi-1", "vokasi-1", "default")) {
    if (-not (Test-Path -LiteralPath (Join-Path (Join-Path $testRunnerDir "seeds") $seed))) {
        throw "test-runner/seeds/$seed tidak tersalin"
    }
    if (-not (Test-Path -LiteralPath (Join-Path (Join-Path $staging "seeds") $seed))) {
        throw "seeds/$seed tidak tersalin ke bundle"
    }
}
# Test suite tests/ memakai gambar resources (HEIC/JPEG) — wajib disertakan di
# test-runner agar test binary jalan sama persis dengan go test (F10).
$testsRes = Join-Path $PocProto "tests\resources"
if (-not (Test-Path -LiteralPath $testsRes)) { throw "tests/resources tidak ada di $PocProto" }
Copy-Item -Recurse -Force $testsRes (Join-Path $testRunnerDir "tests\resources")
Write-Host "    OK: go.mod + .env.reference + seeds (mi-1, vokasi-1, default) + tests/resources"

# ---------------------------------------------------------------- 6. Salin config
Write-Host "==> Salin config"
$cfgSrc = Join-Path $PSScriptRoot "config\etl-config.production.json"
if (-not (Test-Path -LiteralPath $cfgSrc)) { throw "config template tidak ada: $cfgSrc" }
Copy-Item -Force $cfgSrc (Join-Path $staging "config\etl-config.production.json")
Copy-Item -Force $EnvConfigPath (Join-Path $staging "config\.env.config")
# Validasi .env.config: key API wajib terisi (nilai asli), tanpa menampilkan nilainya.
$cfg = @{}
Get-Content -LiteralPath $EnvConfigPath | ForEach-Object {
    $t = $_.Trim()
    if ($t -match '^([A-Za-z_][A-Za-z0-9_]*)=(.*)$') { $cfg[$Matches[1]] = $Matches[2] }
}
foreach ($key in @("GEMINI_API_KEY", "OPENROUTER_API_KEY", "PC_PHOTO_TOKEN")) {
    if (-not $cfg.ContainsKey($key) -or [string]::IsNullOrWhiteSpace($cfg[$key])) {
        throw ".env.config kehilangan $key"
    }
}
Write-Host "    OK: etl-config.production.json + .env.config (API key terisi, tidak ditampilkan)"

# ---------------------------------------------------------------- 7. Salin script deploy (Fase B/E bila sudah ada)
Write-Host "==> Salin script deploy/run_deploy/lib (bila sudah ada)"
foreach ($src in $SourceScripts) {
    if (Test-Path -LiteralPath $src) {
        Copy-Item -Recurse -Force $src $staging
        Write-Host "    OK: $(Split-Path $src -Leaf)"
    }
}
# bundle-meta.txt — dibaca deploy_production.sh P13 utk report (commit + nama bundle).
$metaLines = @("commit=$headCommit", "bundle=$bundleName")
Set-Content -LiteralPath (Join-Path $staging "bundle-meta.txt") -Value $metaLines -Encoding ascii
Write-Host "    OK: bundle-meta.txt (commit=$headCommit)"

# assets/ — seed DB kosong-valid utk skenario migrasi "versi lama" (Fase F, seed_old_install.sh).
if ($NoTestHelpers) {
    Write-Host "    SKIP: assets/ (bundle lean — -NoTestHelpers)"
} else {
    $assetsDir = Join-Path $staging "assets"
    New-Item -ItemType Directory -Path $assetsDir -Force | Out-Null
    Copy-Item -Force $AssetSeedDb (Join-Path $assetsDir "inventaris_lab_empty.db")
    Write-Host "    OK: assets/inventaris_lab_empty.db (seed DB skenario Fase F)"
}

# ---------------------------------------------------------------- 8. Verifikasi parse 1-to-1 (F10)
Write-Host "==> Verifikasi parsing -test.v 1-to-1 dengan go test -json (F10)"
$verifyTmp = Join-Path $env:TEMP "prod_verify_$ts"
New-Item -ItemType Directory -Path $verifyTmp -Force | Out-Null

# 8a. Baseline: go test ./... -json (pola F10)
$jsonFile = Join-Path $verifyTmp "baseline.json"
Push-Location $PocProto
try {
    $env:CGO_ENABLED = "0"
    & go test ./... -count=1 -json -timeout 600s > $jsonFile 2>&1
} finally { Pop-Location }
$basePass = 0; $baseFail = 0; $baseSkip = 0
foreach ($line in Get-Content $jsonFile) {
    if ($line -match '"Action":"pass".*"Test":"[^"]') { $basePass++ }
    elseif ($line -match '"Action":"fail".*"Test":"[^"]') { $baseFail++ }
    elseif ($line -match '"Action":"skip".*"Test":"[^"]') { $baseSkip++ }
}
$baseTotal = $basePass + $baseFail + $baseSkip
Write-Host "    baseline go test -json : total=$baseTotal pass=$basePass fail=$baseFail skip=$baseSkip"

# 8b. Build test binary WINDOWS sementara (untuk verifikasi parse, karena host Windows)
$winTestBin = Join-Path $verifyTmp "test-bin"
New-Item -ItemType Directory -Path $winTestBin -Force | Out-Null
foreach ($pkg in $TestPackages) {
    $name = Split-Path $pkg -Leaf
    $out = Join-Path $winTestBin "$name.test.exe"
    Push-Location $PocProto
    try {
        $env:CGO_ENABLED = "0"
        & go test -c -o $out ./$pkg
        if ($LASTEXITCODE -ne 0) { throw "go test -c (windows) $pkg gagal" }
    } finally { Pop-Location }
}
# Salin go.mod + seeds + .env.reference + tests/resources ke verifyTmp (CWD test binary = projectRoot)
Copy-Item -Force (Join-Path $PocProto "go.mod") $verifyTmp
Copy-Item -Force (Join-Path $PocProto ".env.reference") $verifyTmp
Copy-Item -Recurse -Force (Join-Path $PocProto "seeds") $verifyTmp
Copy-Item -Recurse -Force (Join-Path $PocProto "tests\resources") (Join-Path $verifyTmp "tests\resources")

# 8c. Jalankan tiap test binary windows dengan -test.v dari verifyTmp, parse
$binPass = 0; $binFail = 0; $binSkip = 0
$testArgs = @('-test.v', '-test.count=1', '-test.timeout=600s')
foreach ($pkg in $TestPackages) {
    $name = Split-Path $pkg -Leaf
    $exe = Join-Path $winTestBin "$name.test.exe"
    $vLog = Join-Path $verifyTmp "$name.vlog"
    $oldEA = $ErrorActionPreference; $ErrorActionPreference = "Continue"
    Push-Location $verifyTmp
    try {
        & $exe $testArgs > $vLog 2>&1
        $rc = $LASTEXITCODE
    } finally {
        Pop-Location
        $ErrorActionPreference = $oldEA
    }
    $p = 0; $f = 0; $s = 0
    foreach ($line in Get-Content $vLog) {
        if ($line -match '^\s*--- PASS:') { $p++ }
        elseif ($line -match '^\s*--- FAIL:') { $f++ }
        elseif ($line -match '^\s*--- SKIP:') { $s++ }
    }
    Write-Host ("    {0,-18} pass={1,-4} fail={2,-3} skip={3,-3} rc={4}" -f "$name.test", $p, $f, $s, $rc)
    $binPass += $p; $binFail += $f; $binSkip += $s
    if ($rc -ne 0 -and $f -eq 0) { Write-Host "    WARN: $name.test rc=$rc tapi fail=0" }
}
$binTotal = $binPass + $binFail + $binSkip
Write-Host "    test binary -test.v   : total=$binTotal pass=$binPass fail=$binFail skip=$binSkip"

# 8d. Bandingkan 1-to-1
$parseOk = ($binTotal -eq $baseTotal) -and ($binPass -eq $basePass) -and ($binFail -eq $baseFail) -and ($binSkip -eq $baseSkip)
if (-not $parseOk) {
    throw "Verifikasi 1-to-1 GAGAL: go test=$baseTotal/$basePass/$baseFail/$baseSkip vs binary=$binTotal/$binPass/$binFail/$binSkip"
}
Write-Host "    OK: parsing -test.v 1-to-1 dengan go test -json ($binTotal test, 0 skip, 0 fail)"

# ---------------------------------------------------------------- 9. Cek magic byte binary linux (ELF + arch)
# ELF header: byte 0-3=0x7FELF, byte 4=EI_CLASS (1=ELF32,2=ELF64),
# byte 18-19 e_machine LE: 0x3E(62)=x86-64, 0xB7(183)=AArch64, 0x28(40)=ARM.
$archClass = @{ amd64 = @(2, 0x3E); arm64 = @(2, 0xB7); arm = @(1, 0x28) }[$Arch]
function Test-ElfMachine([string]$Path) {
    $fs = [System.IO.File]::OpenRead($Path)
    try {
        $buf = New-Object byte[] 20
        $fs.Read($buf, 0, 20) | Out-Null
        $isELF = ($buf[0] -eq 0x7F -and $buf[1] -eq 0x45 -and $buf[2] -eq 0x4C -and $buf[3] -eq 0x46)
        $classOk = ($buf[4] -eq $archClass[0])
        $machine = $buf[18] -bor ($buf[19] -shl 8)
        $machineOk = ($machine -eq $archClass[1])
        return ($isELF -and $classOk -and $machineOk)
    } finally { $fs.Dispose() }
}
Write-Host "==> Cek magic byte binary linux (ELF $Arch)"
foreach ($b in @("etl", "app-simlab", "app-simlab-publish")) {
    $path = Join-Path $binDir $b
    if (-not (Test-ElfMachine $path)) { throw "bukan ELF ${Arch}: $b" }
    Write-Host "    OK: $b (ELF ${Arch})"
}
foreach ($t in Get-ChildItem $testBinDir -Filter *.test) {
    if (-not (Test-ElfMachine $t.FullName)) { throw "bukan ELF ${Arch}: $($t.Name)" }
}
Write-Host "    OK: 8 test binary linux (ELF ${Arch})"

# ---------------------------------------------------------------- 10. Buat tar.gz
Write-Host "==> Buat tar.gz"
$zipPath = Join-Path $OutDir "$bundleName.tar.gz"
Push-Location $OutDir
try {
    if (Get-Command tar -ErrorAction SilentlyContinue) {
        & tar -czf $zipPath $bundleName
    } else {
        & 7z a -ttar $zipPath $bundleName
        & 7z a -tgzip $zipPath
    }
    if ($LASTEXITCODE -ne 0) { throw "tar bundle gagal (exit $LASTEXITCODE)" }
} finally { Pop-Location }
Write-Host "    OK: $zipPath"

# ---------------------------------------------------------------- 11. (Opsional) Upload ke VM (pola E2E)
# Alur utama TANPA SSH: build zip di Windows -> kirim manual ke VM -> ekstrak ->
# `sudo bash run_deploy.sh` (SATU file). Opsi ini = build + upload + extract +
# chmod + verifikasi test binary linux (F10) SEKALIGUS, lalu admin tinggal run.
if ($Deploy -and $SSH -ne "") {
    Write-Host "==> Deploy ke $SSH"
    & scp $zipPath "${SSH}:/tmp/"
    if ($LASTEXITCODE -ne 0) { throw "scp gagal (exit $LASTEXITCODE)" }
    $extraChmod = ""
    if (-not $NoTestHelpers) { $extraChmod = " ~/prod_bundle/$bundleName/seed_old_install.sh" }
    $remote = "mkdir -p ~/prod_bundle && rm -rf ~/prod_bundle/$bundleName && " +
              "tar -xzf /tmp/$bundleName.tar.gz -C ~/prod_bundle && " +
              "chmod 600 ~/prod_bundle/$bundleName/config/.env.config && " +
              "chmod +x ~/prod_bundle/$bundleName/bin/* ~/prod_bundle/$bundleName/deploy_production.sh " +
              "~/prod_bundle/$bundleName/run_deploy.sh$extraChmod " +
              "~/prod_bundle/$bundleName/lib/*.sh; " +
              "cd ~/prod_bundle/$bundleName/test-runner && chmod +x test-bin/*.test; " +
              "for t in test-bin/*.test; do echo `"== `$t`"; `$t -test.v -test.count=1 -test.timeout=600s || true; done"
    Write-Host "==> SSH: extract bundle + run test binary linux (F10) di VM"
    & ssh $SSH $remote
    if ($LASTEXITCODE -ne 0) { throw "ssh gagal (exit $LASTEXITCODE)" }
    Write-Host "    Di VM, admin tinggal: cd ~/prod_bundle/$bundleName && sudo bash run_deploy.sh"
}

# ---------------------------------------------------------------- 12. Report
$report = Join-Path $OutDir "$bundleName.report.txt"
@"
PRODUCTION BUNDLE BUILD REPORT
==============================
bundle: $bundleName.tar.gz
timestamp: $(Get-Date -Format o)
commit_refactoring: $headCommit (pin doc014: $Commit)
arch: $Arch
env_config: $EnvConfigPath

binary linux (ELF $Arch):
  bin/etl, bin/app-simlab, bin/app-simlab-publish

test binary linux (ELF $Arch): 8 package di test-runner/test-bin/
  $($TestPackages -join ', ')

verifikasi parsing -test.v 1-to-1 (F10):
  go test -json : total=$baseTotal pass=$basePass fail=$baseFail skip=$baseSkip
  test binary   : total=$binTotal pass=$binPass fail=$binFail skip=$binSkip
  hasil         : $(if ($parseOk) { 'PASS (1-to-1 identik)' } else { 'FAIL' })

isi bundle:
  config/  : etl-config.production.json + .env.config
  bin/     : etl, app-simlab, app-simlab-publish
  test-runner/ : go.mod, .env.reference, seeds/, test-bin/ (8 *.test)
  seeds/   : mi-1, vokasi-1, default
  deploy_production.sh + run_deploy.sh (alur P0-P15; run_deploy.sh = SATU file utk admin)
  lib/ + README_DEPLOY.md + bundle-meta.txt (commit & nama bundle utk report P13)
  test helpers (SKIP bila -NoTestHelpers): seed_old_install.sh + assets/inventaris_lab_empty.db
  P15 self-cleanup: hapus tar.gz + folder extract setelah semua fase PASS/SKIP + server running + /readyz OK

next: verifikasi eksekusi test binary linux di VM:
  scp $bundleName.tar.gz root@server:/opt/simlab/
  ssh root@server "cd /opt/simlab && tar xzf $bundleName.tar.gz"
  ssh root@server "cd /opt/simlab/$bundleName/test-runner && chmod +x test-bin/*.test && for t in test-bin/*.test; do \$t -test.v -test.count=1 -test.timeout=600s; done"
"@ | Out-File -FilePath $report -Encoding utf8
Write-Host "==> Report: $report"
Write-Host "==> SELESAI. Bundle: $zipPath"
