# Sistem Inventaris Laboratorium Komputer

Sistem manajemen inventaris laboratorium komputer dengan visualisasi grid PC, OCR logbook, tracking software & perangkat. Sekarang dengan Auto Deploy workflow, develop di laptop, deploy ke HP Android (Termux) via SSH + Tailscale.

## Tech Stack

- **Backend**: Go 1.25+ dengan Gin Framework
- **Database**: SQLite (lokal) / PostgreSQL / Neon DB (production)
- **Frontend**: Bootstrap 5 + vanilla JS
- **OCR**: OpenRouter (primary) → Google Gemini (fallback)
- **Image**: WASM-based HEIC decoder (heic-to via CDN)

## Branch Strategy

| Branch | Tujuan |
|--------|--------|
| `deploy_test` | **Stabil** — branch deploy yang sudah teruji |
| `refinement` | **Development** — fitur baru, perbaikan bug |
| *(future)* `deploy/windows` | Deployment khusus Windows |
| *(future)* `deploy/linux` | Deployment khusus Linux |

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
├── web/static/          # CSS, JS
├── uploads/             # Upload foto
└── scripts/
    ├── build_termux.sh  # Build untuk Android/Termux
    ├── deploy.sh        # Auto-deploy via SSH
    └── README_DEPLOY.md # Panduan auto-deploy
```

## Development Lokal (SQLite)

```bash
cd poc_prototype
cp .env.example .env

# Run
go run cmd/server/main.go
# → http://localhost:8080
```

## Auto Deploy (Laptop → HP Android via Tailscale + SSH)

**Prasyarat:**
- HP Android (Termux) dengan `sshd` berjalan di port 8022
- HP dan laptop dalam 1 Tailscale network
- SSH key laptop sudah terdaftar di HP (passwordless)

**Setup sekali:**

```powershell
# Di laptop
git config --global alias.deploy "!git push origin refinement && ssh -p 8022 -i C:/Users/Gallan/.ssh/id_sim_lab_mi galaxy-a52s-5g.taila6b5cf.ts.net 'cd ~/lab_kom_sim && ./scripts/deploy.sh'"
```

**Cara pakai:**

```bash
git deploy
```

Satu perintah → push ke GitHub → SSH ke HP → git pull → build → restart server.

## Deploy ke Android (Termux + Neon DB + Tailscale)

### Prasyarat
- HP Android dengan Termux & Tailscale terinstall
- Akun Neon DB (gratis di neon.tech)

### Setup di Termux

```bash
pkg update && pkg upgrade -y
pkg install golang git openssh -y

git clone -b refinement https://github.com/gerrymoeis/lab_kom_sim.git
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
SESSION_SECRET=random-string
GEMINI_API_KEY=your-key
OPENROUTER_API_KEY=sk-or-your-key
```

```bash
CGO_ENABLED=0 go build -tags nodynamic -o app-simlab ./cmd/server/main.go
./app-simlab
```

## Fitur

- ✅ Dashboard grid 40 PC (8×5) dengan status color-coded
- ✅ CRUD PC dengan upload foto serial & front panel
- ✅ Manajemen perangkat (device types, loans, usages) — mutual exclusive dropdown
- ✅ Software catalog (required + others) — many-to-many dengan toggle per PC
- ✅ Batch edit software assignment (dari catalog, centang per PC)
- ✅ OCR logbook absensi via OpenRouter → Gemini (retry + fallback)
- ✅ Activity log / audit trail (success + failure)
- ✅ Export Excel (PC, device, logbook, software catalog)
- ✅ HEIC/HEIF photo upload (WASM client-side conversion)
- ✅ PostgreSQL via Neon DB (production) / SQLite (development)
- ✅ Auto-deploy via SSH + Tailscale

## Default Login

- **Username**: admin
- **Password**: admin123

## Catatan Penting

- **Database**: `DATABASE_URL` diisi = PostgreSQL (Neon), kosong = SQLite lokal
- **Placeholder**: `?` di query otomatis dikonversi ke `$N` untuk PostgreSQL (DB wrapper)
- **Image**: HEIC dikonversi di browser via CDN library, server terima JPEG
- **OCR**: OpenRouter primary (free vision model), Gemini fallback jika gagal
- **Build**: WAJIB `CGO_ENABLED=0 -tags nodynamic` untuk Termux/Android
