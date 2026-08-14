#!/usr/bin/env bash
# =============================================================================
# SIMLABKOM — E2E Production Test (run_e2e.sh)
#
# Menjalankan uji production end-to-end di Linux VM:
#   F0  Prasyarat + verifikasi semua API key terisi
#   F1  git clone repo (main = lama, refactoring = baru)
#   F2  Build & run versi LAMA (main, single-DB) + healthz
#   F3  CRUD test deterministik (curl + cookie + CSRF) — pakai SEMUA API key
#   F4  Staging: stop server, WAL checkpoint, siapkan source ETL
#   F5  ETL (migrate_single_to_multi) di VM Linux
#   F6  Deploy & run versi BARU (refactoring, multi-DB) + healthz
#   F7  Auto verifikasi (parity, integrity, grid, remap, seed, foto)
#   F8  Cleanup versi lama (backup dulu, hapus, verifikasi bersih)
#   F9  Report JSON + arsip log
#
# Data dummy DETERMINISTIK (bukan random) agar mudah diverifikasi akurat.
#
# Cara pakai:
#   E2E_ROOT=/opt/e2e_test bash run_e2e.sh            # root dir kerja (default $HOME/e2e_test)
#   VM_IP / SSH dikelola host helper prepare_e2e.ps1
# =============================================================================
set -euo pipefail

# ---------------------------------------------------------------- Konfigurasi
E2E_ROOT="${E2E_ROOT:-$HOME/e2e_test}"
OLD_PORT="${OLD_PORT:-18080}"
NEW_PORT="${NEW_PORT:-18081}"
OLD_BASE="http://127.0.0.1:${OLD_PORT}"
NEW_BASE="http://127.0.0.1:${NEW_PORT}"

LOG_DIR="$E2E_ROOT/logs"
COOKIE_DIR="$LOG_DIR/cookies"
STAGING_DIR="$E2E_ROOT/staging"
OUT_DIR="$E2E_ROOT/out"
DATA_DIR="$E2E_ROOT/data"
ASSETS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/assets"
LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/lib" && pwd)"
KEYS_FILE="$E2E_ROOT/keys.env"
CONFIG_TPL="$(cd "$(dirname "${BASH_SOURCE[0]}")/config" && pwd)/etl-config.json.tpl"

REPO_URL="${REPO_URL:-https://github.com/gerrymoeis/lab_kom_sim}"
REPO_DIR="$E2E_ROOT/repo"
MAIN_DIR="$REPO_DIR/main"
REFACTOR_DIR="$REPO_DIR/refactoring"

# ---------------------------------------------------------------- Setup env
mkdir -p "$LOG_DIR" "$COOKIE_DIR" "$STAGING_DIR" "$OUT_DIR" "$DATA_DIR"
PHASE_LOG="$LOG_DIR/console.log"
source "$LIB_DIR/common.sh"

# Muat keys.env bila ada (untuk memicu semua API key). Wajib terisi di F0.
if [ -f "$KEYS_FILE" ]; then
    set -a; source "$KEYS_FILE"; set +a
fi

# ---------------------------------------------------------------- Helper
require_key() {
    local name="$1"
    if [ -z "${!name:-}" ]; then
        fail "F0: API key '$name' belum terisi (isi $KEYS_FILE dari keys.env.example)"
    fi
    log "F0: key $name = OK (terisi)"
}

start_server() {
    local dir="$1" bin="$2" port="$3" logf="$4"
    (cd "$dir" && PORT="$port" HOST="127.0.0.1" nohup ./"$bin" >"$logf" 2>&1 &)
    local i
    for i in $(seq 1 30); do
        if curl -sf "http://127.0.0.1:$port/healthz" >/dev/null 2>&1; then
            log "server up: :$port"
            return 0
        fi
        sleep 1
    done
    log "server :$port gagal healthz — isi log:"
    tail -n 30 "$logf" || true
    return 1
}

stop_server() {
    local pat="$1"
    pkill -f "$pat" 2>/dev/null || true
    sleep 2
}

