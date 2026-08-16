# SIMLABKOM — Panduan Deploy & Cleanup Production (Linux)

Bundle `deploy_production_<ts>.tar.gz` berisi tools deploy untuk server Linux production.
Ikuti urutan di bawah. Semua perintah dijalankan sebagai `root` via SSH.

## Isi Bundle

```
deploy_production_<ts>/
├── bin/                          # binary linux (ELF amd64): etl, app-simlab, app-simlab-publish
├── config/                       # etl-config.production.json + .env.config
├── test-runner/                  # full test suite (test binary linux, 8 package)
├── seeds/                        # mi-1, vokasi-1, default
├── deploy_production.sh          # tools deploy (Fase E: tahap P0–PK)
├── cleanup_production.sh         # tools hapus bundle+zip setelah aman (komponen #8)
├── lib/common.sh                 # helper (konstanta, phase, log)
└── bundle-meta.txt               # commit & nama bundle (dipakai report P13)
```

## 1. Upload & Extract

```sh
scp deploy_production_<ts>.tar.gz root@server:/opt/simlab/
ssh root@server
cd /opt/simlab && tar xzf deploy_production_<ts>.tar.gz
cd deploy_production_<ts>
```

> Catatan: folder `/opt/simlab/app` (symlink ke release), `/opt/simlab/data` (data+backup),
> `data/backups/`, dan release TIDAK disentuh oleh cleanup.

## 2. Deploy

```sh
sudo bash deploy_production.sh
# atau skip migrasi / skip full test suite bila memang sudah pernah:
sudo bash deploy_production.sh --skip-migrate
sudo bash deploy_production.sh --skip-migrate --skip-test
```

Tahap yang dijalankan (P0–PK):
- **P0** validasi prasyarat + bundle lengkap (STOP bila gagal)
- **P1** deteksi format `.env` (single/multi) + regenerate
- **P2** backup penuh `data/` + `.env` + release aktif
- **P3** stop service + tunggu WAL/SHM
- **P4** deteksi migrasi; jalankan ETL bila perlu (ROLLBACK bila gagal)
- **P5–P6** siapkan release + deploy binary + atomic symlink swap
- **P7** generate public site (WARN bila gagal)
- **P8–P9** start service + health check `/healthz`
- **P10** readiness check `/readyz` (deep)
- **P11** verifikasi read-only `app-simlab -verify`
- **P12** full test suite (test binary linux) — FAIL/SKIP → ROLLBACK
- **P13** report JSON `deploy_report_<ts>.json`
- **P14** cleanup (release keep 3, single DB, uploads flat)
- **PK** auto-run server + verify final — report digenerate ulang (memuat `PK_autorun`)

Setiap kegagalan tahap dengan tindakan ROLLBACK akan: stop service, restore symlink + data
dari backup, start, health check → server tetap RUNNING (release sebelumnya).

Setelah selesai:
- Cek ringkasan di akhir output (status service, URL akses, report).
- Cek report: `cat /opt/simlab/data/backups/deploy_report_<ts>.json`
  — wajib memuat `"PK_autorun": "PASS"`.
- Buka URL akses di browser + verifikasi via `/readyz`.

## 3. Cleanup (setelah semua aman & sesuai)

Hapus artefak bundle (folder extract + zip + tar.gz sementara di `/tmp`):

```sh
cd deploy_production_<ts>          # masih di folder extract
sudo bash cleanup_production.sh
```

- Safety check otomatis: service `simlab` active DAN `/readyz` OK DAN report deploy terbaru
  `PK_autorun: PASS`. Jika belum → berhenti (pesan jelas).
- Konfirmasi interaktif `[y/N]` sebelum menghapus.
- Bila ingin memaksa (mis. service sudah tidak ada): `sudo bash cleanup_production.sh --force`.

Verifikasi pasca-cleanup: tidak ada sisa bundle di `/opt/simlab/`, service tetap RUNNING + `/readyz` OK.
Log: `/opt/simlab/data/backups/cleanup_<ts>.log`.

## Troubleshooting Singkat

- Service tidak jalan: `journalctl -u simlab -n 50`
- Ingin ulang deploy setelah gagal: bundle/zip masih ada (deploy tidak menghapusnya).
- Rollback otomatis sudah memastikan server RUNNING; lihat report & log untuk diagnosis.