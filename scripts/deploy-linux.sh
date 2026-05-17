#!/bin/bash
# Deploy script for Linux
# Usage: bash scripts/deploy-linux.sh [--install-service]
# Options:
#   --install-service   Install systemd service (requires root)
#
# Detects systemd availability. Falls back to nohup if systemd not found.

set -e
cd "$(dirname "$0")/.."

BINARY="app-simlab"
SERVICE_NAME="inventaris-lab"
INSTALL_DIR="/opt/$SERVICE_NAME"
ENV_FILE="/etc/$SERVICE_NAME/.env"
DATA_DIR="/var/lib/$SERVICE_NAME"

echo "========================================"
echo "  Deploy - Inventaris Lab Komputer      "
echo "========================================"
echo ""

# 1. Build
echo "[1/3] Building binary..."
CGO_ENABLED=0 go build -ldflags="-s -w" -o "$BINARY" ./cmd/server/main.go
size=$(du -h "$BINARY" | cut -f1)
echo "  Build selesai: $BINARY ($size)"

# 2. Check vendor assets
if [ ! -f "web/static/vendor/bootstrap/css/bootstrap.min.css" ]; then
    echo "[  *] Downloading vendor assets..."
    bash scripts/download-vendor.sh
fi

# 3. Deploy
USE_SYSTEMD=false
if command -v systemctl &>/dev/null; then
    USE_SYSTEMD=true
fi

if [ "$USE_SYSTEMD" = true ] || [ "$1" = "--install-service" ]; then
    echo "[2/3] Installing systemd service..."
    sudo mkdir -p "$INSTALL_DIR" "$DATA_DIR" "/etc/$SERVICE_NAME"
    sudo cp "$BINARY" "$INSTALL_DIR/"
    if [ -f ".env" ]; then
        sudo cp .env "$ENV_FILE"
    fi
    if [ -d "uploads" ]; then
        sudo cp -r uploads "$DATA_DIR/"
    fi
    sudo cp "scripts/$SERVICE_NAME.service" /etc/systemd/system/
    sudo sed -i "s|/opt/inventaris-lab|$INSTALL_DIR|g" "/etc/systemd/system/$SERVICE_NAME.service"
    sudo sed -i "s|/etc/inventaris-lab|/etc/$SERVICE_NAME|g" "/etc/systemd/system/$SERVICE_NAME.service"
    sudo sed -i "s|/var/lib/inventaris-lab|$DATA_DIR|g" "/etc/systemd/system/$SERVICE_NAME.service"
    sudo systemctl daemon-reload
    sudo systemctl enable "$SERVICE_NAME"
    sudo systemctl restart "$SERVICE_NAME"
    echo "  Service installed: sudo systemctl status $SERVICE_NAME"
else
    echo "[2/3] systemd not found. Using nohup..."
    pkill -f "$BINARY" 2>/dev/null || true
    sleep 1
    nohup "./$BINARY" > server.log 2>&1 &
    echo "  PID: $!"
    echo "  Log: $(pwd)/server.log"
fi

echo "[3/3] Done."
echo ""
echo "  Akses: http://localhost:8080"
echo "========================================"