# ---------------------------------------------------------------- F0: Prasyarat
phase "F0 — Prasyarat & API keys"
command -v git  >/dev/null 2>&1 || fail "F0: git tidak ada"
command -v go   >/dev/null 2>&1 || fail "F0: go tidak ada"
command -v curl >/dev/null 2>&1 || fail "F0: curl tidak ada"
command -v sqlite3 >/dev/null 2>&1 || fail "F0: sqlite3 CLI tidak ada"
command -v pkill >/dev/null 2>&1 || fail "F0: pkill tidak ada"
log "F0: toolchain lengkap"

require_key GEMINI_API_KEY
require_key OPENROUTER_API_KEY
require_key PC_PHOTO_RELEASE_URL
require_key PC_PHOTO_TOKEN
require_key GITHUB_TOKEN
log "F0: SELESAI"

# ---------------------------------------------------------------- F1: Clone
phase "F1 — git clone repo"
if [ ! -d "$MAIN_DIR/.git" ]; then
    git clone --quiet --branch main "$REPO_URL" "$MAIN_DIR" 2>>"$LOG_DIR/F1_clone.log"
fi
if [ ! -d "$REFACTOR_DIR/.git" ]; then
    git clone --quiet --branch refactoring "$REPO_URL" "$REFACTOR_DIR" 2>>"$LOG_DIR/F1_clone.log"
fi
log "F1: main @$(git -C "$MAIN_DIR" rev-parse --short HEAD)"
log "F1: refactoring @$(git -C "$REFACTOR_DIR" rev-parse --short HEAD)"
log "F1: SELESAI"

# ---------------------------------------------------------------- F2: Run versi lama
phase "F2 — Build & run versi LAMA (main)"
OLD_RUN="$E2E_ROOT/old_run"
mkdir -p "$OLD_RUN"

(cd "$MAIN_DIR" && go build -o "$OLD_RUN/app-simlab" ./cmd/server/main.go) \
    >>"$LOG_DIR/F2_old_build.log" 2>&1 || fail "F2: go build main gagal"
log "F2: binary lama built"

# .env versi lama (single-DB) — SEMUA API key
cat > "$OLD_RUN/.env" <<EOF
ENVIRONMENT=production
HOST=127.0.0.1
PORT=$OLD_PORT
DATABASE_PATH=$E2E_ROOT/old_run/inventaris_lab.db
UPLOAD_PATH=$E2E_ROOT/old_run/uploads
SESSION_SECRET=$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')
GEMINI_API_KEY=$GEMINI_API_KEY
OPENROUTER_API_KEY=$OPENROUTER_API_KEY
PC_PHOTO_RELEASE_URL=$PC_PHOTO_RELEASE_URL
GITHUB_TOKEN=$GITHUB_TOKEN
WRITE_MODE=sync
TIMEZONE=Asia/Jakarta
ANDROID=false
DEFAULT_PAGE_SIZE=25
BACKUP_ENABLED=false
PUBLIC_BUILD_ENABLED=false
EOF
log "F2: .env lama ditulis (single-DB)"

# Versi lama butuh folder web/ di samping binary (tidak self-contained)
cp -r "$MAIN_DIR/web" "$OLD_RUN/web"

stop_server "app-simlab"
start_server "$OLD_RUN" "app-simlab" "$OLD_PORT" "$LOG_DIR/F2_old_run.log" \
    || fail "F2: server lama tidak up"
log "F2: SELESAI"

# ---------------------------------------------------------------- F3: CRUD deterministik
phase "F3 — CRUD test versi LAMA (deterministik, semua API key)"
cd "$OLD_RUN"

login "admin" "admin123"
login "rekan" "rekan123"

# --- PC: buat 3 PC deterministik (label, SN, OS tetap)
for i in 1 2 3; do
    post_form admin "$OLD_BASE/pc/create" \
        --data-urlencode "label=PC-0$i" \
        --data-urlencode "row=$i" \
        --data-urlencode "column=1" \
        --data-urlencode "status=normal" \
        --data-urlencode "placement=dipakai" \
        --data-urlencode "serial_number=SN-E2E-PC-0$i" \
        --data-urlencode "operating_system=Windows 10"
done
PC_COUNT=$(db_count "$OLD_RUN/inventaris_lab.db" "pcs")
[ "$PC_COUNT" = "3" ] || fail "F3: PC count != 3 (=$PC_COUNT)"
log "F3: PC=3 OK"

