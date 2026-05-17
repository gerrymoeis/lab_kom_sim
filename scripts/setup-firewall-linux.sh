#!/bin/bash
# Setup Linux Firewall untuk Inventaris Lab Komputer
# Usage: sudo bash scripts/setup-firewall-linux.sh
# Mendukung ufw, firewalld, dan iptables

set -e

PORT=${1:-8080}

if [ "$EUID" -ne 0 ]; then
    echo "ERROR: Jalankan sebagai root (sudo)."
    echo "  sudo bash scripts/setup-firewall-linux.sh"
    exit 1
fi

echo "========================================"
echo "  Setup Firewall - Inventaris Lab Kom  "
echo "========================================"
echo ""

if command -v ufw &>/dev/null; then
    echo "Menggunakan ufw..."
    ufw allow "$PORT/tcp" comment "Inventaris Lab Komputer"
    ufw reload 2>/dev/null || true
    echo "  Port $PORT opened via ufw"
elif command -v firewall-cmd &>/dev/null; then
    echo "Menggunakan firewalld..."
    firewall-cmd --add-port="$PORT/tcp" --permanent
    firewall-cmd --reload
    echo "  Port $PORT opened via firewalld"
elif command -v iptables &>/dev/null; then
    echo "Menggunakan iptables..."
    iptables -A INPUT -p tcp --dport "$PORT" -j ACCEPT
    echo "  Port $PORT opened via iptables"
    echo "  NOTE: iptables rules are not persistent by default."
    echo "  To save: sudo apt install iptables-persistent (Debian/Ubuntu)"
    echo "  Or: sudo dnf install iptables-services (Fedora/RHEL)"
else
    echo "ERROR: Tidak ada firewall tool terdeteksi (ufw/firewalld/iptables)."
    echo "Install salah satu:"
    echo "  Debian/Ubuntu: sudo apt install ufw"
    echo "  Fedora/RHEL:   sudo dnf install firewalld"
    echo "  Arch:          sudo pacman -S iptables"
    exit 1
fi

echo ""
echo "Setup selesai. Port $PORT terbuka."
echo "Cek akses: http://$(hostname -I | awk '{print $1}'):$PORT"
echo "========================================"
