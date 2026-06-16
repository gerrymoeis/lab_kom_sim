#!/bin/bash
# Build script for Linux
# Usage: bash scripts/build-linux.sh
# No C compiler needed — SQLite via modernc.org/sqlite (pure Go)

set -e
cd "$(dirname "$0")/.."

echo "Building app-simlab for Linux..."
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o app-simlab ./cmd/server/main.go

if [ -f app-simlab ]; then
    size=$(du -h app-simlab | cut -f1)
    echo "Build selesai: ./app-simlab ($size)"
else
    echo "Build gagal"
    exit 1
fi