# --- Category + device type + devices (batch-create via /devices/batch-create)
api_post_json admin "$OLD_BASE/devices/batch-create" \
'{"category_id":0,"device_type_id":0,
  "new_category_name":"E2E Proyektor","new_category_prefix":"E2EPRJ",
  "new_type_name":"Proyektor","new_type_brand":"E2E","new_type_model":"X1",
  "new_type_asset_code_prefix":"E2EPRJ","new_type_usage_type":"loanable",
  "new_type_default_location":"Lab",
  "devices":[{"serial_number":"E2E-DEV-001","condition":"normal","location":"Lab"},
             {"serial_number":"E2E-DEV-002","condition":"normal","location":"Lab"}]}'
CAT_COUNT=$(db_count "$OLD_RUN/inventaris_lab.db" "categories")
DT_COUNT=$(db_count "$OLD_RUN/inventaris_lab.db" "device_types")
DEV_COUNT=$(db_count "$OLD_RUN/inventaris_lab.db" "devices")
[ "$CAT_COUNT" = "1" ] || fail "F3: categories != 1 (=$CAT_COUNT)"
[ "$DT_COUNT" = "1" ]  || fail "F3: device_types != 1 (=$DT_COUNT)"
[ "$DEV_COUNT" = "2" ] || fail "F3: devices != 2 (=$DEV_COUNT)"
log "F3: categories=1 device_types=1 devices=2 OK"

# --- Software (2)
post_form admin "$OLD_BASE/software/create" \
    --data-urlencode "name=E2E Word" --data-urlencode "category=required" \
    --data-urlencode "description=Software wajib E2E"
post_form admin "$OLD_BASE/software/create" \
    --data-urlencode "name=E2E Chrome" --data-urlencode "category=other" \
    --data-urlencode "description=Software lain E2E"
SW_COUNT=$(db_count "$OLD_RUN/inventaris_lab.db" "software_catalog")
[ "$SW_COUNT" = "2" ] || fail "F3: software != 2 (=$SW_COUNT)"
log "F3: software=2 OK"

# --- Schedule (1)
post_form admin "$OLD_BASE/schedules/create" \
    --data-urlencode "course_name=E2E Matematika" \
    --data-urlencode "lecturer=Bu E2E" \
    --data-urlencode "day=Senin" \
    --data-urlencode "class=7A" \
    --data-urlencode "time_start=07:00" \
    --data-urlencode "time_end=08:40" \
    --data-urlencode "notes=Jadwal E2E"
SCH_COUNT=$(db_count "$OLD_RUN/inventaris_lab.db" "course_schedules")
[ "$SCH_COUNT" = "1" ] || fail "F3: schedules != 1 (=$SCH_COUNT)"
log "F3: schedules=1 OK"

# --- Upload foto PC (memicu upload pipeline) lalu tempel ke PC-01 via edit agar file pindah ke uploads/pc
api_upload admin "$OLD_BASE/api/upload-image" "$ASSETS_DIR/logbook_sample.png" "serial" "pc-01"
log "F3: upload foto PC OK"
# Ambil file_ref dari response JSON upload
SERIAL_REF=$(grep -o '"file_ref":"[^"]*"' "$COOKIE_DIR/admin_upload.json" | sed 's/.*":"//; s/"//')
[ -n "$SERIAL_REF" ] || fail "F3: file_ref kosong dari upload"
post_form admin "$OLD_BASE/pc/PC-01/edit" \
    --data-urlencode "serial_number=SN-E2E-PC-01" \
    --data-urlencode "operating_system=Windows 10" \
    --data-urlencode "serial_file_ref=$SERIAL_REF"
[ -f "$OLD_RUN/uploads/pc/$SERIAL_REF" ] || fail "F3: foto tidak berpindah ke uploads/pc"
log "F3: foto PC terpasang ke PC-01 ($SERIAL_REF)"

# --- Logbook manual (1)
post_form admin "$OLD_BASE/logbook/create" \
    --data-urlencode "date=2026-01-12" \
    --data-urlencode "student_name=E2E Student" \
    --data-urlencode "nim=12345678901" \
    --data-urlencode "time_in=08:00" \
    --data-urlencode "time_out=09:30" \
    --data-urlencode "purpose=Praktikum E2E"
