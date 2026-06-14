#!/bin/bash
# Deploy script for Linux — pull + build + restart via deploy-linux.sh
# Usage: ./scripts/deploy.sh

set -e
cd "$(dirname "$0")/.."

echo "[deploy] Pulling latest code..."
git pull origin deploy_linux

echo "[deploy] Building & restarting..."
bash scripts/deploy-linux.sh
