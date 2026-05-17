# Sistem Inventaris Laboratorium Komputer — Linux

Deployment untuk Linux. Menggunakan **SQLite** sebagai database lokal via pure Go driver (no C compiler needed). PostgreSQL/Neon code tetap tersedia untuk scale up di masa depan.

HEIC menggunakan **native libheif** (lebih cepat dari WASM). Jika libheif tidak terinstall, otomatis fallback ke WASM decoder.

## Tech Stack

- **Backend**: Go 1.25+ dengan Gin Framework
- **Database**: SQLite (lokal) — pure Go (modernc.org/sqlite, no CGO)
- **Frontend**: Bootstrap 5 + vanilla JS
- **OCR**: OpenRouter (primary) → Google Gemini (fallback)
- **Image**: Native libheif (Linux) — WASM fallback jika tidak tersedia

## Struktur Project

```
poc_prototype/
├── cmd/server/          # Entry point aplikasi
├── internal/
│   ├── config/          # Konfigurasi (.env)
│   ├── database/        # Database (SQLite + PostgreSQL)
│   ├── models/          # Data models
│   ├── handlers/        # HTTP handlers
│   ├── services/        # Business logic
│   └── middleware/      # Auth, session
├── web/templates/       # HTML templates (Go templates)
├── web/static/          # CSS, JS, vendor
├── uploads/             # Upload foto
└── scripts/
    ├── build-linux.sh       # Build static binary
    ├── deploy-linux.sh      # Deploy (systemd / nohup)
    ├── setup-firewall-linux.sh  # Firewall setup
    ├── inventaris-lab.service   # systemd unit
    └── download-vendor.sh   # Download vendor assets
```

## Deploy ke Linux

### Prasyarat

- Linux 64-bit (x86_64 atau aarch64)
- [Go 1.25+](https://go.dev/dl/) — pastikan `go` available di PATH
- **Tidak perlu** C compiler — SQLite pure Go (modernc.org/sqlite)
- Git (opsional, untuk clone)

### Setup

```bash
# Clone repositori
git clone -b deploy_linux https://github.com/gerrymoeis/lab_kom_sim.git
cd lab_kom_sim

# Buat .env dari contoh
cp .env.example .env

# Edit .env sesuai kebutuhan
nano .env
```

Isi `.env`:
```env
ENVIRONMENT=production
HOST=0.0.0.0
PORT=8080
DATABASE_PATH=inventaris_lab.db
SESSION_SECRET=random-string
GEMINI_API_KEY=your-key
OPENROUTER_API_KEY=sk-or-your-key
```

### Build & Jalankan

```bash
# Build static binary
CGO_ENABLED=0 go build -ldflags="-s -w" -o app-simlab ./cmd/server/main.go

# Atau pakai script
bash scripts/build-linux.sh

# Jalankan langsung
./app-simlab

# Atau deploy script (otomatis pilih systemd atau nohup)
bash scripts/deploy-linux.sh
```

Akses: http://localhost:8080

### Instalasi systemd Service (Production)

```bash
# Build + install service (sekalian)
sudo bash scripts/deploy-linux.sh --install-service

# Atau manual:
sudo cp scripts/inventaris-lab.service /etc/systemd/system/
sudo mkdir -p /opt/inventaris-lab /etc/inventaris-lab /var/lib/inventaris-lab
sudo cp app-simlab /opt/inventaris-lab/
sudo cp .env /etc/inventaris-lab/
sudo cp -r uploads /var/lib/inventaris-lab/
sudo systemctl daemon-reload
sudo systemctl enable --now inventaris-lab

# Cek status
sudo systemctl status inventaris-lab

# Lihat log
sudo journalctl -u inventaris-lab -f
```

### HEIC Native (Opsional — Performa Lebih Baik)

Secara default HEIC fallback ke WASM decoder. Untuk native performance:

```bash
# Debian / Ubuntu
sudo apt install libheif-dev

# Fedora / RHEL
sudo dnf install libheif-devel

# Arch Linux
sudo pacman -S libheif

# openSUSE
sudo zypper install libheif-devel
```

Native loading otomatis terdeteksi — tidak perlu rebuild.

### Firewall (Akses dari Device Lain)

```bash
# Jalankan sebagai root
sudo bash scripts/setup-firewall-linux.sh

# Atau manual:
# ufw:    sudo ufw allow 8080/tcp
# firewalld: sudo firewall-cmd --add-port=8080/tcp --permanent
# iptables:  sudo iptables -A INPUT -p tcp --dport 8080 -j ACCEPT
```

## Perbedaan dengan Branch Lain

| Aspek | deploy_linux | deploy_android | deploy_windows |
|-------|-------------|----------------|----------------|
| OS Target | Linux | Android (Termux) | Windows |
| Database | SQLite (pure Go) | SQLite (CGO) | SQLite (pure Go) |
| C Compiler | Tidak perlu (modernc) | WAJIB (gcc via pkg) | Tidak perlu (modernc) |
| Build | `CGO_ENABLED=0` | `CGO_ENABLED=1 -tags nodynamic` | `CGO_ENABLED=0` |
| HEIC | **Native libheif** | WASM via wazero | WASM via wazero |
| Service | systemd / nohup | Termux bootstrap | NSSM / background |

## Fitur

- ✅ Dashboard grid 40 PC (8×5) dengan status color-coded
- ✅ CRUD PC dengan upload foto serial & front panel
- ✅ Manajemen perangkat (device types, loans, usages)
- ✅ Software catalog (required + others)
- ✅ OCR logbook absensi via OpenRouter → Gemini (retry + fallback)
- ✅ Activity log / audit trail (success + failure)
- ✅ Export Excel (PC, device, logbook, software catalog)
- ✅ HEIC/HEIF photo upload (native libheif + WASM fallback)
- ✅ SQLite database (pure Go) — PostgreSQL siap scale up
- ✅ systemd service + nohup fallback untuk semua distro

## Default Login

- **Username**: admin
- **Password**: admin123

## Catatan Penting

- **Database**: `DATABASE_URL` kosong = SQLite lokal, diisi = PostgreSQL/Neon
- **CGO**: `CGO_ENABLED=0` — tidak perlu C compiler, SQLite via pure Go (modernc.org/sqlite)
- **HEIC**: Native via libheif (auto-detect). Install `libheif` untuk performa maksimal.
- **Cross-distro**: Static binary (musl/glibc compatible). Script mendukung systemd & nohup.
- **Image**: HEIC konversi server-side native + WASM fallback
- **OCR**: OpenRouter primary (free vision model), Gemini fallback jika gagal
