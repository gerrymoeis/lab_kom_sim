#!/bin/bash
# =============================================================================
# SIMLABKOM — Production Deploy (deploy_production.sh)
#
# Orchestrator deploy tools production Linux (/opt/simlab). Dijalankan dari
# folder hasil extract bundle:
#   cd deploy_production_<ts>
#   sudo bash deploy_production.sh [--skip-migrate] [--skip-test]
#
# Tahap (Fase E: P0–PK lengkap; Fase P-B: N-Lab aware; Fase P-C: PostgreSQL aware):
#   P0  Validasi prasyarat + bundle lengkap                  → STOP
#   P1  Deteksi format .env (single/multi) + regenerate      → STOP
#       (N-Lab: REQUIRED_KEYS dari LABS_<N>_* terdeteksi;
#        P-C: DATABASE_URL terisi → backend PostgreSQL, DATABASE_URL lama
#        dipertahankan saat regenerate dari template)
#   P2  Backup penuh data/ + .env + release aktif            → STOP
#       (P-C: di Postgres data DB ada di luar; backup lokal = uploads/.env/release)
#   P3  Stop service + tunggu WAL/SHM                        → STOP
#       (P-C: skip tunggu WAL/SHM — tidak ada SQLite di Postgres)
#   P4  Deteksi migrasi; jalankan ETL bila perlu             → ROLLBACK
#       (N-Lab: config ETL digenerate dari .env; verifikasi lab source;
#        P-C: ETL SQLite-only dilewati di Postgres — verifikasi via app/readyz)
#   P5  Siapkan release dir + seeds (tanpa marker)           → ROLLBACK
#       (N-Lab: seeds per lab terdeteksi / fallback default)
#   P6  Deploy binary + atomic symlink swap                  → ROLLBACK
#   P7  Generate public site (app-simlab-publish)            → WARN
#   P8  Start service                                        → ROLLBACK
#   P9  Health check /healthz                                → ROLLBACK
#   P10 Readiness check /readyz (deep)                       → ROLLBACK
#       (P-C: satu-satunya verifikasi DB di Postgres — ping via app)
#   P11 Verifikasi read-only app-simlab -verify              → ROLLBACK
#       (P-C: -verify SQLite-only dilewati di Postgres)
#   P12 Full test suite refactoring (test binary)            → ROLLBACK
#   P13 Report JSON deploy_report_<ts>.json                  → WARN
#       (P-C: field database.backend = sqlite|postgres, url_set)
#   P14 Cleanup (release keep 3, single DB, uploads flat)    → WARN
#       (N-Lab: single DB pakai MIG_SOURCE_STEM terdeteksi;
#        P-C: di Postgres file .db lokal TIDAK dihapus)
#   PK  Auto-run server + verify final (report digenerate    → WARN
#       ulang agar memuat PK_autorun)
#
# ROLLBACK: stop service, restore symlink + data dari backup, start, health.
# Jalur sukses maupun rollback selalu berakhir dengan server RUNNING.
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib/common.sh"

SKIP_MIGRATE=false
SKIP_TEST=false
for arg in "$@"; do
    case "${arg}" in
        --skip-migrate) SKIP_MIGRATE=true ;;
        --skip-test)    SKIP_TEST=true ;;
    esac
done

# ---------------------------------------------------------------- Variabel global
BACKUP_DIR=""
SAVED_CURRENT=""
MIGRATION_RAN=0
SOURCE_DB="${DATA_DIR}/inventaris_lab.db"

# ---- Variabel utk P13 report (diisi di fase terkait)
MIG_STATUS="skipped"
MIG_GLOBAL_USERS=0
MIG_SUPER_ADMIN=0
MIG_ROWS_PC=0
MIG_UPLOAD_FILES=0
VERIFY_INTEGRITY="n/a"
VERIFY_SUPER_ADMIN=0
VERIFY_ORPHAN=0
VERIFY_UPLOADS=false
VERIFY_SEED=false
READYZ_STATUS="n/a"
SERVER_URL=""

# ---- Variabel utk P12 test suite (diisi di fase terkait)
TEST_TOTAL=0
TEST_PASS=0
TEST_FAIL=0
TEST_SKIP=0
TEST_PKG_OK=""
TEST_PKG_JSON="[]"
TEST_STATUS="not_run"
TEST_RUN_DIR=""
LAB_COUNT=0
SEED_MISSING=0
MIG_SOURCE_STEM="inventaris_lab"
DATABASE_URL=""
DB_BACKEND="sqlite"

# ============================================================================
# N-Lab helpers (Fase P-B): baca daftar lab dari ENV_FILE (.env format V2)
# ============================================================================
# parse_env_labs: output satu baris per lab: "<N>\t<ID>\t<DB>\t<TITLE>\t<URL>"
#   (N = indeks LABS_<N>_*, dimulai 1). Skip bila ID/DB kosong.
parse_env_labs() {
    local n=1 id db title url
    while :; do
        id=$(baca_env "LABS_${n}_ID")
        db=$(baca_env "LABS_${n}_DB")
        [ -n "${id}" ] || break
        [ -n "${db}" ] || break
        title=$(baca_env "LABS_${n}_TITLE")
        url=$(baca_env "LABS_${n}_URL")
        [ -n "${url}" ] || url=$(printf '%s' "${id}" | tr '[:upper:]' '[:lower:]')
        printf '%s\t%s\t%s\t%s\t%s\n' "${n}" "${id}" "${db}" "${title}" "${url}"
        n=$((n + 1))
    done
}
# source_lab_db: DB path lab pertama (mode source ETL) — pola LABS_1_DB.
# Tidak pakai `head -1` (SIGPIPE di bawah set -euo pipefail); baca baris pertama
# langsung dari parse_env_labs. Output kosong bila tidak ada lab.
source_lab_db() {
    local n id db title url
    while IFS=$'\t' read -r n id db title url; do
        printf '%s\n' "${db}"
        return 0
    done < <(parse_env_labs)
    return 0
}
# is_lab_db: true bila path $1 terdaftar sebagai DB salah satu lab di .env.
is_lab_db() {
    local cand="$1" n id db title url
    while IFS=$'\t' read -r n id db title url; do
        [ -n "${db}" ] || continue
        [ "${db}" = "${cand}" ] && return 0
    done < <(parse_env_labs)
    return 1
}
# etl_layout_for_url: fragmen JSON layout utk sebuah lab URL (default: 8/baris).
etl_layout_for_url() {
    local url="$1"
    case "${url}" in
        lab-mi|lab-kom-mi|labkom-mi)
            printf '%s' '"cols": [8, 8, 8, 8, 8], "has_gap": false, "gap_pos": 0, "row_gaps": [[], [], [], [], []]' ;;
        lab-vokasi-1|lab-kom-vokasi-1|labkom-vokasi-1|vokasi)
            printf '%s' '"cols": [11, 9, 11, 11], "has_gap": true, "gap_pos": 5, "row_gaps": [[5], [5], [5], [5]]' ;;
        *)
            printf '%s' '"cols": [8, 8, 8, 8, 8], "has_gap": false, "gap_pos": 0, "row_gaps": [[], [], [], [], []]' ;;
    esac
}

