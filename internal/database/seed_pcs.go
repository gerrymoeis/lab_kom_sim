package database

import "fmt"

type pcSeedData struct {
	Number     int
	SN         string
	OS         string
	RequiredSW []string
	OtherSW    []string
}

func seedPCs(db *DB) error {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM pcs").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check PC count: %w", err)
	}
	if count > 0 {
		return nil
	}

	pcs := []pcSeedData{
		{1, "0A23460005250060214", "Windows 11 Pro 23H2",
			[]string{"Visual Studio Code", "Cisco Packet Tracer", "Wireshark", "Postman", "PHP + Xampp", "Composer", "Unity", "Blender", "Android Studio", "Figma", "Node.js", "Python"},
			[]string{"Counter Strike Condition Zero", "Roblox", "Photoshop", "Curseforge", "Opera Browser"}},
		{2, "0A23460005250010116", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Postman", "PHP + Xampp", "Blender", "Android Studio", "Figma", "Python", "SQL Server", "Git"},
			[]string{"Brave Browser", "Growtopia", "Roblox", "Weather Zero"}},
		{3, "0A23460005250000712", "Windows 10 Pro 22H2",
			[]string{"Visual Studio Code", "PHP + Xampp", "Android Studio", "Python"},
			[]string{"Roblox"}},
		{4, "0A23460005250030815", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Cisco Packet Tracer", "Wireshark", "Postman", "PHP + Xampp", "Composer", "Unity", "Blender", "Figma", "Python", "Git"},
			[]string{"Roblox", "Grass", "C++", "IDM", "Winrar"}},
		{5, "0A23460005250020411", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Cisco Packet Tracer", "Wireshark", "Postman", "PHP + Xampp", "Composer", "Unity", "Blender", "Android Studio", "Figma", "Python", "Git"},
			[]string{"Brave Browser", "Mendeley", "Point Blank", "Windsurf", "Winrar", "WPS Office", "Opera Browser", "Roblox"}},
		{6, "0A23460005250080913", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Cisco Packet Tracer", "Wireshark", "Postman", "PHP + Xampp", "Composer", "Unity", "Blender", "Android Studio", "Python", "Node.js", "SQL Server"},
			[]string{"Steam", "Marvel Rivals", "Brave Browser", "Growtopia", "Ubisoft Connect", "Unresolved Case", "Rainmeter"}},
		{7, "0A23460005250060909", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Postman", "PHP + Xampp", "Composer", "Unity", "Blender", "Python", "Git"},
			[]string{"Winrar", "Capcut", "OBS Studio"}},
		{8, "0A23460005250050224", "Windows 11 Pro 23H2",
			[]string{"Visual Studio Code", "Cisco Packet Tracer", "Wireshark", "Postman", "PHP + Xampp", "Composer", "Unity", "Blender", "Python", "Git"},
			[]string{"WPS Office"}},
		{9, "0A23170003635900077", "Windows 10 Pro 22H2",
			[]string{"Visual Studio Code", "Postman", "PHP + Xampp", "Composer", "Unity"},
			[]string{"Roblox", "WPS Office"}},
		{10, "0A23470005309001017", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Cisco Packet Tracer", "Blender", "Composer", "PHP + Xampp", "Python", "Unity", "Wireshark", "Postman"},
			[]string{"Google Play Games", "Point Blank", "Roblox", "WPS Office", "Z-Launcher", "Winrar"}},
		{11, "0A23170003635500093", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Blender", "Cisco Packet Tracer", "Composer", "PHP + Xampp", "Figma", "Node.js", "Python", "Unity", "Wireshark"},
			[]string{"Foxit Reader", "Mozilla Firefox", "Opera Browser", "Riot Client", "Roblox", "Valorant", "Webadvisor McAfee", "Winrar"}},
		{12, "0A23470005309015601", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Blender", "Cisco Packet Tracer", "Composer", "PHP + Xampp", "Node.js", "Python", "Postman", "Unity", "Wireshark"},
			[]string{"Gamehouse", "Left 4 Dead 2", "OBS Studio", "Oracle VirtualBox", "Steam", "War Thunder", "Weather Zero", "Webadvisor McAfee", "Winrar", "WPS Office"}},
		{13, "0A23470005309061519", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Blender", "Cisco Packet Tracer", "Composer", "PHP + Xampp", "Figma", "Git", "Postman", "Python", "SQL Server", "Unity", "Wireshark"},
			[]string{"Brave Browser", "Google Chrome", "Lively Wallpaper", "Rainmeter", "Riot Client", "Valorant", "Winrar"}},
		{14, "0A23170003635800177", "Windows 11 Pro 23H2",
			[]string{"Visual Studio Code", "Android Studio", "Blender", "Cisco Packet Tracer", "Composer", "PHP + Xampp", "Figma", "Git", "Node.js", "Python", "Unity", "Wireshark"},
			[]string{"Brave Browser", "Foxit Reader", "Mozilla Firefox", "Google Chrome", "Opera Browser", "Rainmeter", "Webadvisor McAfee", "Winrar"}},
		{15, "0A23170003635400194", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Android Studio", "Blender", "Cisco Packet Tracer", "Composer", "PHP + Xampp", "Postman", "Python", "Unity", "Wireshark"},
			[]string{"Brave Browser", "Foxit Reader", "Google Chrome", "Lively Wallpaper", "Mozilla Firefox", "Rainmeter", "Winrar"}},
		{16, "0A23170003635900317", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "PHP + Xampp", "Python", "Unity"},
			[]string{"Foxit Reader", "Mozilla Firefox", "MSYS2", "OBS Studio", "Roblox"}},
		{17, "0A23470005309011312", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Blender", "Cisco Packet Tracer", "Composer", "Git", "Node.js", "Postman", "Python", "PHP + Xampp", "Unity", "Wireshark"},
			[]string{"Brave Browser", "Github Desktop", "Google Play Games", "Oracle VirtualBox", "R.E.P.O", "Rainmeter", "Riot Client", "Steam", "Valorant", "Roblox", "WPS Office"}},
		{18, "0A23460005250050919", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Android Studio", "Blender", "Cisco Packet Tracer", "Composer", "PHP + Xampp", "Git", "Postman", "Python", "Unity", "Wireshark"},
			[]string{"Custom Cursor", "Garena", "R.E.P.O", "Riot Client", "Roblox", "Steam", "Valorant", "Wallpaper Engine", "WPS Office"}},
		{19, "0A23170003635200205", "Windows 11 Pro 23H2",
			[]string{"Visual Studio Code", "SQL Server", "Blender", "Cisco Packet Tracer", "Composer", "PHP + Xampp", "Postman", "Python", "Unity", "Wireshark"},
			[]string{"Antigravity", "Foxit Reader", "Google Chrome", "Roblox", "Winrar"}},
		{20, "0A23170003635300105", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Android Studio", "Blender", "Cisco Packet Tracer", "PHP + Xampp", "Postman", "Python", "Unity", "Wireshark"},
			[]string{"Foxit Reader", "Genshin Impact", "Google Chrome", "Makehuman", "OBS Studio", "Roblox", "Steam", "Winrar", "Spotify"}},
		{21, "0A23170003635100065", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Blender", "Cisco Packet Tracer", "Composer", "PHP + Xampp", "Git", "Python", "Unity", "Wireshark"},
			[]string{"Alan Wake", "Cheat Engine", "Counter Strike 2", "Firewatch", "Forza Horizon 4", "Foxit Reader", "Google Chrome", "Laragon", "Mozilla Firefox", "MSYS2", "Paladins", "Point Blank", "R.E.P.O", "REDLauncher", "Sleeping Dogs", "Steam", "Stumble Guys", "The Witcher 3", "Thief Simulator", "Winrar", "Z-Launcher"}},
		{23, "0A23170003635000021", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Android Studio", "Blender", "Cisco Packet Tracer", "Composer", "PHP + Xampp", "Git", "Postman", "Python", "Unity", "Wireshark"},
			[]string{"Brave Browser", "Foxit Reader", "Google Chrome", "Growtopia", "Laragon", "Makehuman", "Mozilla Firefox", "Roblox", "Winrar"}},
		{24, "0A23170003635100118", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Cisco Packet Tracer", "Composer", "PHP + Xampp", "Postman", "Python", "Unity"},
			[]string{"Foxit Reader", "Google Chrome", "Laragon", "Mozilla Firefox", "Point Blank", "Riot Client", "Valorant", "Winrar", "Z-Launcher"}},
		{25, "0A23470005309080814", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Android Studio", "Blender", "Cisco Packet Tracer", "Composer", "PHP + Xampp", "Git", "Postman", "Python", "Unity", "Wireshark"},
			[]string{"7-Zip", "Cursor", "Google Chrome", "Phoenix Code", "R.E.P.O", "Steam", "WPS Office"}},
	}

	// Default values
	const (
		defDeviceType     = "PC All-in-one"
		defBrandModel     = "Axioo Mypc One Pro K7-24 (16N9)"
		defProcessor      = "Intel Core i7"
		defRAM            = "16GB DDR4"
		defStorage        = "1TB NVMe"
		defAccessories    = "Keyboard & Mouse Axioo (Wired Set)"
		defStatus         = "normal"
		defCondition      = "baik"
	)

	// Assign row/column based on position in grid (8 columns)
	rowFor := func(n int) int { return ((n - 1) / 8) + 1 }
	colFor := func(n int) int { return ((n - 1) % 8) + 1 }

	for _, pc := range pcs {
		if pc.Number == 22 {
			continue
		}

		_, err := db.Exec(`
			INSERT INTO pcs (pc_number, "row", "column", status, processor, ram, storage,
				serial_number, operating_system, device_type, brand_model, accessories,
				physical_condition, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
			pc.Number, rowFor(pc.Number), colFor(pc.Number),
			defStatus, defProcessor, defRAM, defStorage,
			pc.SN, pc.OS, defDeviceType, defBrandModel, defAccessories, defCondition)
		if err != nil {
			return fmt.Errorf("failed to seed PC-%d: %w", pc.Number, err)
		}

		// Get PC ID
		var pcID int
		err = db.QueryRow(`SELECT id FROM pcs WHERE pc_number = ?`, pc.Number).Scan(&pcID)
		if err != nil {
			return fmt.Errorf("failed to get PC-%d ID: %w", pc.Number, err)
		}

		// Seed required software (installed = true)
		for _, swName := range pc.RequiredSW {
			var swID int
			err := db.QueryRow(`SELECT id FROM software_catalog WHERE name = ?`, swName).Scan(&swID)
			if err != nil {
				continue
			}
			var exists int
			db.QueryRow(`SELECT COUNT(*) FROM pc_software WHERE pc_id = ? AND software_id = ?`, pcID, swID).Scan(&exists)
			if exists == 0 {
				db.Exec(`INSERT INTO pc_software (pc_id, software_id, installed) VALUES (?, ?, TRUE)`, pcID, swID)
			}
		}

		for _, swName := range pc.OtherSW {
			var swID int
			err := db.QueryRow(`SELECT id FROM software_catalog WHERE name = ?`, swName).Scan(&swID)
			if err != nil {
				pgErr := db.QueryRow(`INSERT INTO software_catalog (name, category) VALUES (?, 'other') RETURNING id`, swName).Scan(&swID)
				if pgErr != nil {
					res, execErr := db.Exec(`INSERT INTO software_catalog (name, category) VALUES (?, 'other')`, swName)
					if execErr != nil {
						continue
					}
					lastID, _ := res.LastInsertId()
					swID = int(lastID)
				}
			}
			var exists int
			db.QueryRow(`SELECT COUNT(*) FROM pc_software WHERE pc_id = ? AND software_id = ?`, pcID, swID).Scan(&exists)
			if exists == 0 {
				db.Exec(`INSERT INTO pc_software (pc_id, software_id, installed) VALUES (?, ?, TRUE)`, pcID, swID)
			}
		}
	}

	fmt.Printf("Seeded %d PCs with software data\n", len(pcs))
	return nil
}
