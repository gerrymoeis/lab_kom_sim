#!/bin/bash
# =============================================================================
# SIMLABKOM — Production Cleanup (cleanup_production.sh) — komponen #8
#
# Menghapus artefak delivery dari server SETELAH user yakin SEMUANYA aman,
# tepat, sesuai, dan berjalan dengan benar. Yang dihapus:
#   - folder hasil extract bundle   : /opt/simlab/deploy_production_<ts>/
#   - file zip bundle               : /opt/simlab/deploy_production_<ts>.tar.gz
#   - tar.gz sementara              : /tmp/deploy_production_<ts>.tar.gz
#
# Prinsip:
#   - DIPISAH dari deploy — deploy_production.sh TIDAK menghapus bundle/zip
#     (jika ada masalah, bundle masih tersedia untuk diagnosis/ulang).
#   - Dijalankan MANUAL oleh user (via SSH) HANYA setelah verifikasi sukses.
#   - Safety guard: TIDAK menyentuh /opt/simlab/app, /opt/simlab/data,
#     data/backups/, release.
#   - Idempotent — aman dijalankan ulang (file yang sudah tidak ada → skip).
#
# Cara pakai (di folder hasil extract bundle):
#   cd deploy_production_<ts>
#   sudo bash cleanup_production.sh          # safety check + konfirmasi Y/n
#   sudo bash cleanup_production.sh --force  # tanpa safety check & konfirmasi
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib/common.sh"

check_root
FORCE=false
for arg in "$@"; do
    case "${arg}" in
        --force) FORCE=true ;;
        *) warn "Argumen tidak dikenal (diabaikan): ${arg}" ;;
    esac
done

LOG_TS="$(date +%Y%m%d-%H%M%S)"
LOG_FILE="${DATA_DIR}/backups/cleanup_${LOG_TS}.log"
mkdir -p "${DATA_DIR}/backups"
log "=== cleanup_production.sh (${LOG_TS}) ==="

BUNDLE_DIR="${SCRIPT_DIR}"
BUNDLE_NAME="$(basename "${SCRIPT_DIR}")"            # deploy_production_<ts>
ZIP_FILE="${INSTALL_DIR}/${BUNDLE_NAME}.tar.gz"       # /opt/simlab/deploy_production_<ts>.tar.gz
TMP_ZIPS=$(find /tmp -maxdepth 1 -name "${BUNDLE_NAME}.tar.gz" 2>/dev/null || true)

# ---------------------------------------------------------------- 1. Safety check
if [ "${FORCE}" = "false" ]; then
    log "Safety check: service active + /readyz OK + report terbaru PK=PASS"
    if ! service_is_active; then
        error "Safety check GAGAL: service ${SERVICE_NAME} tidak active. Jalankan dengan --force bila yakin."
    fi
    if ! readyz_check; then
        error "Safety check GAGAL: /readyz tidak OK. Jalankan dengan --force bila yakin."
    fi
    LATEST_REPORT=$(ls -1t "${DATA_DIR}"/backups/deploy_report_*.json 2>/dev/null | head -1 || true)
    if [ -z "${LATEST_REPORT}" ] || ! grep -q '"PK_autorun":"PASS"' "${LATEST_REPORT}"; then
        error "Safety check GAGAL: report deploy terbaru tidak ada / PK_autorun != PASS (${LATEST_REPORT:-none}). Jalankan dengan --force bila yakin."
    fi
    log "Safety check OK: service active + /readyz OK + report ${LATEST_REPORT} PK=PASS"
fi

# ---------------------------------------------------------------- 2. Konfirmasi (Y/n)
log "Akan dihapus:"
log "  - folder extract: ${BUNDLE_DIR}"
log "  - zip bundle    : ${ZIP_FILE}"
for z in ${TMP_ZIPS}; do log "  - /tmp zip      : ${z}"; done

if [ "${FORCE}" = "false" ]; then
    printf "Hapus artefak bundle di atas? [y/N] "
    read -r ans || ans=""
    case "${ans}" in
        y|Y|yes|YES) ;;
        *) log "Dibatalkan — tidak ada yang dihapus. Server tetap RUNNING."; exit 0 ;;
    esac
fi

# ---------------------------------------------------------------- 3. Hapus folder extract
# Aman: script sudah dibaca penuh oleh bash sebelum di-rm. Hanya hapus bila
# nama folder memang berpola deploy_production_* (guard ekstra).
if [[ "${BUNDLE_NAME}" == deploy_production_* ]]; then
    log "Hapus folder extract: ${BUNDLE_DIR}"
    rm -rf "${BUNDLE_DIR}"
else
    warn "Nama folder tidak berpola deploy_production_* — folder extract TIDAK dihapus: ${BUNDLE_DIR}"
fi

# ---------------------------------------------------------------- 4. Hapus zip + /tmp
if [ -f "${ZIP_FILE}" ]; then
    log "Hapus zip bundle: ${ZIP_FILE}"
    rm -f "${ZIP_FILE}"
fi
for z in ${TMP_ZIPS}; do
    if [ -f "${z}" ]; then
        log "Hapus /tmp zip: ${z}"
        rm -f "${z}"
    fi
done

# ---------------------------------------------------------------- 5. Verifikasi
log "Verifikasi pasca-cleanup:"
LEFT=0
for f in "${INSTALL_DIR}"/deploy_production_*.tar.gz; do
    [ -e "${f}" ] || continue
    warn "masih ada zip bundle: ${f}"
    LEFT=1
done
if [ -d "${BUNDLE_DIR}" ]; then
    warn "folder extract masih ada: ${BUNDLE_DIR}"
    LEFT=1
fi
if ! service_is_active; then
    warn "service ${SERVICE_NAME} tidak active setelah cleanup"
    LEFT=1
fi
if ! readyz_check; then
    warn "/readyz tidak OK setelah cleanup"
    LEFT=1
fi

# ---------------------------------------------------------------- 6. Ringkasan + log
if [ "${LEFT}" -eq 0 ]; then
    ok "CLEANUP SELESAI — artefak bundle dihapus, server tetap RUNNING"
else
    warn "CLEANUP SELESAI dengan sisa — periksa log di atas"
fi
log "Dihapus: ${BUNDLE_DIR} ${ZIP_FILE} ${TMP_ZIPS}" >> "${LOG_FILE}" 2>/dev/null || true
log "Log: ${LOG_FILE}"