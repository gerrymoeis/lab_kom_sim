package database

import (
	"database/sql"
	"fmt"
)

type catSeed struct {
	name, prefix string
}

type dtSeed struct {
	categoryName, name, brand, model, prefix, location, usageType string
}

func seedDevicesIfEmpty(db *DB) error {
	var catCount int
	db.QueryRow("SELECT COUNT(*) FROM categories").Scan(&catCount)
	if catCount > 0 {
		return nil
	}

	categories := []catSeed{
		{"Peripheral", "PERIPHERAL"},
		{"Network", "NETWORK"},
		{"Power", "POWER"},
		{"Display", "DISPLAY"},
		{"Printer", "PRINTER"},
		{"Consumable", "CONSUMABLE"},
		{"Audio", "AUDIO"},
		{"Tools", "TOOLS"},
		{"Server", "SERVER"},
		{"Security", "SECURITY"},
	}

	catMap := make(map[string]int)
	for _, c := range categories {
		res, err := db.Exec("INSERT INTO categories (name, default_prefix) VALUES (?, ?)", c.name, c.prefix)
		if err != nil {
			return fmt.Errorf("failed to seed category %s: %w", c.name, err)
		}
		id, _ := res.LastInsertId()
		catMap[c.name] = int(id)
	}

	deviceTypes := []dtSeed{
		{"Peripheral", "Pen Tab Wacom Intuos", "Wacom", "Intuos", "PENTAB", "Lemari Kaca", "loanable"},
		{"Peripheral", "Mouse Axioo", "Axioo", "", "MOUSE", "Lemari Kaca", "loanable"},
		{"Peripheral", "Keyboard Axioo", "Axioo", "", "KEYBOARD", "Lemari Kaca", "loanable"},
		{"Network", "Switch Ruijie RG-ES116G", "Ruijie", "RG-ES116G", "SWITCH-RJ16", "Lemari Kaca", "installable"},
		{"Network", "Router Nirkabel MikroTik", "MikroTik", "RB941-2nD", "ROUTER-MT", "Lemari Kaca", "installable"},
		{"Network", "Access Point Ubiquiti U6-LR", "Ubiquiti", "U6-LR", "UNIFI-AP", "Lemari Kaca", "installable"},
		{"Network", "Kabel UTP Belden CAT6", "Belden", "CAT6 NON PLENUM", "CABLE-UTP", "Lemari Kaca", "consumable"},
		{"Power", "Adaptor PC Set Axioo", "Axioo", "", "ADAPTOR-PC", "Lemari Kaca", "loanable"},
		{"Power", "UPS APC Easy UPS", "APC", "Easy UPS", "UPS-APC", "Lemari Kaca", "installable"},
		{"Power", "Kabel Listrik SPEDER", "SPEDER", "MONSTER", "CABLE-PWR", "Lemari Kaca", "consumable"},
		{"Power", "Stop Kontak OB isi 4", "UTICON", "", "STOPKONTAK", "Lemari Kaca", "loanable"},
		{"Display", "Proyektor Hitachi", "HITACHI", "", "PROJ-HTC", "Tergantung di atap", "installable"},
		{"Display", "Kabel HDMI 10m", "HDTV Premium", "", "HDMI-10M", "Lemari Kaca", "loanable"},
		{"Printer", "Printer EPSON EcoTank L3250", "EPSON", "EcoTank L3250", "PRINTER-EP", "Ruang Lab", "installable"},
		{"Consumable", "Isolasi Bening", "", "", "ISOLASI-BEN", "Lemari Kayu no.2", "consumable"},
		{"Consumable", "Double Tip Kecil", "", "", "DOUBLETIP", "Lemari Kayu no.2", "consumable"},
		{"Consumable", "MicroSD Card SanDisk 512GB", "SanDisk", "Ultra 512GB UHS-I Class 10", "SDCARD", "Lemari Kaca", "consumable"},
		{"Consumable", "HDD Seagate SkyHawk 6TB", "Seagate", "SkyHawk 6TB", "HDD-SATA", "Lemari Kaca", "consumable"},
		{"Audio", "Loudspeaker System JBL", "JBL", "Pasion", "SPEAKER-JBL", "Ruang Lab", "installable"},
		{"Audio", "Mikrofon Nirkabel Champion", "Champion", "Dual Channel UHF/PLL", "MIC-WIRELESS", "Ruang Lab", "loanable"},
		{"Tools", "Hydraulic Crimping Tool YQK-240", "YQK", "YQK-240", "CRIMP-HYD", "Lemari Kaca", "loanable"},
		{"Server", "Server Komputer DELL", "DELL Technologies", "", "SERVER-DELL", "Ruang Server", "installable"},
		{"Security", "CCTV HIKVISION Smart Hybrid Light PT", "HIKVISION", "Smart Hybrid Light PT", "CCTV-CAM", "Ruang Lab", "installable"},
		{"Security", "DVR HIKVISION AcuSense", "HIKVISION", "AcuSense TURBO HD PRO", "DVR-HIKV", "Ruang Server", "installable"},
	}

	for _, dt := range deviceTypes {
		catID := catMap[dt.categoryName]
		brand := sql.NullString{String: dt.brand, Valid: dt.brand != ""}
		model := sql.NullString{String: dt.model, Valid: dt.model != ""}
		loc := sql.NullString{String: dt.location, Valid: dt.location != ""}
		_, err := db.Exec(
			"INSERT INTO device_types (category_id, name, brand, model, asset_code_prefix, usage_type, default_location) VALUES (?, ?, ?, ?, ?, ?, ?)",
			catID, dt.name, brand, model, dt.prefix, dt.usageType, loc,
		)
		if err != nil {
			return fmt.Errorf("failed to seed device_type %s: %w", dt.name, err)
		}
	}

	fmt.Println("Seeded categories and device_types with real inventory data")
	return nil
}
