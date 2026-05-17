# Sistem Inventaris Laboratorium Komputer — Android (Termux)

Deployment untuk HP Android via Termux. Menggunakan **SQLite** sebagai database lokal via pure Go driver (no C compiler needed). PostgreSQL/Neon code tetap tersedia untuk scale up di masa depan.

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
    ├── build_termux.sh  # Build untuk Android/Termux
    ├── deploy.sh        # Auto-deploy via SSH
    └── download-vendor.sh
```

## Deploy ke Android (Termux + SQLite)

### Prasyarat

- HP Android dengan Termux & Tailscale terinstall
- **Tidak perlu** C compiler (gcc/clang) — SQLite pure Go
- Go 1.25+ (`pkg install golang`)

### Setup di Termux

```bash
pkg update && pkg upgrade -y
pkg install golang git openssh -y

git clone -b deploy_android https://github.com/gerrymoeis/lab_kom_sim.git
cd lab_kom_sim

# Buat .env
cp .env.example .env
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
# Build (pure Go — cepat, tidak perlu CGO)
CGO_ENABLED=0 go build -o app-simlab ./cmd/server/main.go

# Atau pakai script
bash scripts/build_termux.sh

# Jalankan
./app-simlab
```

Akses: http://localhost:8080

### Auto Deploy (Laptop → HP Android via Tailscale + SSH)

**Setup sekali:**
```powershell
# Di laptop (PowerShell)
git config --global alias.deploy "!git push origin deploy_android && ssh -p 8022 -i C:/Users/Gallan/.ssh/id_sim_lab_mi galaxy-a52s-5g.taila6b5cf.ts.net 'cd ~/lab_kom_sim && bash scripts/deploy.sh'"
```

**Cara pakai:**
```bash
git deploy
```

Satu perintah → push ke GitHub → SSH ke HP → git pull → build → restart server.

## Perbedaan dengan Branch Lain

| Aspek | deploy_android | deploy_windows | deploy_linux |
|-------|---------------|----------------|--------------|
| OS Target | Android (Termux) | Windows | Linux |
| Database | SQLite (pure Go) | SQLite (pure Go) | SQLite (pure Go) |
| C Compiler | **Tidak perlu** (modernc) | Tidak perlu (modernc) | Tidak perlu (modernc) |
| Build | `CGO_ENABLED=0` | `CGO_ENABLED=0` | `CGO_ENABLED=0` |
| HEIC | WASM via wazero | WASM via wazero | Native libheif |
| Service | Termux bootstrap | NSSM / background | systemd / nohup |

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
- ✅ Auto-deploy via SSH + Tailscale

## Default Login

- **Username**: admin
- **Password**: admin123

## Catatan Penting

- **Database**: `DATABASE_URL` kosong = SQLite lokal, diisi = PostgreSQL/Neon
- **CGO**: `CGO_ENABLED=0` — tidak perlu C compiler, SQLite via pure Go (modernc.org/sqlite)
- **GCC**: Tidak perlu diinstall. Build cepat tanpa CGO.
- **Image**: HEIC dikonversi via WASM browser-side + server-side fallback
- **OCR**: OpenRouter primary (free vision model), Gemini fallback jika gagal
