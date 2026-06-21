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

Git diperlukan untuk clone repositori dan auto-update. Cek dulu apakah sudah terinstall:

```bash
which git
# Jika output: /usr/bin/git → sudah terinstall (skip ke step 2)
# Jika tidak ada output → lanjut install di bawah
```

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

📎 [Official docs: git-scm.com](https://git-scm.com/book/en/v2/Getting-Started-Installing-Git)

### 2. Install Go (Golang)

Cek dulu apakah sudah terinstall dan versinya:

```bash
go version
# Jika output: go version go1.25+ → sudah terinstall (skip ke step 3)
# Jika command not found → lanjut install di bawah
```

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

📎 [Official docs: go.dev/doc/install](https://go.dev/doc/install)

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

**Setup passwordless SSH (agar deploy satu perintah tanpa password):**

```bash
# Di laptop, generate SSH key (jika belum punya)
ssh-keygen -t ed25519 -C "laptop@example.com"

# Copy public key ke server
ssh-copy-id user@100.x.x.x

# Test — login tanpa password prompt
ssh user@100.x.x.x
```

---

## Clone & Konfigurasi .env

```bash
# Clone
git clone -b deploy_linux https://github.com/gerrymoeis/lab_kom_sim.git
cd lab_kom_sim

# Copy .env.example
cp .env.example .env

# Generate SESSION_SECRET random (64 karakter hex)
SESSION_SECRET=$(openssl rand -hex 32)
echo "SESSION_SECRET=$SESSION_SECRET"

# Edit .env — set SESSION_SECRET, GEMINI_API_KEY, OPENROUTER_API_KEY
nano .env
```

**Konfigurasi minimal — Single-Lab (copy-paste dengan nilai Anda):**

```env
ENVIRONMENT=production
HOST=0.0.0.0
PORT=8080
DATABASE_PATH=/opt/simlab/app/data/inventaris_lab.db
SESSION_SECRET=isi-dengan-output-openssl-rand-hex-32
COOKIE_SECURE=false
TIMEZONE=Asia/Jakarta
UPLOAD_PATH=/opt/simlab/app/data/uploads
GEMINI_API_KEY=your-gemini-api-key
OPENROUTER_API_KEY=sk-or-your-openrouter-api-key
BACKUP_ENABLED=true
BACKUP_DIR=/opt/simlab/app/data/backups
```

**Konfigurasi Multi-Lab (ganti DATABASE_PATH + tambah LABS):**

```env
GLOBAL_DB_PATH=/opt/simlab/app/data/global.db        # DB global (users, permissions)
LABS=MI-1:/opt/simlab/app/data/lab_mi_1.db:Lab Kom MI 1:lab-kom-mi,VOKASI-1:/opt/simlab/app/data/lab_vokasi_1.db:Lab Kom Vokasi:vokasi
# Format LABS: LAB-ID:dbPath:Title:urlPath (comma-separated)
# Saat LABS diisi, DATABASE_PATH diabaikan — setiap lab punya DB sendiri.
# LAB-ID = lookup folder seeds/<lowercase(LAB-ID)>/
# urlPath = routing slug (menentukan cookie, upload, backup folder per-lab)
```

> **Catatan:** `openssl rand -hex 32` menghasilkan 64 karakter hex random. Setiap server harus punya `SESSION_SECRET` unik.

Lihat file `.env.example` (auto-sync dari branch refactoring) untuk dokumentasi lengkap semua opsi.

---

## Build & Run

### Build Binary

```bash
# Build static binary untuk Linux (zero dependencies)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o app-simlab ./cmd/server/main.go

# Atau pakai script (recommended)
bash scripts/build-linux.sh
```

> **Catatan:** Jika build di **Windows**, pastikan `GOOS=linux GOARCH=amd64` diset (seperti contoh di atas). Tanpa ini, `go build` menghasilkan binary Windows (PE32+) yang tidak bisa jalan di Linux. Rekomendasi: build langsung di server Linux untuk hasil terjamin.

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
sudo cp scripts/simlab.service /etc/systemd/system/

# Buat directory
sudo mkdir -p /opt/simlab/app/data/uploads /opt/simlab/app/data/backups
sudo mkdir -p /opt/simlab

# Copy binary & config
sudo cp app-simlab /opt/simlab/app/
sudo cp .env /opt/simlab/

# Set permissions
sudo chown -R simlab:simlab /opt/simlab/ 2>/dev/null || true
sudo chmod 755 /opt/simlab/app/app-simlab

# Reload systemd & enable service
sudo systemctl daemon-reload
sudo systemctl enable --now simlab

# Cek status
sudo systemctl status simlab

# Lihat log real-time
sudo journalctl -u simlab -f
```

### Service Management

```bash
sudo systemctl restart simlab   # Restart
sudo systemctl stop simlab      # Stop
sudo systemctl start simlab     # Start
sudo journalctl -u simlab -n 100 --no-pager  # Last 100 lines
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

## Auto-Deploy (Update Server dari Laptop)

**Cara kerja:** Push ke `refactoring` → GitHub Actions otomatis test + sync ke `deploy_linux` (~90 detik) → kamu SSH + deploy.

### Setup (Sekali)

**1. Passwordless SSH** — agar bisa SSH tanpa prompt password:

```bash
# Di laptop (bash/WSL), generate SSH key jika belum punya
ssh-keygen -t ed25519 -C "laptop@example.com"

# Copy public key ke server
ssh-copy-id user@100.x.x.x

# Test — login tanpa password prompt
ssh user@100.x.x.x
```

**2. (Opsional) Git Alias** — agar deploy cukup 1 perintah dari laptop:

```bash
git config --global alias.deploy-lin "!git push origin refactoring && ssh user@100.x.x.x 'cd ~/lab_kom_sim && bash scripts/deploy.sh'"
# Setelah ini: git deploy-lin → push refactoring + deploy ke server
```

### Satu Perintah Deploy (Setiap Update)

```bash
# 1. Push perubahan ke refactoring
git push origin refactoring

# 2. Tunggu ~90 detik sampai workflow selesai
#    (cek progress di GitHub → Actions tab)

# 3. Deploy ke server — pull + build + restart
ssh user@100.x.x.x 'cd ~/lab_kom_sim && bash scripts/deploy.sh'
```

---

## Maintenance

### Backup Database

Backup otomatis tiap ada perubahan data (CUD operation, debounce 30 detik). Konfigurasi via `.env`:

```bash
BACKUP_ENABLED=true        # Aktif
BACKUP_INTERVAL=30         # 30 detik setelah CUD terakhir
BACKUP_DIR=/opt/simlab/app/data/backups   # Lokasi backup
BACKUP_RETENTION=20        # Simpan 20 backup terakhir
BACKUP_COMPRESS=true       # Kompres .gz
```

Cek daftar backup:
```bash
ls -lh /opt/simlab/app/data/backups/
```

Restore dari backup:
```bash
# 1. Hentikan server
sudo systemctl stop simlab

# 2. Backup DB corrupt (untuk investigasi)
cp /opt/simlab/app/data/inventaris_lab.db /opt/simlab/app/data/inventaris_lab.db.corrupt

# 3. Copy backup terbaru
cp /opt/simlab/app/data/backups/inventaris_lab.db.backup_20260613_120405 /opt/simlab/app/data/inventaris_lab.db

# 4. Start server
sudo systemctl start simlab
```

### Database Recovery (Migration Failure)

Server auto-run migration setiap startup. Jika setelah update server langsung crash atau error "migration failed", lakukan:

**1. Cek log untuk tahu penyebab:**

```bash
sudo journalctl -u simlab -n 100 --no-pager | grep -i "migration\|error\|fatal"
```

**2. Restore database dari backup (jika corruption):**

```bash
# Cari backup terbaru
ls -t /opt/simlab/app/data/backups/ | head -5

# Hentikan server
sudo systemctl stop simlab

# Backup DB corrupt untuk analisis
cp /opt/simlab/app/data/inventaris_lab.db /opt/simlab/app/data/inventaris_lab.db.corrupt

# Balikkan ke backup sebelum update
cp $(ls -t /opt/simlab/app/data/backups/ | head -1) /opt/simlab/app/data/inventaris_lab.db

# Start server (akan re-run migration yang sudah sukses sebelumnya — skip)
sudo systemctl start simlab
```

**3. Rollback binary (jika bug di kode baru):**

```bash
# Ambil binary lama dari backup git
cd ~/lab_kom_sim
git log --oneline -5 origin/deploy_linux
git checkout COMMIT_HASH_SEBELUMNYA -- cmd/ go.mod go.sum internal/ web/
CGO_ENABLED=0 go build -ldflags="-s -w" -o app-simlab ./cmd/server/main.go
sudo cp app-simlab /opt/simlab/app/app-simlab
sudo systemctl restart simlab
```

**4. Reset ke database baru (jika semua gagal):**

```bash
sudo systemctl stop simlab
rm /opt/simlab/app/data/inventaris_lab.db
sudo systemctl start simlab
# Database baru akan ter-create + seed data otomatis
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

Dokumentasi lengkap semua environment variable ada di file `.env.example` (auto-sync dari branch `refactoring` via GitHub Actions). File ini selalu terupdate — buka langsung di server:

```bash
cat .env.example | grep -E "^(#|$|[A-Z])" | head -80
```

Atau lihat [.env.example](.env.example) untuk semua opsi termasuk: LABS multi-lab, GLOBAL_DB_PATH, LOG_RETENTION_DAYS, WRITE_MODE async, PC photo seeding, dan PUBLIC_BUILD.

### Catatan Penting

- **Multi-Lab:** Saat `LABS` diisi, setiap lab punya database, session (cookie `inventaris_session_{urlPath}`), upload folder (`uploads/{urlPath}/`), dan backup folder sendiri. `DATABASE_PATH` diabaikan.
- **Global DB** (`GLOBAL_DB_PATH`): Menyimpan user global, lab_permissions, grid_layouts — wajib ada bahkan di mode single-lab.
- **Auto-Sync Middleware:** Setiap kali user login ke lab, sistem otomatis membuat/update row di per-lab `users` table (sync `full_name`, `role`, `is_super_admin`). Jadi data global cukup diatur di `/admin/users` — per-lab users sinkron otomatis.
- **DATABASE_URL** (PostgreSQL/Neon): Jika diisi, semua SQLite path diabaikan — server menggunakan PostgreSQL. Berlaku untuk semua lab (single database server).

---

## Troubleshooting

| Masalah | Penyebab | Solusi |
|---------|----------|--------|
| `CGO_ENABLED=0` build gagal | Go version outdated | `go version` — harus 1.25+ |
| `tailscaled` tidak bisa start | Kernel module `tun` tidak ada | `sudo modprobe tun` atau `tailscaled --tun=userspace-networking` |
| Server tidak bisa diakses via Tailscale | Firewall port 8080 | Pastikan `HOST=0.0.0.0` (bukan localhost) |
| Database error `UNIQUE constraint` | Data duplikat | Normalisasi otomatis jalan di startup. Jika masih error, hapus row duplikat manual via SQLite CLI |
| `exec format error` saat run binary | Build untuk OS/arsitektur salah (misal binary Windows diupload ke Linux) | Cek dengan `file ./binary` — harus `ELF 64-bit`. Build dengan `GOOS=linux GOARCH=amd64 CGO_ENABLED=0`, atau build langsung di server Linux |
| Backup disk penuh | Retention terlalu besar | Kecilkan `BACKUP_RETENTION` atau `BACKUP_MIN_DISK_MB` |
| OCR gagal terus | API key expired/invalid | Cek `.env` → `GEMINI_API_KEY` dan `OPENROUTER_API_KEY` |
| Foto tidak muncul di upload | Path upload salah | Pastikan `UPLOAD_PATH` absolute path dan writable |
| PostgreSQL gagal konek | `DATABASE_URL` salah / IP not allowlisted | Cek Neon dashboard → Connection details. Allowlist IP server di Neon |
| SSG build tidak push ke git | Git auth belum diatur | Setup SSH key atau HTTPS PAT. Test: `git push --dry-run` dari `PUBLIC_BUILD_REPO_DIR` |
| Server lambat dengan banyak PC | WRITE_MODE=sync kena bottleneck | Ganti ke `WRITE_MODE=async` di .env |
| Login selalu 403 Forbidden | `COOKIE_SECURE=true` tapi server HTTP | Set `COOKIE_SECURE=false` di `.env` jika server belum HTTPS |
