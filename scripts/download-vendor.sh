#!/bin/sh
# Download vendor static assets for offline deployment
# Run from project root: scripts/download-vendor.sh

VENDOR="web/static/vendor"
mkdir -p "$VENDOR/bootstrap/css" "$VENDOR/bootstrap/js" "$VENDOR/bootstrap-icons/fonts"

echo "Downloading Bootstrap CSS..."
curl -sL "https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css" -o "$VENDOR/bootstrap/css/bootstrap.min.css"

echo "Downloading Bootstrap JS..."
curl -sL "https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/js/bootstrap.bundle.min.js" -o "$VENDOR/bootstrap/js/bootstrap.bundle.min.js"

echo "Downloading Bootstrap Icons..."
curl -sL "https://cdn.jsdelivr.net/npm/bootstrap-icons@1.11.0/font/bootstrap-icons.min.css" -o "$VENDOR/bootstrap-icons/bootstrap-icons.min.css"
curl -sL "https://cdn.jsdelivr.net/npm/bootstrap-icons@1.11.0/font/fonts/bootstrap-icons.woff2" -o "$VENDOR/bootstrap-icons/fonts/bootstrap-icons.woff2"
curl -sL "https://cdn.jsdelivr.net/npm/bootstrap-icons@1.11.0/font/fonts/bootstrap-icons.woff" -o "$VENDOR/bootstrap-icons/fonts/bootstrap-icons.woff"

# Fix cache-buster query strings in CSS
sed -i 's/?[a-f0-9]\{32\}//g' "$VENDOR/bootstrap-icons/bootstrap-icons.min.css"

echo "Downloading heic-to..."
curl -sL "https://cdn.jsdelivr.net/npm/heic-to@1.4.2/dist/iife/heic-to.js" -o "$VENDOR/heic-to.js"

echo "Done. Vendor files downloaded to $VENDOR/"
ls -lh "$VENDOR/bootstrap/css" "$VENDOR/bootstrap/js" "$VENDOR/bootstrap-icons" "$VENDOR/bootstrap-icons/fonts" "$VENDOR/heic-to.js"