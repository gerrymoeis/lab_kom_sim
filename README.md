# Sistem Inventaris Laboratorium Komputer

Sistem manajemen inventaris laboratorium komputer dengan visualisasi grid PC, OCR logbook, tracking software & perangkat. Sekarang dengan Auto Deploy workflow, develop di laptop, deploy ke HP Android (Termux) via SSH + Tailscale.

## Tech Stack

- **Backend**: Go 1.25+ dengan Gin Framework
- **Database**: SQLite (lokal) / PostgreSQL / Neon DB (production)
- **Frontend**: Bootstrap 5 + vanilla JS
- **OCR**: OpenRouter (primary) → Google Gemini (fallback)
- **Image**: WASM-based HEIC decoder (heic-to via CDN)

### Prasyarat
- Server `deploy_android` berjalan di HP (Termux) dengan WRITE_MODE=async
- Laptop dan HP dalam 1 Tailscale network
- `go` terinstall di laptop

### Cara pakai

```bash
# Phase 1: 10K request — validasi coverage + error rate
go run cmd/stress_test/main.go --url http://100.x.y.z:8080 --total-requests 10000

# Phase 2: 100K request — scaling test
go run cmd/stress_test/main.go --url http://100.x.y.z:8080 --total-requests 100000 --workers 20

# Phase 3: 1M request — target 3-5 menit
go run cmd/stress_test/main.go --url http://100.x.y.z:8080 --total-requests 1000000 --workers 50 --ramp-up 10s
```

### Flags

| Flag | Default | Deskripsi |
|------|---------|-----------|
| `--url` | `http://localhost:8080` | Target server URL (wajib, pakai Tailscale IP) |
| `--total-requests` | `10000` | Total request yang dikirim |
| `--workers` | `10` | Jumlah concurrent workers |
| `--mode` | `mix` | Test mode: `read`, `write`, `mix` |
| `--read-pct` | `50` | Persentase read operation di mix mode |
| `--ramp-up` | `5s` | Durasi ramp-up bertahap |
| `--setup-users` | `20` | Jumlah stress test users yang dibuat |
| `--verbose` | `false` | Log tiap request |

### Catatan
- Login sebagai `rekan` untuk setup users, tiap worker login dengan akun unik
- Semua entity (PC, device, software, logbook, schedule, dll) akan di-create/diupdate/didelete
- Report ditampilkan setelah selesai: latency percentiles per-operation & per-entity

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