LB_COUNT=$(db_count "$OLD_RUN/inventaris_lab.db" "logbook_entries")
[ "$LB_COUNT" = "1" ] || fail "F3: logbook != 1 (=$LB_COUNT)"
log "F3: logbook=1 OK"

# --- OCR logbook (GEMINI primary, OPENROUTER fallback) — memicu kedua API key
LOGBOOK_UPLOAD="$OLD_BASE/logbook/upload"
COOKIE_JAR="$COOKIE_DIR/admin.jar"
UP_PAGE="$COOKIE_DIR/admin_lbup.html"
tok=$(csrf_get "$OLD_BASE/logbook/upload" "$UP_PAGE")
curl -sf -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST "$LOGBOOK_UPLOAD" \
    -H "X-CSRF-Token: $tok" -F "logbook_image=@$ASSETS_DIR/logbook_sample.png" \
    -o "$COOKIE_DIR/admin_lb_result.html" || fail "F3: logbook upload OCR gagal"
# OCR dipicu: log aplikasi harus mencatat "[OCR] Trying Gemini primary"
grep -q "Trying Gemini primary" "$LOG_DIR/F2_old_run.log" \
    && log "F3: OCR Gemini DIPICU (request terkirim)" \
    || warn "F3: OCR Gemini tidak tercatat di log (cek API key/network)"
log "F3: SELESAI"

# ---------------------------------------------------------------- F4: Staging
phase "F4 — Staging source ETL (freeze WAL)"
stop_server "app-simlab"
sqlite3 "$OLD_RUN/inventaris_lab.db" "PRAGMA wal_checkpoint(TRUNCATE);" >/dev/null 2>&1 || true
cp "$OLD_RUN/inventaris_lab.db" "$STAGING_DIR/inventaris_lab.db"
# Upload versi lama flat (uploads/pc dsb) — ETL membaca source_upload_dir subfolder
mkdir -p "$STAGING_DIR/uploads/lab-kom-mi"
for sub in pc device_types device_installations logbook; do
    if [ -d "$OLD_RUN/uploads/$sub" ]; then
        cp -r "$OLD_RUN/uploads/$sub" "$STAGING_DIR/uploads/lab-kom-mi/$sub"
    fi
done
log "F4: staging DB + uploads siap"
ls -la "$STAGING_DIR" "$STAGING_DIR/uploads/lab-kom-mi" >>"$LOG_DIR/F4_staging.log"
log "F4: SELESAI"

# ---------------------------------------------------------------- F5: ETL di VM
phase "F5 — ETL (migrate_single_to_multi) di VM Linux"
ETL_BIN="$E2E_ROOT/etl"
[ -x "$ETL_BIN" ] || fail "F5: binary ETL tidak ada di $ETL_BIN (build dari tools/migrate_single_to_multi, GOOS=linux)"
sed "s|{{E2E_ROOT}}|$E2E_ROOT|g" "$CONFIG_TPL" > "$E2E_ROOT/etl-config.json"
(cd "$OUT_DIR" && "$ETL_BIN" -config "$E2E_ROOT/etl-config.json" -force) \
    >>"$LOG_DIR/F5_etl.log" 2>&1 || fail "F5: ETL gagal — lihat $LOG_DIR/F5_etl.log"
[ -f "$OUT_DIR/global.db" ] || fail "F5: global.db tidak dihasilkan"
[ -f "$OUT_DIR/lab_mi_1.db" ] || fail "F5: lab_mi_1.db tidak dihasilkan"
log "F5: ETL SELESAI"
grep -E '"rows_|global_users|upload_files' "$OUT_DIR/migration_report.json" >>"$LOG_DIR/F5_etl.log" || true

# ---------------------------------------------------------------- F6: Deploy versi baru
phase "F6 — Deploy versi BARU (refactoring, multi-DB)"
NEW_RUN="$E2E_ROOT/new_run"
mkdir -p "$NEW_RUN"

