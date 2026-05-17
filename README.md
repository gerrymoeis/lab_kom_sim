# Sistem Inventaris Laboratorium Komputer — Windows

Deployment untuk Windows. Menggunakan **SQLite** sebagai database lokal via pure Go driver (no C compiler needed). PostgreSQL/Neon code tetap tersedia untuk scale up di masa depan.

## Tech Stack

- **Backend**: Go 1.25+ dengan Gin Framework
- **Database**: SQLite (lokal) — pure Go (modernc.org/sqlite, no CGO)
- **Frontend**: Bootstrap 5 + vanilla JS
- **OCR**: OpenRouter (primary) → Google Gemini (fallback)
- **Image**: WASM-based HEIC decoder (heic-to via CDN)

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
    ├── build-windows.ps1    # Build untuk Windows
    ├── deploy-windows.ps1   # Deploy (build + run background)
    ├── download-vendor.ps1  # Download vendor assets
    └── setup_firewall.ps1   # Firewall setup
```

## Deploy ke Windows

### Prasyarat

- Windows 10/11 64-bit
- [Go 1.25+](https://go.dev/dl/) — pastikan `go` available di PATH
- **Tidak perlu** C compiler (MinGW/TDM-GCC/MSVC) — SQLite pure Go
- Git (opsional, untuk clone repo)

### Setup

```powershell
# Clone repositori
git clone -b deploy_windows https://github.com/gerrymoeis/lab_kom_sim.git
cd lab_kom_sim

# Buat .env dari contoh
copy .env.example .env

# Edit .env sesuai kebutuhan
notepad .env
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

```powershell
# Build
CGO_ENABLED=0 go build -ldflags="-s -w" -o app-simlab.exe ./cmd/server/main.go

# Atau pakai script
.\scripts\build-windows.ps1

# Jalankan langsung
.\app-simlab.exe

# Atau deploy script (background process)
.\scripts\deploy-windows.ps1
```

Akses: http://localhost:8080

### Firewall (Akses dari Device Lain)

Jika ingin mengakses dari HP/device lain di jaringan yang sama:

```powershell
# Jalankan sebagai Administrator
.\scripts\setup_firewall.ps1
```

Script ini otomatis membuka port 8080 di Windows Firewall dan menampilkan URL akses.

### Menjalankan sebagai Service (Opsional)

Gunakan [NSSM](https://nssm.cc/) untuk menjalankan sebagai Windows Service:

```powershell
# Install NSSM
winget install nssm

# Install service
nssm install InventarisLab "C:\path\to\lab_kom_sim\app-simlab.exe"
nssm set InventarisLab AppDirectory "C:\path\to\lab_kom_sim"
nssm set InventarisLab AppEnvironmentExtra "CGO_ENABLED=0"
nssm start InventarisLab
```

## Perbedaan dengan Branch Lain

| Aspek | deploy_windows | deploy_android | deploy_linux |
|-------|---------------|----------------|--------------|
| OS Target | Windows | Android (Termux) | Linux |
| Database | SQLite (pure Go) | SQLite (CGO) | SQLite (pure Go) |
| C Compiler | Tidak perlu (modernc) | WAJIB (gcc via pkg) | Tidak perlu (modernc) |
| Build | `CGO_ENABLED=0` | `CGO_ENABLED=1 -tags nodynamic` | `CGO_ENABLED=0` |
| HEIC | WASM via wazero | WASM via wazero | Native via libheif |

## Fitur

- ✅ Dashboard grid 40 PC (8×5) dengan status color-coded
- ✅ CRUD PC dengan upload foto serial & front panel
- ✅ Manajemen perangkat (device types, loans, usages)
- ✅ Software catalog (required + others)
- ✅ OCR logbook absensi via OpenRouter → Gemini (retry + fallback)
- ✅ Activity log / audit trail (success + failure)
- ✅ Export Excel (PC, device, logbook, software catalog)
- ✅ HEIC/HEIF photo upload (WASM client-side conversion)
- ✅ SQLite database (pure Go) — PostgreSQL siap scale up
- ✅ Windows Firewall setup script

## Default Login

- **Username**: admin
- **Password**: admin123

## Catatan Penting

- **Database**: `DATABASE_URL` kosong = SQLite lokal, diisi = PostgreSQL/Neon
- **CGO**: `CGO_ENABLED=0` — tidak perlu C compiler, SQLite via pure Go (modernc.org/sqlite)
- **HEIC Windows**: WASM via wazero (native Windows loading belum support di gen2brain/heic)
- **HEIC Linux**: Native via system libheif (lebih cepat)
- **Image**: HEIC dikonversi via WASM browser-side + server-side fallback
- **OCR**: OpenRouter primary (free vision model), Gemini fallback jika gagal
