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
# INSTALL_DIR_EXPLICIT: nilai env INSTALL_DIR yang diberikan user SEBELUM default
# diterapkan (dipakai resolve_install_dir utk metode "override"). Empty = tidak di-set.
INSTALL_DIR_EXPLICIT="${INSTALL_DIR:-}"
INSTALL_DIR="${INSTALL_DIR_EXPLICIT:-/opt/simlab}"
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
# Auto-discovery (doc 017): root direktori yang diizinkan utk bounded scan.
ALLOWED_ROOTS="${ALLOWED_ROOTS:-/opt /srv /usr/local /var /home /data /app}"
DETECT_METHOD=""         # override|systemd|process|scan|default (hasil resolve_install_dir)
DETECT_CANDIDATES=0      # jumlah kandidat lokasi valid yang ditemukan saat scan

# ---------------------------------------------------------------- Auto-discovery lokasi install (doc 017)
# Temukan letak asli SIMLab di-deploy & dikonfigurasi ketika lokasi TIDAK diketahui
# tools. Prioritas hierarkis: override → systemd → proses → bounded scan → default.
# Dipanggil oleh deploy_production.sh / cleanup_production.sh SETELAH source common.sh.
DETECT_ENV_FILE=""       # hasil deteksi (EnvironmentFile/ENV_PATH) utk ENV_FILE bila valid
SCAN_CANDIDATES=()       # daftar kandidat lokasi valid (dari bounded scan)
# set_install_dir DIR METHOD: tetapkan INSTALL_DIR + seluruh turunan + DETECT_METHOD.
set_install_dir() {
    local dir="$1" method="$2"
    INSTALL_DIR="${dir}"
    APP_DIR="${INSTALL_DIR}/app"
    RELEASES_DIR="${APP_DIR}/releases"
    CURRENT_DIR="${APP_DIR}/current"
    DATA_DIR="${INSTALL_DIR}/data"
    UPLOADS_DIR="${DATA_DIR}/uploads"
    ENV_FILE="${INSTALL_DIR}/.env"
    ENV_CONFIG_DIR="${INSTALL_DIR}/.env.config.orig"
    DETECT_METHOD="${method}"
    if [ -n "${DETECT_ENV_FILE}" ] && [ -f "${DETECT_ENV_FILE}" ]; then
        ENV_FILE="${DETECT_ENV_FILE}"
    fi
}
# validate_install_dir CAND: pastikan CAND adalah lokasi install SIMLab valid.
# 0 = valid, 1 = tidak. Cek: app/current symlink → app/releases/<ts>, .env ber-marker
# (SESSION_SECRET + GLOBAL_DB_PATH [V2/global] ATAU DATABASE_PATH [V1 legacy]),
# dan data/global.db ATAU data/inventaris_lab.db (V1 legacy single-DB) ATAU
# backend PostgreSQL aktif (DATABASE_URL terisi di .env).
validate_install_dir() {
    local cand="$1" tgt base_real
    [ -n "${cand}" ] || return 1
    [ -d "${cand}" ] || return 1
    [ -L "${cand}/app/current" ] || return 1
    tgt="$(readlink -f "${cand}/app/current" 2>/dev/null || true)"
    base_real="$(readlink -f "${cand}" 2>/dev/null || true)"
    case "${tgt}" in
        "${base_real}"/app/releases/*) ;;
        *) return 1 ;;
    esac
    [ -d "${tgt}" ] || return 1
    [ -f "${cand}/.env" ] || return 1
    grep -qE '^SESSION_SECRET=' "${cand}/.env" || return 1
    if ! grep -qE '^GLOBAL_DB_PATH=' "${cand}/.env" && ! grep -qE '^DATABASE_PATH=' "${cand}/.env"; then
        return 1
    fi
    if [ -f "${cand}/data/global.db" ] || [ -f "${cand}/data/inventaris_lab.db" ]; then
        return 0
    fi
    # PostgreSQL backend aktif: nilai DATABASE_URL NON-kosong (harus diawali
    # karakter non-spasi/non-komentar; baris `DATABASE_URL=  # komentar` bukan nilai).
    if grep -qE '^DATABASE_URL=[^[:space:]#]' "${cand}/.env" 2>/dev/null; then
        return 0
    fi
    return 1
}
# _scan_add_candidate: tambah kandidat unik (tanpa duplikat) + count.
_scan_add_candidate() {
    local c="$1" i
    for i in "${SCAN_CANDIDATES[@]}"; do
        [ "${i}" = "${c}" ] && return 0
    done
    SCAN_CANDIDATES+=("${c}")
    DETECT_CANDIDATES=$((DETECT_CANDIDATES + 1))
}
# scan_install_dir: bounded scan pada ALLOWED_ROOTS (whitelist, maxdepth) utk marker
# struktur SIMLab: (a) app/releases/<ts>/app-simlab, (b) .env ber SESSION_SECRET +
# (GLOBAL_DB_PATH ATAU DATABASE_PATH [V1 legacy]). Mengisi SCAN_CANDIDATES &
# DETECT_CANDIDATES. TIDAK scan seluruh '/', tidak me-log isi .env.
# Return 0 bila ≥1 kandidat.
scan_install_dir() {
    local root releases envf cand
    SCAN_CANDIDATES=()
    DETECT_CANDIDATES=0
    for root in ${ALLOWED_ROOTS}; do
        [ -d "${root}" ] || continue
        while IFS= read -r releases; do
            [ -n "${releases}" ] || continue
            if ls "${releases}"/*/app-simlab >/dev/null 2>&1; then
                cand="$(dirname "$(dirname "${releases}")")"
                _scan_add_candidate "${cand}"
            fi
        done < <(find "${root}" -maxdepth 6 -type d -path '*/app/releases' 2>/dev/null || true)
        while IFS= read -r envf; do
            [ -n "${envf}" ] || continue
            if grep -qE '^(GLOBAL_DB_PATH|DATABASE_PATH)=' "${envf}" 2>/dev/null && \
               grep -qE '^SESSION_SECRET=' "${envf}" 2>/dev/null; then
                cand="$(dirname "${envf}")"
                _scan_add_candidate "${cand}"
            fi
        done < <(find "${root}" -maxdepth 3 -name '.env' 2>/dev/null || true)
    done
    [ "${DETECT_CANDIDATES}" -gt 0 ]
}
# locate_by_systemd: baca WorkingDirectory/EnvironmentFile dari unit systemd
# (parsed properties systemd, source of truth service). Isi DETECT_CAND (INSTALL_DIR)
# dan DETECT_ENV_FILE bila EnvironmentFile valid. Return 0 bila ketemu.
locate_by_systemd() {
    command -v systemctl >/dev/null 2>&1 || return 1
    local work="" envf="" base=""
    DETECT_CAND=""
    work=$(systemctl show "${SERVICE_NAME}" -p WorkingDirectory --value 2>/dev/null || true)
    if [ -z "${work}" ]; then
        work=$(systemctl show "${SERVICE_NAME}" -p WorkingDirectory 2>/dev/null | sed -n 's/^WorkingDirectory=//p' | head -1 || true)
    fi
    [ -n "${work}" ] || return 1
    base="$(dirname "$(dirname "${work}")")"   # .../app/current → naik 2 level
    [ -n "${base}" ] || return 1
    envf=$(systemctl show "${SERVICE_NAME}" -p EnvironmentFile --value 2>/dev/null || true)
    if [ -z "${envf}" ]; then
        envf=$(systemctl show "${SERVICE_NAME}" -p EnvironmentFile 2>/dev/null | sed -n 's/^EnvironmentFile=//p' | head -1 || true)
    fi
    if [ -n "${envf}" ] && [ -f "${envf}" ]; then DETECT_ENV_FILE="${envf}"; fi
    DETECT_CAND="${base}"
}
# locate_by_process: baca CWD/ENV_PATH proses app-simlab yang berjalan.
# Isi DETECT_CAND (INSTALL_DIR) & DETECT_ENV_FILE bila ENV_PATH valid.
locate_by_process() {
    local pid="" cwd="" envp="" base=""
    DETECT_CAND=""
    pid=$(pgrep -f "app-simlab" 2>/dev/null | head -1 || true)
    [ -n "${pid}" ] || return 1
    [ -d "/proc/${pid}" ] || return 1
    cwd=$(readlink -f "/proc/${pid}/cwd" 2>/dev/null || true)
    [ -n "${cwd}" ] || return 1
    base="$(dirname "$(dirname "$(dirname "${cwd}")")")"   # .../app/releases/<ts> → naik 3
    [ -n "${base}" ] || return 1
    envp=$(tr '\0' '\n' < "/proc/${pid}/environ" 2>/dev/null | sed -n 's/^ENV_PATH=//p' | head -1 || true)
    if [ -n "${envp}" ] && [ -f "${envp}" ]; then DETECT_ENV_FILE="${envp}"; fi
    DETECT_CAND="${base}"
}
# resolve_install_dir: orchestrator deteksi hierarkis (doc 017 §2).
# Menetapkan INSTALL_DIR + turunan + DETECT_METHOD. Aman dipanggil berulang (idempotent).
resolve_install_dir() {
    local cand=""
    DETECT_ENV_FILE=""
    DETECT_CAND=""
    SCAN_CANDIDATES=()
    DETECT_CANDIDATES=0
    # 1) override eksplisit
    if [ -n "${INSTALL_DIR_EXPLICIT}" ]; then
        if validate_install_dir "${INSTALL_DIR_EXPLICIT}"; then
            set_install_dir "${INSTALL_DIR_EXPLICIT}" "override"
            log "Deteksi lokasi install: override (${INSTALL_DIR_EXPLICIT})"
            return 0
        fi
        warn "INSTALL_DIR=${INSTALL_DIR_EXPLICIT} tidak valid (bukan struktur SIMLab) — lanjut deteksi otomatis"
    fi
    # 2) systemd
    if locate_by_systemd; then
        cand="${DETECT_CAND}"
        if [ -n "${cand}" ] && validate_install_dir "${cand}"; then
            set_install_dir "${cand}" "systemd"
            log "Deteksi lokasi install: systemd (${cand})"
            return 0
        fi
    fi
    # 3) proses berjalan
    if locate_by_process; then
        cand="${DETECT_CAND}"
        if [ -n "${cand}" ] && validate_install_dir "${cand}"; then
            set_install_dir "${cand}" "process"
            log "Deteksi lokasi install: process (${cand})"
            return 0
        fi
    fi
    # 4) bounded scan
    if scan_install_dir; then
        if [ "${DETECT_CANDIDATES}" -eq 1 ]; then
            cand="${SCAN_CANDIDATES[0]}"
            if validate_install_dir "${cand}"; then
                set_install_dir "${cand}" "scan"
                log "Deteksi lokasi install: scan (${cand})"
                return 0
            fi
        else
            log "Ambigu: ${DETECT_CANDIDATES} kandidat lokasi SIMLab ditemukan:"
            for cand in "${SCAN_CANDIDATES[@]}"; do
                log "  - ${cand}"
            done
            if [ -t 0 ]; then
                printf "Pilih lokasi (ketik path, kosong=default): "
                read -r cand || cand=""
                if [ -n "${cand}" ] && validate_install_dir "${cand}"; then
                    set_install_dir "${cand}" "scan"
                    log "Deteksi lokasi install: scan (pilihan user: ${cand})"
                    return 0
                fi
                warn "Pilihan tidak valid — lanjut default"
            else
                error "Deteksi lokasi install ambigu (${DETECT_CANDIDATES} kandidat) & non-interaktif — tentukan INSTALL_DIR secara eksplisit"
            fi
        fi
    fi
    # 5) default (backward-compat /opt/simlab)
    set_install_dir "/opt/simlab" "default"
    log "Deteksi lokasi install: default (/opt/simlab)"
    return 0
}

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
# Nilai dinormalisasi seperti godotenv (app): trim spasi + buang komentar inline
# ('#' dianggap komentar HANYA bila didahului spasi/tab). Tanpa ini baris template
# `DATABASE_URL=  # komentar` terbaca non-empty → salah deteksi backend PostgreSQL
# (doc 019 T1 / doc 020 F1).
baca_env() {
    local key="$1" val=""
    val=$(grep -E "^${key}=" "${ENV_FILE}" 2>/dev/null | head -1 | cut -d= -f2- \
        | sed -E 's/[[:space:]]+#.*$//; s/^[[:space:]]+//; s/[[:space:]]+$//' || true)
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

# ---------------------------------------------------------------- Lifecycle helpers (dual-mode: systemd ATAU proses)
# Realita produksi (doc 020): server lama dijalankan MANUAL oleh admin (nohup binary
# dari release dir), tanpa systemd & tanpa user service. Konsekuensi: seluruh
# stop/start/status harus dual-mode — "systemd" bila unit service terdaftar,
# "process" bila tidak (mirror cara admin menjalankan).
RUN_MODE=""
# detect_run_mode: tentukan mode lifecycle. Idempotent (hasil dicache di RUN_MODE).
detect_run_mode() {
    [ -n "${RUN_MODE}" ] && { echo "${RUN_MODE}"; return 0; }
    if command -v systemctl >/dev/null 2>&1 && systemctl cat "${SERVICE_NAME}" >/dev/null 2>&1; then
        RUN_MODE="systemd"
    else
        RUN_MODE="process"
    fi
    echo "${RUN_MODE}"
}
server_stop() {
    local i mode
    mode="$(detect_run_mode)"
    if [ "${mode}" = "systemd" ]; then
        log "Menghentikan service ${SERVICE_NAME} (systemd)..."
        systemctl stop "${SERVICE_NAME}" 2>/dev/null || true
    else
        log "Menghentikan server (mode proses / manual-run)..."
        pkill -TERM -x "${APP_NAME}" 2>/dev/null || true
        pkill -TERM -f "app-simlab" 2>/dev/null || true
    fi
    for i in $(seq 1 30); do
        if ! pgrep -x "${APP_NAME}" >/dev/null 2>&1 && ! pgrep -f "app-simlab" >/dev/null 2>&1; then
            log "Server berhenti setelah ${i}s"
            [ "${mode}" = "process" ] && rm -f "${DATA_DIR}/app.pid"
            return 0
        fi
        sleep 1
    done
    warn "Server tidak berhenti setelah 30s — force kill..."
    pkill -9 -x "${APP_NAME}" 2>/dev/null || true
    pkill -9 -f "app-simlab" 2>/dev/null || true
    [ "${mode}" = "process" ] && rm -f "${DATA_DIR}/app.pid"
    sleep 2
}
server_start() {
    local mode run_dir
    mode="$(detect_run_mode)"
    if [ "${mode}" = "systemd" ]; then
        systemctl start "${SERVICE_NAME}"
        log "Service ${SERVICE_NAME} dimulai (systemd)"
    else
        # Jalankan release AKTIF (target symlink CURRENT_DIR) — pada deploy normal
        # sudah menunjuk RELEASE_DIR; pada rollback menunjuk release lama yg di-restore.
        run_dir="$(readlink -f "${CURRENT_DIR}" 2>/dev/null || true)"
        [ -n "${run_dir}" ] || run_dir="${RELEASE_DIR:-${CURRENT_DIR}}"
        [ -x "${run_dir}/app-simlab" ] || error "server_start: ${run_dir}/app-simlab tidak ada/tidak executable"
        log "Menjalankan server (mode proses / manual-run) dari ${run_dir}..."
        (
            cd "${run_dir}"
            set -a
            [ -f .env ] && . ./.env
            set +a
            nohup ./app-simlab >> "${DATA_DIR}/app.log" 2>&1 &
            echo $! > "${DATA_DIR}/app.pid"
        )
        sleep 2
        log "Server diluncurkan (pid $(cat "${DATA_DIR}/app.pid" 2>/dev/null || echo '?')), log → ${DATA_DIR}/app.log"
    fi
}
server_is_running() {
    local mode
    mode="$(detect_run_mode)"
    if [ "${mode}" = "systemd" ]; then
        [ "$(systemctl is-active "${SERVICE_NAME}" 2>/dev/null || true)" = "active" ]
    else
        pgrep -f "app-simlab" >/dev/null 2>&1
    fi
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
    chown_optional "${DATA_DIR}"
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
    command -v tar >/dev/null 2>&1 || error "tar tidak terinstall"
    # systemctl hanya WAJIB saat run mode = systemd (ada unit service terdaftar);
    # mode proses (manual-run) tidak membutuhkan systemd sama sekali (doc 020 R2).
    if [ "$(detect_run_mode)" = "systemd" ] && ! command -v systemctl >/dev/null 2>&1; then
        error "systemctl tidak ditemukan padahal unit ${SERVICE_NAME} terdaftar (mode systemd)"
    fi
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
# app_user_exists: true bila user service ${APP_NAME} ada (hasil di-cache, R3).
APP_USER_CACHE=""
app_user_exists() {
    if [ -z "${APP_USER_CACHE}" ]; then
        if id -u "${APP_NAME}" >/dev/null 2>&1; then APP_USER_CACHE="yes"; else APP_USER_CACHE="no"; fi
    fi
    [ "${APP_USER_CACHE}" = "yes" ]
}
# chown_optional: pindahkan ownership target ke user service ${APP_NAME}.
#   Bila user service tidak ada (manual-run tanpa install.sh) → warn dan pertahankan
#   ownership pemakai yang menjalankan deploy (server dijalankan oleh user tsb —
#   realita doc 020 R3); JANGAN crash. Kegagalan chown tetap non-fatal.
chown_optional() {
    local target="$1"
    if app_user_exists; then
        if chown -R "${APP_NAME}:${APP_NAME}" "${target}" 2>/dev/null; then
            log "Ownership ${target} → ${APP_NAME}:${APP_NAME}"
        else
            warn "chown '${target}' → ${APP_NAME} gagal (lanjut, non-fatal)"
        fi
    else
        warn "ownership '${target}' dipertahankan milik $(id -un) (user service ${APP_NAME} tidak ada)"
    fi
}
# chown_data: pastikan ownership data dir ke service user (bila user ada, R3).
chown_data() {
    chown_optional "${DATA_DIR}"
}
