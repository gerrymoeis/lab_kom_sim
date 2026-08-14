# SIMLABKOM — E2E Production Test

Automasi uji produksi end-to-end untuk migrasi **`main` (single-DB) → `refactoring` (multi-DB)** di mesin Linux (VM). Menjalankan 9 fase (F0–F9): prasyarat, clone, build & run versi lama, CRUD deterministik (pakai SEMUA API key asli), staging ETL, migrasi, deploy & run versi baru, verifikasi otomatis, cleanup total, report.

## Struktur

```
e2e_test/
├── run_e2e.sh              # Script utama (jalankan di VM Linux)
├── prepare_e2e.ps1         # Helper HOST Windows (opsional): build ETL linux + bundle
├── lib/common.sh           # Helper: log, curl+cookie+CSRF, db_count
├── config/
│   ├── etl-config.json.tpl # Template config ETL (placeholder {{E2E_ROOT}})
│   └── keys.env.example    # Template API keys (GEMINI, OPENROUTER, PC_PHOTO, GITHUB)
└── assets/
    └── logbook_sample.png  # Gambar OCR deterministik utk uji logbook
```

## Alur fase (ringkas)

| Fase | Aksi |
|------|------|
| F0 | Cek toolchain (git/go/curl/sqlite3/pkill) + semua API key terisi |
| F1 | Clone `main` & `refactoring` dari `gerrymoeis/lab_kom_sim` |
| F2 | Build & run versi **lama** (single-DB) di port `18080` |
| F3 | CRUD deterministik via curl+cookie+CSRF: 3 PC, 1 kategori, 1 tipe, 2 device, 2 software, 1 jadwal, 1 logbook, upload foto, OCR logbook |
| F4 | Staging: stop server, WAL checkpoint, siapkan source ETL (DB + uploads flat → `uploads/lab-kom-mi/<sub>`) |
| F5 | Jalankan ETL `migrate_single_to_multi` → `out/global.db`, `out/lab_mi_1.db`, uploads per-lab |
| F6 | Build & run versi **baru** (multi-DB) di port `18081`; taruh hasil ETL + seeds; marker `.seed_done` utk lab source |
| F7 | Verifikasi: healthz, parity (pcs/devices/software/schedules/logbook), integrity, grid, seed vokasi, marker, foto |
| F8 | Backup → cleanup versi lama (DB single, uploads flat, binary, dist, release lama) → verifikasi bersih |
| F9 | Tulis `e2e_report.json` + arsip `e2e_results_*.tar.gz` |

## Cara pakai di VM Linux

```bash
# 1. Upload bundle (dari Windows: prepare_e2e.ps1 -SSH user@vm -Deploy)
# 2. Di VM:
mkdir -p ~/e2e_test
tar -xzf e2e_bundle_*.tar.gz -C ~/e2e_test
cd ~/e2e_test
# 3. Isi API key ASLI (jangan commit)
cp config/keys.env.example keys.env
nano keys.env
# 4. Jalankan
E2E_ROOT="$HOME/e2e_test" bash run_e2e.sh
```

## Prasyarat VM

- Ubuntu/Debian Linux dengan `git`, `go`, `curl`, `sqlite3`, `pkill`
- Akses internet untuk: clone repo, API Gemini/OpenRouter (OCR), GitHub Release (PC photos)
- SEMUA API key di `keys.env` **wajib terisi** (F0 memaksa): `GEMINI_API_KEY`, `OPENROUTER_API_KEY`, `PC_PHOTO_RELEASE_URL`, `PC_PHOTO_TOKEN`, `GITHUB_TOKEN`

## Data dummy (deterministik)

- **3 PC**: `PC-01..PC-03`, SN `SN-E2E-PC-0N`, OS `Windows 10`
- **1 kategori** `E2E Proyektor` (prefix `E2EPRJ`), **1 tipe** `Proyektor`
- **2 devices**: `E2E-DEV-001`, `E2E-DEV-002`
- **2 software**: `E2E Word` (required), `E2E Chrome` (other)
- **1 jadwal**: `E2E Matematika` (Senin 07:00–08:40, kelas 7A)
- **1 logbook manual** + **1 OCR logbook** (gambar `assets/logbook_sample.png`)
- **1 foto PC** terpasang ke `PC-01` via edit

## Git & privasi

- Folder ini adalah repo git **mandiri** (remote `gerrymoeis/lab_kom_sim`, branch `e2e-production-test`)
- `keys.env` dan seluruh artefak runtime di-ignore (`.gitignore`); **jangan commit API key**
- Konfigurasi ETL memakai template dengan placeholder; file `etl-config.json` hasil substitusi di-ignore

## Referensi

- Plan lengkap: `docs_and_backup/docs/test_production/009_PLAN_IMPLEMENTASI_SCRIPT_AUTOMASI_E2E_PRODUCTION_TEST.md`
- BACKLOG: `docs_and_backup/BACKLOG_FINISHING_SIMLABKOM.md` item 59