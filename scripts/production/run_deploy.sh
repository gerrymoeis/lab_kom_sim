#!/bin/bash
# =============================================================================
# SIMLABKOM — Production Deploy (run_deploy.sh) — SATU FILE utk deploy/update/migrasi
#
# Pola E2E (run_e2e.sh): jalankan dari folder bundle yang ter-extract, dari mana
# saja. Self-locating: memakai folder sendiri utk menemukan deploy_production.sh
# + bin/config/seeds/lib/assets. Delegasikan SEMUA argumen.
#
#   cd deploy_production_<ts>
#   sudo bash run_deploy.sh [--skip-migrate] [--skip-test] [--install-dir X] [--allow-roots "a b"]
#                         [--force] [--keep-bundle]
#   --force       = dipertahankan utk kompatibilitas; P15 kini AUTO-CLEAN saat gate lolos
#   --keep-bundle = P15 skip self-delete (bundle dipertahankan)
# =============================================================================
set -euo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec bash "${DIR}/deploy_production.sh" "$@"