# Sistem Inventaris Laboratorium Komputer - PoC Prototype

Proof of Concept untuk sistem manajemen inventaris laboratorium komputer dengan fitur visualisasi grid PC, OCR logbook, dan tracking software.

## Tech Stack

- **Backend**: Golang 1.24+ dengan Gin Framework
- **Database**: SQLite
- **Frontend**: HTMX + Alpine.js + Bootstrap 5
- **OCR**: Tesseract + gosseract
- **Real-time**: Gorilla WebSocket

## Struktur Project

```
poc_prototype/
├── cmd/
│   └── server/          # Entry point aplikasi
├── internal/
│   ├── config/          # Konfigurasi aplikasi
│   ├── database/        # Database connection & migrations
│   ├── models/          # Data models
│   ├── handlers/        # HTTP handlers
│   ├── services/        # Business logic
│   └── middleware/      # Middleware (auth, logging, dll)
├── web/
│   ├── templates/       # HTML templates
│   └── static/          # CSS, JS, images
├── uploads/             # Upload folder untuk foto OCR
├── go.mod
└── README.md
```

## Instalasi

### Prerequisites

1. Go 1.24+
2. Tesseract OCR (untuk fitur OCR logbook)

### Setup

```bash
# Clone atau copy project
cd poc_prototype

# Install dependencies
go mod download

# Run aplikasi
go run cmd/server/main.go
```

Aplikasi akan berjalan di `http://localhost:8080`

## Fitur

### Fase 1 (MVP)
- ✅ Dashboard grid visualisasi 40 PC (8×5)
- ✅ CRUD manajemen PC
- ✅ Authentication (login/logout)
- ✅ User management (admin & dosen)

### Fase 2 (Development)
- 🔄 Manajemen perangkat lain
- 🔄 Software tracking
- 🔄 OCR logbook absensi
- 🔄 Export Excel

## Default Login

- **Username**: admin
- **Password**: admin123

⚠️ **Penting**: Ganti password default setelah login pertama!

## Development

```bash
# Run dengan auto-reload (jika menggunakan air)
air

# Build binary
go build -o inventaris-lab-kom.exe cmd/server/main.go

# Run tests
go test ./...
```

## Dokumentasi

Dokumentasi lengkap ada di folder `docs_and_backup/`

## License

Internal use only - Prodi Laboratorium Komputer
