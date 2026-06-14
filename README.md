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

### 1. Download Installer

Buka [go.dev/dl](https://go.dev/dl/) dan download **Microsoft Windows** installer (`.msi`) sesuai arsitektur:

| Arsitektur | Link Download |
|-----------|--------------|
| **x86-64** (Intel/AMD, 64-bit) | [go1.26.4.windows-amd64.msi](https://go.dev/dl/go1.26.4.windows-amd64.msi) |
| **ARM64** (Snapdragon X, Surface Pro) | [go1.26.4.windows-arm64.msi](https://go.dev/dl/go1.26.4.windows-arm64.msi) |
| **x86** (32-bit — jarang) | [go1.26.4.windows-386.msi](https://go.dev/dl/go1.26.4.windows-386.msi) |

> Jika ragu, cek arsitektur: `(Get-CimInstance Win32_ComputerSystem).SystemType` di PowerShell.

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

> Jika `go` tidak dikenali, tutup dan buka ulang PowerShell, atau restart Windows.

---

## Instalasi Git

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

# Edit dengan notepad
notepad .env
```

**Konfigurasi minimal untuk production:**

```env
ENVIRONMENT=production
HOST=0.0.0.0
PORT=8080
DATABASE_PATH=inventaris_lab.db
SESSION_SECRET=generate-random-string-panjang-32-64-karakter
TIMEZONE=Asia/Jakarta
UPLOAD_PATH=uploads
GEMINI_API_KEY=your-gemini-api-key
OPENROUTER_API_KEY=sk-or-your-openrouter-api-key
BACKUP_ENABLED=true
BACKUP_DIR=./backups
```

Lihat [Panduan .env Reference](#panduan-env-reference) untuk semua opsi.

---

## Build & Run

### Build Binary

```powershell
# Build (CGO_ENABLED=0 — tidak perlu compiler)
$env:CGO_ENABLED = "0"
go build -ldflags="-s -w" -o app-simlab.exe ./cmd/server/main.go

# Atau pakai script
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

## Auto-Deploy Workflow (GitHub Actions)

Branch `refactoring` memiliki workflow `.github/workflows/auto-deploy.yml` yang otomatis sync ke `deploy_windows` setiap ada push.

**Cara trigger update:**

1. Push perubahan ke `refactoring` dari laptop development
2. GitHub Actions: test → merge ke `deploy_windows` → restore deploy-specific files → build verify → push
3. Di Windows: `git pull origin deploy_windows` → rebuild → restart service

**README.md di branch deploy_windows TIDAK akan ditimpa** — workflow mereset README.md ke versi deploy branch (OLD_HEAD) setiap sync.

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

### Backup Database

Backup otomatis via `BACKUP_ENABLED=true`. Manual:

```powershell
Copy-Item inventaris_lab.db "inventaris_lab.db.backup_$(Get-Date -Format 'yyyyMMdd_HHmmss')"
```

### Update Aplikasi

```powershell
# Pull dari GitHub
git pull origin deploy_windows

# Rebuild
$env:CGO_ENABLED = "0"
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

Semua konfigurasi via file `.env`. Copy dari `.env.example`.

```env
# ============================
# APLIKASI
# ============================
ENVIRONMENT=production
HOST=0.0.0.0
PORT=8080

# ============================
# DATABASE
# ============================
# SQLite — path relatif atau absolut
DATABASE_PATH=inventaris_lab.db
# PostgreSQL — kosongkan untuk SQLite
# DATABASE_URL=postgres://user:pass@...

# ============================
# WRITE MODE
# ============================
WRITE_MODE=sync

# ============================
# SECURITY
# ============================
SESSION_SECRET=change-this-secret-in-production-to-random-string

# ============================
# TIMEZONE
# ============================
TIMEZONE=Asia/Jakarta

# ============================
# UPLOAD
# ============================
UPLOAD_PATH=uploads

# ============================
# OCR API KEYS
# ============================
GEMINI_API_KEY=your-gemini-api-key-here
OPENROUTER_API_KEY=sk-or-your-openrouter-api-key-here

# ============================
# ANDROID MODE — false untuk Windows
# ============================
ANDROID=false

# ============================
# PC PHOTO SEEDING
# ============================
PC_PHOTO_RELEASE_URL=
GITHUB_TOKEN=

# ============================
# PAGINATION
# ============================
DEFAULT_PAGE_SIZE=25

# ============================
# BACKUP (SQLite only)
# ============================
BACKUP_ENABLED=true
BACKUP_INTERVAL=30
BACKUP_DIR=./backups
BACKUP_RETENTION=20
BACKUP_MIN_DISK_MB=500
BACKUP_COMPRESS=true

# ============================
# PUBLIC SITE (SSG)
# ============================
PUBLIC_BUILD_ENABLED=false
PUBLIC_BUILD_INTERVAL=30
PUBLIC_BUILD_OUT=dist
PUBLIC_BUILD_TEMPLATE_DIR=web/templates/public
PUBLIC_BUILD_STATIC_DIR=web/static
PUBLIC_BUILD_REPO_DIR=
PUBLIC_BUILD_BRANCH=main
```

---

## Troubleshooting

| Masalah | Penyebab | Solusi |
|---------|----------|--------|
| `go build` gagal `CGO_ENABLED=0` | Go version outdated | `go version` — harus 1.25+. Download dari go.dev |
| Tailscale tidak muncul di system tray | Service belum jalan | `Start-Service Tailscale` atau restart Windows |
| Port 8080 sudah dipakai | Aplikasi lain | Ganti `PORT` di `.env`, atau stop aplikasi lain: `netstat -ano \| findstr :8080` |
| Firewall blocking akses | Rule belum ada | Jalankan `.\scripts\setup_firewall.ps1` sebagai Administrator |
| NSSM service gagal start | Path binary salah | `nssm edit SimLabServer` → cek `Application Path` |
| `exec format error` | Build untuk arsitektur salah | `go env GOARCH` — harus `amd64` |
| Upload foto gagal | Path upload tidak writable | Pastikan `UPLOAD_PATH` ada dan bisa ditulis |
| Database `UNIQUE constraint` | Data duplikat | Restart server — normalisasi auto jalan |
| Backup gagal "disk space" | Storage minimal | Kosongkan disk atau kecilkan `BACKUP_MIN_DISK_MB` |
| Antivirus block binary | False positive | Tambah exception di Windows Defender untuk `app-simlab.exe` |
| PostgreSQL gagal konek | `DATABASE_URL` salah / firewall | Cek Neon dashboard → Connection details. Pastikan koneksi internet |
| SSG build tidak push ke git | Git auth belum diatur | Setup Git Credential Manager atau SSH key. Test: `git push --dry-run` |
| Server lambat dengan banyak PC | WRITE_MODE=sync kena bottleneck | Ganti ke `WRITE_MODE=async` di .env |