# ============================================================================
# ROLLBACK — dipicu kegagalan P4–P11
# ============================================================================
rollback() {
    log "🔄 ROLLBACK dimulai..."
    systemctl stop "${SERVICE_NAME}" 2>/dev/null || true

    # 1) restore symlink release sebelumnya
    if [ -n "${SAVED_CURRENT}" ] && [ -d "${SAVED_CURRENT}" ]; then
        ln -sfn "${SAVED_CURRENT}" "${CURRENT_DIR}.new"
        mv -T "${CURRENT_DIR}.new" "${CURRENT_DIR}"
        log "ROLLBACK: symlink → ${SAVED_CURRENT}"
    fi

    # 2) hapus file hasil ETL (bila migrasi baru berjalan) lalu restore backup.
    #    N-Lab aware: hapus DB + uploads semua lab terdeteksi (dari .env),
    #    bukan hardcode lab_mi_1.db / lab_vokasi_1.db.
    if [ "${MIGRATION_RAN}" -eq 1 ] && [ -n "${BACKUP_DIR}" ] && [ -d "${BACKUP_DIR}" ]; then
        rm -f "${DATA_DIR}/global.db" "${DATA_DIR}/global.db-shm" "${DATA_DIR}/global.db-wal"
        while IFS=$'\t' read -r n id db title url; do
            [ -n "${id}" ] || continue
            [ -n "${db}" ] || continue
            rm -f "${db}" "${db}-shm" "${db}-wal"
            rm -rf "${UPLOADS_DIR}/${url}"
        done < <(parse_env_labs)
        rm -f "${DATA_DIR}/migration_report.json"
    fi
    if [ -n "${BACKUP_DIR}" ] && [ -d "${BACKUP_DIR}" ]; then
        restore_backup "${BACKUP_DIR}"
        # Bila release sebelumnya single-lab, pulihkan .env ASLI (sebelum P1 regenerate),
        # bukan .env multi-lab hasil regenerate (env.bak).
        if [ -f "${BACKUP_DIR}/env.single_lab.bak" ]; then
            cp "${BACKUP_DIR}/env.single_lab.bak" "${ENV_FILE}"
            chmod 600 "${ENV_FILE}"
            log "ROLLBACK: .env dikembalikan ke versi single-lab asli"
        fi
    fi

    # 3) start + health
    systemctl start "${SERVICE_NAME}" 2>/dev/null || true
    sleep 3
    if health_check; then
        ok "🔄 ROLLBACK BERHASIL — server kembali RUNNING"
        phase_summary
        exit 1
    fi
    error "Rollback juga gagal — butuh intervensi manual. Cek: journalctl -u ${SERVICE_NAME} -n 50"
}

# ============================================================================
# P0 — VALIDASI PRASYARAT + BUNDLE
# ============================================================================
declare_phase "P0" "Validasi prasyarat & bundle"
check_root
check_cmds
check_disk
mkdir -p "${RELEASES_DIR}" "${DATA_DIR}" "${DATA_DIR}/uploads" "${DATA_DIR}/backups"

# Validasi isi bundle (bin/ + config/ + seeds/)
# Catatan: tar dari build machine (Windows) menyimpan perm 0644 tanpa exec bit;
# kewajiban -f (ada), exec bit di-set saat dipakai (P4 etl, P6 app-simlab).
[ -f "${SCRIPT_DIR}/bin/etl" ]          || error "P0: bundle tidak lengkap — bin/etl hilang"
[ -f "${SCRIPT_DIR}/bin/app-simlab" ]   || error "P0: bundle tidak lengkap — bin/app-simlab hilang"
[ -f "${SCRIPT_DIR}/bin/app-simlab-publish" ] || error "P0: bundle tidak lengkap — bin/app-simlab-publish hilang"
[ -f "${SCRIPT_DIR}/config/etl-config.production.json" ] || error "P0: config/etl-config.production.json hilang"
[ -f "${SCRIPT_DIR}/config/.env.config" ] || error "P0: config/.env.config hilang (API key)"
for seed in mi-1 vokasi-1 default; do
    [ -d "${SCRIPT_DIR}/seeds/${seed}" ] || error "P0: seeds/${seed} hilang di bundle"
done
[ -d "${SCRIPT_DIR}/test-runner/test-bin" ] || warn "P0: test-runner/test-bin belum ada (Fase D)"
# User service (dibuat install.sh). Bila belum ada → warning jelas: chown akan gagal.
if id -u "${APP_NAME}" >/dev/null 2>&1; then
    log "P0: user ${APP_NAME} ada (uid $(id -u "${APP_NAME}"))"
else
    warn "P0: user '${APP_NAME}' tidak ditemukan — jalankan install.sh dulu (membuat user + service),"
    warn "    atau buat manual: useradd -r -s /usr/sbin/nologin ${APP_NAME}. chown nanti akan gagal."
