#!/data/data/com.termux/files/usr/bin/bash
# Build script for Termux (Android ARM64)
cd "$(dirname "$0")/.."
echo "Building app-simlab for ARM64 (Termux)..."
CGO_ENABLED=0 go build -tags nodynamic -o app-simlab ./cmd/server/main.go
if [ -f app-simlab ]; then
    echo "✅ Build selesai: ./app-simlab ($(du -h app-simlab | cut -f1))"
else
    echo "❌ Build gagal"
    exit 1
fi
