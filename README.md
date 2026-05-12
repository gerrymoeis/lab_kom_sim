# Sistem Inventaris Laboratorium Komputer

Sistem manajemen inventaris laboratorium komputer dengan visualisasi grid PC, OCR logbook, tracking software & perangkat.

## Tech Stack

- **Backend**: Go 1.25+ dengan Gin Framework
- **Database**: SQLite (lokal) / PostgreSQL / Neon DB (production)
- **Frontend**: HTMX + Alpine.js + Bootstrap 5
- **OCR**: Google Gemini API
- **Image**: WASM-based HEIC decoder (gen2brain/heic)

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
├── web/templates/       # HTML templates
├── web/static/          # CSS, JS
├── uploads/             # Upload foto
└── scripts/
    ├── build_termux.sh  # Build untuk Android/Termux
    └── reset_database.sql / reset_database_pg.sql
```

## Development Lokal (SQLite)

```bash
cd poc_prototype
cp .env.example .env

# Run
go run cmd/server/main.go
# → http://localhost:8080
```

## Deploy ke Android (Termux + Neon DB + Tailscale)

### Prasyarat
- HP Android dengan Termux (dari F-Droid) & Tailscale terinstall
- Akun Neon DB (gratis di neon.tech)
- Akun Tailscale (gratis, 3 users)

### 1. Buat Database Neon DB

1. Daftar di https://neon.tech
2. Buat project baru (region: Singapore)
3. Copy connection string: `postgres://user:pass@ep-xxx.ap-southeast-1.aws.neon.tech/neondb?sslmode=require`

### 2. Clone & Build di Termux

```bash
pkg update && pkg upgrade -y
pkg install golang git -y

git clone -b deploy_test https://github.com/gerrymoeis/lab_kom_sim.git
cd lab_kom_sim

# Buat .env
nano .env
```

Isi `.env`:
```env
ENVIRONMENT=production
HOST=0.0.0.0
PORT=8080
DATABASE_URL=postgres://user:pass@ep-xxx.ap-southeast-1.aws.neon.tech/neondb?sslmode=require
SESSION_SECRET=generate-random-string-panjang
UPLOAD_PATH=uploads
GEMINI_API_KEY=your-key
```

```bash
# Build (WAJIB pakai CGO_ENABLED=0 + tags nodynamic)
CGO_ENABLED=0 go build -tags nodynamic -o app-simlab ./cmd/server/main.go

# Jalankan
./app-simlab
# → Server: http://0.0.0.0:8080
```

### 3. Setup Tailscale

1. Install Tailscale di HP (Play Store) dan laptop
2. Login di akun yang sama
3. Di HP, pastikan Tailscale aktif (icon di status bar)
4. Dari laptop: buka `http://[IP-TAILSCALE-HP]:8080`

> MagicDNS: Jika diaktifkan, bisa akses via `http://[nama-perangkat]:8080`

### 4. Reset Neon DB (jika perlu)

Via Neon Console (https://console.neon.tech):
1. Pilih project → **Branches**
2. Main branch → **Reset**
3. Atau via SQL Editor:

```sql
DROP SCHEMA public CASCADE;
CREATE SCHEMA public;
```

Kemudian restart aplikasi — migrasi akan jalan dari awal.

### 5. Deploy ke HP Lain

Di HP baru:
```bash
pkg install golang git tailscale -y
git clone -b deploy_test https://github.com/gerrymoeis/lab_kom_sim.git
# Buat .env (isi DATABASE_URL sama)
CGO_ENABLED=0 go build -tags nodynamic -o app-simlab ./cmd/server/main.go
./app-simlab
```

Pastikan HP baru join ke tailnet yang sama (via Tailscale app).

## Default Login

- **Username**: admin
- **Password**: admin123

⚠️ Ganti password setelah login pertama!

## Fitur

- Dashboard grid 40 PC (8×5) dengan status color-coded
- CRUD PC dengan foto serial & front panel
- Manajemen perangkat (device types, loans, usages)
- Software tracking per PC
- OCR logbook absensi via Google Gemini
- Activity log / audit trail
- Export Excel

## Catatan Penting

- HEIC/HEIF photos dari iPhone didukung (WASM decoder)
- PostgreSQL mode aktif jika `DATABASE_URL` diisi
- SQLite otomatis untuk development tanpa Neon
- `?` placeholder di query otomatis dikonversi untuk PostgreSQL oleh wrapper