fi
phase_pass "P0"

# ============================================================================
# P1 — DETEKSI FORMAT .env
# ============================================================================
declare_phase "P1" "Deteksi format .env (single/multi-lab)"
BACKUP_DIR="$(backup_dir)"
ENV_CONFIG_TEMPLATE="${SCRIPT_DIR}/config/.env.config"
SECRET=""

regenerate_env() {
    # $1 = secret yang akan dipakai; bila kosong → generate baru
    local secret="$1"
    if [ -z "${secret}" ]; then
        secret="$(generate_session_secret)"
        log "SESSION_SECRET baru digenerate"
    else
        log "SESSION_SECRET dipreserve dari .env lama"
    fi
    # P-C: preserve DATABASE_URL bila lama terisi (regenerate dari template
    # menimpa .env; DATABASE_URL template kosong → backend Postgres hilang).
    local old_db_url
    old_db_url="$(baca_env DATABASE_URL)"
    sed "s/__AUTO_GENERATE__/${secret}/g" "${ENV_CONFIG_TEMPLATE}" > "${ENV_FILE}"
    if [ -n "${old_db_url}" ]; then
        # Ganti baris DATABASE_URL secara aman (grep buang + append; nilai URL
        # bisa memuat &, ?, = yang tidak aman untuk delimiter sed).
        grep -v '^DATABASE_URL=' "${ENV_FILE}" > "${ENV_FILE}.tmp" || true
        printf 'DATABASE_URL=%s\n' "${old_db_url}" >> "${ENV_FILE}.tmp"
        mv "${ENV_FILE}.tmp" "${ENV_FILE}"
        log "DATABASE_URL lama dipertahankan (backend PostgreSQL)"
    fi
    chmod 600 "${ENV_FILE}"
    log "✅ .env diregenerate dari template (SESSION_SECRET ${#secret} char)"
}

if [ -f "${ENV_FILE}" ]; then
    if is_multi_lab_env; then
        log "P1: .env sudah format multi-lab — skip regenerate"
        SECRET="$(baca_env SESSION_SECRET)"
    else
        log "P1: .env format single-lab (lama) — backup & regenerate dari template"
        cp "${ENV_FILE}" "${BACKUP_DIR}/env.single_lab.bak"
        chmod 600 "${BACKUP_DIR}/env.single_lab.bak"
        SECRET="$(baca_env SESSION_SECRET)"
        if [ -n "${SECRET}" ] && [ "${SECRET}" != "__AUTO_GENERATE__" ]; then
            regenerate_env "${SECRET}"
        else
            regenerate_env ""
        fi
    fi
else
    log "P1: .env belum ada — generate dari template"
    regenerate_env ""
fi

# Validasi key wajib ada (nilai tidak di-log). Daftar lab dibaca dari .env
# (N-Lab aware): REQUIRED_KEYS dibangun dari LABS_<N>_* yang terdeteksi,
# bukan hardcode LABS_1/LABS_2.
REQUIRED_KEYS="GLOBAL_DB_PATH SESSION_SECRET UPLOAD_PATH"
LAB_COUNT=0
while IFS=$'\t' read -r n id db title url; do
    [ -n "${id}" ] || continue
    REQUIRED_KEYS="${REQUIRED_KEYS} LABS_${n}_ID LABS_${n}_DB LABS_${n}_TITLE LABS_${n}_URL"
    LAB_COUNT=$((LAB_COUNT + 1))
done < <(parse_env_labs)
[ "${LAB_COUNT}" -ge 1 ] || error "P1: tidak ada LABS_<N>_* terdeteksi di ${ENV_FILE}"
for key in ${REQUIRED_KEYS}; do
    val=$(baca_env "${key}")
    [ -n "${val}" ] || error "P1: key wajib '${key}' kosong di ${ENV_FILE}"
done
for key in GEMINI_API_KEY OPENROUTER_API_KEY PC_PHOTO_TOKEN; do
    val=$(baca_env "${key}")
    [ -n "${val}" ] || warn "P1: API key '${key}' kosong (server fitur terkait tidak jalan)"
done
# P-C: deteksi backend DB. DATABASE_URL terisi → PostgreSQL (Neon); nilai tidak
# di-log (sekret). ETL/backup/-verify SQLite-only → alur menyesuaikan (lihat P3/P4/P11/P14).
DATABASE_URL="$(baca_env DATABASE_URL)"
if [ -n "${DATABASE_URL}" ]; then
    DB_BACKEND="postgres"
    log "P1: DATABASE_URL terisi → backend PostgreSQL aktif"
    warn "P1: PostgreSQL aktif — ETL (SQLite-only) & -verify dilewati; backup data lokal hanya uploads/.env/release;"
    warn "    verifikasi DB Postgres via app (/readyz, P10). File .db lokal tidak dihapus (P14)."
else
    DB_BACKEND="sqlite"
    log "P1: DATABASE_URL kosong → backend SQLite (default)"
fi
phase_pass "P1"

# ============================================================================
# P2 — BACKUP PENUH
# ============================================================================
declare_phase "P2" "Backup penuh (data/ + .env + release aktif)"
tarball_backup "${BACKUP_DIR}"
phase_pass "P2"

# ============================================================================
# P3 — STOP SERVICE + TUNGGU WAL/SHM
# ============================================================================
declare_phase "P3" "Stop service & tunggu WAL/SHM tertutup"
service_stop
if [ "${DB_BACKEND}" = "postgres" ]; then
    log "P3: backend PostgreSQL — tidak ada WAL/SHM SQLite, skip tunggu_wal_closed"
else
    tunggu_wal_closed
fi
phase_pass "P3"

# ============================================================================
# P4 — DETEKSI MIGRASI + ETL
# ============================================================================
declare_phase "P4" "Deteksi migrasi single→multi (ETL bila perlu)"

