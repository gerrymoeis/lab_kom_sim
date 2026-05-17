package database

import (
	"database/sql"
	"fmt"
)

type dtSeed struct {
	name, category, brand, model, itemType, prefix, location string
	loanable, consumable bool
	notes string
}

func seedDeviceTypesIfEmpty(db *DB, bt, bf string) error {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM device_types").Scan(&count); err != nil {
		return fmt.Errorf("failed to check device_types count: %w", err)
	}
	if count > 0 { return nil }

	exec := func(s dtSeed) {
		loan, cons := bf, bf
		if s.loanable { loan = bt }
		if s.consumable { cons = bt }
		brand := sql.NullString{String: s.brand, Valid: s.brand != ""}
		model := sql.NullString{String: s.model, Valid: s.model != ""}
		q := `INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location`
		v := `VALUES (? ,?, ?, ?, ?, ?, ?, ?, ?`
		args := []any{s.name, s.category, brand, model, s.itemType, loan, cons, s.prefix, s.location}
		if s.notes != "" {
			q += `, notes_template`
			v += `, ?`
			args = append(args, s.notes)
		}
		if _, err := db.Exec(q+`) `+v+`)`, args...); err != nil {
			fmt.Printf("Warning: Failed to seed device_types: %v\n", err)
		}
	}

	seeds := []dtSeed{
		{"Pen Tab Wacom Intuos", "peripheral", "Wacom", "Intuos", "individual", "PENTAB", "Lemari Kaca", true, false, "Untuk menggantikan mouse dan memungkinkan pengguna membuat karya seni digital, ilustrasi, dan desain dengan presisi tinggi."},
		{"Mouse Axioo", "peripheral", "Axioo", "", "individual", "MOUSE", "Lemari Kaca", true, false, ""},
		{"Keyboard Axioo", "peripheral", "Axioo", "", "individual", "KEYBOARD", "Lemari Kaca", true, false, ""},
		{"Switch Ruijie RG-ES116G 16 Port", "network", "Ruijie", "RG-ES116G", "individual", "SWITCH-RJ16", "Lemari Kaca", true, false, "Switch gigabit non-PoE unmanaged dengan 16 port 10/100/1000Mbps untuk jaringan stabil."},
		{"Switch Ruijie 48 Port", "network", "Ruijie", "", "individual", "SWITCH-RJ48", "Lemari Kaca", true, false, ""},
		{"Switch Gigabit Linksys LGS108AP 8 Port", "network", "Linksys", "LGS108AP", "individual", "SWITCH-LK", "Lemari Kaca", true, false, ""},
		{"Router Nirkabel MikroTik hAP lite", "network", "MikroTik", "RB941-2nD", "individual", "ROUTER-MT", "Lemari Kaca", true, false, "Memiliki 4 port Fast Ethernet dan satu titik akses nirkabel 2.4 GHz dengan RouterOS."},
		{"Ubiquiti UniFi Access Point U6-LR", "network", "Ubiquiti", "U6-LR", "individual", "UNIFI-AP", "Lemari Kaca", true, false, "Wireless access point untuk menyediakan konektivitas Wi-Fi."},
		{"PoE Adapter Ubiquiti U-POE-AF", "network", "Ubiquiti", "U-POE-AF", "individual", "POE-UBNT", "Lemari Kaca", true, false, "PoE Injector untuk menyalurkan daya listrik melalui kabel UTP ke access point atau kamera CCTV."},
		{"Kabel UTP Belden CAT6", "network", "Belden", "CAT6 NON PLENUM", "consumable", "CABLE-UTP", "Lemari Kaca", false, true, "Media transmisi data dalam jaringan komputer. Kemasan 305 meter (1000 kaki) per roll."},
		{"RJ45 Connectors", "network", "NYK", "", "consumable", "CONN-RJ45", "Lemari Kaca", false, true, "Konektor kabel ethernet untuk membuat kabel patch jaringan. 100 buah per kotak."},
		{"RJ45 Plug Boot Belden", "network", "Belden", "AP700021", "consumable", "BOOT-RJ45", "Lemari Kaca", false, true, ""},
		{"Crimping RJ45", "network", "Ou Bao", "", "individual", "CRIMP-RJ45", "Lemari Kaca", true, false, "Kompatibel dengan konektor RJ45, RJ11, dan RJ12. Dilengkapi dengan pemotong dan pengupas kawat."},
		{"Penguji Kabel LAN", "network", "MAXLINE", "NSS-468A", "individual", "TESTER-LAN", "Lemari Kaca", true, false, "Untuk memeriksa konektivitas RJ45 dan RJ11 dengan indikator LED."},
		{"Adaptor PC Set Axioo", "power", "Axioo", "", "individual", "ADAPTOR-PC", "Lemari Kaca", true, false, ""},
		{"UPS APC Easy UPS", "power", "APC", "Easy UPS", "individual", "UPS-APC", "Lemari Kaca", true, false, "Menyediakan daya cadangan dan melindungi perangkat dari lonjakan atau pemadaman listrik."},
		{"Kabel Listrik SPEDER Cable", "power", "SPEDER", "MONSTER", "consumable", "CABLE-PWR", "Lemari Kaca", false, true, ""},
		{"Kabel Listrik BLITZ NYYHY", "power", "BLITZ", "NYYHY 2x2.5mm", "consumable", "CABLE-NYY", "Lemari Kaca", false, true, ""},
		{"Stop Kontak OB isi 4", "power", "UTICON", "", "individual", "STOPKONTAK", "Lemari Kaca", true, false, ""},
		{"Steker Arde", "power", "MEVAL", "", "individual", "STEKER", "Lemari Kaca", true, false, ""},
		{"Proyektor Hitachi", "display", "HITACHI", "", "individual", "PROJ-HTC", "Tergantung di atap", true, false, "Resolusi XGA (1024 x 768), kecerahan 2700 ANSI lumens, teknologi 3LCD."},
		{"Kabel HDMI 10 meter", "display", "HDTV Premium", "", "individual", "HDMI-10M", "Lemari Kaca", true, false, ""},
		{"Kabel HDMI 20 meter", "display", "VENTION", "", "individual", "HDMI-20M", "Lemari Kaca", true, false, ""},
		{"Kabel VGA", "display", "", "", "individual", "CABLE-VGA", "Lemari Kaca", true, false, ""},
		{"Wall Socket Module HDMI Websong", "display", "Websong", "", "individual", "SOCKET-HDMI", "Dinding", true, false, ""},
		{"Remote Proyektor Hitachi", "display", "Hitachi", "R017F", "individual", "REMOTE-PROJ", "Lemari Kaca", true, false, ""},
		{"Printer EPSON EcoTank L3250", "printer", "EPSON", "EcoTank L3250", "individual", "PRINTER-EP", "Ruang Lab", false, false, "Printer Multifungsi (Print, Scan, Copy) dengan teknologi EcoTank."},
		{"CD/DVD Windows Driver Set", "consumable", "Axioo", "", "consumable", "MEDIA-DVD", "Lemari Kaca", true, true, "DVD Windows Driver. Satu set dalam plastik."},
		{"Isolasi Bening", "consumable", "", "", "consumable", "ISOLASI-BEN", "Lemari Kayu no.2", false, true, ""},
		{"Isolasi Hitam", "consumable", "", "", "consumable", "ISOLASI-HTM", "Lemari Kayu no.2", false, true, ""},
		{"Double Tip Kecil", "consumable", "", "", "consumable", "DOUBLETIP", "Lemari Kayu no.2", false, true, ""},
		{"MicroSD Card SanDisk 512GB", "consumable", "SanDisk", "Ultra 512GB UHS-I Class 10", "consumable", "SDCARD", "Lemari Kaca", true, true, "Media penyimpanan untuk kamera CCTV. Kecepatan baca hingga 100 MB/s."},
		{"Hard Disk Drive Seagate SkyHawk 6TB", "consumable", "Seagate", "SkyHawk 6TB", "consumable", "HDD-SATA", "Lemari Kaca", false, true, "HDD internal SATA untuk surveillance recording 24/7 di sistem CCTV/DVR/NVR."},
		{"Speaker", "audio", "", "", "individual", "SPEAKER", "Lemari Kaca", true, false, ""},
		{"Loudspeaker System JBL", "audio", "JBL", "PasiÃƒÂ³n", "individual", "SPEAKER-JBL", "Ruang Lab", true, false, "Loudspeaker Pasif (membutuhkan amplifier eksternal) dirancang oleh HARMAN."},
		{"Braket Speaker Dinding BMB", "audio", "BMB", "", "individual", "BRAKET-SPK", "Ruang Lab", true, false, ""},
		{"Mixing Amplifier HA AUDIO MA-2600", "audio", "HA AUDIO", "MA-2600", "individual", "AMP-MIXER", "Ruang Lab", true, false, "Power Amplifier dan Mixing untuk Loudspeaker. Fitur Digital Korea Echo untuk Karaoke."},
		{"Mikrofon Nirkabel Champion 1", "audio", "Champion", "Dual Channel UHF/PLL", "individual", "MIC-WIRELESS", "Ruang Lab", true, false, "Mikrofon Nirkabel Profesional dengan teknologi UHF/PLL Dual Channel."},
		{"Hydraulic Crimping Tool YQK-240", "tools", "YQK", "YQK-240", "individual", "CRIMP-HYD", "Lemari Kaca", true, false, "Alat Press Hidrolik untuk menghubungkan kabel dengan konektor berukuran besar."},
		{"Gunting", "tools", "", "", "individual", "GUNTING", "Lemari Kaca", true, false, ""},
		{"Server Komputer DELL", "server", "DELL Technologies", "", "individual", "SERVER-DELL", "Ruang Server", false, false, "Pusat komputasi dan penyimpanan data untuk Laboratorium Komputer. Tipe Rack-Mount."},
		{"Kamera CCTV HIKVISION Smart Hybrid Light PT", "security", "HIKVISION", "Smart Hybrid Light PT", "individual", "CCTV-CAM", "Ruang Lab", false, false, "PT Camera (Pan/Tilt) dengan teknologi Smart Hybrid Light untuk pengawasan area luas."},
		{"Digital Video Recorder HIKVISION AcuSense", "security", "HIKVISION", "AcuSense TURBO HD PRO", "individual", "DVR-HIKV", "Ruang Server", false, false, "Perekam video dan hub sentral untuk kamera CCTV. Mendukung TURBO HD dan Hybrid dengan teknologi AI AcuSense."},
		{"Buku Besar", "stationery", "Paperline", "", "individual", "BUKU", "Meja Laboran", false, false, ""},
		{"Bolpoin", "stationery", "Snowman", "", "consumable", "BOLPOIN", "Meja Laboran", false, true, ""},
		{"Penggaris", "stationery", "Microtop", "", "individual", "PENGGARIS", "Lemari Kayu no.2", true, false, ""},
	}

	for _, s := range seeds { exec(s) }
	fmt.Println("Seeded device_types with 46 templates from real inventory data")
	return nil
}
