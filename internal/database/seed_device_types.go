package database

import (
	"database/sql"
	"fmt"
)

func seedDeviceTypesIfEmpty(db *sql.DB, boolTrue, boolFalse string) error {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM device_types").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check device_types count: %w", err)
	}
	if count > 0 {
		return nil
	}

	seeds := []string{
		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) VALUES ('Pen Tab Wacom Intuos', 'peripheral', 'Wacom', 'Intuos', 'individual', %s, %s, 'PENTAB', 'Lemari Kaca', 'Untuk menggantikan mouse dan memungkinkan pengguna membuat karya seni digital, ilustrasi, dan desain dengan presisi tinggi.')`, boolTrue, boolFalse),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) VALUES ('Mouse Axioo', 'peripheral', 'Axioo', NULL, 'individual', %s, %s, 'MOUSE', 'Lemari Kaca')`, boolTrue, boolFalse),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) VALUES ('Keyboard Axioo', 'peripheral', 'Axioo', NULL, 'individual', %s, %s, 'KEYBOARD', 'Lemari Kaca')`, boolTrue, boolFalse),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) VALUES ('Switch Ruijie RG-ES116G 16 Port', 'network', 'Ruijie', 'RG-ES116G', 'individual', %s, %s, 'SWITCH-RJ16', 'Lemari Kaca', 'Switch gigabit non-PoE unmanaged dengan 16 port 10/100/1000Mbps untuk jaringan stabil.')`, boolTrue, boolFalse),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) VALUES ('Switch Ruijie 48 Port', 'network', 'Ruijie', NULL, 'individual', %s, %s, 'SWITCH-RJ48', 'Lemari Kaca')`, boolTrue, boolFalse),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) VALUES ('Switch Gigabit Linksys LGS108AP 8 Port', 'network', 'Linksys', 'LGS108AP', 'individual', %s, %s, 'SWITCH-LK', 'Lemari Kaca')`, boolTrue, boolFalse),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) VALUES ('Router Nirkabel MikroTik hAP lite', 'network', 'MikroTik', 'RB941-2nD', 'individual', %s, %s, 'ROUTER-MT', 'Lemari Kaca', 'Memiliki 4 port Fast Ethernet dan satu titik akses nirkabel 2.4 GHz dengan RouterOS.')`, boolTrue, boolFalse),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) VALUES ('Ubiquiti UniFi Access Point U6-LR', 'network', 'Ubiquiti', 'U6-LR', 'individual', %s, %s, 'UNIFI-AP', 'Lemari Kaca', 'Wireless access point untuk menyediakan konektivitas Wi-Fi.')`, boolTrue, boolFalse),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) VALUES ('PoE Adapter Ubiquiti U-POE-AF', 'network', 'Ubiquiti', 'U-POE-AF', 'individual', %s, %s, 'POE-UBNT', 'Lemari Kaca', 'PoE Injector untuk menyalurkan daya listrik melalui kabel UTP ke access point atau kamera CCTV.')`, boolTrue, boolFalse),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) VALUES ('Kabel UTP Belden CAT6', 'network', 'Belden', 'CAT6 NON PLENUM', 'consumable', %s, %s, 'CABLE-UTP', 'Lemari Kaca', 'Media transmisi data dalam jaringan komputer. Kemasan 305 meter (1000 kaki) per roll.')`, boolFalse, boolTrue),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) VALUES ('RJ45 Connectors', 'network', 'NYK', NULL, 'consumable', %s, %s, 'CONN-RJ45', 'Lemari Kaca', 'Konektor kabel ethernet untuk membuat kabel patch jaringan. 100 buah per kotak.')`, boolFalse, boolTrue),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) VALUES ('RJ45 Plug Boot Belden', 'network', 'Belden', 'AP700021', 'consumable', %s, %s, 'BOOT-RJ45', 'Lemari Kaca')`, boolFalse, boolTrue),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) VALUES ('Crimping RJ45', 'network', 'Ou Bao', NULL, 'individual', %s, %s, 'CRIMP-RJ45', 'Lemari Kaca', 'Kompatibel dengan konektor RJ45, RJ11, dan RJ12. Dilengkapi dengan pemotong dan pengupas kawat.')`, boolTrue, boolFalse),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) VALUES ('Penguji Kabel LAN', 'network', 'MAXLINE', 'NSS-468A', 'individual', %s, %s, 'TESTER-LAN', 'Lemari Kaca', 'Untuk memeriksa konektivitas RJ45 dan RJ11 dengan indikator LED.')`, boolTrue, boolFalse),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) VALUES ('Adaptor PC Set Axioo', 'power', 'Axioo', NULL, 'individual', %s, %s, 'ADAPTOR-PC', 'Lemari Kaca')`, boolTrue, boolFalse),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) VALUES ('UPS APC Easy UPS', 'power', 'APC', 'Easy UPS', 'individual', %s, %s, 'UPS-APC', 'Lemari Kaca', 'Menyediakan daya cadangan dan melindungi perangkat dari lonjakan atau pemadaman listrik.')`, boolTrue, boolFalse),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) VALUES ('Kabel Listrik SPEDER Cable', 'power', 'SPEDER', 'MONSTER', 'consumable', %s, %s, 'CABLE-PWR', 'Lemari Kaca')`, boolFalse, boolTrue),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) VALUES ('Kabel Listrik BLITZ NYYHY', 'power', 'BLITZ', 'NYYHY 2x2.5mm', 'consumable', %s, %s, 'CABLE-NYY', 'Lemari Kaca')`, boolFalse, boolTrue),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) VALUES ('Stop Kontak OB isi 4', 'power', 'UTICON', NULL, 'individual', %s, %s, 'STOPKONTAK', 'Lemari Kaca')`, boolTrue, boolFalse),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) VALUES ('Steker Arde', 'power', 'MEVAL', NULL, 'individual', %s, %s, 'STEKER', 'Lemari Kaca')`, boolTrue, boolFalse),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) VALUES ('Proyektor Hitachi', 'display', 'HITACHI', NULL, 'individual', %s, %s, 'PROJ-HTC', 'Tergantung di atap', 'Resolusi XGA (1024 x 768), kecerahan 2700 ANSI lumens, teknologi 3LCD.')`, boolTrue, boolFalse),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) VALUES ('Kabel HDMI 10 meter', 'display', 'HDTV Premium', NULL, 'individual', %s, %s, 'HDMI-10M', 'Lemari Kaca')`, boolTrue, boolFalse),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) VALUES ('Kabel HDMI 20 meter', 'display', 'VENTION', NULL, 'individual', %s, %s, 'HDMI-20M', 'Lemari Kaca')`, boolTrue, boolFalse),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) VALUES ('Kabel VGA', 'display', NULL, NULL, 'individual', %s, %s, 'CABLE-VGA', 'Lemari Kaca')`, boolTrue, boolFalse),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) VALUES ('Wall Socket Module HDMI Websong', 'display', 'Websong', NULL, 'individual', %s, %s, 'SOCKET-HDMI', 'Dinding')`, boolTrue, boolFalse),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) VALUES ('Remote Proyektor Hitachi', 'display', 'Hitachi', 'R017F', 'individual', %s, %s, 'REMOTE-PROJ', 'Lemari Kaca')`, boolTrue, boolFalse),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) VALUES ('Printer EPSON EcoTank L3250', 'printer', 'EPSON', 'EcoTank L3250', 'individual', %s, %s, 'PRINTER-EP', 'Ruang Lab', 'Printer Multifungsi (Print, Scan, Copy) dengan teknologi EcoTank.')`, boolFalse, boolFalse),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) VALUES ('CD/DVD Windows Driver Set', 'consumable', 'Axioo', NULL, 'consumable', %s, %s, 'MEDIA-DVD', 'Lemari Kaca', 'DVD Windows Driver. Satu set dalam plastik.')`, boolTrue, boolTrue),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) VALUES ('Isolasi Bening', 'consumable', NULL, NULL, 'consumable', %s, %s, 'ISOLASI-BEN', 'Lemari Kayu no.2')`, boolFalse, boolTrue),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) VALUES ('Isolasi Hitam', 'consumable', NULL, NULL, 'consumable', %s, %s, 'ISOLASI-HTM', 'Lemari Kayu no.2')`, boolFalse, boolTrue),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) VALUES ('Double Tip Kecil', 'consumable', NULL, NULL, 'consumable', %s, %s, 'DOUBLETIP', 'Lemari Kayu no.2')`, boolFalse, boolTrue),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) VALUES ('MicroSD Card SanDisk 512GB', 'consumable', 'SanDisk', 'Ultra 512GB UHS-I Class 10', 'consumable', %s, %s, 'SDCARD', 'Lemari Kaca', 'Media penyimpanan untuk kamera CCTV. Kecepatan baca hingga 100 MB/s.')`, boolTrue, boolTrue),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) VALUES ('Hard Disk Drive Seagate SkyHawk 6TB', 'consumable', 'Seagate', 'SkyHawk 6TB', 'consumable', %s, %s, 'HDD-SATA', 'Lemari Kaca', 'HDD internal SATA untuk surveillance recording 24/7 di sistem CCTV/DVR/NVR.')`, boolFalse, boolTrue),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) VALUES ('Speaker', 'audio', NULL, NULL, 'individual', %s, %s, 'SPEAKER', 'Lemari Kaca')`, boolTrue, boolFalse),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) VALUES ('Loudspeaker System JBL', 'audio', 'JBL', 'PasiÃ³n', 'individual', %s, %s, 'SPEAKER-JBL', 'Ruang Lab', 'Loudspeaker Pasif (membutuhkan amplifier eksternal) dirancang oleh HARMAN.')`, boolTrue, boolFalse),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) VALUES ('Braket Speaker Dinding BMB', 'audio', 'BMB', NULL, 'individual', %s, %s, 'BRAKET-SPK', 'Ruang Lab')`, boolTrue, boolFalse),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) VALUES ('Mixing Amplifier HA AUDIO MA-2600', 'audio', 'HA AUDIO', 'MA-2600', 'individual', %s, %s, 'AMP-MIXER', 'Ruang Lab', 'Power Amplifier dan Mixing untuk Loudspeaker. Fitur Digital Korea Echo untuk Karaoke.')`, boolTrue, boolFalse),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) VALUES ('Mikrofon Nirkabel Champion 1', 'audio', 'Champion', 'Dual Channel UHF/PLL', 'individual', %s, %s, 'MIC-WIRELESS', 'Ruang Lab', 'Mikrofon Nirkabel Profesional dengan teknologi UHF/PLL Dual Channel.')`, boolTrue, boolFalse),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) VALUES ('Hydraulic Crimping Tool YQK-240', 'tools', 'YQK', 'YQK-240', 'individual', %s, %s, 'CRIMP-HYD', 'Lemari Kaca', 'Alat Press Hidrolik untuk menghubungkan kabel dengan konektor berukuran besar.')`, boolTrue, boolFalse),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) VALUES ('Gunting', 'tools', NULL, NULL, 'individual', %s, %s, 'GUNTING', 'Lemari Kaca')`, boolTrue, boolFalse),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) VALUES ('Server Komputer DELL', 'server', 'DELL Technologies', NULL, 'individual', %s, %s, 'SERVER-DELL', 'Ruang Server', 'Pusat komputasi dan penyimpanan data untuk Laboratorium Komputer. Tipe Rack-Mount.')`, boolFalse, boolFalse),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) VALUES ('Kamera CCTV HIKVISION Smart Hybrid Light PT', 'security', 'HIKVISION', 'Smart Hybrid Light PT', 'individual', %s, %s, 'CCTV-CAM', 'Ruang Lab', 'PT Camera (Pan/Tilt) dengan teknologi Smart Hybrid Light untuk pengawasan area luas.')`, boolFalse, boolFalse),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) VALUES ('Digital Video Recorder HIKVISION AcuSense', 'security', 'HIKVISION', 'AcuSense TURBO HD PRO', 'individual', %s, %s, 'DVR-HIKV', 'Ruang Server', 'Perekam video dan hub sentral untuk kamera CCTV. Mendukung TURBO HD dan Hybrid dengan teknologi AI AcuSense.')`, boolFalse, boolFalse),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) VALUES ('Buku Besar', 'stationery', 'Paperline', NULL, 'individual', %s, %s, 'BUKU', 'Meja Laboran')`, boolFalse, boolFalse),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) VALUES ('Bolpoin', 'stationery', 'Snowman', NULL, 'consumable', %s, %s, 'BOLPOIN', 'Meja Laboran')`, boolFalse, boolTrue),

		fmt.Sprintf(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) VALUES ('Penggaris', 'stationery', 'Microtop', NULL, 'individual', %s, %s, 'PENGGARIS', 'Lemari Kayu no.2')`, boolTrue, boolFalse),
	}

	for _, s := range seeds {
		if _, err := db.Exec(s); err != nil {
			fmt.Printf("Warning: Failed to seed device_types: %v\nSQL: %s\n", err, s)
		}
	}
	fmt.Println("✅ Seeded device_types with 46 templates from real inventory data")
	return nil
}
