#!/data/data/com.termux/files/usr/bin/bash
# Build script for Termux (deploy_android — ARM64, SQLite)
cd "$(dirname "$0")/.."
echo "Building app-simlab for ARM64 (Termux) — pure Go, no CGO..."
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -tags nodynamic -o app-simlab ./cmd/server/main.go
if [ -f app-simlab ]; then
    echo "✅ Build selesai: ./app-simlab ($(du -h app-simlab | cut -f1))"
else
    echo "❌ Build gagal"
    exit 1
fi
