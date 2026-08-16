#!/bin/bash
# =============================================================================
# SIMLABKOM — Production Tools — lib/common.sh
# Helper bersama untuk deploy_production.sh / cleanup_production.sh.
# Wajib di-source dari script yang sama-sama memakai pola:
#   set -euo pipefail
#   SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
#   source "${SCRIPT_DIR}/lib/common.sh"
# =============================================================================

# ---------------------------------------------------------------- Konstanta
APP_NAME="${APP_NAME:-simlab}"
INSTALL_DIR="${INSTALL_DIR:-/opt/simlab}"
APP_DIR="${INSTALL_DIR}/app"
RELEASES_DIR="${APP_DIR}/releases"
CURRENT_DIR="${APP_DIR}/current"
DATA_DIR="${INSTALL_DIR}/data"
UPLOADS_DIR="${DATA_DIR}/uploads"
ENV_FILE="${INSTALL_DIR}/.env"
ENV_CONFIG_DIR="${INSTALL_DIR}/.env.config.orig"
SERVICE_NAME="${APP_NAME}.service"
PORT="${PORT:-8080}"
RELEASE_KEEP=3

# ---------------------------------------------------------------- Atomic symlink swap (portable)
# Ganti symlink CURRENT_DIR menuju target secara portabel (tanpa GNU-only `mv -T`
# yang tidak ada di busybox/BSD). Idiom: ln -sfn ke nama temp lalu mv rename;
# bila CURRENT_DIR symlink lama masih ada, rm dulu link-nya (bukan target dir),
# karena `mv` tanpa -T akan memindah ke DALAM direktori tujuan bila dest adalah
# symlink ke direktori. Service sudah stop saat dipakai (P6/rollback/restore).
swap_symlink() {
    local target="$1" tmp="${CURRENT_DIR}.new"
    ln -sfn "${target}" "${tmp}"
    if [ -L "${CURRENT_DIR}" ]; then
        rm -f "${CURRENT_DIR}"
    elif [ -e "${CURRENT_DIR}" ]; then
        error "swap_symlink: ${CURRENT_DIR} bukan symlink — tidak aman utk diganti"
    fi
    mv "${tmp}" "${CURRENT_DIR}"
}

# ---------------------------------------------------------------- Logging
log()  { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"; }
warn() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] ⚠️  $*"; }
error() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] ❌ $*"; exit 1; }
ok()   { echo "[$(date '+%Y-%m-%d %H:%M:%S')] ✅ $*"; }

# ---------------------------------------------------------------- Fase tracking
PHASES=()
PHASE_STATUS=()
declare_phase() {
    local name="$1" desc="$2"
    PHASES+=("$name")
    PHASE_STATUS+=("RUNNING")
    log "─────────────── ${name}: ${desc} ───────────────"
}
phase_pass() {
    local name="$1"
    for i in "${!PHASES[@]}"; do
        if [ "${PHASES[$i]}" = "$name" ]; then PHASE_STATUS[$i]="PASS"; fi
    done
    ok "${name}: PASS"
}
phase_skip() {
    local name="$1" reason="$2"
    for i in "${!PHASES[@]}"; do
        if [ "${PHASES[$i]}" = "$name" ]; then PHASE_STATUS[$i]="SKIP"; fi
    done
    log "${name}: SKIP (${reason})"
}
phase_warn() {
    local name="$1" reason="$2"
    for i in "${!PHASES[@]}"; do
        if [ "${PHASES[$i]}" = "$name" ]; then PHASE_STATUS[$i]="WARN"; fi
    done
    warn "${name}: WARN (${reason})"
}
phase_fail() {
    local name="$1" reason="$2"
    for i in "${!PHASES[@]}"; do
        if [ "${PHASES[$i]}" = "$name" ]; then PHASE_STATUS[$i]="FAIL"; fi
    done
    log "${name}: FAIL (${reason})"
}
phase_summary() {
    log "============== RINGKASAN FASE =============="
    for i in "${!PHASES[@]}"; do
        printf "  %-6s %s\n" "${PHASES[$i]}: ${PHASE_STATUS[$i]}" ""
    done
    log "============================================"
}
# phase_report_key: nama key JSON report utk fase (pola doc 014 komponen #4).
phase_report_key() {
    case "$1" in
        P0)  echo "P0_validate" ;;
        P1)  echo "P1_env" ;;
        P2)  echo "P2_backup" ;;
        P3)  echo "P3_stop" ;;
        P4)  echo "P4_migrate" ;;
        P5)  echo "P5_seed" ;;
        P6)  echo "P6_deploy" ;;
        P7)  echo "P7_publish" ;;
        P8)  echo "P8_start" ;;
        P9)  echo "P9_healthz" ;;
        P10) echo "P10_readyz" ;;
        P11) echo "P11_verify" ;;
        P12) echo "P12_test" ;;
        P13) echo "P13_report" ;;
        P14) echo "P14_cleanup" ;;
        PK)  echo "PK_autorun" ;;
        *)   echo "$1" ;;
    esac
}
# phase_json: output PHASES/PHASE_STATUS sebagai objek JSON
# (key = phase_report_key, status = PASS|SKIP|WARN|FAIL|RUNNING).
phase_json() {
    local i key val out="" first=1
    for i in "${!PHASES[@]}"; do
        key="$(phase_report_key "${PHASES[$i]}")"
        val="${PHASE_STATUS[$i]}"
        if [ "$first" -eq 1 ]; then first=0; else out="${out},"; fi
        out="${out}\"${key}\":\"${val}\""
    done
    echo "{${out}}"
}

