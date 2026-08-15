#!/bin/bash
# =============================================================================
# SIMLABKOM — Production Deploy (deploy_production.sh)
#
# Orchestrator deploy tools production Linux (/opt/simlab). Dijalankan dari
# folder hasil extract bundle:
#   cd deploy_production_<ts>
#   sudo bash deploy_production.sh [--skip-migrate] [--skip-test]
#
# Tahap (Fase B: P0–P9 + P14, tanpa test suite):
#   P0  Validasi prasyarat + bundle lengkap                  → STOP
#   P1  Deteksi format .env (single/multi) + regenerate      → STOP
#   P2  Backup penuh data/ + .env + release aktif            → STOP
#   P3  Stop service + tunggu WAL/SHM                        → STOP
#   P4  Deteksi migrasi; jalankan ETL bila perlu             → ROLLBACK
#   P5  Siapkan release dir + seeds (tanpa marker)           → ROLLBACK
#   P6  Deploy binary + atomic symlink swap                  → ROLLBACK
#   P7  Generate public site (app-simlab-publish)            → WARN
#   P8  Start service                                        → ROLLBACK
#   P9  Health check /healthz                                → ROLLBACK
#   P14 Cleanup (release keep 3, single DB, uploads flat)    → WARN
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

# ============================================================================
# ROLLBACK — dipicu kegagalan P4–P9
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

    # 2) hapus file hasil ETL (bila migrasi baru berjalan) lalu restore backup
    if [ "${MIGRATION_RAN}" -eq 1 ] && [ -n "${BACKUP_DIR}" ] && [ -d "${BACKUP_DIR}" ]; then
        rm -f "${DATA_DIR}/global.db" "${DATA_DIR}/global.db-shm" "${DATA_DIR}/global.db-wal"
        rm -f "${DATA_DIR}/lab_mi_1.db" "${DATA_DIR}/lab_mi_1.db-shm" "${DATA_DIR}/lab_mi_1.db-wal"
        rm -f "${DATA_DIR}/lab_vokasi_1.db" "${DATA_DIR}/lab_vokasi_1.db-shm" "${DATA_DIR}/lab_vokasi_1.db-wal"
        rm -f "${DATA_DIR}/migration_report.json"
        rm -rf "${DATA_DIR}/uploads/lab-mi" "${DATA_DIR}/uploads/lab-vokasi-1"
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
check_disk 500
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
    sed "s/__AUTO_GENERATE__/${secret}/g" "${ENV_CONFIG_TEMPLATE}" > "${ENV_FILE}"
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

# Validasi key wajib ada (nilai tidak di-log)
REQUIRED_KEYS="GLOBAL_DB_PATH LABS_1_ID LABS_1_DB LABS_1_TITLE LABS_1_URL LABS_2_ID LABS_2_DB LABS_2_TITLE LABS_2_URL SESSION_SECRET UPLOAD_PATH"
for key in ${REQUIRED_KEYS}; do
    val=$(baca_env "${key}")
    [ -n "${val}" ] || error "P1: key wajib '${key}' kosong di ${ENV_FILE}"
done
for key in GEMINI_API_KEY OPENROUTER_API_KEY PC_PHOTO_TOKEN; do
    val=$(baca_env "${key}")
    [ -n "${val}" ] || warn "P1: API key '${key}' kosong (server fitur terkait tidak jalan)"
done
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
tunggu_wal_closed
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
        echo "${f}"
        return 0
    done
    echo "${SOURCE_DB}"
}
detect_source_upload_dir() {
    # Baseline-aware: server single-lab menyimpan di uploads/<urlPath>/<sub>/
    # (urlPath = lowercase nama file DB). Fallback "lab-kom-mi" (default ETL).
    local dbname cand
    dbname="$(basename "${SOURCE_DB}" .db)"
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
    sed \
        -e "s|\"source_db\": *\"[^\"]*\"|\"source_db\": \"${src_db}\"|" \
        -e "s|\"source_upload_dir\": *\"[^\"]*\"|\"source_upload_dir\": \"${src_upload_dir}\"|" \
        "${SCRIPT_DIR}/config/etl-config.production.json" > "${runtime}"
    echo "${runtime}"
}

