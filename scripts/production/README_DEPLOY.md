# SIMLABKOM — Panduan Deploy & Cleanup Production (Linux)

Bundle `deploy_production_<ts>.tar.gz` berisi tools deploy untuk server Linux production.
Ikuti urutan di bawah. Semua perintah dijalankan sebagai `root` via SSH.

## Prasyarat Server (Linux)

- **Sistem**: Linux dengan `bash` + `systemd` (systemd dibutuhkan utk service unit `simlab.service`).
  Diuji utama di **Debian/Ubuntu** (coreutils). Script portabel: `swap_symlink` memakai idiom tanpa
  GNU-only `mv -T`, `hostname -I` punya fallback `hostname` → `localhost` — kompatibel dengan
  distro minimal/busybox (Alpine, container) untuk alur shell.
- **Arsitektur**: binary statik (`CGO_ENABLED=0`) — sama bisa jalan di glibc maupun musl.
  Bundle default **amd64**; server ARM64/ARM32 butuh bundle dibangun dengan `-Arch` (lihat
  "Membangun Bundle"). Cek arsitektur server: `uname -m`.
- **User & service**: `install.sh` membuat user `simlab` + unit `simlab.service`
  (`EnvironmentFile=/opt/simlab/.env`, `ExecStart=/opt/simlab/app/current/app-simlab`,
  `Restart=on-failure`). Deploy memakai user ini utk `chown`. Bila belum ada, P0 memberi
  warning jelas (deploy tetap lanjut, chown akan gagal) — jalankan `install.sh` dulu.
- **Dependensi** (dibutuhkan `install.sh`): `curl`, `procps`, `systemd`. Server harus online
  utk download release (jalur `update.sh`); bundle lokal tidak butuh internet saat deploy.
- **Disk**: deploy cek ruang (`check_disk`, default 500 MB) — pastikan `/opt/simlab` punya
  ruang cukup untuk data + backup + 3 release.

## Isi Bundle