should_migrate() {
    # migrasi perlu bila ada single DB (source) DAN belum ada global.db
    local sd
    sd=$(detect_source_db)
    [ -f "${sd}" ] || return 1
    [ -f "${DATA_DIR}/global.db" ] && return 1
    return 0
}
detect_source_db() {
    if [ -f "${SOURCE_DB}" ]; then
        echo "${SOURCE_DB}"
        return 0
    fi
    local f base
    for f in "${DATA_DIR}"/*.db; do
        [ -e "${f}" ] || continue
        base="$(basename "${f}")"
        case "${base}" in
            global.db|lab_*.db) continue ;;
        esac
        # N-Lab aware: skip DB yang terdaftar sebagai lab di .env
        is_lab_db "${f}" && continue
        echo "${f}"
        return 0
    done
    echo "${SOURCE_DB}"
}
detect_source_upload_dir() {
    # Baseline-aware: server single-lab menyimpan di uploads/<urlPath>/<sub>/
    # (urlPath = lowercase nama file DB). $1 = path source DB hasil deteksi
    # (default ${SRC_DB}, lalu ${SOURCE_DB}). Fallback "lab-kom-mi" (default ETL).
    local src_db="${1:-}" dbname cand
    [ -n "${src_db}" ] || src_db="${SRC_DB:-${SOURCE_DB}}"
    dbname="$(basename "${src_db}" .db)"
    if [ -d "${UPLOADS_DIR}/${dbname}/pc" ]; then echo "${dbname}"; return 0; fi
    if [ -d "${UPLOADS_DIR}/lab-kom-mi/pc" ]; then echo "lab-kom-mi"; return 0; fi
    for cand in "${UPLOADS_DIR}"/*/; do
        [ -d "${cand}" ] || continue
        if [ -d "${cand}pc" ]; then echo "$(basename "${cand}")"; return 0; fi
    done
    echo "lab-kom-mi"
}
generate_etl_config() {
    local src_db="$1" src_upload_dir="$2" runtime
    runtime="${BACKUP_DIR}/etl-config.runtime.json"
    # Bangun labs[] dari daftar lab terdeteksi di .env (N-Lab aware),
    # bukan sed atas template statis 2-lab. Mode: lab pertama = source
    # (penerima copy data), sisanya seed. Bila tidak ada lab → fallback
    # default MI-1+VOKASI-1 (backward-compat).
    local n id db title url layout mode
    local first=1
    {
        echo "{"
        echo "  \"source_db\": \"${src_db}\","
        echo "  \"source_uploads\": \"${UPLOADS_DIR}\","
        echo "  \"source_upload_dir\": \"${src_upload_dir}\","
        echo "  \"global_db\": \"${DATA_DIR}/global.db\","
        echo "  \"uploads_dest\": \"${UPLOADS_DIR}\","
        echo "  \"main_account_suffix\": \"123\","
        echo "  \"labs\": ["
        local has_lab=0 printed=0
        while IFS=$'\t' read -r n id db title url; do
            [ -n "${id}" ] || continue
            has_lab=1
            if [ "${first}" -eq 1 ]; then
                mode="source"
                first=0
            else
                mode="seed"
            fi
            layout="$(etl_layout_for_url "${url}")"
            if [ "${printed}" -eq 1 ]; then
                printf ',\n'
            fi
            if [ -n "${db}" ]; then
                printf '    {\n      "id": "%s",\n      "url": "%s",\n      "db": "%s",\n      "title": "%s",\n      "mode": "%s",\n      %s\n    }' \
                    "${id}" "${url}" "${db}" "${title}" "${mode}" "${layout}"
            else
                printf '    {\n      "id": "%s",\n      "url": "%s",\n      "title": "%s",\n      "mode": "%s",\n      %s\n    }' \
                    "${id}" "${url}" "${title}" "${mode}" "${layout}"
            fi
            printed=1
        done < <(parse_env_labs)
        if [ "${has_lab}" -eq 0 ]; then
            # Fallback default 2 lab (backward-compat, sama template lama)
            printf '    {\n      "id": "MI-1",\n      "url": "lab-mi",\n      "db": "%s",\n      "title": "Lab Kom MI",\n      "mode": "source",\n      %s\n    },\n' \
                "${DATA_DIR}/lab_mi_1.db" "$(etl_layout_for_url "lab-mi")"
            printf '    {\n      "id": "VOKASI-1",\n      "url": "lab-vokasi-1",\n      "db": "%s",\n      "title": "Lab Kom Vokasi 1",\n      "mode": "seed",\n      %s\n    }\n' \
                "${DATA_DIR}/lab_vokasi_1.db" "$(etl_layout_for_url "lab-vokasi-1")"
        fi
        echo "  ]"
        echo "}"
    } > "${runtime}"
    echo "${runtime}"
}

if [ "${SKIP_MIGRATE}" = "true" ]; then
    phase_skip "P4" "--skip-migrate"
elif [ "${DB_BACKEND}" = "postgres" ]; then
    # P-C: ETL SQLite-only — tidak bisa baca data PostgreSQL. Bila server
    # single-DB Postgres → bukan SQLite file; bila sudah multi → global.db
    # tidak dipakai. Verifikasi DB Postgres via app (/readyz, P10).
    log "P4: backend PostgreSQL — ETL dilewati (SQLite-only), verifikasi via app"
    phase_skip "P4" "PostgreSQL aktif (ETL SQLite-only)"
    MIG_STATUS="postgres"
