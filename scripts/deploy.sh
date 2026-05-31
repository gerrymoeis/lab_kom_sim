#!/bin/bash
# Server-side deploy script for simlab.service (Linux production)
# Called by cron polling: git fetch on deploy_linux → if HEAD changed → run this script
# Usage: ./scripts/deploy.sh
#
# Flow: git pull → go build → backup DB → symlink swap → systemctl restart → health check → rollback → cleanup

set -euo pipefail

# Production directory structure:
#   /opt/simlab/
#     .env              ← shared config (PORT, DATABASE_PATH, SESSION_SECRET, dll)
#     app/
#       repo/           ← clone dari GitHub (lab_kom_sim)
#       releases/       ← immutable release directories (keep 3 newest)
#       current/        ← symlink ke releases/TIMESTAMP/ (yang aktif)
#       data/           ← shared database (inventaris_lab.db)
#         inventaris_lab.db
#
# Aplikasi jalan dari: /opt/simlab/app/current/app-simlab
# Database dibaca dari: DATABASE_PATH di /opt/simlab/.env
# Semua release baca file database yang SAMA (tidak pernah di-copy saat deploy).
REPO_DIR="/opt/simlab/app/repo"
RELEASES_DIR="/opt/simlab/app/releases"
DATA_DIR="/opt/simlab/app/data"
CURRENT_DIR="/opt/simlab/app/current"

# Baca PORT dari /opt/simlab/.env untuk health check (fallback 8080)
PORT=$(grep '^PORT=' /opt/simlab/.env 2>/dev/null | cut -d= -f2-)
PORT=${PORT:-8080}

TIMESTAMP=$(date +%Y%m%d-%H%M%S)
RELEASE_DIR="$RELEASES_DIR/$TIMESTAMP"

echo "[deploy] === Deploy $TIMESTAMP ==="

# 1. Pull latest code from deploy_linux
cd "$REPO_DIR"
git pull origin deploy_linux

# 2. Save current release for rollback
PREVIOUS_RELEASE=""
if [ -L "$CURRENT_DIR" ] && [ -d "$(readlink -f "$CURRENT_DIR" 2>/dev/null)" ]; then
  PREVIOUS_RELEASE=$(readlink -f "$CURRENT_DIR")
fi

# 3. Create release directory
mkdir -p "$RELEASE_DIR"

# 4. Copy web assets
cp -r "$REPO_DIR/web" "$RELEASE_DIR/"

# 5. Build binary with modernc (pure Go, no CGO needed)
CGO_ENABLED=0 go build \
  -ldflags="-s -w" \
  -o "$RELEASE_DIR/app-simlab" \
  "$REPO_DIR/cmd/server/main.go"

# 6. Verify build
go vet "$REPO_DIR/..."
go test "$REPO_DIR/..." -short

# 7. Backup database
if [ -f "$DATA_DIR/inventaris_lab.db" ]; then
  cp "$DATA_DIR/inventaris_lab.db" "$DATA_DIR/backup-pre-$TIMESTAMP.db"
fi

# 8. Atomic symlink swap
ln -sfn "$RELEASE_DIR" "$CURRENT_DIR.new"
mv -T "$CURRENT_DIR.new" "$CURRENT_DIR"

# 9. Restart service
systemctl restart simlab.service

# 10. Health check (retry 5x, 2s interval)
for i in $(seq 1 5); do
  if curl -sf "http://localhost:$PORT/healthz" > /dev/null 2>&1; then
    echo "[deploy] ✅ Health check passed (attempt $i)"

    # 11. Cleanup old releases (keep last 3)
    ls -1t "$RELEASES_DIR" | tail -n +4 | xargs -I {} rm -rf "$RELEASES_DIR/{}"

    echo "[deploy] ✅ Deploy $TIMESTAMP selesai"
    exit 0
  fi
  echo "[deploy] ⏳ Health check attempt $i/5 failed, retrying in 2s..."
  sleep 2
done

# 12. Rollback on health check failure
echo "[deploy] ❌ Health check gagal — rollback ke release sebelumnya"
if [ -n "$PREVIOUS_RELEASE" ] && [ -d "$PREVIOUS_RELEASE" ]; then
  ln -sfn "$PREVIOUS_RELEASE" "$CURRENT_DIR.new"
  mv -T "$CURRENT_DIR.new" "$CURRENT_DIR"
  systemctl restart simlab.service
  echo "[deploy] 🔄 Rollback ke $PREVIOUS_RELEASE selesai"
else
  echo "[deploy] ⚠️ Tidak ada release sebelumnya untuk rollback"
fi

exit 1