```
deploy_production_<ts>/
├── bin/                          # binary linux (ELF amd64/arm64/arm): etl, app-simlab, app-simlab-publish
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

## Sekret & Keamanan

- `config/.env.config` di bundle memuat **API key nyata** (GEMINI/OPENROUTER/PC_PHOTO_TOKEN).
  Setelah extract, kunci akses file tsb (tar dari Windows menyimpan perm 0644):
  `chmod 600 config/.env.config`. `cleanup_production.sh` akan menghapus bundle setelah deploy
  selesai.
- `/opt/simlab/.env` (EnvironmentFile service) memuat `SESSION_SECRET`, API key, dan opsional
  `DATABASE_URL` (kredensial Postgres). Deploy set `chmod 600` saat meregenerate/restore —
  jangan ubah perm-nya. **Nilai `DATABASE_URL` tidak pernah di-log** oleh deploy tools.
- Backup `.env` (`data/backups/env.bak`, `env.single_lab.bak`) juga di-`chmod 600`.
- Jangan membagikan bundle/laporan yang memuat `.env.config` (API key) ke pihak yang tidak
  berwenang.

## N-Lab (multi-lab)

Server bisa punya **beberapa lab** (mis. `MI-1`, `VOKASI-1`, dst). Daftar lab dibaca otomatis
dari `.env` (`LABS_<N>_*`, format V2 multi-lab), bukan hardcode:

- **P1** `parse_env_labs` mendeteksi semua lab di `.env`; bila tidak ada lab (format V1
  single-lab) → fallback default `MI-1` + `VOKASI-1` (backward-compat, `detect_source_db`).
- **P4** ETL: lab **pertama** = `source` (penerima copy data), sisanya `seed`.
  `MIG_SOURCE_STEM` diambil dari nama DB source utk P14 single-DB cleanup.
- **P5** seeds per lab: `seeds/<lowercase id>` ATAU fallback `seeds/default` (pola
  `resolveSeedFolder` app). Warning bila lab tak punya seed.
- **Rollback** menghapus DB + uploads **semua lab** yang terdeteksi (bukan hardcode 2 lab).
- **P14** verifier single-DB/`*.db-wal`/`*.db-shm` memakai `MIG_SOURCE_STEM` + semua lab.

## 2. Deploy

```sh
sudo bash deploy_production.sh
# atau skip migrasi / skip full test suite bila memang sudah pernah:
sudo bash deploy_production.sh --skip-migrate
sudo bash deploy_production.sh --skip-migrate --skip-test
# lokasi install / root scan eksplisit (lihat Auto-discovery di bawah):
sudo bash deploy_production.sh --install-dir /srv/simlab
sudo bash deploy_production.sh --allow-roots "/srv /data"
```

## Auto-discovery Lokasi Install (doc 017)

Deploy dan cleanup **menemukan sendiri letak asli SIMLab** di server — tidak perlu asumsi
`/opt/simlab`. Urutan prioritas (berhenti di yang pertama valid):

1. **Override eksplisit** — flag `--install-dir <path>` atau env `INSTALL_DIR=<path>`.
2. **systemd** — `systemctl show simlab.service -p WorkingDirectory/EnvironmentFile`
   (source of truth service; `--value` dengan fallback parse, portabel untuk systemd tua).
3. **Proses berjalan** — `pgrep -f app-simlab` + baca `/proc/<pid>/cwd` dan `ENV_PATH`
   dari `/proc/<pid>/environ`.
4. **Bounded scan** — cari marker struktur SIMLab (`app/releases/<ts>/app-simlab` atau
   `.env` ber `GLOBAL_DB_PATH` + `LABS_1_ID`/`SESSION_SECRET`) di bawah root whitelist
   `ALLOWED_ROOTS` (default: `/opt /srv /usr/local /var /home /data /app`), dengan batas
   kedalaman. Diubah via flag `--allow-roots "<r1 <r2>"` atau env `ALLOWED_ROOTS`.
   **Tidak pernah scan seluruh `/`**.
5. **Default** — `/opt/simlab` (backward-compat bila tidak ada yang terdeteksi).

Hasil deteksi (metode, lokasi, env file) di-log di P0 dan dicatat di report P13
(`"location": {"install_dir", "method", "env_file", "candidates"}`).

Aturan perilaku:
- Kandidat divalidasi sebelum dipakai: `app/current` symlink → `app/releases/<ts>`, `.env`
  lengkap (`GLOBAL_DB_PATH` + `SESSION_SECRET` + minimal 1 `LABS_<N>_ID`), dan `data/global.db`
  ATAU `DATABASE_URL` terisi (PostgreSQL).
- Bila scan menemukan **lebih dari satu** kandidat: saat interaktif diminta memilih path;
  saat non-interaktif deploy **berhenti** (minta `--install-dir` eksplisit) — tidak menebak.
- Bila override tidak valid, tetap lanjut ke deteksi otomatis (warning jelas).
- Nilai `.env` tidak pernah di-log (hanya lokasi file). `SOURCE_DB`/`DATA_DIR` dst. otomatis
  mengikuti lokasi hasil deteksi.

> Catatan: `install.sh`/`update.sh` tetap memakai `/opt/simlab` (installer standar). Auto-discovery
> melayani server yang sudah terpasang di lokasi non-standar/random.

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

## PostgreSQL (opsional)

Server bisa memakai **PostgreSQL** sebagai backend selain SQLite lokal (default). Aktifkan
dengan mengisi `DATABASE_URL` di `/opt/simlab/.env` (format `postgres://user:pass@host:5432/db`).
Bila kosong → backend SQLite (file `.db` di `/opt/simlab/data/`).

Alur deploy menyesuaikan otomatis saat `DATABASE_URL` terisi (backend `postgres`):
- **P1** log `backend PostgreSQL aktif`; `DATABASE_URL` lama **dipertahankan** bila `.env`
  diregenerate dari template.
- **P3** tunggu WAL/SHM SQLite dilewati (PostgreSQL tidak memakai file WAL lokal).
- **P4** ETL (SQLite-only) **dilewati** — migrasi data Postgres tidak dipakai jalur ini.
- **P10** `/readyz` menjadi verifikasi DB utama (ping global + semua lab via app).
- **P11** `app-simlab -verify` (SQLite-only) **dilewati**.
- **P14** file `.db` lokal **tidak dihapus** di backend Postgres (data ada di server Postgres).
- **P13** report memuat `"database": {"backend": "postgres", "url_set": true}`.