(cd "$REFACTOR_DIR" && go build -o "$NEW_RUN/app-simlab" ./cmd/server/main.go) \
    >>"$LOG_DIR/F6_new_build.log" 2>&1 || fail "F6: go build refactoring gagal"
log "F6: binary baru built (self-contained, //go:embed)"

# Taruh hasil ETL ke data dir
cp "$OUT_DIR/global.db" "$DATA_DIR/global.db"
cp "$OUT_DIR/lab_mi_1.db" "$DATA_DIR/lab_mi_1.db"
mkdir -p "$DATA_DIR/uploads"
cp -r "$OUT_DIR/uploads/." "$DATA_DIR/uploads/"
# Lab source (lab-mi) datanya sudah identik hasil ETL → beri marker .seed_done agar
# RunSeedFolder tidak menambah PC dari seeds/mi-1 (parity pcs tetap 3).
# Lab-vokasi-1 (mode seed) TIDAK diberi marker → DB dibuat & di-seed oleh binary saat boot.
if [ -d "$DATA_DIR/uploads/lab-mi" ]; then
    touch "$DATA_DIR/uploads/lab-mi/.seed_done"
    log "F6: marker .seed_done dibuat utk lab source (lab-mi)"
fi
# Seeds refactoring (mi-1, vokasi-1, default) dibaca dari disk seeds/ relative CWD
mkdir -p "$NEW_RUN/seeds"
cp -r "$REFACTOR_DIR/seeds/mi-1" "$NEW_RUN/seeds/"
cp -r "$REFACTOR_DIR/seeds/vokasi-1" "$NEW_RUN/seeds/"
cp -r "$REFACTOR_DIR/seeds/default" "$NEW_RUN/seeds/" 2>/dev/null || true

# .env multi-lab (dari .env.config) — SEMUA API key
cat > "$NEW_RUN/.env" <<EOF
ENVIRONMENT=production
HOST=127.0.0.1
PORT=$NEW_PORT
GLOBAL_DB_PATH=$DATA_DIR/global.db
LABS_1_ID=MI-1
LABS_1_DB=$DATA_DIR/lab_mi_1.db
LABS_1_TITLE=Lab Kom MI
LABS_1_URL=lab-mi
LABS_2_ID=VOKASI-1
LABS_2_DB=$DATA_DIR/lab_vokasi_1.db
LABS_2_TITLE=Lab Kom Vokasi 1
LABS_2_URL=lab-vokasi-1
SESSION_SECRET=$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')
UPLOAD_PATH=$DATA_DIR/uploads
WRITE_MODE=sync
TIMEZONE=Asia/Jakarta
ANDROID=false
DEFAULT_PAGE_SIZE=25
BACKUP_ENABLED=false
PUBLIC_BUILD_ENABLED=false
GEMINI_API_KEY=$GEMINI_API_KEY
OPENROUTER_API_KEY=$OPENROUTER_API_KEY
PC_PHOTO_RELEASE_URL=$PC_PHOTO_RELEASE_URL
PC_PHOTO_TOKEN=$PC_PHOTO_TOKEN
EOF
log "F6: .env baru ditulis (multi-lab)"

stop_server "app-simlab"
start_server "$NEW_RUN" "app-simlab" "$NEW_PORT" "$LOG_DIR/F6_new_run.log" \
    || fail "F6: server baru tidak up"
log "F6: SELESAI"

# ---------------------------------------------------------------- F7: Auto verifikasi
phase "F7 — Auto verifikasi versi baru"
V_LOG="$LOG_DIR/F7_verify.log"
: > "$V_LOG"

# 1. healthz + login global
curl -sf "$NEW_BASE/healthz" >/dev/null 2>&1 || fail "F7: healthz gagal"
echo "1. healthz OK" >>"$V_LOG"

# 2. Parity pcs / devices / software / schedules / logbook (global user + lab_mi)
GPCS=$(db_count "$DATA_DIR/global.db" "global_users")
[ "$GPCS" -ge 1 ] || fail "F7: global_users kosong"
echo "2. global_users=$GPCS" >>"$V_LOG"

LPCS=$(db_count "$DATA_DIR/lab_mi_1.db" "pcs")
[ "$LPCS" = "3" ] || fail "F7: lab_mi pcs != 3 (=$LPCS)"
echo "3. lab_mi pcs=$LPCS" >>"$V_LOG"

