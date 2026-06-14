# Sistem Inventaris Laboratorium Komputer — Android (Termux + Tailscale)

Branch `deploy_android` — khusus deployment ke **HP Android via Termux**. Server dioperasikan sepenuhnya dari terminal Termux, diakses dari laptop/device lain via SSH melalui Tailscale.

Database: **SQLite** (pure Go via `modernc.org/sqlite`, zero CGO, `-tags nodynamic`).

---

## Daftar Isi

1. [Prasyarat](#prasyarat)
2. [Instalasi Termux (Wajib dari F-Droid)](#instalasi-termux-wajib-dari-f-droid)
3. [Setup Termux: Paket & SSH](#setup-termux-paket--ssh)
4. [Instalasi Tailscale di Termux](#instalasi-tailscale-di-termux)
5. [Clone Repositori & Konfigurasi .env](#clone-repositori--konfigurasi-env)
6. [Build & Run](#build--run)
7. [Auto Deploy (Laptop → HP via Tailscale + SSH)](#auto-deploy-laptop--hp-via-tailscale--ssh)
8. [Maintenance](#maintenance)
9. [Auto-Deploy Workflow (GitHub Actions)](#auto-deploy-workflow-github-actions)
10. [Panduan .env Reference](#panduan-env-reference)
11. [Troubleshooting](#troubleshooting)

---

## Prasyarat

- **HP Android** — minimal Android 11 (API 30), recommended Android 13+
- **Termux** — install dari **F-Droid** (bukan Play Store — versi Play Store sudah deprecated/tidak update)
- **Go 1.25+** — install via `pkg install golang`
- **Tailscale** — install di Termux via `pkg install tailscale`
- **Akun Tailscale** — [daftar gratis](https://login.tailscale.com)
- **Laptop/PC kedua** — untuk akses SSH dan development (opsional, server bisa dioperasikan langsung dari HP dengan keyboard eksternal)
- **Koneksi internet** — untuk download dependencies dan GitHub sync
- **Baterai/charging** — disarankan HP selalu dicharge saat server berjalan

---

## Instalasi Termux (Wajib dari F-Droid)

> **⚠️ JANGAN install Termux dari Google Play Store.** Versi Play Store sudah deprecated (tidak update sejak 2023). Gunakan F-Droid.

1. Download F-Droid: buka [f-droid.org](https://f-droid.org) di browser HP → download APK → install
2. Buka F-Droid, cari "Termux"
3. Install **Termux** (bukan Termux:API atau yang lain)
4. Buka Termux, jalankan update:

```bash
pkg update && pkg upgrade -y
```

5. Grant storage access (untuk akses file):

```bash
termux-setup-storage
```

> **Tips:** Jika keyboard HP tidak nyaman, gunakan pairing keyboard Bluetooth atau akses via SSH dari laptop setelah Tailscale terinstall.

---

## Setup Termux: Paket Dasar

### 1. Update Package Manager

```bash
pkg update && pkg upgrade -y
```

### 2. Install Git

Git untuk clone repositori dan update kode:

```bash
pkg install git -y
```

**Verifikasi:**
```bash
git version
# Output: git version 2.x.x
```

**Konfigurasi Git (wajib untuk commit):**
```bash
git config --global user.name "Nama Anda"
git config --global user.email "email@example.com"
```

### 3. Install Go (Golang)

Termux menyediakan Go versi terbaru (saat ini **Go 1.26.3**) langsung dari repositori resmi:

```bash
pkg install golang -y
```

Proses instalasi memakan waktu karena Go di-compile dari source oleh Termux. Pastikan koneksi internet stabil.

**Verifikasi:**
```bash
go version
# Output: go version go1.26.3 android/arm64
```

> **Catatan:** Go di Termux sudah terkonfigurasi dengan `CGO_ENABLED=0` secara default. Tidak perlu konfigurasi PATH tambahan.

### 4. Install SSH Server

Diperlukan untuk akses remote dari laptop:

```bash
pkg install openssh -y
```

### 5. Install Text Editor (Opsional)

```bash
pkg install nano -y
# Atau: pkg install vim -y
```

### 6. Set Password SSH

```bash
passwd
# Ketik password (tidak akan terlihat di layar), konfirmasi sekali lagi
```

### Start SSH Server

Termux SSH server berjalan di **port 8022** (bukan 22 — karena Android restriction):

```bash
sshd
```

Agar SSH auto-start tiap buka Termux, tambah ke `~/.bashrc`:

```bash
echo "sshd" >> ~/.bashrc
```

### Cek Username Termux

```bash
whoami
# Output: u0_aXXX (misal u0_a124)
```

Catat username ini — akan dipakai untuk SSH dari laptop.

### Setup SSH Key untuk GitHub (Opsional)

Diperlukan jika ingin menggunakan fitur SSG Public Site Auto-Build (git push otomatis dari HP).

```bash
# Generate SSH key
ssh-keygen -t ed25519 -C "hp-termux@example.com"
# Enter file: tekan Enter (default)
# Enter passphrase: kosongkan atau isi sesuai preferensi

# Tampilkan public key
cat ~/.ssh/id_ed25519.pub
```

Copy output `ssh-ed25519 AAAA...` → buka [GitHub → Settings → SSH and GPG keys](https://github.com/settings/keys) → **New SSH key** → paste → save.

Test koneksi:
```bash
ssh -T git@github.com
# Output: Hi username! You've successfully authenticated...
```

---

## Instalasi Tailscale di Termux

### 1. Generate Auth Key (dari laptop browser)

1. Buka [Tailscale Admin Console → Keys](https://login.tailscale.com/admin/settings/keys)
2. Klik **Generate auth key**
3. Setting:
   - **Reusable**: ✅ centang
   - **Expiry**: 30 days atau Never
4. Copy auth key: `tskey-auth-xxxxxxxxxxxxxxxxxxxxxxxx`

### 2. Install & Jalankan Tailscale

```bash
# Install dari repositori Termux
pkg install tailscale

# Start daemon dengan userspace networking (wajib untuk Android tanpa root)
tailscaled --tun=userspace-networking --statedir=$PREFIX/var/lib/tailscale &

# Authenticate
tailscale up --authkey tskey-auth-xxxxxxxxxxxxxxxxxxxxxxxx
```

**Penjelasan:** Android tidak mengizinkan akses `tun` device tanpa root. `--tun=userspace-networking` membuat Tailscale berfungsi penuh dalam mode userland.

### 3. Verifikasi & Catat IP

```bash
tailscale status
tailscale ip
# Output: 100.x.x.x — ini Tailscale IP HP Anda
```

### 4. Setup Autostart Tailscale

Agar Tailscale tetap jalan meski Termux di-close (pakai Termux:Boot atau `termux-wake-lock`):

```bash
# Cegah Termux dimatikan background
termux-wake-lock

# Tambah ke ~/.bashrc
echo "termux-wake-lock" >> ~/.bashrc
echo "tailscaled --tun=userspace-networking --statedir=\$PREFIX/var/lib/tailscale &" >> ~/.bashrc
echo "sshd" >> ~/.bashrc
```

> **Catatan:** Untuk uptime server yang andal, aktifkan **Battery Optimization exception** untuk Termux di Settings HP.

---

## Clone Repositori & Konfigurasi .env

```bash
git clone -b deploy_android https://github.com/gerrymoeis/lab_kom_sim.git
cd lab_kom_sim

cp .env.example .env
nano .env
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
ANDROID=true
BACKUP_ENABLED=true
BACKUP_DIR=./backups
```

Lihat [Panduan .env Reference](#panduan-env-reference) untuk semua opsi.

---

## Build & Run

### Build

```bash
# WAJIB: CGO_ENABLED=0 + -tags nodynamic
# -tags nodynamic mencegah purego menggunakan CGO untuk dynamic library loading
CGO_ENABLED=0 go build -tags nodynamic -ldflags="-s -w" -o app-simlab ./cmd/server/main.go

# Atau pakai script
bash scripts/build_termux.sh
```

### Run

```bash
./app-simlab
```

Akses dari browser HP: `http://localhost:8080`

Akses dari laptop (via Tailscale): `http://100.x.x.x:8080`

**Default login:** `admin` / `admin123`

### Run di Background

Agar server tetap jalan meski terminal ditutup:

```bash
nohup ./app-simlab > server.log 2>&1 &
```

Cek log:
```bash
tail -f server.log
```

Matikan:
```bash
pkill app-simlab
```

---

## Auto Deploy (Laptop → HP via Tailscale + SSH)

### Setup Sekali (di Laptop)

```powershell
# PowerShell — simpan script ini sebagai deploy-hp.ps1
$tsHost = "100.x.x.x"  # Ganti dengan Tailscale IP HP
$sshPort = 8022
$sshKey = "C:\Users\Gallan\.ssh\id_sim_lab_mi"  # Path SSH private key

ssh -p $sshPort -i $sshKey "u0_aXXX@${tsHost}" 'cd ~/lab_kom_sim && git pull origin deploy_android && CGO_ENABLED=0 go build -tags nodynamic -o app-simlab ./cmd/server/main.go && pkill app-simlab; nohup ./app-simlab > server.log 2>&1 &'
```

### Satu Perintah Deploy

```bash
# Dari laptop (bash/WSL):
ssh -p 8022 -i ~/.ssh/id_sim_lab_mi u0_aXXX@100.x.x.x \
  'cd ~/lab_kom_sim && git pull origin deploy_android && \
   CGO_ENABLED=0 go build -tags nodynamic -ldflags="-s -w" -o app-simlab ./cmd/server/main.go && \
   pkill app-simlab; nohup ./app-simlab > server.log 2>&1 &'
```

### Auto-Deploy via Git Alias (Optional)

```bash
git config --global alias.deploy-hp "!git push origin refactoring && ssh -p 8022 -i ~/.ssh/id_sim_lab_mi u0_aXXX@100.x.x.x 'cd ~/lab_kom_sim && bash scripts/deploy.sh'"
```

---

## Maintenance

### Cek Proses Server

```bash
ps aux | grep app-simlab
```

### Cek Log

```bash
tail -f server.log
```

### Restart Server

```bash
pkill app-simlab
nohup ./app-simlab > server.log 2>&1 &
```

### Cek Disk Usage

```bash
df -h
du -sh ~/lab_kom_sim/backups
```

### Backup Database (Manual)

Backup otomatis sudah berjalan via `BACKUP_ENABLED=true`. Untuk backup manual:

```bash
cp inventaris_lab.db inventaris_lab.db.manual_$(date +%Y%m%d_%H%M%S)
```

---

## Auto-Deploy Workflow (GitHub Actions)

Branch `refactoring` memiliki workflow `.github/workflows/auto-deploy.yml` yang otomatis sync ke `deploy_android` setiap ada push.

**Cara trigger update:**

1. Push ke `refactoring` dari laptop
2. Workflow: test → merge → preserve deploy-specific files → build verify → push ke `deploy_android`
3. Di HP (Termux): `git pull origin deploy_android` → rebuild → restart

**README.md di branch deploy_android TIDAK akan ditimpa** oleh workflow — workflow mereset README.md ke versi deploy branch (OLD_HEAD) setiap sync, sehingga panduan spesifik Android tetap aman.

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

1. **Setup SSH key untuk git**: `ssh-keygen -t ed25519` → add public key ke GitHub Settings → SSH keys
2. Clone repo public site: `git clone git@github.com:user/public-repo.git ~/public-repo`
3. **Set di .env**:
   - `PUBLIC_BUILD_ENABLED=true`
   - `PUBLIC_BUILD_REPO_DIR=$HOME/public-repo`
   - `PUBLIC_BUILD_BRANCH=main`
4. Konfigurasi git di Termux: `git config --global user.name "name"` dan `git config --global user.email "email"`

Server akan rebuild & push otomatis tiap CUD operation (debounce `PUBLIC_BUILD_INTERVAL` detik). Git auth via SSH key — Termux support penuh.

### Async Write Mode

- `WRITE_MODE=sync` (default): setiap write langsung ke SQLite — aman, cocok untuk beban normal
- `WRITE_MODE=async`: queue-based batch writer — lebih cepat untuk burst request. Gunakan jika ada 50+ PC di-grid dengan concurrent akses tinggi

### Backup Multi-Path

Untuk redundancy, backup bisa dikirim ke multiple direktori sekaligus. Pisahkan dengan koma:

```bash
BACKUP_DIR="./backups, /storage/emulated/0/simlab-backups"
```

Path ke internal storage (`/storage/emulated/0/`) berguna agar backup bisa diakses tanpa Termux.

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
# SQLite: path relatif atau absolut
DATABASE_PATH=inventaris_lab.db
# PostgreSQL — kosongkan untuk SQLite
# DATABASE_URL=postgres://user:pass@...

# ============================
# WRITE MODE
# ============================
# sync (default) | async
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
# ANDROID MODE — WAJIB true untuk Termux
# ============================
ANDROID=true

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
| `go build` gagal dengan `purego` error | Tidak pakai `-tags nodynamic` | Build: `CGO_ENABLED=0 go build -tags nodynamic ...` |
| Tailscale tidak bisa `up` | `tailscaled` belum jalan | `tailscaled --tun=userspace-networking --statedir=$PREFIX/var/lib/tailscale &` |
| SSH connection refused | `sshd` belum jalan | Jalankan `sshd` di Termux. Cek port 8022 |
| Server mati setelah Termux di-close | Background process killed | Pakai `nohup ./app-simlab > log &` + `termux-wake-lock` |
| Build lambat | RAM HP terbatas | Tutup app lain. Atau build di laptop, SCP binary ke HP |
| Foto upload gagal/error | ANDROID=false di .env | Set `ANDROID=true` untuk client-side compress |
| Database `UNIQUE constraint` | Data duplikat | Normalisasi auto jalan di startup. Restart server |
| `git pull` conflict | Ada perubahan lokal | `git stash` sebelum pull, atau `git reset --hard origin/deploy_android` |
| Storage penuh | Backup menumpuk | Kecilkan `BACKUP_RETENTION`. Hapus backup lama: `rm backups/*.gz` |
| PostgreSQL gagal konek | `DATABASE_URL` salah / IP not allowlisted | Cek Neon dashboard → Connection details. Pastikan HP ada koneksi internet |
| SSG build tidak push ke git | SSH key belum terdaftar | `cat ~/.ssh/id_ed25519.pub` → add ke GitHub. Test: `ssh -T git@github.com` |
| Server lambat dengan banyak PC | WRITE_MODE=sync kena bottleneck | Ganti ke `WRITE_MODE=async` di .env |