# ---------------------------------------------------------------- .env helpers
# baca_env KEY: ambil nilai KEY dari ENV_FILE (baris pertama yang cocok).
baca_env() {
    local key="$1" val=""
    val=$(grep -E "^${key}=" "${ENV_FILE}" 2>/dev/null | head -1 | cut -d= -f2- || true)
    echo "$val"
}
# is_multi_lab_env: true bila .env memakai format multi-lab (GLOBAL_DB_PATH + LABS_1_ID).
is_multi_lab_env() {
    local gdb labs1
    gdb=$(baca_env "GLOBAL_DB_PATH")
    labs1=$(baca_env "LABS_1_ID")
    [ -n "${gdb}" ] && [ -n "${labs1}" ]
}
# preserve_session_secret: simpan SESSION_SECRET lama bila ada ke file tujuan.
# return 0 bila ada secret dipreserve, 1 bila tidak ada secret sama sekali.
preserve_session_secret() {
    local old secret
    old=$(baca_env "SESSION_SECRET")
    if [ -n "${old}" ] && [ "${old}" != "__AUTO_GENERATE__" ]; then
        echo "${old}" > "${ENV_CONFIG_DIR}/SESSION_SECRET"
        log "SESSION_SECRET lama dipreserve"
        return 0
    fi
    log "Tidak ada SESSION_SECRET lama — akan digenerate saat first run"
    return 1
}
generate_session_secret() {
    local secret=""
    secret=$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')
    if [ -z "${secret}" ] && command -v openssl >/dev/null 2>&1; then
        secret=$(openssl rand -hex 32)
    fi
    if [ -z "${secret}" ]; then
        secret=$(date +%s%N | sha256sum | head -c 64 | tr -d '\n')
    fi
    echo "${secret}"
}

