#!/bin/bash
# seed_old_install.sh — menanam "versi lama" SIMLab di sembarang lokasi (Fase F, doc 018).
# Jalankan DARI folder bundle yang sudah ter-extract (memakai bin/ + assets/ milik bundle).
#
# usage: ./seed_old_install.sh <LOC> [v1|v2] [--service|--start] [--replace] [--db <file>] [--bin <path>]
#   (otomatis sudo bila dijalankan bukan root)
#   <LOC>      direktori install (lokasi random utk studi kasus auto-discovery)
#   v1         format single-lab legacy (DATABASE_PATH) — DEFAULT, kasus nyata
#              "versi lama"; deploy TUNTAS di sini (migrasi single→multi)
#   v2         format multi-lab — utk uji deteksi/struktur; deploy TUNTAS hanya
#              bila lokasi pernah di-deploy penuh (lihat -verify P11)
#   --service  tulis + enable unit systemd simlab.service yang menunjuk <LOC>
#   --start    jalankan app-simlab sbg proses latar (nohup)
#   --replace  hentikan server berjalan + hapus <LOC>/app dan <LOC>/data dulu
#              (mengganti install terbaru dgn app lama yg bersih; skenario EXISTING)
#   --db FILE  gunakan FILE sbg source single-DB (v1) — default: assets/inventaris_lab_empty.db
#   --bin PATH gunakan PATH sbg binary app (mis. app LAMA dari build_old_main.ps1);
#              web/ di samping PATH ikut disalin bila ada (app lama baca web/ dari
#              disk relatif CWD). Tanpa --bin: pakai bin/app-simlab milik bundle
#              (jalankan dari folder bundle)
set -euo pipefail

# Root dibutuhkan (mkdir/cp/systemctl di /opt, /etc). Bila bukan root, jalankan
# ulang via sudo secara otomatis agar perintah guide (./seed_old_install.sh ...)
# TIDAK pernah gagal "Ijin ditolak" (fix 19 Agu malam - run FRESH sukses karena
# sudo bash run_deploy.sh; run EXISTING dijalankan tanpa root).
if [ "$(id -u)" -ne 0 ]; then
    if command -v sudo >/dev/null 2>&1; then
        echo "seed_old_install.sh butuh root - jalankan ulang via sudo" >&2
        exec sudo bash "$0" "$@"
    fi
    echo "ERROR: butuh root dan sudo tidak tersedia. Jalankan: sudo bash $0 $*" >&2
    exit 1
fi

LOC=""
FMT="v1"
MODE=""
DB_ARG=""
BIN_ARG=""
REPLACE=""
while [ "$#" -gt 0 ]; do
    case "$1" in
        --bin) BIN_ARG="${2:-}"; shift 2 ;;
        --db) DB_ARG="${2:-}"; shift 2 ;;
        --service|--start) MODE="$1"; shift ;;
        --replace) REPLACE=1; shift ;;
        v1|v2) FMT="$1"; shift ;;
        *) LOC="${LOC:-$1}"; shift ;;
    esac
done
LOC="${LOC:?usage: seed_old_install.sh <LOC> [v1|v2] [--service|--start] [--replace] [--db FILE] [--bin PATH]}"
SELF_DIR="$(cd "$(dirname "$0")" && pwd)"

if [ -n "${BIN_ARG}" ]; then
    [ -f "${BIN_ARG}" ] || { echo "ERROR: --bin tidak ditemukan: ${BIN_ARG}" >&2; exit 1; }
else
    [ -d "${SELF_DIR}/bin" ] || { echo "ERROR: jalankan dari folder bundle (bin/ tidak ada di ${SELF_DIR}) atau beri --bin <path>" >&2; exit 1; }
fi

REL="${LOC}/app/releases/old"

if [ "${REPLACE}" = "1" ]; then
    echo "SEED --replace: hentikan server + hapus ${LOC}/app dan ${LOC}/data"
    if systemctl is-active --quiet simlab 2>/dev/null; then sudo systemctl stop simlab; fi
    sudo pkill -x app-simlab 2>/dev/null || true
    sleep 1
    sudo rm -rf "${LOC}/app" "${LOC}/data"
