# Sistem Inventaris Laboratorium Komputer — Linux (Headless Server)

Branch `deploy_linux` — khusus deployment ke **Linux server headless** (tanpa desktop UI). Semua operasi via terminal/SSH.

Database: **SQLite** (pure Go via `modernc.org/sqlite`, zero CGO). PostgreSQL/Neon tetap tersedia untuk scale up.

---

## Daftar Isi

1. [Prasyarat](#prasyarat)
2. [Instalasi Tailscale (Headless)](#instalasi-tailscale-headless)
3. [Clone & Konfigurasi .env](#clone--konfigurasi-env)
4. [Build & Run](#build--run)
5. [systemd Service (Production)](#systemd-service-production)
6. [Firewall](#firewall)
7. [Auto-Deploy Workflow (GitHub Actions)](#auto-deploy-workflow-github-actions)
8. [Maintenance](#maintenance)
9. [Panduan .env Reference](#panduan-env-reference)
10. [Troubleshooting](#troubleshooting)

---

## Prasyarat

- **OS**: Linux 64-bit (x86_64 atau aarch64/ARM64) — Debian, Ubuntu, Fedora, Arch, dll
- **Tidak perlu C compiler** — SQLite pure Go (modernc.org/sqlite, `CGO_ENABLED=0`)
- **systemd** — untuk production service (hampir semua distro modern)
- **Koneksi internet** — untuk download dependencies dan GitHub sync
- **Akun Tailscale** — [daftar gratis](https://login.tailscale.com)

### 1. Install Git

Git diperlukan untuk clone repositori dan auto-update.

**Debian / Ubuntu / Mint:**
```bash
sudo apt update
sudo apt install -y git
```

**Fedora / RHEL / CentOS:**
```bash
sudo dnf install -y git
```

**Arch / Manjaro:**
```bash
sudo pacman -S --noconfirm git
```

**openSUSE:**
```bash
sudo zypper install -y git
```

**Verifikasi:**
```bash
git version
# Output: git version 2.x.x
```

### 2. Install Go (Golang)

> **Perhatian:** Project ini membutuhkan **Go 1.25+**. Versi package manager bawaan distro mungkin lebih lama — gunakan official tarball dari go.dev untuk hasil terjamin.

**Metode 1 (Recommended) — Official Tarball dari go.dev:**

Cara ini memberikan versi Go terbaru (saat ini **Go 1.26.4**) dan tidak bergantung pada package manager.

```bash
# Deteksi arsitektur CPU
ARCH=$(uname -m)
if [ "$ARCH" = "x86_64" ]; then
    GOARCH="amd64"
elif [ "$ARCH" = "aarch64" ]; then
    GOARCH="arm64"
else
    echo "Arsitektur tidak didukung: $ARCH"
    exit 1
fi

# Download Go 1.26.4
wget https://go.dev/dl/go1.26.4.linux-${GOARCH}.tar.gz

# Hapus instalasi lama (jika ada) dan extract
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.26.4.linux-${GOARCH}.tar.gz

# Tambahkan Go ke PATH (tambahkan ke ~/.profile agar permanen)
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.profile
source ~/.profile
```

**Metode 2 — Package Manager (Go 1.26 untuk Ubuntu 26.04+):**

Gunakan jika distro Anda sudah menyediakan Go 1.25+.

```bash
# Debian 13+ / Ubuntu 26.04+:
sudo apt update
sudo apt install -y golang-go

# Fedora 42+:
sudo dnf install -y golang

# Arch:
sudo pacman -S --noconfirm go
```

**Verifikasi:**
```bash
go version
# Output: go version go1.26.4 linux/amd64
```

### 3. Cek systemd

```bash
systemctl --version
# Output: systemd 250+ (hampir semua distro modern sudah termasuk)
```

### 4. Siapkan SSH Key (untuk GitHub)

Diperlukan untuk git push dari server (misal untuk SSG auto-build):

```bash
ssh-keygen -t ed25519 -C "server-linux@example.com"
cat ~/.ssh/id_ed25519.pub
# Copy output → tambahkan ke GitHub → Settings → SSH and GPG keys
```

Test koneksi:
```bash
ssh -T git@github.com
# Output: Hi username! You've successfully authenticated...
```

---

## Instalasi Tailscale (Headless)

Karena server Linux headless (tanpa browser), kita perlu **auth key** untuk autentikasi.

### 1. Generate Auth Key

1. Buka [Tailscale Admin Console → Keys](https://login.tailscale.com/admin/settings/keys)
2. Klik **Generate auth key**
3. Setting:
   - **Reusable**: ✅ centang (agar bisa dipakai untuk multiple device)
   - **Expiry**: pilih `30 days` atau `Never`
   - **Tags**: opsional (contoh: `tag:server`)
4. Klik **Generate key**
5. **Copy** auth key — format: `tskey-auth-xxxxxxxxxxxxxxxxxxxxxxxx`

### 2. Install Tailscale

```bash
# Metode 1: Official install script (recommended)
curl -fsSL https://tailscale.com/install.sh | sh

# Metode 2: Package manager
# Debian/Ubuntu:
sudo apt update && sudo apt install -y tailscale

# Fedora/RHEL:
sudo dnf install -y tailscale

# Arch:
sudo pacman -S tailscale

# openSUSE:
sudo zypper install tailscale
```

### 3. Start & Authenticate

```bash
# Start daemon
sudo systemctl enable --now tailscaled

# Authenticate dengan auth key (headless — tidak perlu browser)
sudo tailscale up --authkey tskey-auth-xxxxxxxxxxxxxxxxxxxxxxxx

# Verifikasi koneksi
tailscale status
tailscale ip    # Output: 100.x.x.x (Tailscale IP)
```

Catat Tailscale IP — ini yang akan dipakai untuk SSH dari device lain.

### 4. SSH Access dari Laptop/Device Lain

Pastikan device lain juga sudah join ke tailnet yang sama.

```bash
# Dari laptop: SSH ke server via Tailscale IP atau hostname
ssh user@100.x.x.x

# Atau pakai MagicDNS (hostname)
ssh user@hostname.tail-network.ts.net
```

---

## Clone & Konfigurasi .env

```bash
# Clone
git clone -b deploy_linux https://github.com/gerrymoeis/lab_kom_sim.git
cd lab_kom_sim

# Copy .env.example
cp .env.example .env

# Edit .env — setidaknya SESSION_SECRET, GEMINI_API_KEY, OPENROUTER_API_KEY
nano .env
```

**Konfigurasi minimal untuk production:**

```env
ENVIRONMENT=production
HOST=0.0.0.0
PORT=8080
DATABASE_PATH=/opt/simlab/app/data/inventaris_lab.db
SESSION_SECRET=generate-random-string-panjang-32-64-karakter
TIMEZONE=Asia/Jakarta
UPLOAD_PATH=/opt/simlab/app/data/uploads
GEMINI_API_KEY=your-gemini-api-key
OPENROUTER_API_KEY=sk-or-your-openrouter-api-key
BACKUP_ENABLED=true
BACKUP_DIR=/opt/simlab/app/data/backups
```

Lihat [Panduan .env Reference](#panduan-env-reference) untuk semua opsi.

---

## Build & Run

### Build Binary

```bash
# Build static binary (zero dependencies)
CGO_ENABLED=0 go build -ldflags="-s -w" -o app-simlab ./cmd/server/main.go

# Atau pakai script
bash scripts/build-linux.sh
```

### Run Langsung (Testing)

```bash
./app-simlab
```

Akses: `http://localhost:8080`

**Default login:** `admin` / `admin123`

### Atur PATH Production

```bash
sudo mkdir -p /opt/simlab/app/data
sudo mv app-simlab /opt/simlab/app/
sudo mv .env /opt/simlab/app/
sudo mv uploads /opt/simlab/app/data/
```

---

## systemd Service (Production)

### Via Script (Auto)

```bash
sudo bash scripts/deploy-linux.sh --install-service
```

### Manual

```bash
# Copy systemd unit
sudo cp scripts/inventaris-lab.service /etc/systemd/system/

# Buat directory
sudo mkdir -p /opt/simlab/app/data/uploads /opt/simlab/app/data/backups
sudo mkdir -p /etc/simlab

# Copy binary & config
sudo cp app-simlab /opt/simlab/app/
sudo cp .env /etc/simlab/

# Set permissions
sudo chown -R simlab:simlab /opt/simlab/ 2>/dev/null || true
sudo chmod 755 /opt/simlab/app/app-simlab

# Reload systemd & enable service
sudo systemctl daemon-reload
sudo systemctl enable --now inventaris-lab

# Cek status
sudo systemctl status inventaris-lab

# Lihat log real-time
sudo journalctl -u inventaris-lab -f
```

### Service Management

```bash
sudo systemctl restart inventaris-lab   # Restart
sudo systemctl stop inventaris-lab      # Stop
sudo systemctl start inventaris-lab     # Start
sudo journalctl -u inventaris-lab -n 100 --no-pager  # Last 100 lines
```

---

## Firewall

Jika server perlu diakses dari device **di luar Tailscale** (LAN lokal), buka port:

```bash
# Via script
sudo bash scripts/setup-firewall-linux.sh

# Manual — ufw
sudo ufw allow 8080/tcp

# Manual — firewalld
sudo firewall-cmd --add-port=8080/tcp --permanent
sudo firewall-cmd --reload

# Manual — iptables
sudo iptables -A INPUT -p tcp --dport 8080 -j ACCEPT
```

**Rekomendasi:** Jangan expose port ke internet publik. Akses via Tailscale saja lebih aman (zero-trust network).

---

## Auto-Deploy Workflow (GitHub Actions)

Branch `refactoring` memiliki GitHub Actions workflow (`.github/workflows/auto-deploy.yml`) yang:

1. **Trigger**: Setiap push ke branch `refactoring`
2. **Test**: Run `go test ./...` (unit + integration)
3. **Sync**: Merge `refactoring` ke `deploy_linux` (dan `deploy_android`, `deploy_windows`)
4. **Preserve**: README.md, scripts, dan file spesifik deploy_linux tetap dipertahankan
5. **Build verify**: Run `go build ./...` + `go vet ./...`
6. **Push**: Jika ada perubahan, push ke `deploy_linux`

**Untuk trigger update dari laptop:**

```bash
# Push ke refactoring → workflow otomatis sync ke deploy_linux
git push origin refactoring

# Workflow akan:
#   1. Run tests
#   2. Merge refactoring → deploy_linux
#   3. Restore deploy-specific files (README.md, scripts/)
#   4. Push deploy_linux
```

**Di server Linux, tarik update:**

```bash
# SSH ke server
ssh user@100.x.x.x
cd ~/lab_kom_sim
git pull origin deploy_linux
bash scripts/deploy-linux.sh
```

Atau setup `deploy.sh` (yang sudah ada) untuk auto-pull + build + restart:

```bash
# Di laptop, satu perintah:
ssh -p 8022 user@tailscale-ip 'cd ~/lab_kom_sim && bash scripts/deploy.sh'
```

---

## Maintenance

### Backup Database

Backup sudah otomatis via `BACKUP_ENABLED=true` — trigger tiap CUD, debounce 30 detik. Lokasi: `BACKUP_DIR`.

Restore manual:
```bash
cp backups/inventaris_lab.db.backup_20260613_120405 inventaris_lab.db
sudo systemctl restart inventaris-lab
```

### Logs

```bash
# Systemd journal
sudo journalctl -u inventaris-lab -n 200 -f

# Atau log file jika dikonfigurasi
tail -f /var/log/simlab/app.log
```

### Update Go Version

Cek `go.mod` untuk version requirement. Install versi baru:
```bash
# Download Go binary (misal 1.26.0)
wget https://go.dev/dl/go1.26.0.linux-amd64.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.26.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
go version
```

### Reset Database (Kembali ke Awal)

```bash
# Hapus database
rm /opt/simlab/app/data/inventaris_lab.db
sudo systemctl restart inventaris-lab
# Database akan auto-create dengan data seed saat pertama kali server jalan
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

1. Clone repo public site: `git clone git@github.com:user/public-repo.git /opt/simlab/public/repo`
2. **Set di .env**:
   - `PUBLIC_BUILD_ENABLED=true`
   - `PUBLIC_BUILD_REPO_DIR=/opt/simlab/public/repo`
   - `PUBLIC_BUILD_BRANCH=main`
3. **Konfigurasi git auth** (SSH key atau HTTPS PAT) — pastikan push tanpa password

Server akan rebuild & push otomatis tiap CUD operation (debounce `PUBLIC_BUILD_INTERVAL` detik).

### Async Write Mode

- `WRITE_MODE=sync` (default): setiap write langsung ke SQLite — aman, cocok untuk beban normal
- `WRITE_MODE=async`: queue-based batch writer — lebih cepat untuk burst request, write di-batch dalam 1 transaksi. Gunakan jika ada 50+ PC di-grid dengan concurrent akses tinggi

### Backup Multi-Path

Untuk redundancy, backup bisa dikirim ke multiple direktori sekaligus. Pisahkan dengan koma (spasi di path pakai quotes):

```bash
BACKUP_DIR="/opt/simlab/app/data/backups, /mnt/nas/backups, /opt/simlab/app/data/backups_secondary"
```

---

## Panduan .env Reference

Semua konfigurasi aplikasi via file `.env`. Copy dari `.env.example`.

```env
# ============================
# APLIKASI
# ============================

# Environment: development | production
ENVIRONMENT=production

# Host binding — 0.0.0.0 untuk akses dari luar localhost
HOST=0.0.0.0

# Port aplikasi
PORT=8080


# ============================
# DATABASE
# ============================

# SQLite: path absolut di production agar survive symlink swap deploy
# Contoh: /opt/simlab/app/data/inventaris_lab.db
DATABASE_PATH=inventaris_lab.db

# PostgreSQL (Neon DB) — isi untuk pakai PostgreSQL, kosongkan untuk SQLite
# DATABASE_URL=postgres://user:pass@ep-xxx.ap-southeast-1.aws.neon.tech/neondb?sslmode=require


# ============================
# WRITE MODE
# ============================

# sync (default): langsung tulis ke SQLite
# async: queue-based batch writer (lebih cepat untuk burst request)
WRITE_MODE=sync


# ============================
# SECURITY
# ============================

# Session secret — WAJIB ganti di production! Minimal 32 karakter random.
# Generate: openssl rand -hex 32
SESSION_SECRET=change-this-secret-in-production-to-random-string


# ============================
# TIMEZONE
# ============================

# IANA timezone — Asia/Jakarta (WIB), Asia/Makassar (WITA), Asia/Jayapura (WIT)
TIMEZONE=Asia/Jakarta


# ============================
# UPLOAD
# ============================

# Path upload — gunakan absolute path di production
# Contoh: /opt/simlab/app/data/uploads
UPLOAD_PATH=uploads


# ============================
# OCR API KEYS
# ============================

# Google Gemini API Key (fallback OCR)
GEMINI_API_KEY=your-gemini-api-key-here

# OpenRouter API Key (primary OCR — pakai free vision model)
# Daftar: https://openrouter.ai/keys
OPENROUTER_API_KEY=sk-or-your-openrouter-api-key-here


# ============================
# ANDROID MODE
# ============================

# false untuk Linux server (server-side compress)
ANDROID=false


# ============================
# PC PHOTO SEEDING (via GitHub Releases)
# ============================

# URL ZIP foto PC — kosongkan jika tidak perlu seeding
PC_PHOTO_RELEASE_URL=
# GitHub PAT (read-only akses ke repo pc-photos)
GITHUB_TOKEN=


# ============================
# PAGINATION
# ============================

# Default page size untuk semua list view (default: 25)
DEFAULT_PAGE_SIZE=25


# ============================
# BACKUP (SQLite only)
# ============================

# Backup otomatis — trigger via CUD, debounce BACKUP_INTERVAL detik
BACKUP_ENABLED=true

# Debounce interval (detik) — 30 = backup 30 detik setelah CUD terakhir
BACKUP_INTERVAL=30

# Direktori backup — comma-separated, pakai quotes jika ada spasi
BACKUP_DIR=/opt/simlab/app/data/backups

# Retention — jumlah file backup maksimal
BACKUP_RETENTION=20

# Minimum disk space (MB) — skip backup jika kurang
BACKUP_MIN_DISK_MB=500

# Kompresi backup (.gz)
BACKUP_COMPRESS=true


# ============================
# PUBLIC SITE (SSG Auto-Build)
# ============================

# Static site generator — rebuild public site tiap CUD
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
| `CGO_ENABLED=0` build gagal | Go version outdated | `go version` — harus 1.25+ |
| `tailscaled` tidak bisa start | Kernel module `tun` tidak ada | `sudo modprobe tun` atau `tailscaled --tun=userspace-networking` |
| Server tidak bisa diakses via Tailscale | Firewall port 8080 | Pastikan `HOST=0.0.0.0` (bukan localhost) |
| Database error `UNIQUE constraint` | Data duplikat | Normalisasi otomatis jalan di startup. Jika masih error, hapus row duplikat manual via SQLite CLI |
| `exec format error` saat run binary | Build untuk arsitektur salah | `go env GOARCH` — harus `amd64` atau `arm64` sesuai server |
| Backup disk penuh | Retention terlalu besar | Kecilkan `BACKUP_RETENTION` atau `BACKUP_MIN_DISK_MB` |
| OCR gagal terus | API key expired/invalid | Cek `.env` → `GEMINI_API_KEY` dan `OPENROUTER_API_KEY` |
| Foto tidak muncul di upload | Path upload salah | Pastikan `UPLOAD_PATH` absolute path dan writable |
| PostgreSQL gagal konek | `DATABASE_URL` salah / IP not allowlisted | Cek Neon dashboard → Connection details. Allowlist IP server di Neon |
| SSG build tidak push ke git | Git auth belum diatur | Setup SSH key atau HTTPS PAT. Test: `git push --dry-run` dari `PUBLIC_BUILD_REPO_DIR` |
| Server lambat dengan banyak PC | WRITE_MODE=sync kena bottleneck | Ganti ke `WRITE_MODE=async` di .env