# ---------------------------------------------------------------- Service helpers
service_stop() {
    log "Menghentikan service ${SERVICE_NAME}..."
    systemctl stop "${SERVICE_NAME}" 2>/dev/null || true
    local i
    for i in $(seq 1 30); do
        if ! pgrep -x "${APP_NAME}" >/dev/null 2>&1 && ! pgrep -f "app-simlab" >/dev/null 2>&1; then
            log "Server berhenti setelah ${i}s"
            return 0
        fi
        sleep 1
    done
    warn "Server tidak berhenti setelah 30s — force kill..."
    pkill -9 -x "${APP_NAME}" 2>/dev/null || true
    pkill -9 -f "app-simlab" 2>/dev/null || true
    sleep 2
}
service_start() {
    systemctl start "${SERVICE_NAME}"
    log "Service ${SERVICE_NAME} dimulai"
}
service_is_active() {
    [ "$(systemctl is-active "${SERVICE_NAME}" 2>/dev/null || true)" = "active" ]
}
# tunggu_wal_closed: tunggu hingga tidak ada *.db-wal / *.db-shm di DATA_DIR.
tunggu_wal_closed() {
    log "Menunggu SQLite WAL/SHM files ditutup..."
    local i wal_count wal_remaining db_file f
    for i in $(seq 1 30); do
        wal_count=0
        while IFS= read -r -d '' f; do
            wal_count=$((wal_count + 1))
        done < <(find "${DATA_DIR}" \( -name "*.db-wal" -o -name "*.db-shm" \) -print0 2>/dev/null || true)
        if [ "${wal_count}" -eq 0 ]; then
            log "Semua WAL/SHM file tertutup (${i}s)"
            return 0
        fi
        if [ "$i" -eq 30 ]; then
            warn "WAL/SHM masih ada setelah 30s — force checkpoint..."
            while IFS= read -r -d '' f; do
                db_file="${f%.db-wal}"
                db_file="${db_file%.db-shm}.db"
                if [ -f "${db_file}" ] && command -v sqlite3 >/dev/null 2>&1; then
                    sqlite3 "${db_file}" "PRAGMA wal_checkpoint(TRUNCATE);" 2>/dev/null || true
                fi
            done < <(find "${DATA_DIR}" \( -name "*.db-wal" -o -name "*.db-shm" \) -print0 2>/dev/null || true)
            sleep 2
            wal_remaining=0
            while IFS= read -r -d '' f; do
                wal_remaining=$((wal_remaining + 1))
            done < <(find "${DATA_DIR}" \( -name "*.db-wal" -o -name "*.db-shm" \) -print0 2>/dev/null || true)
            if [ "${wal_remaining}" -gt 0 ]; then
                warn "${wal_remaining} WAL/SHM file masih ada — SQLite akan recovery saat startup"
            fi
        fi
        sleep 1
    done
    return 0
}

# ---------------------------------------------------------------- Health helpers
# get_port: ambil PORT dari .env (default 8080).
get_port() {
    local p
    p=$(baca_env "PORT")
    [ -n "${p}" ] || p="8080"
    echo "${p}"
}
health_check() {
    local port i
    port="$(get_port)"
    for i in $(seq 1 10); do
        if curl -sf "http://localhost:${port}/healthz" >/dev/null 2>&1; then
            log "Health check berhasil (percobaan ${i})"
            return 0
        fi
        sleep 2
    done
    return 1
}
readyz_check() {
    local port i
    port="$(get_port)"
    for i in $(seq 1 15); do
        if curl -sf "http://localhost:${port}/readyz" >/dev/null 2>&1; then
            log "Readiness check berhasil (percobaan ${i})"
            return 0
        fi
        sleep 2
    done
    return 1
}