elif should_migrate; then
    SRC_DB="$(detect_source_db)"
    SRC_UPLOAD_DIR="$(detect_source_upload_dir "${SRC_DB}")"
    log "P4: migrasi diperlukan — source_db=${SRC_DB} source_upload_dir=${SRC_UPLOAD_DIR}"
    if [ ! -f "${SRC_DB}" ]; then
        phase_fail "P4" "source DB tidak ditemukan: ${SRC_DB}"
        rollback
    fi

    RUNTIME_CFG="$(generate_etl_config "${SRC_DB}" "${SRC_UPLOAD_DIR}")"
    log "P4: config ETL runtime: ${RUNTIME_CFG}"

    chmod +x "${SCRIPT_DIR}/bin/etl"
    if ! (cd "${SCRIPT_DIR}" && ./bin/etl -config "${RUNTIME_CFG}" -force) 2>&1 | tee "${BACKUP_DIR}/etl.log"; then
        phase_fail "P4" "ETL gagal — lihat ${BACKUP_DIR}/etl.log"
        rollback
    fi
    MIGRATION_RAN=1

    # Verifikasi hasil ETL (N-Lab aware: lab source = lab pertama dari .env)
    SRC_LAB_DB="$(source_lab_db)"
    [ -n "${SRC_LAB_DB}" ] || { phase_fail "P4" "tidak ada lab source terdeteksi"; rollback; }
    [ -f "${DATA_DIR}/global.db" ]       || { phase_fail "P4" "global.db tidak dihasilkan"; rollback; }
    [ -f "${SRC_LAB_DB}" ]               || { phase_fail "P4" "DB lab source tidak dihasilkan: ${SRC_LAB_DB}"; rollback; }
    [ -f "${DATA_DIR}/migration_report.json" ] || { phase_fail "P4" "migration_report.json hilang"; rollback; }
    MIG_SOURCE_STEM="$(basename "${SRC_DB}" .db)"
    SUPER_ADMIN=$(grep -o '"super_admin_count": *[0-9]*' "${DATA_DIR}/migration_report.json" | grep -o '[0-9]*$' || true)
    ROWS_PC=$(grep -o '"rows_pcs": *[0-9]*' "${DATA_DIR}/migration_report.json" | grep -o '[0-9]*$' || true)
    [ -n "${SUPER_ADMIN}" ] && [ "${SUPER_ADMIN}" -ge 1 ] \
        || { phase_fail "P4" "super_admin_count=${SUPER_ADMIN:-0} < 1 di report ETL"; rollback; }
    [ -n "${ROWS_PC}" ] && [ "${ROWS_PC}" -gt 0 ] \
        || warn "P4: rows_pcs=${ROWS_PC:-0} (0) — periksa data source kosong?"
    log "P4: ETL OK — super_admin=${SUPER_ADMIN} rows_pcs=${ROWS_PC:-0}"
    MIG_STATUS="ran"
    MIG_SUPER_ADMIN="${SUPER_ADMIN:-0}"
    MIG_ROWS_PC="${ROWS_PC:-0}"
    MIG_GLOBAL_USERS=$(grep -o '"global_users": *[0-9]*' "${DATA_DIR}/migration_report.json" | grep -o '[0-9]*$' || echo 0)
    MIG_UPLOAD_FILES=$(grep -o '"upload_files_copied": *[0-9]*' "${DATA_DIR}/migration_report.json" | grep -o '[0-9]*$' || echo 0)
    chown_data
    phase_pass "P4"
else
    phase_skip "P4" "sudah multi-DB / tidak ada source single-DB"
fi

# ============================================================================
# P5 — SIAPKAN RELEASE DIR + SEEDS (tanpa marker .seed_done)
# ============================================================================
declare_phase "P5" "Siapkan release dir + seeds (tanpa marker)"
TS="$(date +%Y%m%d-%H%M%S)"
RELEASE_DIR="${RELEASES_DIR}/${TS}"
mkdir -p "${RELEASE_DIR}/seeds"
cp -r "${SCRIPT_DIR}/seeds/." "${RELEASE_DIR}/seeds/"
# Verifikasi seeds (N-Lab aware): tiap lab terdeteksi harus punya folder
# seeds/<lowercase id> atau fallback seeds/default (pola resolveSeedFolder app).
SEED_MISSING=0
while IFS=$'\t' read -r n id db title url; do
    [ -n "${id}" ] || continue
    seed_id="$(printf '%s' "${id}" | tr '[:upper:]' '[:lower:]')"
    if [ ! -d "${RELEASE_DIR}/seeds/${seed_id}" ] && [ ! -d "${RELEASE_DIR}/seeds/default" ]; then
        warn "P5: seeds/${seed_id} tidak ada & seeds/default tidak ada — lab ${id} tak punya seed"
        SEED_MISSING=1
    fi
done < <(parse_env_labs)
[ "${SEED_MISSING}" -eq 0 ] || { phase_fail "P5" "seeds lab tidak lengkap"; rollback; }
# JANGAN buat marker .seed_done — RunSeedFolder menulisnya sendiri saat boot.
if find "${RELEASE_DIR}/seeds" -name ".seed_done" -print -quit | grep -q .; then
    { phase_fail "P5" "marker .seed_done terdeteksi di seeds — hapus manual"; rollback; }
fi
chown -R "${APP_NAME}:${APP_NAME}" "${RELEASE_DIR}"
log "P5: release dir ${RELEASE_DIR} + seeds siap"
phase_pass "P5"

# ============================================================================
# P6 — DEPLOY BINARY + ATOMIC SYMLINK SWAP
# ============================================================================
declare_phase "P6" "Deploy binary + atomic symlink swap"
cp "${SCRIPT_DIR}/bin/app-simlab" "${SCRIPT_DIR}/bin/app-simlab-publish" "${RELEASE_DIR}/"
chmod +x "${RELEASE_DIR}/app-simlab" "${RELEASE_DIR}/app-simlab-publish"
cp "${ENV_FILE}" "${RELEASE_DIR}/.env"
chmod 600 "${RELEASE_DIR}/.env"

if [ -L "${CURRENT_DIR}" ] && [ -d "$(readlink -f "${CURRENT_DIR}" 2>/dev/null)" ]; then
    SAVED_CURRENT="$(readlink -f "${CURRENT_DIR}")"
    log "P6: release sebelumnya: ${SAVED_CURRENT}"
fi
ln -sfn "${RELEASE_DIR}" "${CURRENT_DIR}.new"
mv -T "${CURRENT_DIR}.new" "${CURRENT_DIR}"
chown -R "${APP_NAME}:${APP_NAME}" "${RELEASE_DIR}"
log "P6: symlink ${CURRENT_DIR} → ${RELEASE_DIR}"
phase_pass "P6"

