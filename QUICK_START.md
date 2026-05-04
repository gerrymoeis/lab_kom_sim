# Quick Start - Inventaris Lab Komputer

## 🚀 Jalankan Server

```bash
cd poc_prototype
go run cmd/server/main.go
```

Server akan berjalan di: `http://localhost:8080`

## 📱 Akses dari HP (WiFi yang sama)

### Langkah 1: Setup Firewall (Sekali saja)

**PowerShell sebagai Administrator**:
```powershell
.\setup_firewall.ps1
```

Atau manual:
```powershell
New-NetFirewallRule -DisplayName "Inventaris Lab - Port 8080" -Direction Inbound -LocalPort 8080 -Protocol TCP -Action Allow
```

### Langkah 2: Cari IP Laptop

```powershell
ipconfig
```

Cari bagian WiFi, contoh: `192.168.1.100`

### Langkah 3: Akses dari HP

Buka browser di HP:
```
http://192.168.1.100:8080
```

Ganti `192.168.1.100` dengan IP laptop Anda!

## 🔑 Login Default

- **Username**: `admin`
- **Password**: `admin123`

⚠️ **Ganti password setelah login pertama!**

## 📚 Dokumentasi Lengkap

- **Setup Firewall**: `../docs_and_backup/CARA_AKSES_DEMO.md`
- **User Guide**: `README.md`
- **API Docs**: `../docs_and_backup/PANDUAN_OCR_LOGBOOK.md`

## 🆘 Troubleshooting

### Tidak bisa akses dari HP?

1. ✅ Server sudah jalan? (`go run cmd/server/main.go`)
2. ✅ Firewall sudah dibuka? (jalankan `setup_firewall.ps1`)
3. ✅ IP address benar? (cek dengan `ipconfig`)
4. ✅ HP dan laptop di WiFi yang sama?

### Port 8080 sudah dipakai?

Ganti port di `.env`:
```
PORT=8081
```

Jangan lupa buka port baru di firewall!

## 🔧 Development

### Seed Data
```bash
go run cmd/seed/main.go
```

### Build Binary
```bash
go build -o inventaris-lab-kom.exe cmd/server/main.go
```

### Run Binary
```bash
.\inventaris-lab-kom.exe
```

---

**Need help?** Check `../docs_and_backup/` for detailed documentation!
