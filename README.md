# Sistem Inventaris Laboratorium Komputer — Windows

Branch `deploy_windows` — khusus deployment ke **Windows 10/11**. Cocok untuk development lokal atau production skala kecil. Bisa diakses via Tailscale dari device lain.

Database: **SQLite** (pure Go via `modernc.org/sqlite`, zero CGO).

---

## Daftar Isi

1. [Prasyarat](#prasyarat)
2. [Instalasi Go](#instalasi-go)
3. [Instalasi Tailscale (Windows)](#instalasi-tailscale-windows)
4. [Clone & Konfigurasi .env](#clone--konfigurasi-env)
5. [Build & Run](#build--run)
6. [Firewall (Akses dari Device Lain)](#firewall-akses-dari-device-lain)
7. [Windows Service (NSSM) — Production](#windows-service-nssm--production)
8. [Auto-Deploy Workflow (GitHub Actions)](#auto-deploy-workflow-github-actions)
9. [Maintenance](#maintenance)
10. [Panduan .env Reference](#panduan-env-reference)
11. [Troubleshooting](#troubleshooting)

---

## Prasyarat

- **Windows 10/11** 64-bit (x86_64 atau ARM64)
- **Tidak perlu C compiler** — SQLite pure Go (modernc.org/sqlite, `CGO_ENABLED=0`)
- **PowerShell 5.1+** (bawaan Windows 10/11)
- **Akun Tailscale** — [daftar gratis](https://login.tailscale.com)
- **Koneksi internet** — untuk download dependencies dan GitHub sync

---

## Instalasi Go (Golang)

Cek dulu apakah sudah terinstall:

```powershell
go version
# Jika output: go version go1.25+ → sudah terinstall (skip ke Instalasi Git)
# Jika error "not recognized" → lanjut install di bawah
```

### 1. Download Installer

Buka [go.dev/dl](https://go.dev/dl/) dan download **Microsoft Windows** installer (`.msi`) sesuai arsitektur:

| Arsitektur | Link Download |
|-----------|--------------|
| **x86-64** (Intel/AMD, 64-bit) | [go1.26.4.windows-amd64.msi](https://go.dev/dl/go1.26.4.windows-amd64.msi) |
| **ARM64** (Snapdragon X, Surface Pro) | [go1.26.4.windows-arm64.msi](https://go.dev/dl/go1.26.4.windows-arm64.msi) |
| **x86** (32-bit — jarang) | [go1.26.4.windows-386.msi](https://go.dev/dl/go1.26.4.windows-386.msi) |

> Jika ragu arsitektur: `(Get-CimInstance Win32_ComputerSystem).SystemType` di PowerShell.

### 2. Jalankan Installer

1. Double klik file `.msi` yang sudah didownload
2. Ikuti wizard installer — **default path** (`C:\Program Files\Go`) sudah benar
3. Klik **Finish** setelah selesai

### 3. Verifikasi

Buka **PowerShell baru** (wajib baru agar PATH terupdate):

```powershell
go version
# Output: go version go1.26.4 windows/amd64
```

> Jika `go` tidak dikenali setelah install, tutup dan buka ulang PowerShell, atau restart Windows.

📎 [Official docs: go.dev/doc/install](https://go.dev/doc/install)

---

## Instalasi Git

Cek dulu apakah sudah terinstall:

```powershell
git version
# Jika output: git version 2.x.x → sudah terinstall (skip ke Setup SSH Key)
# Jika error "not recognized" → lanjut install di bawah
```

### Metode 1 (Recommended) — Git for Windows

Download dari [git-scm.com/download/win](https://git-scm.com/download/win) — installer akan otomatis mendeteksi arsitektur.

1. Jalankan installer `.exe`
2. **Wizard settings** (pilih sesuai kebutuhan):
   - **Select Components** → biarkan default
   - **Choosing default editor** → pilih editor (Notepad, VS Code, Nano, dll)
   - **Adjusting PATH** → **"Git from the command line and also from 3rd-party software"** (recommended)
   - **Choosing SSH executable** → **"Use bundled OpenSSH"**
   - **Line ending conversions** → **"Checkout as-is, commit as-is"**
   - **Terminal emulator** → **"Use MinTTY"**
3. Finish → centang **"Launch Git Bash"** jika ingin cek langsung

### Metode 2 — winget (PowerShell)

```powershell
winget install --id Git.Git -e --source winget
```

### Verifikasi

```powershell
git version
# Output: git version 2.x.x.windows.1
```

### Konfigurasi Awal Git

```powershell
git config --global user.name "Nama Anda"
git config --global user.email "email@example.com"
```

📎 [Official docs: git-scm.com](https://git-scm.com/book/en/v2/Getting-Started-Installing-Git)

---

## Setup SSH Key untuk GitHub (Opsional)

Diperlukan jika ingin menggunakan fitur SSG Public Site Auto-Build (git push otomatis dari Windows).

```powershell
# Generate SSH key
ssh-keygen -t ed25519 -C "pc-windows@example.com"
# Enter file: tekan Enter (default C:\Users\Anda\.ssh\id_ed25519)
# Enter passphrase: kosongkan atau isi sesuai preferensi

# Tampilkan public key
Get-Content ~\.ssh\id_ed25519.pub
```

Copy output `ssh-ed25519 AAAA...` → buka [GitHub → Settings → SSH and GPG keys](https://github.com/settings/keys) → **New SSH key** → paste → save.

Test koneksi:
```powershell
ssh -T git@github.com
# Output: Hi username! You've successfully authenticated...
```

---

## Instalasi Tailscale (Windows)

### Opsi 1: GUI Installer (Mudah)

1. Download dari [tailscale.com/download/windows](https://tailscale.com/download/windows)
2. Jalankan `.exe` installer
3. Setelah install, klik icon Tailscale di system tray
4. Pilih **Log in** → login via browser
5. Verifikasi:

```powershell
tailscale status
tailscale ip
# Output: 100.x.x.x
```

### Opsi 2: Silent Install via PowerShell (Headless)

Untuk server tanpa GUI atau instalasi via script:

```powershell
# Download MSI installer
$msiUrl = "https://pkgs.tailscale.com/stable/tailscale-setup-latest-amd64.msi"
$msiPath = "$env:TEMP\tailscale.msi"
Invoke-WebRequest -Uri $msiUrl -OutFile $msiPath

# Silent install
Start-Process msiexec.exe -Wait -ArgumentList "/i `"$msiPath`" /qb TS_UNATTENDEDMODE=always"

# Generate auth key dari Tailscale Admin Console → Keys
# Lalu authenticate headless:
& "C:\Program Files\Tailscale\tailscale.exe" up --authkey tskey-auth-xxxxxxxxxxxxxxxxxxxxxxxx --unattended
```

### Cek & Catat Tailscale IP

```powershell
tailscale ip
# Contoh: 100.80.1.25
```

Catat IP ini — akan dipakai untuk akses dari device lain.

### Akses dari Device Lain via Tailscale

```powershell
# Dari laptop lain yang join tailnet yang sama:
ssh user@100.x.x.x       # SSH
# Atau Remote Desktop (RDP)
mstsc /v:100.x.x.x
```

---

## Clone & Konfigurasi .env

```powershell
# Clone repositori
git clone -b deploy_windows https://github.com/gerrymoeis/lab_kom_sim.git
cd lab_kom_sim

# Copy .env
copy .env.example .env

# Generate SESSION_SECRET random (32 karakter ASCII printable)
$randomBytes = [byte[]]::new(32)
[Security.Cryptography.RNGCryptoServiceProvider]::Create().GetBytes($randomBytes)
$sessionSecret = [System.Convert]::ToBase64String($randomBytes)
Write-Host "SESSION_SECRET=$sessionSecret"

# Edit dengan notepad
notepad .env
```

**Konfigurasi minimal — Single-Lab (copy-paste dengan nilai Anda):**

```env
ENVIRONMENT=production
HOST=0.0.0.0
PORT=8080
DATABASE_PATH=inventaris_lab.db
SESSION_SECRET=isi-dengan-output-perintah-generate
COOKIE_SECURE=false
TIMEZONE=Asia/Jakarta
UPLOAD_PATH=uploads
GEMINI_API_KEY=your-gemini-api-key
OPENROUTER_API_KEY=sk-or-your-openrouter-api-key
BACKUP_ENABLED=true
BACKUP_DIR=./backups
```

**Konfigurasi Multi-Lab (ganti DATABASE_PATH + tambah LABS):**

```env
GLOBAL_DB_PATH=data/global.db        # DB global (users, permissions)
LABS=MI-1:data/lab_mi_1.db:Lab Kom MI 1:lab-kom-mi,VOKASI-1:data/lab_vokasi_1.db:Lab Kom Vokasi:vokasi
# Format LABS: LAB-ID:dbPath:Title:urlPath (comma-separated)
# Saat LABS diisi, DATABASE_PATH diabaikan — setiap lab punya DB sendiri.
```

> **Catatan:** Perintah PowerShell di atas menghasilkan 32 byte random → Base64 (44 karakter). Setiap server harus punya `SESSION_SECRET` unik.

Lihat file `.env.example` (auto-sync dari branch refactoring) untuk dokumentasi lengkap semua opsi.

---

## Build & Run

### Build Binary

```powershell
# Build (CGO_ENABLED=0 — tidak perlu compiler)
$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = "amd64"
go build -ldflags="-s -w" -o app-simlab.exe ./cmd/server/main.go

# Atau pakai script (recommended)
.\scripts\build-windows.ps1
```

### Run Langsung (Foreground)

```powershell
.\app-simlab.exe
```

Akses: `http://localhost:8080`

**Default login:** `admin` / `admin123`

### Run Background (PowerShell)

```powershell
# Start-Process — proses tetap jalan meski PowerShell ditutup
Start-Process -FilePath ".\app-simlab.exe" -WindowStyle Hidden

# Atau pakai script deploy
.\scripts\deploy-windows.ps1
```

---

## Firewall (Akses dari Device Lain)

Jika ingin mengakses server dari HP/device lain di **jaringan LAN yang sama** (bukan via Tailscale):

```powershell
# Jalankan PowerShell sebagai Administrator, lalu:
.\scripts\setup_firewall.ps1

# Atau manual:
New-NetFirewallRule -DisplayName "SimLab App" -Direction Inbound -Protocol TCP -LocalPort 8080 -Action Allow
```

**Akses via Tailscale** tidak perlu firewall — Tailscale langsung konek via WireGuard tunnel.

---

## Windows Service (NSSM) — Production

Agar aplikasi auto-start meski Windows restart, gunakan **NSSM** (Non-Sucking Service Manager):

### Install NSSM

```powershell
# Via winget
winget install nssm

# Atau download manual dari https://nssm.cc/download
```

### Install Service

```powershell
# Setup service
nssm install SimLabServer "C:\path\to\lab_kom_sim\app-simlab.exe"
nssm set SimLabServer AppDirectory "C:\path\to\lab_kom_sim"
nssm set SimLabServer AppEnvironmentExtra "CGO_ENABLED=0"
nssm set SimLabServer DisplayName "Sistem Inventaris Lab Komputer"
nssm set SimLabServer Description "Server aplikasi inventaris laboratorium komputer"
nssm set SimLabServer Start SERVICE_AUTO_START
nssm set SimLabServer AppStdout "C:\path\to\lab_kom_sim\server.log"
nssm set SimLabServer AppStderr "C:\path\to\lab_kom_sim\server.log"
nssm set SimLabServer AppRotateFiles 1
nssm set SimLabServer AppRotateSeconds 86400

# Start service
nssm start SimLabServer

# Cek status
nssm status SimLabServer
```

### Manajemen Service

```powershell
nssm restart SimLabServer     # Restart
nssm stop SimLabServer        # Stop
nssm edit SimLabServer        # Edit konfigurasi (GUI)
```

---

## Auto-Deploy (Update Server dari Laptop)

**Cara kerja:** Push ke `refactoring` → GitHub Actions otomatis test + sync ke `deploy_windows` (~90 detik) → SSH ke Windows server → deploy.

### Setup OpenSSH Server (Sekali di Windows)

Untuk deploy jarak jauh tanpa RDP, aktifkan **OpenSSH Server** di Windows:

```powershell
# Di Windows server (PowerShell sebagai Administrator)
# Cek apakah sudah terinstall
Get-WindowsCapability -Online | Where-Object Name -like 'OpenSSH.Server*'

# Jika belum, install
Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0

# Start service
Start-Service sshd
Set-Service -Name sshd -StartupType 'Automatic'

# Konfirmasi firewall
New-NetFirewallRule -DisplayName 'OpenSSH Server' -Direction Inbound -Protocol TCP -LocalPort 22 -Action Allow
```

### Setup Passwordless SSH (Sekali dari Laptop)

```powershell
# Di laptop (PowerShell)
# Cek SSH key
ls ~\.ssh\id_ed25519.pub
# Jika tidak ada: ssh-keygen -t ed25519 -C "laptop@example.com"

# Copy public key ke Windows server
type $env:USERPROFILE\.ssh\id_ed25519.pub | ssh user@100.x.x.x "mkdir -p ~\.ssh && cat >> ~\.ssh\authorized_keys"

# Test (login tanpa password)
ssh user@100.x.x.x
```

### (Opsional) Git Alias — 1 Perintah dari Laptop

```bash
git config --global alias.deploy-win "!git push origin refactoring && ssh user@100.x.x.x 'cd C:\path\to\lab_kom_sim && .\scripts\deploy-windows.ps1'"
# Setelah ini: git deploy-win → push refactoring + deploy ke Windows
```

### Satu Perintah Deploy (Setiap Update)

```powershell
# 1. Push ke refactoring
git push origin refactoring

# 2. Tunggu ~90 detik (workflow selesai)

# 3. Deploy ke Windows — pull + build + restart
ssh user@100.x.x.x 'cd C:\path\to\lab_kom_sim && git pull origin deploy_windows && $env:CGO_ENABLED="0"; $env:GOOS="windows"; $env:GOARCH="amd64"; go build -ldflags="-s -w" -o app-simlab.exe .\cmd\server\main.go; nssm restart SimLabServer'
```

Atau jalankan script deploy yang sudah ada:

```powershell
ssh user@100.x.x.x 'cd C:\path\to\lab_kom_sim && .\scripts\deploy-windows.ps1'
```

---

## Maintenance

### Cek Log

```powershell
# Log file (jika dikonfigurasi NSSM)
Get-Content server.log -Tail 50 -Wait

# Event Viewer (jika NSSM error)
Get-EventLog -LogName Application -Source "SimLabServer" -Newest 20
```

### Restart Service

```powershell
nssm restart SimLabServer
```

### Backup & Restore Database

Backup otomatis tiap ada perubahan data (CUD operation, debounce 30 detik). Konfigurasi via `.env`:

```powershell
BACKUP_ENABLED=true        # Aktif
BACKUP_INTERVAL=30         # 30 detik setelah CUD terakhir
BACKUP_DIR=./backups       # Lokasi backup (bisa multi-path)
BACKUP_RETENTION=20        # Simpan 20 backup terakhir
BACKUP_COMPRESS=true       # Kompres .gz
```

Cek daftar backup:
```powershell
Get-ChildItem .\backups\ | Sort-Object LastWriteTime -Descending | Select-Object -First 10
```

Restore dari backup:
```powershell
# 1. Hentikan service
nssm stop SimLabServer

# 2. Backup DB corrupt untuk investigasi
Copy-Item inventaris_lab.db inventaris_lab.db.corrupt

# 3. Cari backup terbaru dan copy ke database aktif
Get-ChildItem .\backups\ | Sort-Object LastWriteTime -Descending | Select-Object -First 1
# Copy manual: Copy-Item .\backups\inventaris_lab.db.backup_20260613_120405 inventaris_lab.db

# 4. Start service
nssm start SimLabServer
```

### Database Recovery (Migration Failure)

Server auto-run migration setiap startup. Jika setelah update server langsung crash:

**1. Cek log untuk tahu penyebab:**
```powershell
Get-Content server.log -Tail 50 | Select-String "migration|error|fatal"
# Atau via Event Viewer:
Get-EventLog -LogName Application -Source "SimLabServer" -Newest 20
```

**2. Restore database dari backup:**
```powershell
# Hentikan service
nssm stop SimLabServer

# Backup DB corrupt
Copy-Item inventaris_lab.db inventaris_lab.db.corrupt.$(Get-Date -Format 'yyyyMMdd_HHmmss')

# Balikkan ke backup terbaru
$latest = Get-ChildItem .\backups\ | Sort-Object LastWriteTime -Descending | Select-Object -First 1
Copy-Item $latest.FullName inventaris_lab.db

# Start ulang
nssm start SimLabServer
```

**3. Rollback binary (jika bug di kode baru):**
```powershell
cd C:\path\to\lab_kom_sim
git log --oneline -5 origin/deploy_windows
git checkout COMMIT_HASH_SEBELUMNYA -- cmd/ go.mod go.sum internal/ web/
$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = "amd64"
go build -ldflags="-s -w" -o app-simlab.exe .\cmd\server\main.go
nssm restart SimLabServer
```

**4. Reset database baru (jika semua gagal):**
```powershell
nssm stop SimLabServer
Remove-Item inventaris_lab.db
nssm start SimLabServer
# Database baru + seed data otomatis
```

### Update Aplikasi

```powershell
# Pull dari GitHub
git pull origin deploy_windows

# Rebuild
$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = "amd64"
go build -ldflags="-s -w" -o app-simlab.exe ./cmd/server/main.go

# Restart service
nssm restart SimLabServer
```

### Reset Database

```powershell
# Hentikan service
nssm stop SimLabServer

# Hapus database
Remove-Item inventaris_lab.db

# Start ulang — database auto-create
nssm start SimLabServer
```

---

## Fitur Lanjutan

### PostgreSQL / Neon (Scale Up)

Untuk migrasi dari SQLite ke PostgreSQL (Neon) — pure Go driver `pgx/v5`, `CGO_ENABLED=0` tetap aman:

1. **Buat akun**: [neon.tech](https://neon.tech) → Create project
2. **Salin DATABASE_URL** dari dashboard Neon
3. **Set di .env**: `DATABASE_URL=postgres://user:pass@ep-xxx.ap-southeast-1.aws.neon.tech/neondb?sslmode=require`
4. **Kosongkan** `DATABASE_PATH` — server akan pakai PostgreSQL

Restart server — migrasi skema & data otomatis saat startup.

### PC Photo Seeding (via GitHub Releases)

Seed foto PC dari ZIP di GitHub Release. Cocok untuk setup awal laboratorium.

1. Buat ZIP folder `uploads/` berisi foto PC (nama file = nomor PC, misal `1.jpg`, `2.jpg`)
2. Upload ke GitHub Release di repo publik/private
3. **Set di .env**:
   - `PC_PHOTO_RELEASE_URL=https://github.com/user/repo/releases/download/v1.0.0/photos.zip`
   - `GITHUB_TOKEN=github_pat_xxx` (generate di GitHub → Settings → Developer → PAT, scope `repo`)

Restart server — foto akan didownload dan diekstrak otomatis. Kosongkan ke 2 variable jika tidak perlu.

### SSG Public Site Auto-Build

Aplikasi auto-generate static site ke `PUBLIC_BUILD_OUT` (default: `dist/`) dan git push ke `PUBLIC_BUILD_REPO_DIR` setiap ada perubahan data.

1. Clone repo public site: `git clone git@github.com:user/public-repo.git C:\repos\public-site`
2. **Set di .env**:
   - `PUBLIC_BUILD_ENABLED=true`
   - `PUBLIC_BUILD_REPO_DIR=C:\repos\public-site`
   - `PUBLIC_BUILD_BRANCH=main`
3. **Konfigurasi git auth** (Git Credential Manager atau SSH key) — pastikan push tanpa password

Server akan rebuild & push otomatis tiap CUD operation (debounce `PUBLIC_BUILD_INTERVAL` detik).

### Async Write Mode

- `WRITE_MODE=sync` (default): setiap write langsung ke SQLite — aman, cocok untuk beban normal
- `WRITE_MODE=async`: queue-based batch writer — lebih cepat untuk burst request, write di-batch dalam 1 transaksi. Gunakan jika ada 50+ PC di-grid dengan concurrent akses tinggi

### Backup Multi-Path

Untuk redundancy, backup bisa dikirim ke multiple direktori sekaligus. Pisahkan dengan koma (spasi di path pakai quotes):

```powershell
BACKUP_DIR=".\backups, D:\backup_lab, \\NAS\share\simlab-backups"
```

---

## Panduan .env Reference

Dokumentasi lengkap semua environment variable ada di file `.env.example` (auto-sync dari branch `refactoring` via GitHub Actions). File ini selalu terupdate — buka langsung di server:

```powershell
Get-Content .env.example | Select-String -Pattern "^[A-Z#]"
```

Atau buka [.env.example](.env.example) untuk semua opsi.

### Catatan Penting

- **Multi-Lab:** Saat `LABS` diisi, setiap lab punya database, session (cookie `inventaris_session_{urlPath}`), upload folder (`uploads/{urlPath}/`), dan backup folder sendiri. `DATABASE_PATH` diabaikan.
- **Global DB** (`GLOBAL_DB_PATH`): Menyimpan user global, lab_permissions, grid_layouts — wajib ada.
- **Auto-Sync Middleware:** Setiap login, sistem otomatis membuat/update row di per-lab `users` table. Data global cukup diatur di `/admin/users`.
- **DATABASE_URL** (PostgreSQL/Neon): Jika diisi, semua SQLite path diabaikan.

---

## Troubleshooting

| Masalah | Penyebab | Solusi |
|---------|----------|--------|
| `go build` gagal `CGO_ENABLED=0` | Go version outdated | `go version` — harus 1.25+. Download dari go.dev |
| Tailscale tidak muncul di system tray | Service belum jalan | `Start-Service Tailscale` atau restart Windows |
| Port 8080 sudah dipakai | Aplikasi lain | Ganti `PORT` di `.env`, atau stop aplikasi lain: `netstat -ano \| findstr :8080` |
| Firewall blocking akses | Rule belum ada | Jalankan `.\scripts\setup_firewall.ps1` sebagai Administrator |
| NSSM service gagal start | Path binary salah | `nssm edit SimLabServer` → cek `Application Path` |
| `exec format error` | Build untuk OS/arsitektur salah | Cek dengan `file ./binary`. Pastikan `GOOS=windows GOARCH=amd64` |
| Upload foto gagal | Path upload tidak writable | Pastikan `UPLOAD_PATH` ada dan bisa ditulis |
| Database `UNIQUE constraint` | Data duplikat | Restart server — normalisasi auto jalan |
| Backup gagal "disk space" | Storage minimal | Kosongkan disk atau kecilkan `BACKUP_MIN_DISK_MB` |
| Antivirus block binary | False positive | Tambah exception di Windows Defender untuk `app-simlab.exe` |
| PostgreSQL gagal konek | `DATABASE_URL` salah / firewall | Cek Neon dashboard → Connection details. Pastikan koneksi internet |
| SSG build tidak push ke git | Git auth belum diatur | Setup Git Credential Manager atau SSH key. Test: `git push --dry-run` |
| Server lambat dengan banyak PC | WRITE_MODE=sync kena bottleneck | Ganti ke `WRITE_MODE=async` di .env |
| Login selalu 403 Forbidden | `COOKIE_SECURE=true` tapi server HTTP | Set `COOKIE_SECURE=false` di `.env` jika server belum HTTPS |