# ---------------------------------------------------------------- Filesystem helpers
# backup_dir: buat direktori backup unik di DATA_DIR/backups/pre_deploy_<ts>.
backup_dir() {
    local ts dir
    ts=$(date +%Y%m%d-%H%M%S)
    dir="${DATA_DIR}/backups/pre_deploy_${ts}"
    mkdir -p "${dir}"
    echo "${dir}"
}
# tarball_backup: backup penuh data/ + .env + release aktif.
#   $1 = dir backup tujuan
tarball_backup() {
    local dest="$1" saved_current
    mkdir -p "${dest}"
    # 1) data/ (semua *.db + uploads), kecuali dir backup itu sendiri
    tar czf "${dest}/data.tar.gz" -C "${DATA_DIR}" \
        --exclude="backups/pre_deploy_*" \
        --exclude="*.db-wal" --exclude="*.db-shm" \
        . 2>/dev/null
    log "Backup data → ${dest}/data.tar.gz"
    # 2) .env
    if [ -f "${ENV_FILE}" ]; then
        cp "${ENV_FILE}" "${dest}/env.bak"
        chmod 600 "${dest}/env.bak"
        log "Backup .env → ${dest}/env.bak"
    fi
    # 3) release aktif (symlink target)
    if [ -L "${CURRENT_DIR}" ] && [ -d "$(readlink -f "${CURRENT_DIR}" 2>/dev/null)" ]; then
        saved_current="$(readlink -f "${CURRENT_DIR}")"
        tar czf "${dest}/current_release.tar.gz" -C "$(dirname "${saved_current}")" "$(basename "${saved_current}")" 2>/dev/null
        log "Backup release aktif → ${dest}/current_release.tar.gz"
    fi
    log "Backup selesai di ${dest}"
}
# restore_backup: pulihkan data/ + .env + release dari dir backup.
#   $1 = dir backup sumber
restore_backup() {
    local dest="$1"
    if [ -f "${dest}/data.tar.gz" ]; then
        tar xzf "${dest}/data.tar.gz" -C "${DATA_DIR}"
        log "Restore data dari ${dest}/data.tar.gz"
    fi
    if [ -f "${dest}/env.bak" ]; then
        cp "${dest}/env.bak" "${ENV_FILE}"
        chmod 600 "${ENV_FILE}"
        log "Restore .env dari ${dest}/env.bak"
    fi
    if [ -f "${dest}/current_release.tar.gz" ]; then
        local tmp
        tmp="${dest}/current_release"
        mkdir -p "${tmp}"
        tar xzf "${dest}/current_release.tar.gz" -C "${tmp}"
        local rel
        rel=$(find "${tmp}" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | head -1 || true)
        if [ -n "${rel}" ]; then
            swap_symlink "${rel}"
            log "Restore release aktif → ${rel}"
        fi
    fi
    chown -R "${APP_NAME}:${APP_NAME}" "${DATA_DIR}" 2>/dev/null || true
    log "Restore backup selesai"
}
# cleanup_old_releases: hapus release lama kecuali keep terbaru (pola update.sh).
cleanup_old_releases() {
    local keep=3 count
    if [ ! -d "${RELEASES_DIR}" ]; then return 0; fi
    count=$(ls -1t "${RELEASES_DIR}" 2>/dev/null | wc -l)
    if [ "${count}" -gt "${keep}" ]; then
        ls -1t "${RELEASES_DIR}" | tail -n +$((keep+1)) | while read -r d; do
            if [ -n "${d}" ] && [ "${d}" != "$(basename "$(readlink -f "${CURRENT_DIR}" 2>/dev/null)")" ]; then
                rm -rf "${RELEASES_DIR}/${d}"
                log "Hapus release lama: ${d}"
            fi
        done
    fi
}
# download... tidak dipakai deploy script (bundle lokal).

# ---------------------------------------------------------------- Validasi prasyarat
check_root() {
    [ "$(id -u)" -eq 0 ] || error "Harap jalankan dengan sudo: sudo bash $(basename "$0")"
}
check_cmds() {
    local c
    for c in curl systemctl tar sqlite3; do
        if ! command -v "${c}" >/dev/null 2>&1; then
            warn "Perintah '${c}' tidak ditemukan (opsional utk beberapa langkah)"
        fi
    done
    command -v curl >/dev/null 2>&1 || error "curl tidak terinstall"
    command -v systemctl >/dev/null 2>&1 || error "systemctl tidak ditemukan"
    command -v tar >/dev/null 2>&1 || error "tar tidak terinstall"
}
# check_disk: pastikan ada ruang disk minimal $1 MB di DATA_DIR.
#   $1 opsional; default ${MIN_DISK_MB:-500} (dapat di-override via env).
check_disk() {
    local need_mb="${1:-${MIN_DISK_MB:-500}}" avail_kb
    avail_kb=$(df -k "${DATA_DIR}" 2>/dev/null | awk 'NR==2 {print $4}' || true)
    if [ -n "${avail_kb}" ]; then
        local avail_mb=$((avail_kb / 1024))
        if [ "${avail_mb}" -lt "${need_mb}" ]; then
            warn "Ruang disk tersedia ${avail_mb}MB < ${need_mb}MB"
        else
            log "Ruang disk OK: ${avail_mb}MB tersedia (min ${need_mb}MB)"
        fi
    fi
}
# chown_data: pastikan ownership data dir ke service user.
chown_data() {
    chown -R "${APP_NAME}:${APP_NAME}" "${DATA_DIR}" 2>/dev/null || true
    log "Ownership ${DATA_DIR} → ${APP_NAME}:${APP_NAME}"
}