# Sistem Inventaris Laboratorium Komputer

Sistem manajemen inventaris laboratorium komputer multi-lab dengan visualisasi grid PC non-uniform, OCR logbook, tracking software & perangkat. Mendukung N SQLite database dalam 1 server — setiap lab memiliki database, session, dan upload folder sendiri.

## Tech Stack

- **Backend**: Go 1.25+ dengan Gin Framework
- **Database**: SQLite multi-DB (N file) / PostgreSQL / Neon DB (production)
- **Frontend**: Bootstrap 5 + vanilla JS
- **OCR**: OpenRouter (primary) → Google Gemini (fallback)
- **Image**: WASM-based HEIC decoder (heic-to via CDN)

## Branch Strategy

| Branch | Tujuan |
|--------|--------|
| `refactoring` | **Development utama** — multi-DB, per-lab upload, grid dinamis |
| `deploy_test` | **Stabil/testing** — branch deploy yang sudah teruji |
| `deploy_android` | **Stabil Android** — sync dari refactoring, untuk Termux |

## Struktur Project

```
poc_prototype/
├── cmd/server/          # Entry point aplikasi
├── internal/
│   ├── config/          # Konfigurasi (.env) — multi-lab
│   ├── database/        # Database (SQLite multi-DB + PostgreSQL)
│   ├── models/          # Data models
│   ├── handlers/        # HTTP handlers — lab-aware via context
│   ├── services/        # Business logic — per-instance per lab
│   └── middleware/      # Auth, session, lab routing
├── web/templates/       # HTML templates (Go templates)
├── web/static/          # CSS, JS
├── uploads/{lab}/       # Upload foto per-lab
├── backups/{lab}/       # Backup per-lab
├── seeds/{lab-id}/      # Seed data per-lab (JSON)
└── data/                # SQLite database files
```

## Development Lokal (SQLite Multi-Lab)

```bash
cd poc_prototype
cp .env.example .env
# Edit .env — set LABS (lihat .env.example untuk format)
go run ./cmd/server
# → http://localhost:8080
# Landing page → pilih lab → login
```

## Konfigurasi .env — Multi-Lab

```env
# Format: LABS=LAB-ID:dbPath:Title:urlPath,...
LABS=MI-1:data/lab_mi_1.db:Lab Kom MI 1:lab-kom-mi,VOKASI-1:data/lab_vokasi_1.db:Lab Kom Vokasi:vokasi

# Jika LABS tidak di-set, fallback ke single-lab (DATABASE_PATH)
DATABASE_PATH=inventaris_lab.db
```

Setiap lab punya:
- Database sendiri (`data/lab_mi_1.db`)
- Session sendiri (`inventaris_session_lab-kom-mi`)
- Upload folder sendiri (`uploads/lab-kom-mi/pc/`)
- Backup folder sendiri (`backups/lab-kom-mi/`)
- Seed data dari folder `seeds/mi-1/` (jika ada)

## Deploy ke Android (Termux)

```bash
cd lab_kom_sim/poc_prototype
cp .env.example .env
nano .env   # sesuaikan LABS, ANDROID=true

CGO_ENABLED=0 go build -tags nodynamic -o app-simlab ./cmd/server/main.go
./app-simlab
```

Untuk auto-deploy dari laptop via SSH + Tailscale:
```bash
git push origin refactoring
ssh -p 8022 user@host 'cd ~/lab_kom_sim && git pull origin refactoring && CGO_ENABLED=0 go build -tags nodynamic -o app-simlab ./cmd/server/main.go && killall app-simlab && ./app-simlab &'
```

## Fitur

- ✅ **Multi-Lab**: N SQLite database dalam 1 server — data terisolasi penuh
- ✅ **Dashboard grid dinamis**: Layout per-lab (5×8, 10/8/9/9, dll) dengan gap visual
- ✅ **PC grid component**: Reusable template component — 1 file untuk semua halaman
- ✅ **CRUD PC** dengan upload foto serial & front panel (per-lab)
- ✅ **Manajemen perangkat**: Device types, loans, usages, installations
- ✅ **Software catalog** (required + others) — many-to-many dengan toggle per PC
- ✅ **OCR logbook absensi** via OpenRouter → Gemini (retry + fallback)
- ✅ **Activity log / audit trail** (success + failure) — per-lab
- ✅ **Export Excel** (PC, device, logbook, software catalog) — per-lab
- ✅ **Auto-backup** per-lab ke `backups/{lab}/` dengan debounce
- ✅ **SSG Public Build** per-lab — static site generator + auto git push
- ✅ **HEIC/HEIF photo upload** (WASM client-side conversion)
- ✅ **PostgreSQL via Neon DB** (production) / **SQLite multi-DB** (development)
- ✅ **Auto-deploy** via SSH + Tailscale

## Default Login (setiap lab)

- **Username**: admin
- **Password**: admin123
- **Username**: rekan
- **Password**: rekan123

## Catatan Penting

- **LABS format**: `LAB-ID:dbPath:Title:urlPath` — 4 segmen, comma-separated
- **Seed folder**: `seeds/<lowercase(LAB-ID)>/` — ada = apply, tidak ada = skip
- **Database**: `DATABASE_URL` diisi = PostgreSQL (Neon), kosong = SQLite multi-DB
- **Upload path**: `uploads/{urlPath}/{category}/` — per-lab, tidak ada shared
- **Backup path**: `backups/{urlPath}/` — per-lab (override dari BACKUP_DIR)
- **Build**: WAJIB `CGO_ENABLED=0 -tags nodynamic` untuk Termux/Android
- **PC Label DECOUPLED** dari posisi — label tetap saat dipindah