Catatan:
- Backup tools mencakup **uploads, `.env`, release** — data PostgreSQL di-backup via penyedia
  DB / tool Postgres (bukan `data.tar.gz`).
- Nilai `DATABASE_URL` tidak pernah di-log (berisi kredensial).

## 3. Membangun Bundle (host Windows)

Bangun bundle dengan `prepare_production.ps1` (host Windows, butuh Go + `.env.config`):

```powershell
.\prepare_production.ps1                    # amd64 (default)
.\prepare_production.ps1 -Arch arm64        # AArch64 (Raspberry Pi 4/arm64 server)
.\prepare_production.ps1 -Arch arm          # ARM32
```

- Verifikasi otomatis: parsing `-test.v` 1-to-1 dengan `go test -json` (F10) + magic byte ELF sesuai
  `-Arch` (amd64: ELF64/x86-64, arm64: ELF64/AArch64, arm: ELF32/ARM).
- `-Deploy -SSH user@vm`: upload bundle + jalankan test binary linux di VM.

## 4. Cleanup (setelah semua aman & sesuai)

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

## Test Manual di VM Linux (pola Fase E)

Sebelum dipakai di production, uji tiap jalur di **VM Linux terisolasi** (mengikuti pola Fase E
dari doc 014 — VirtualBox/VM dengan Debian/Ubuntu, forward port 8080). Jalur yang wajib:

**A. Instalasi + deploy dasar (SQLite 2-lab default)**
1. `chmod +x install.sh update.sh && sudo ./install.sh` (membuat user `simlab` + service).
2. Extract bundle → `chmod 600 config/.env.config` → `sudo bash deploy_production.sh`.
3. Verifikasi: report `PK_autorun: PASS`, `/readyz` OK, akses `http://<vm>:8080`.

**B. N-lab (multi-lab)** — P-B
1. Isi `.env` format V2 dgn 3 lab (`LABS_1_ID` … `LABS_3_ID`, pola `LABS_<N>_*`).
2. Deploy: ETL harus menjalankan lab pertama sebagai `source`, sisanya `seed`;
   setiap lab punya seed (`seeds/<lowercase id>` atau `seeds/default`).
3. Rollback test: simulasikan kegagalan P4–P11 → DB + uploads **semua lab** terhapus,
   symlink kembali ke release sebelumnya, service RUNNING.
4. Fallback test: `.env` tanpa lab (V1 single-lab) → P4 memakai default `MI-1`+`VOKASI-1`
   (backward-compat).

**C. PostgreSQL** — P-C
1. Isi `DATABASE_URL` (Postgres test server) di `.env` → deploy.
2. Verifikasi: P4 ETL **dilewati** (MIG_STATUS postgres), P11 `-verify` dilewati,
   P14 tidak menghapus `.db` lokal, report `"database":{"backend":"postgres"}`.
3. `DATABASE_URL` lama dipertahankan setelah P1 regenerate (cek `.env` hasil).
4. Hapus `DATABASE_URL` → deploy kembali berperilaku SQLite (default).

**D. Portabilitas** — P-D
1. Bangun bundle `-Arch arm64` → jalankan test binary linux di VM ARM64 (atau `qemu-aarch64`);
   magic-byte check harus lulus.
2. Uji `swap_symlink` di distro busybox (Alpine): deploy + rollback tetap jalan tanpa `mv -T`.

**E. Cleanup** (setiap selesai uji)
1. `sudo bash cleanup_production.sh` — hanya jalan bila service active + `/readyz` OK +
   report terbaru `PK_autorun: PASS`; konfirmasi `[y/N]`.
2. Verifikasi: bundle/zip hilang, service tetap RUNNING.

## Troubleshooting Singkat

- Service tidak jalan: `journalctl -u simlab -n 50`
- Ingin ulang deploy setelah gagal: bundle/zip masih ada (deploy tidak menghapusnya).
- Rollback otomatis sudah memastikan server RUNNING; lihat report & log untuk diagnosis.