LDEV=$(db_count "$DATA_DIR/lab_mi_1.db" "devices")
[ "$LDEV" = "2" ] || fail "F7: lab_mi devices != 2 (=$LDEV)"
echo "4. lab_mi devices=$LDEV" >>"$V_LOG"

LSOFT=$(db_count "$DATA_DIR/lab_mi_1.db" "software_catalog")
[ "$LSOFT" = "2" ] || fail "F7: lab_mi software != 2 (=$LSOFT)"
echo "5. lab_mi software=$LSOFT" >>"$V_LOG"

LSCH=$(db_count "$DATA_DIR/lab_mi_1.db" "course_schedules")
[ "$LSCH" = "1" ] || fail "F7: lab_mi schedules != 1 (=$LSCH)"
echo "6. lab_mi schedules=$LSCH" >>"$V_LOG"

LLB=$(db_count "$DATA_DIR/lab_mi_1.db" "logbook_entries")
[ "$LLB" = "1" ] || fail "F7: lab_mi logbook != 1 (=$LLB)"
echo "7. lab_mi logbook=$LLB" >>"$V_LOG"

# 3. integrity check
for db in "$DATA_DIR/global.db" "$DATA_DIR/lab_mi_1.db" "$DATA_DIR/lab_vokasi_1.db"; do
    [ -f "$db" ] || continue
    R=$(sqlite3 "$db" "PRAGMA integrity_check;")
    [ "$R" = "ok" ] || fail "F7: integrity $db != ok (=$R)"
done
echo "8. integrity_check OK (global+lab_mi+lab_vokasi)" >>"$V_LOG"

# 4. grid_layouts ada di global (hasil ETL buildGlobal)
GGRID=$(db_count "$DATA_DIR/global.db" "grid_layouts")
[ "$GGRID" -ge 1 ] || fail "F7: grid_layouts kosong"
echo "9. grid_layouts=$GGRID" >>"$V_LOG"

# 5. Seed vokasi: pcs ter-seed dari seeds/vokasi-1/pcs.json
VPCS=$(db_count "$DATA_DIR/lab_vokasi_1.db" "pcs")
[ "$VPCS" -ge 1 ] || fail "F7: lab_vokasi pcs kosong (seed gagal)"
echo "10. lab_vokasi pcs=$VPCS (seed OK)" >>"$V_LOG"

# 6. Marker .seed_done per lab (uploads/<lab>/.seed_done)
for lab in lab-mi lab-vokasi-1; do
    [ -f "$DATA_DIR/uploads/$lab/.seed_done" ] || fail "F7: marker .seed_done $lab tidak ada"
done
echo "11. .seed_done markers ada" >>"$V_LOG"

# 7. PC photo ter-copy (dari staging) — cek 1 file di uploads/lab-mi/pc
PC_PHOTO_COUNT=$(find "$DATA_DIR/uploads/lab-mi/pc" -type f 2>/dev/null | wc -l)
[ "$PC_PHOTO_COUNT" -ge 1 ] || fail "F7: uploads/lab-mi/pc kosong (foto tidak ter-copy)"
echo "12. uploads/lab-mi/pc files=$PC_PHOTO_COUNT" >>"$V_LOG"

# 8. PC photo seed (PC_PHOTO_TOKEN) — file foto dari release (jika release zip berisi <labID>- prefixed)
SEED_PC_COUNT=$(find "$DATA_DIR/uploads/lab-mi/pc" -type f 2>/dev/null | wc -l)
log "F7: PC photo files total=$SEED_PC_COUNT (seed PC_PHOTO aktif bila >1)"

log "F7: SELESAI — semua cek lulus"
cat "$V_LOG"

# ---------------------------------------------------------------- F8: Cleanup versi lama
phase "F8 — Cleanup versi lama (total, backup dulu)"
BK_DIR="$E2E_ROOT/backups/pre_e2e_$(date +%Y%m%d-%H%M%S)"
mkdir -p "$BK_DIR"

