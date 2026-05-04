# Setup Windows Firewall untuk Inventaris Lab Komputer
# Script ini membuka port 8080 untuk akses dari device lain di network

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  Setup Firewall - Inventaris Lab Kom  " -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Check if running as Administrator
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)

if (-not $isAdmin) {
    Write-Host "❌ ERROR: Script ini harus dijalankan sebagai Administrator!" -ForegroundColor Red
    Write-Host ""
    Write-Host "Cara menjalankan:" -ForegroundColor Yellow
    Write-Host "1. Klik kanan pada PowerShell" -ForegroundColor Yellow
    Write-Host "2. Pilih 'Run as Administrator'" -ForegroundColor Yellow
    Write-Host "3. Jalankan script ini lagi" -ForegroundColor Yellow
    Write-Host ""
    Read-Host "Tekan Enter untuk keluar"
    exit 1
}

Write-Host "✅ Running as Administrator" -ForegroundColor Green
Write-Host ""

# Check if rule already exists
$existingRule = Get-NetFirewallRule -DisplayName "Inventaris Lab - Port 8080" -ErrorAction SilentlyContinue

if ($existingRule) {
    Write-Host "⚠️  Firewall rule sudah ada!" -ForegroundColor Yellow
    Write-Host ""
    $response = Read-Host "Hapus dan buat ulang? (y/n)"
    
    if ($response -eq "y" -or $response -eq "Y") {
        Write-Host "🗑️  Menghapus rule lama..." -ForegroundColor Yellow
        Remove-NetFirewallRule -DisplayName "Inventaris Lab - Port 8080"
        Write-Host "✅ Rule lama dihapus" -ForegroundColor Green
        Write-Host ""
    } else {
        Write-Host "❌ Dibatalkan" -ForegroundColor Red
        Read-Host "Tekan Enter untuk keluar"
        exit 0
    }
}

# Create new firewall rule
Write-Host "🔧 Membuat firewall rule baru..." -ForegroundColor Cyan

try {
    New-NetFirewallRule `
        -DisplayName "Inventaris Lab - Port 8080" `
        -Direction Inbound `
        -LocalPort 8080 `
        -Protocol TCP `
        -Action Allow `
        -Profile Any `
        -Description "Allow inbound connections to Inventaris Lab Komputer on port 8080" | Out-Null
    
    Write-Host "✅ Firewall rule berhasil dibuat!" -ForegroundColor Green
    Write-Host ""
} catch {
    Write-Host "❌ ERROR: Gagal membuat firewall rule" -ForegroundColor Red
    Write-Host $_.Exception.Message -ForegroundColor Red
    Write-Host ""
    Read-Host "Tekan Enter untuk keluar"
    exit 1
}

# Get IP Address
Write-Host "📡 Mencari IP Address..." -ForegroundColor Cyan
Write-Host ""

$ipAddresses = Get-NetIPAddress -AddressFamily IPv4 | Where-Object { 
    $_.IPAddress -notlike "127.*" -and 
    $_.IPAddress -notlike "169.254.*" 
}

if ($ipAddresses) {
    Write-Host "🌐 IP Address yang tersedia:" -ForegroundColor Green
    Write-Host ""
    
    foreach ($ip in $ipAddresses) {
        $adapter = Get-NetAdapter -InterfaceIndex $ip.InterfaceIndex
        Write-Host "   Interface: $($adapter.Name)" -ForegroundColor Yellow
        Write-Host "   IP Address: $($ip.IPAddress)" -ForegroundColor Cyan
        Write-Host "   Status: $($adapter.Status)" -ForegroundColor $(if ($adapter.Status -eq "Up") { "Green" } else { "Red" })
        Write-Host ""
    }
    
    # Get the first active WiFi or Ethernet adapter
    $activeIP = $ipAddresses | Where-Object { 
        $adapter = Get-NetAdapter -InterfaceIndex $_.InterfaceIndex
        $adapter.Status -eq "Up" -and ($adapter.Name -like "*Wi-Fi*" -or $adapter.Name -like "*Ethernet*")
    } | Select-Object -First 1
    
    if ($activeIP) {
        Write-Host "========================================" -ForegroundColor Green
        Write-Host "  ✅ SETUP SELESAI!" -ForegroundColor Green
        Write-Host "========================================" -ForegroundColor Green
        Write-Host ""
        Write-Host "Untuk akses dari HP/device lain:" -ForegroundColor Yellow
        Write-Host ""
        Write-Host "   http://$($activeIP.IPAddress):8080" -ForegroundColor Cyan
        Write-Host ""
        Write-Host "Pastikan:" -ForegroundColor Yellow
        Write-Host "1. Server sudah berjalan (go run cmd/server/main.go)" -ForegroundColor White
        Write-Host "2. HP dan laptop di WiFi yang sama" -ForegroundColor White
        Write-Host "3. Buka URL di atas di browser HP" -ForegroundColor White
        Write-Host ""
    }
} else {
    Write-Host "⚠️  Tidak ada IP address yang ditemukan" -ForegroundColor Yellow
    Write-Host "Jalankan 'ipconfig' untuk cek IP address manual" -ForegroundColor Yellow
    Write-Host ""
}

# Test if port is listening
Write-Host "🔍 Mengecek apakah server sudah berjalan..." -ForegroundColor Cyan
$portTest = Test-NetConnection -ComputerName localhost -Port 8080 -WarningAction SilentlyContinue

if ($portTest.TcpTestSucceeded) {
    Write-Host "✅ Server sudah berjalan di port 8080" -ForegroundColor Green
} else {
    Write-Host "⚠️  Server belum berjalan" -ForegroundColor Yellow
    Write-Host "Jalankan: go run cmd/server/main.go" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Read-Host "Tekan Enter untuk keluar"
