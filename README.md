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
- **Go 1.25+** — download dari [go.dev/dl](https://go.dev/dl/) atau via package manager
- **Tidak perlu C compiler** — SQLite pure Go (modernc.org/sqlite, `CGO_ENABLED=0`)
- **systemd** — untuk production service (hampir semua distro modern)
- **Git** — untuk clone repositori
- **Koneksi internet** — untuk download dependencies dan GitHub sync
- **Akun Tailscale** — [daftar gratis](https://login.tailscale.com)

**Cek prasyarat:**
```bash
go version    # Harus go1.25+
systemctl --version  # Harus ada (systemd 250+)
git version
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