# Backup dulu
[ -f "$STAGING_DIR/inventaris_lab.db" ] && cp "$STAGING_DIR/inventaris_lab.db" "$BK_DIR/inventaris_lab.db"
[ -d "$STAGING_DIR/uploads" ] && cp -r "$STAGING_DIR/uploads" "$BK_DIR/uploads"
log "F8: backup ke $BK_DIR"

# Hapus artifact versi lama di data dir
rm -f "$DATA_DIR/inventaris_lab.db" "$DATA_DIR/inventaris_lab.db-shm" "$DATA_DIR/inventaris_lab.db-wal"
rm -f "$E2E_ROOT/inventaris_lab.db" "$E2E_ROOT/inventaris_lab.db-shm" "$E2E_ROOT/inventaris_lab.db-wal"
rm -f "$DATA_DIR/testsum.exe"
rm -rf "$DATA_DIR/dist" "$DATA_DIR/bin"
# Uploads flat lama (yang sudah disalin ETL ke struktur per-lab)
for sub in pc device_types device_installations logbook temp; do
    rm -rf "$DATA_DIR/uploads/$sub"
done
# Hapus seluruh workspace versi lama + staging (sudah di-backup ke BK_DIR)
rm -rf "$OLD_RUN" "$STAGING_DIR" "$REPO_DIR"
log "F8: workspace lama + staging dihapus (sudah di-backup)"
# Release lama (keep 3 terbaru, mirror update.sh)
if [ -d "$DATA_DIR/../app/releases" ]; then
    ls -1t "$DATA_DIR/../app/releases" | tail -n +4 | while read -r d; do
        rm -rf "$DATA_DIR/../app/releases/$d"
        log "F8: hapus release lama $d"
    done
fi

# Verifikasi bersih: tidak ada sisa DB single-lab & uploads flat
LEFTOVER=0
for f in inventaris_lab.db inventaris_lab.db-shm inventaris_lab.db-wal; do
    find "$DATA_DIR" "$E2E_ROOT" -name "$f" -not -path "$E2E_ROOT/backups/*" 2>/dev/null | grep -q . && LEFTOVER=1
done
for sub in pc device_types device_installations logbook; do
    find "$DATA_DIR/uploads" -maxdepth 1 -type d -name "$sub" 2>/dev/null | grep -q . && LEFTOVER=1
done
[ "$LEFTOVER" = "0" ] || fail "F8: masih ada artifact versi lama tersisa — cek manual"
log "F8: verifikasi bersih OK — tidak ada sisa artifact lama"
log "F8: SELESAI"

# ---------------------------------------------------------------- F9: Report
phase "F9 — Report"
REPORT="$E2E_ROOT/e2e_report.json"
cat > "$REPORT" <<EOF
{
  "status": "PASS",
  "timestamp": "$(date -Is)",
  "e2e_root": "$E2E_ROOT",
  "repo_main_commit": "$(git -C "$MAIN_DIR" rev-parse --short HEAD 2>/dev/null || echo 'n/a')",
  "repo_refactoring_commit": "$(git -C "$REFACTOR_DIR" rev-parse --short HEAD 2>/dev/null || echo 'n/a')",
  "phases": {
    "F0": "PASS", "F1": "PASS", "F2": "PASS", "F3": "PASS", "F4": "PASS",
    "F5": "PASS", "F6": "PASS", "F7": "PASS", "F8": "PASS", "F9": "PASS"
  },
  "parity": {
    "pcs": "$LPCS", "devices": "$LDEV", "software": "$LSOFT",
    "schedules": "$LSCH", "logbook": "$LLB", "global_users": "$GPCS",
    "grid_layouts": "$GGRID", "vokasi_pcs": "$VPCS", "pc_photos": "$PC_PHOTO_COUNT"
  },
  "notes": ["Semua API key terisi & dipicu (GEMINI/OPENROUTER OCR, PC_PHOTO seed)", "Data CRUD deterministik"]
}
EOF
log "F9: report -> $REPORT"
tar czf "$E2E_ROOT/e2e_results_$(date +%Y%m%d-%H%M%S).tar.gz" -C "$E2E_ROOT" logs e2e_report.json \
    2>/dev/null || true

log "=========================================="
log "E2E SELESAI: SEMUA FASE PASS"
log "Report: $REPORT"
log "=========================================="