# ============================================================================
# P7 — GENERATE PUBLIC SITE (non-fatal)
# ============================================================================
declare_phase "P7" "Generate public site (app-simlab-publish)"
if [ -x "${RELEASE_DIR}/app-simlab-publish" ]; then
    if (cd "${RELEASE_DIR}" && ./app-simlab-publish) 2>&1 | tee "${BACKUP_DIR}/publish.log"; then
        log "P7: public site generated"
        phase_pass "P7"
    else
        warn "P7: public build gagal — server akan generate otomatis saat ada perubahan data"
        phase_warn "P7" "publish gagal (non-fatal)"
    fi
else
    warn "P7: app-simlab-publish tidak ditemukan — skip"
    phase_warn "P7" "binary hilang"
fi

# ============================================================================
# P8 — START SERVICE
# ============================================================================
declare_phase "P8" "Start service ${SERVICE_NAME}"
if ! service_start 2>&1 | tee "${BACKUP_DIR}/start.log"; then
    phase_fail "P8" "systemctl start gagal"
    rollback
fi
sleep 3
if ! service_is_active; then
    phase_fail "P8" "service tidak active setelah start"
    rollback
fi
log "P8: service active"
phase_pass "P8"

# ============================================================================
# P9 — HEALTH CHECK
# ============================================================================
declare_phase "P9" "Health check /healthz"
if ! health_check; then
    phase_fail "P9" "healthz gagal"
    rollback
fi
phase_pass "P9"

# ============================================================================
# P10 — READINESS CHECK (deep)
# ============================================================================
declare_phase "P10" "Readiness check /readyz (semua DB lab + global + uploads)"
if ! readyz_check; then
    READYZ_STATUS="fail"
    phase_fail "P10" "readyz gagal"
    rollback
fi
READYZ_STATUS="ok"
phase_pass "P10"

# ============================================================================
# P11 — VERIFIKASI READ-ONLY (app-simlab -verify)
# ============================================================================
declare_phase "P11" "Verifikasi read-only app-simlab -verify"
# -verify read-only: integrity, super_admin>=1, orphan FK, uploads subdir, marker .seed_done.
# Dijalankan dari RELEASE_DIR (memuat .env yang benar via config.Load CWD).
VERIFY_LOG="${BACKUP_DIR}/verify.log"
if [ "${DB_BACKEND}" = "postgres" ]; then
    # P-C: verify.Run SQLite-only (membuka file .db langsung) — di PostgreSQL DB
    # tidak berupa file lokal. Verifikasi DB Postgres via app (/readyz, P10).
    log "P11: backend PostgreSQL — app-simlab -verify (SQLite-only) dilewati"
    phase_skip "P11" "PostgreSQL aktif (-verify SQLite-only)"
elif [ -x "${RELEASE_DIR}/app-simlab" ]; then
    if (cd "${RELEASE_DIR}" && ./app-simlab -verify) > "${VERIFY_LOG}" 2>&1; then
        log "P11: app-simlab -verify OK (exit 0) — lihat ${VERIFY_LOG}"
        phase_pass "P11"
    else
        phase_fail "P11" "app-simlab -verify exit != 0 — lihat ${VERIFY_LOG}"
        rollback
    fi
else
    phase_fail "P11" "app-simlab tidak ditemukan di RELEASE_DIR"
    rollback
fi
# Parsing ringkasan verify (untuk P13 report)
if [ -f "${VERIFY_LOG}" ]; then
    if grep -q "integrity_check ok" "${VERIFY_LOG}"; then VERIFY_INTEGRITY="ok"; fi
    VERIFY_SUPER_ADMIN=$(grep -o 'super_admin=[0-9]*' "${VERIFY_LOG}" | head -1 | cut -d= -f2)
    [ -n "${VERIFY_SUPER_ADMIN}" ] || VERIFY_SUPER_ADMIN=0
    if grep -q "orphan FK=0 ok" "${VERIFY_LOG}"; then VERIFY_ORPHAN=0; fi
    if grep -q "pc/ ok" "${VERIFY_LOG}"; then VERIFY_UPLOADS=true; fi
    if grep -q "marker .seed_done=true" "${VERIFY_LOG}"; then VERIFY_SEED=true; fi
fi

# ============================================================================
# P12 — FULL TEST SUITE REFACTORING (test binary, tanpa go CLI)
# ============================================================================
declare_phase "P12" "Full test suite refactoring (test binary, 0-skip 0-error)"
# Komponen #6: jalankan SEMUA test binary dari bundle (test-runner/test-bin)
# dengan -test.v -test.count=1 -test.timeout=600s, parse Total/Passed/Failed/Skipped
# (pola F10). Bila ada FAIL atau SKIP → fase FAIL → ROLLBACK.
TEST_RUN_DIR="${BACKUP_DIR}/test_run"
if [ "${SKIP_TEST}" = "true" ]; then
    TEST_STATUS="skipped"
    phase_skip "P12" "--skip-test"
elif [ ! -d "${SCRIPT_DIR}/test-runner/test-bin" ]; then
    TEST_STATUS="missing"
    warn "P12: test-runner/test-bin tidak ada di bundle — skip (Fase D bundle belum lengkap?)"
    phase_warn "P12" "test binary hilang di bundle"
