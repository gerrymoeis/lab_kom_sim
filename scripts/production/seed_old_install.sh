#!/bin/bash
# seed_old_install.sh — menanam "versi lama" SIMLab di sembarang lokasi (Fase F, doc 018).
# Jalankan DARI folder bundle yang sudah ter-extract (memakai bin/ + assets/ milik bundle).
#
# usage: ./seed_old_install.sh <LOC> [v1|v2] [--service|--start] [--db <file>]
#   <LOC>      direktori install (lokasi random utk studi kasus auto-discovery)
#   v1         format single-lab legacy (DATABASE_PATH) — DEFAULT, kasus nyata
#              "versi lama"; deploy TUNTAS di sini (migrasi single→multi)
#   v2         format multi-lab — utk uji deteksi/struktur; deploy TUNTAS hanya
#              bila lokasi pernah di-deploy penuh (lihat -verify P11)
#   --service  tulis + enable unit systemd simlab.service yang menunjuk <LOC>
#   --start    jalankan app-simlab sbg proses latar (nohup)
#   --db FILE  gunakan FILE sbg source single-DB (v1) — default: assets/inventaris_lab_empty.db
set -euo pipefail
LOC="${1:?usage: seed_old_install.sh <LOC> [v1|v2] [--service|--start] [--db FILE]}"
FMT="${2:-v1}"
MODE="${3:-}"
DB_ARG="${4:-}"
SELF_DIR="$(cd "$(dirname "$0")" && pwd)"

[ -d "${SELF_DIR}/bin" ] || { echo "ERROR: jalankan dari folder bundle (bin/ tidak ada di ${SELF_DIR})" >&2; exit 1; }

REL="${LOC}/app/releases/old"
mkdir -p "${REL}" "${LOC}/data/uploads" "${LOC}/data/backups"
ln -sfn "${REL}" "${LOC}/app/current"
cp -f "${SELF_DIR}/bin/app-simlab" "${REL}/app-simlab"
chmod +x "${REL}/app-simlab"

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
    ( cd "${LOC}/app/current" && set -a && . "${LOC}/.env" && set +a && nohup ./app-simlab >"${LOC}/data/app.log" 2>&1 & )
    sleep 2
    ;;
esac

echo "SEED OK: ${LOC} (${FMT}${MODE:+ ${MODE}})"