if [ "${SKIP_MIGRATE}" = "true" ]; then
    phase_skip "P4" "--skip-migrate"
elif should_migrate; then
    SRC_DB="$(detect_source_db)"
    SRC_UPLOAD_DIR="$(detect_source_upload_dir)"
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

    # Verifikasi hasil ETL
    [ -f "${DATA_DIR}/global.db" ]       || { phase_fail "P4" "global.db tidak dihasilkan"; rollback; }
    [ -f "${DATA_DIR}/lab_mi_1.db" ]     || { phase_fail "P4" "lab_mi_1.db tidak dihasilkan"; rollback; }
    [ -f "${DATA_DIR}/migration_report.json" ] || { phase_fail "P4" "migration_report.json hilang"; rollback; }
    SUPER_ADMIN=$(grep -o '"super_admin_count": *[0-9]*' "${DATA_DIR}/migration_report.json" | grep -o '[0-9]*$' || true)
    ROWS_PC=$(grep -o '"rows_pcs": *[0-9]*' "${DATA_DIR}/migration_report.json" | grep -o '[0-9]*$' || true)
    [ -n "${SUPER_ADMIN}" ] && [ "${SUPER_ADMIN}" -ge 1 ] \
        || { phase_fail "P4" "super_admin_count=${SUPER_ADMIN:-0} < 1 di report ETL"; rollback; }
    [ -n "${ROWS_PC}" ] && [ "${ROWS_PC}" -gt 0 ] \
        || warn "P4: rows_pcs=${ROWS_PC:-0} (0) — periksa data source kosong?"
    log "P4: ETL OK — super_admin=${SUPER_ADMIN} rows_pcs=${ROWS_PC:-0}"
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
for seed in mi-1 vokasi-1 default; do
    [ -d "${RELEASE_DIR}/seeds/${seed}" ] || { phase_fail "P5" "seeds/${seed} tidak tersalin"; rollback; }
done
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
# P14 — CLEANUP + VERIFIER
# ============================================================================
declare_phase "P14" "Cleanup (release keep 3, single DB, uploads flat)"
LEFTOVER=0

# 1) release lama keep 3 (kecuali release aktif)
cleanup_old_releases

# 2) single DB lama (sudah di-backup)
for f in inventaris_lab.db inventaris_lab.db-shm inventaris_lab.db-wal; do
    if [ -e "${DATA_DIR}/${f}" ]; then
        rm -f "${DATA_DIR}/${f}"
        log "P14: hapus single DB lama ${f}"
    fi
done

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
for f in inventaris_lab.db inventaris_lab.db-shm inventaris_lab.db-wal; do
    if find "${DATA_DIR}" -name "${f}" -not -path "${BACKUP_DIR}/*" 2>/dev/null | grep -q .; then
        LEFTOVER=1
    fi
done
for sub in pc device_types device_installations logbook; do
    [ -d "${DATA_DIR}/uploads/${sub}" ] && LEFTOVER=1
done
REL_COUNT=$(ls -1t "${RELEASES_DIR}" 2>/dev/null | wc -l)
[ "${REL_COUNT}" -le 3 ] || LEFTOVER=1
if find "${DATA_DIR}" \( -name "*.db-wal" -o -name "*.db-shm" \) 2>/dev/null | grep -q .; then
    LEFTOVER=1
fi

if [ "${LEFTOVER}" -eq 0 ]; then
    log "P14: verifier bersih — tidak ada single DB/uploads flat/release berlebih"
    phase_pass "P14"
else
    warn "P14: masih ada artifact tersisa (periksa: ${DATA_DIR})"
    phase_warn "P14" "artifact tersisa"
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
ok "   URL    : http://$(hostname -I 2>/dev/null | awk '{print $1}'):$(get_port)"
ok "   Backup : ${BACKUP_DIR}"
ok "==============================================="
log "Langkah berikutnya (Fase C/D/E): /readyz, app-simlab -verify, test suite, cleanup_production.sh"