else
    # 1) Siapkan staging test-runner (go.mod + seeds/ + .env.reference) dari bundle
    rm -rf "${TEST_RUN_DIR}"
    mkdir -p "${TEST_RUN_DIR}"
    cp -r "${SCRIPT_DIR}/test-runner/." "${TEST_RUN_DIR}/"
    chmod +x "${TEST_RUN_DIR}"/test-bin/*.test
    log "P12: staging test-runner → ${TEST_RUN_DIR}"

    # 2) Jalankan tiap test binary dari test-runner dir (TestMain `tests` chdir
    #    ke projectRoot = dir berisi go.mod → seeds/ + .env.reference harus ada).
    pkg_log="" pass=0 fail=0 skip=0 pkg="" t="" logfile="" rc=0
    for t in "${TEST_RUN_DIR}"/test-bin/*.test; do
        [ -f "${t}" ] || continue
        pkg="$(basename "${t}" .test)"
        logfile="${TEST_RUN_DIR}/P12_${pkg}.log"
        if (cd "${TEST_RUN_DIR}" && "./test-bin/${pkg}.test" -test.v -test.count=1 -test.timeout=600s) > "${logfile}" 2>&1; then
            rc=0
        else
            rc=$?
        fi
        pass=$(grep -c '^--- PASS:' "${logfile}" || true)
        fail=$(grep -c '^--- FAIL:' "${logfile}" || true)
        skip=$(grep -c '^--- SKIP:' "${logfile}" || true)
        TEST_PASS=$((TEST_PASS + pass))
        TEST_FAIL=$((TEST_FAIL + fail))
        TEST_SKIP=$((TEST_SKIP + skip))
        TEST_TOTAL=$((TEST_TOTAL + pass + fail + skip))
        log "P12: ${pkg}.test → pass=${pass} fail=${fail} skip=${skip} rc=${rc}"
        if [ "${fail}" -eq 0 ] && [ "${skip}" -eq 0 ]; then
            TEST_PKG_OK="${TEST_PKG_OK} ${pkg}"
        fi
    done

    # 3) Verifikasi 0-skip 0-error (pola F10)
    if [ "${TEST_FAIL}" -gt 0 ]; then
        log "P12: daftar FAILED:"
        grep -h '^--- FAIL:' "${TEST_RUN_DIR}"/P12_*.log || true
    fi
    if [ "${TEST_SKIP}" -gt 0 ]; then
        log "P12: daftar SKIPPED:"
        grep -h '^--- SKIP:' "${TEST_RUN_DIR}"/P12_*.log || true
    fi
    if [ "${TEST_FAIL}" -eq 0 ] && [ "${TEST_SKIP}" -eq 0 ] && [ "${TEST_TOTAL}" -ge 1 ]; then
        TEST_STATUS="ran"
        # Bangun JSON array packages dari TEST_PKG_OK (spasi-separated).
        TEST_PKG_JSON="["
        local_pkg_json=""
        for pkg in ${TEST_PKG_OK}; do
            if [ -n "${local_pkg_json}" ]; then TEST_PKG_JSON="${TEST_PKG_JSON}, "; fi
            TEST_PKG_JSON="${TEST_PKG_JSON}\"${pkg}\""
            local_pkg_json=1
        done
        TEST_PKG_JSON="${TEST_PKG_JSON}]"
        log "P12: SELESAI — semua test lolos (total=${TEST_TOTAL} pass=${TEST_PASS})"
        phase_pass "P12"
    else
        TEST_STATUS="failed"
        phase_fail "P12" "test suite GAGAL (pass=${TEST_PASS} fail=${TEST_FAIL} skip=${TEST_SKIP}) — lihat ${TEST_RUN_DIR}"
        rollback
    fi
fi

# ============================================================================
# P13 — REPORT JSON
# ============================================================================
declare_phase "P13" "Report JSON deploy_report_<ts>.json"
# Komponen #4: pola e2e_report.json + migration_report.json.
# write_report: fungsi agar report bisa digenerate ulang (P13 awal & setelah PK
# agar memuat PK_autorun).
REPORT_TS="$(date +%Y%m%d-%H%M%S)"
REPORT_FILE="${DATA_DIR}/backups/deploy_report_${REPORT_TS}.json"
BUNDLE_COMMIT="$(grep '^commit=' "${SCRIPT_DIR}/bundle-meta.txt" 2>/dev/null | head -1 | cut -d= -f2- || echo unknown)"

write_report() {
    # SERVER_URL: fallback bila hostname -I kosong (distro minimal/container).
    # Guard `|| true`: hostname -I tidak ada → command substitution gagal di
    # bawah set -euo pipefail, jangan biarkan write_report exit.
    local server_ip=""
    server_ip="$(hostname -I 2>/dev/null | awk '{print $1}')" || true
    [ -n "${server_ip}" ] || server_ip="$(hostname 2>/dev/null || echo 'localhost')"
    SERVER_URL="http://${server_ip}:$(get_port)"
    {
        echo "{"
        echo "  \"timestamp\": \"$(date -Is)\","
        echo "  \"release_tag\": \"bundle-${REPORT_TS}\","
        echo "  \"commit\": \"${BUNDLE_COMMIT}\","
        echo "  \"environment\": \"production\","
        echo "  \"database\": {"
        echo "    \"backend\": \"${DB_BACKEND}\","
        echo "    \"url_set\": $( [ -n "${DATABASE_URL}" ] && echo true || echo false )"
        echo "  },"
        echo "  \"phases\": $(phase_json),"
        echo "  \"migration\": {"
        echo "    \"status\": \"${MIG_STATUS}\","
        echo "    \"global_users\": ${MIG_GLOBAL_USERS},"
        echo "    \"rows_pcs\": ${MIG_ROWS_PC},"
        echo "    \"super_admin\": ${MIG_SUPER_ADMIN},"
        echo "    \"upload_files_copied\": ${MIG_UPLOAD_FILES}"
        echo "  },"
        echo "  \"verify\": {"
        echo "    \"integrity\": \"${VERIFY_INTEGRITY}\","
        echo "    \"super_admin_count\": ${VERIFY_SUPER_ADMIN},"
        echo "    \"orphan_fk\": ${VERIFY_ORPHAN},"
        echo "    \"uploads_ok\": ${VERIFY_UPLOADS},"
        echo "    \"seed_done\": ${VERIFY_SEED}"
        echo "  },"
        echo "  \"tests\": {"
        echo "    \"status\": \"${TEST_STATUS}\","
        echo "    \"total\": ${TEST_TOTAL},"
        echo "    \"pass\": ${TEST_PASS},"
        echo "    \"fail\": ${TEST_FAIL},"
        echo "    \"skip\": ${TEST_SKIP},"
        echo "    \"packages\": ${TEST_PKG_JSON},"
        echo "    \"log\": \"${TEST_RUN_DIR}\""
        echo "  },"
        echo "  \"server\": {"
        echo "    \"service_active\": $(service_is_active && echo true || echo false),"
        echo "    \"readyz\": \"${READYZ_STATUS}\","
        echo "    \"url\": \"${SERVER_URL}\""
        echo "  }"
        echo "}"
    } > "${REPORT_FILE}"
    if [ -s "${REPORT_FILE}" ]; then
        chmod 640 "${REPORT_FILE}"
        log "report → ${REPORT_FILE}"
    else
        warn "report tidak tertulis"
        phase_warn "P13" "report gagal ditulis"
    fi
}

phase_pass "P13"
write_report

# ============================================================================
# P14 — CLEANUP + VERIFIER
# ============================================================================
declare_phase "P14" "Cleanup (release keep 3, single DB, uploads flat)"
LEFTOVER=0

# 1) release lama keep 3 (kecuali release aktif)
cleanup_old_releases

# 2) single DB lama (sudah di-backup) — nama dari source terdeteksi
#    (MIG_SOURCE_STEM), default inventaris_lab bila migrasi tidak berjalan.
#    P-C: skip bila PostgreSQL — file .db lokal TIDAK dihapus (data di Postgres).
if [ "${DB_BACKEND}" != "postgres" ]; then
    for f in "${MIG_SOURCE_STEM}.db" "${MIG_SOURCE_STEM}.db-shm" "${MIG_SOURCE_STEM}.db-wal"; do
        if [ -e "${DATA_DIR}/${f}" ]; then
            rm -f "${DATA_DIR}/${f}"
            log "P14: hapus single DB lama ${f}"
        fi
    done
fi

# 3) uploads flat lama (pc, device_types, device_installations, logbook, temp)
for sub in pc device_types device_installations logbook temp; do
    if [ -d "${DATA_DIR}/uploads/${sub}" ]; then
        rm -rf "${DATA_DIR}/uploads/${sub}"
        log "P14: hapus uploads flat ${sub}"
    fi
done

# 4) artifact lama di release dir (dist/, bin/, testsum.exe) — cleanup.go juga
#    menanganinya saat boot; ini pembersih ekstra agar verifier bersih.
for a in dist bin testsum.exe; do
    if [ -e "${RELEASE_DIR}/${a}" ]; then
        rm -rf "${RELEASE_DIR}/${a}"
        log "P14: hapus artifact ${a} di release"
    fi
done

# ---- Cleanup verifier (komponen #5) ----
# P-C: di PostgreSQL tidak ada file DB lokal yang harus bersih; verifier DB
# (single DB + WAL/SHM) dilewati — uploads flat & release tetap diperiksa.
if [ "${DB_BACKEND}" != "postgres" ]; then
    for f in "${MIG_SOURCE_STEM}.db" "${MIG_SOURCE_STEM}.db-shm" "${MIG_SOURCE_STEM}.db-wal"; do
        if find "${DATA_DIR}" -name "${f}" -not -path "${BACKUP_DIR}/*" 2>/dev/null | grep -q .; then
            LEFTOVER=1
        fi
    done
fi
for sub in pc device_types device_installations logbook; do
    [ -d "${DATA_DIR}/uploads/${sub}" ] && LEFTOVER=1
done
REL_COUNT=$(ls -1t "${RELEASES_DIR}" 2>/dev/null | wc -l)
[ "${REL_COUNT}" -le 3 ] || LEFTOVER=1
if [ "${DB_BACKEND}" != "postgres" ]; then
    if find "${DATA_DIR}" \( -name "*.db-wal" -o -name "*.db-shm" \) 2>/dev/null | grep -q .; then
        LEFTOVER=1
    fi
fi

if [ "${LEFTOVER}" -eq 0 ]; then
    log "P14: verifier bersih — tidak ada single DB/uploads flat/release berlebih"
    phase_pass "P14"
else
    warn "P14: masih ada artifact tersisa (periksa: ${DATA_DIR})"
    phase_warn "P14" "artifact tersisa"
fi

# ============================================================================
# PK — AUTO-RUN SERVER + VERIFY FINAL
# ============================================================================
declare_phase "PK" "Auto-run server + verify final"
# Syarat user: server otomatis di-run setelah tools selesai. Diposisikan PALING
# AKHIR setelah semua tahap. Jalur sukses: server RUNNING + /readyz OK +
# report digenerate ulang agar memuat PK_autorun.
PK_OK=1
if ! service_is_active; then
    warn "PK: service ${SERVICE_NAME} tidak active — butuh intervensi manual (journalctl -u ${SERVICE_NAME} -n 50)"
    phase_warn "PK" "service tidak active"
    PK_OK=0
else
    log "PK: service ${SERVICE_NAME} active"
fi
if ! readyz_check; then
    warn "PK: /readyz tidak OK — butuh intervensi manual"
    phase_warn "PK" "readyz gagal"
    PK_OK=0
else
    log "PK: /readyz OK"
fi
if [ "${PK_OK}" -eq 1 ]; then
    phase_pass "PK"
fi
# Regenerate report agar PK_autorun ikut tercatat (komponen #4 + syarat §5).
if [ -n "${REPORT_FILE}" ]; then
    write_report
fi

# ============================================================================
# SELESAI — ringkasan
# ============================================================================
phase_summary
ok "==============================================="
ok "✅ DEPLOY PRODUCTION SELESAI"
ok "   Release: ${RELEASE_DIR}"
ok "   Service: ${SERVICE_NAME}"
ok "   Status : $(systemctl is-active "${SERVICE_NAME}" 2>/dev/null || echo unknown)"
ok "   URL    : ${SERVER_URL}"
ok "   Backup : ${BACKUP_DIR}"
if [ -n "${REPORT_FILE}" ]; then
    ok "   Report : ${REPORT_FILE}"
fi
ok "==============================================="
log "Langkah berikutnya (Fase E): cleanup_production.sh utk hapus bundle+zip setelah semua aman"