fi

mkdir -p "${REL}" "${LOC}/data/uploads" "${LOC}/data/backups"
ln -sfn "${REL}" "${LOC}/app/current"

if [ -n "${BIN_ARG}" ]; then
    cp -f "${BIN_ARG}" "${REL}/app-simlab"
else
    cp -f "${SELF_DIR}/bin/app-simlab" "${REL}/app-simlab"
fi
chmod +x "${REL}/app-simlab"

if [ -n "${BIN_ARG}" ] && [ -d "$(dirname "${BIN_ARG}")/web" ]; then
    cp -rf "$(dirname "${BIN_ARG}")/web" "${REL}/web"
fi

if [ "${FMT}" = "v1" ]; then
    if [ -n "${DB_ARG}" ]; then
        cp -f "${DB_ARG}" "${LOC}/data/inventaris_lab.db"
    else
        cp -f "${SELF_DIR}/assets/inventaris_lab_empty.db" "${LOC}/data/inventaris_lab.db"
    fi
    cat > "${LOC}/.env" <<EOF
DATABASE_PATH=${LOC}/data/inventaris_lab.db
SESSION_SECRET=legacy_secret_v1
UPLOAD_PATH=${LOC}/data/uploads
PORT=18081
EOF
else
    # v2 = multi-lab. Data file disalin dari seed kosong-valid supaya struktur
    # terdeteksi/valid (validate_install_dir). CATATAN: deploy TUNTAS di lokasi
    # ini hanya bila sudah pernah di-deploy penuh (global.db+lab DB+seeds utk
    # -verify P11); bila tidak, deploy aman rollback di P11 (perilaku benar).
    cp -f "${SELF_DIR}/assets/inventaris_lab_empty.db" "${LOC}/data/global.db"
    cp -f "${SELF_DIR}/assets/inventaris_lab_empty.db" "${LOC}/data/lab_mi_1.db"
    cp -f "${SELF_DIR}/assets/inventaris_lab_empty.db" "${LOC}/data/lab_vokasi_1.db"
    cat > "${LOC}/.env" <<EOF
GLOBAL_DB_PATH=${LOC}/data/global.db
LABS_1_ID=MI-1
LABS_1_DB=${LOC}/data/lab_mi_1.db
LABS_1_TITLE=Lab Kom MI
LABS_1_URL=lab-mi
LABS_2_ID=VOKASI-1
LABS_2_DB=${LOC}/data/lab_vokasi_1.db
LABS_2_TITLE=Lab Kom Vokasi 1
LABS_2_URL=lab-vokasi-1
UPLOAD_PATH=${LOC}/data/uploads
BACKUP_DIR=${LOC}/data/backups
SESSION_SECRET=multi_secret_v2
DATABASE_URL=
PORT=18081
EOF
fi
chmod 600 "${LOC}/.env"

case "${MODE}" in
--service)
    sudo tee /etc/systemd/system/simlab.service >/dev/null <<EOF
[Unit]
Description=SIMLab App
After=network.target

[Service]
ExecStart=${LOC}/app/current/app-simlab
WorkingDirectory=${LOC}/app/current
EnvironmentFile=${LOC}/.env
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF
    sudo systemctl daemon-reload
    sudo systemctl enable --now simlab
    sleep 1
    ;;
--start)
    # App memuat .env sendiri via godotenv dari CWD (config.Load). Salin .env ke
    # release dir (pola produksi deploy P6) dan JANGAN source via bash — nilai
    # ber-spasi/CRLF mematikan shell (doc 023 BUG-1). ENV_PATH TIDAK di-load app.
    cp -f "${LOC}/.env" "${LOC}/app/current/.env"
    ( cd "${LOC}/app/current" && nohup ./app-simlab >"${LOC}/data/app.log" 2>&1 & )
    sleep 2
    ;;
esac

echo "SEED OK: ${LOC} (${FMT}${MODE:+ ${MODE}})"