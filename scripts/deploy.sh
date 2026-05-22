#!/data/data/com.termux/files/usr/bin/bash
# Deploy script for Termux — called via SSH from laptop
# Usage: ./scripts/deploy.sh

set -e

cd ~/lab_kom_sim

echo "[deploy] Switching to refinement branch..."
git checkout refinement 2>/dev/null || true

echo "[deploy] Pulling latest code..."
git pull origin refinement

echo "[deploy] Checking vendor assets..."
if [ ! -f "web/static/vendor/bootstrap/css/bootstrap.min.css" ]; then
    echo "[deploy] Downloading vendor assets..."
    bash scripts/download-vendor.sh
fi

echo "[deploy] Building binary..."
CGO_ENABLED=0 go build -ldflags="-s -w" -tags nodynamic -o app-simlab ./cmd/server/main.go

echo "[deploy] Stopping existing server..."
pkill -f app-simlab 2>/dev/null || true
sleep 1

echo "[deploy] Starting new server..."
nohup ./app-simlab > /dev/null 2>&1 &

echo "[deploy] Done. Server restarted."
