package database

import "fmt"

type pcSeedData struct {
	Number     int
	SN         string
	OS         string
	RequiredSW []string
	OtherSW    []string
	Status     string
	Notes      string
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
			[]string{"Counter Strike Condition Zero", "Roblox", "Photoshop", "Curseforge", "Opera Browser"},
			"", ""},
		{2, "0A23460005250010116", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Postman", "PHP + Xampp", "Blender", "Android Studio", "Figma", "Python", "SQL Server", "Git"},
			[]string{"Brave Browser", "Growtopia", "Roblox", "Weather Zero"},
			"", ""},
		{3, "0A23460005250000712", "Windows 10 Pro 22H2",
			[]string{"Visual Studio Code", "PHP + Xampp", "Android Studio", "Python"},
			[]string{"Roblox"},
			"", ""},
		{4, "0A23460005250030815", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Cisco Packet Tracer", "Wireshark", "Postman", "PHP + Xampp", "Composer", "Unity", "Blender", "Figma", "Python", "Git"},
			[]string{"Roblox", "Grass", "C++", "IDM", "Winrar"},
			"", ""},
		{5, "0A23460005250020411", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Cisco Packet Tracer", "Wireshark", "Postman", "PHP + Xampp", "Composer", "Unity", "Blender", "Android Studio", "Figma", "Python", "Git"},
			[]string{"Brave Browser", "Mendeley", "Point Blank", "Windsurf", "Winrar", "WPS Office", "Opera Browser", "Roblox"},
			"", ""},
		{6, "0A23460005250080913", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Cisco Packet Tracer", "Wireshark", "Postman", "PHP + Xampp", "Composer", "Unity", "Blender", "Android Studio", "Python", "Node.js", "SQL Server"},
			[]string{"Steam", "Marvel Rivals", "Brave Browser", "Growtopia", "Ubisoft Connect", "Unresolved Case", "Rainmeter"},
			"", ""},
		{7, "0A23460005250060909", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Postman", "PHP + Xampp", "Composer", "Unity", "Blender", "Python", "Git"},
			[]string{"Winrar", "Capcut", "OBS Studio"},
			"", ""},
		{8, "0A23460005250050224", "Windows 11 Pro 23H2",
			[]string{"Visual Studio Code", "Cisco Packet Tracer", "Wireshark", "Postman", "PHP + Xampp", "Composer", "Unity", "Blender", "Python", "Git"},
			[]string{"WPS Office"},
			"", ""},
		{9, "0A23170003635900077", "Windows 10 Pro 22H2",
			[]string{"Visual Studio Code", "Postman", "PHP + Xampp", "Composer", "Unity"},
			[]string{"Roblox", "WPS Office"},
			"", ""},
		{10, "0A23470005309001017", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Cisco Packet Tracer", "Blender", "Composer", "PHP + Xampp", "Python", "Unity", "Wireshark", "Postman"},
			[]string{"Google Play Games", "Point Blank", "Roblox", "WPS Office", "Z-Launcher", "Winrar"},
			"", ""},
		{11, "0A23170003635500093", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Blender", "Cisco Packet Tracer", "Composer", "PHP + Xampp", "Figma", "Node.js", "Python", "Unity", "Wireshark"},
			[]string{"Foxit Reader", "Mozilla Firefox", "Opera Browser", "Riot Client", "Roblox", "Valorant", "Webadvisor McAfee", "Winrar"},
			"", ""},
		{12, "0A23470005309015601", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Blender", "Cisco Packet Tracer", "Composer", "PHP + Xampp", "Node.js", "Python", "Postman", "Unity", "Wireshark"},
			[]string{"Gamehouse", "Left 4 Dead 2", "OBS Studio", "Oracle VirtualBox", "Steam", "War Thunder", "Weather Zero", "Webadvisor McAfee", "Winrar", "WPS Office"},
			"", ""},
		{13, "0A23470005309061519", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Blender", "Cisco Packet Tracer", "Composer", "PHP + Xampp", "Figma", "Git", "Postman", "Python", "SQL Server", "Unity", "Wireshark"},
			[]string{"Brave Browser", "Google Chrome", "Lively Wallpaper", "Rainmeter", "Riot Client", "Valorant", "Winrar"},
			"", ""},
		{14, "0A23170003635800177", "Windows 11 Pro 23H2",
			[]string{"Visual Studio Code", "Android Studio", "Blender", "Cisco Packet Tracer", "Composer", "PHP + Xampp", "Figma", "Git", "Node.js", "Python", "Unity", "Wireshark"},
			[]string{"Brave Browser", "Foxit Reader", "Mozilla Firefox", "Google Chrome", "Opera Browser", "Rainmeter", "Webadvisor McAfee", "Winrar"},
			"", ""},
		{15, "0A23170003635400194", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Android Studio", "Blender", "Cisco Packet Tracer", "Composer", "PHP + Xampp", "Postman", "Python", "Unity", "Wireshark"},
			[]string{"Brave Browser", "Foxit Reader", "Google Chrome", "Lively Wallpaper", "Mozilla Firefox", "Rainmeter", "Winrar"},
			"", ""},
		{16, "0A23170003635900317", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "PHP + Xampp", "Python", "Unity"},
			[]string{"Foxit Reader", "Mozilla Firefox", "MSYS2", "OBS Studio", "Roblox"},
			"", ""},
		{17, "0A23470005309011312", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Blender", "Cisco Packet Tracer", "Composer", "Git", "Node.js", "Postman", "Python", "PHP + Xampp", "Unity", "Wireshark"},
			[]string{"Brave Browser", "Github Desktop", "Google Play Games", "Virtual Box Oracle", "R.E.P.O", "Rainmeter", "Riot Client", "Steam", "Valorant", "Roblox", "WPS Office", "Free Fire Max"},
			"", ""},
		{18, "0A23460005250050919", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Android Studio", "Blender", "Cisco Packet Tracer", "Composer", "PHP + Xampp", "Git", "Postman", "Python", "Unity", "Wireshark"},
			[]string{"Custom Cursor", "Garena", "R.E.P.O", "Riot Client", "Roblox", "Steam", "Valorant", "Wallpaper Engine", "WPS Office"},
			"", ""},
		{19, "0A23170003635200205", "Windows 11 Pro 23H2",
			[]string{"Visual Studio Code", "SQL Server", "Blender", "Cisco Packet Tracer", "Composer", "PHP + Xampp", "Postman", "Python", "Unity", "Wireshark"},
			[]string{"Antigravity", "Foxit Reader", "Google Chrome", "Roblox", "Winrar"},
			"", ""},
		{20, "0A23170003635300105", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Android Studio", "Blender", "Cisco Packet Tracer", "PHP + Xampp", "Postman", "Python", "Unity", "Wireshark"},
			[]string{"Foxit Reader", "Genshin Impact", "Google Chrome", "Makehuman", "OBS Studio", "Roblox", "Steam", "Winrar", "Spotify"},
			"", ""},
		{21, "0A23170003635100065", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Blender", "Cisco Packet Tracer", "Composer", "PHP + Xampp", "Git", "Python", "Unity", "Wireshark"},
			[]string{"Alan Wake", "Cheat Engine", "Counter Strike 2", "Firewatch", "Forza Horizon 4", "Foxit Reader", "Google Chrome", "Laragon", "Mozilla Firefox", "MSYS2", "Paladins", "Point Blank", "R.E.P.O", "REDLauncher", "Sleeping Dogs", "Steam", "Stumble Guys", "The Witcher 3", "Thief Simulator", "Winrar", "Z-Launcher"},
			"", ""},
		{22, "0A23170003635600233", "Windows 11 Pro 23H2",
			[]string{"Visual Studio Code", "Blender", "Cisco Packet Tracer", "Git", "Postman", "Python", "Unity", "Wireshark", "PHP + Xampp"},
			[]string{"7zip", "Java", "Brave Browser", "Foxit reader", "Google Chrome", "Internet Download Manager (IDM)", "Laragon", "Makehuman", "Mozilla Firefox", "Osu", "Roblox", "Steam", "VFunLauncher", "Winrar"},
			"", ""},
		{23, "0A23170003635000021", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Android Studio", "Blender", "Cisco Packet Tracer", "Composer", "PHP + Xampp", "Git", "Postman", "Python", "Unity", "Wireshark"},
			[]string{"Brave Browser", "Foxit Reader", "Google Chrome", "Growtopia", "Laragon", "Makehuman", "Mozilla Firefox", "Roblox", "Winrar"},
			"", ""},
		{24, "0A23170003635100118", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Cisco Packet Tracer", "Composer", "PHP + Xampp", "Postman", "Python", "Unity"},
			[]string{"Foxit Reader", "Google Chrome", "Laragon", "Mozilla Firefox", "Point Blank", "Riot Client", "Valorant", "Winrar", "Z-Launcher"},
			"", ""},
		{25, "0A23470005309080814", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Android Studio", "Blender", "Cisco Packet Tracer", "Composer", "PHP + Xampp", "Git", "Postman", "Python", "Unity", "Wireshark"},
			[]string{"7zip", "Cursor", "Google Chrome", "Phoenix Code", "R.E.P.O", "Steam", "WPS Office"},
			"", ""},
		{26, "0A23460005250000126", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Android Studio", "Blender", "SQL Server", "Cisco Packet Tracer", "Postman", "Python", "Unity", "Wireshark", "PHP + Xampp"},
			[]string{"Bandicam", "Bluestacks", "Chatgpt", "Genymotion", "Google Chrome", "Google Play", "MEmu", "Need For Speed", "Oracle VM Virtual Box", "Riot Client", "Roblox", "Steam", "Tiktok Live Studio", "Whatsapp", "Winrar"},
			"", ""},
		{27, "0A23460005250030120", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Android Studio", "Cisco Packet Tracer", "SQL Server", "Composer", "PHP + Xampp", "Git", "Node.js", "Postman", "Python", "Unity"},
			[]string{"Bandicam", "Epic Online Services", "Google Chrome", "Riot Client", "Roblox", "Steam", "Spotify", "Winrar"},
			"", ""},
		{28, "0A23170003635800033", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Android Studio", "Blender", "Cisco Packet Tracer", "Composer", "PHP + Xampp", "Figma", "Postman", "Python", "Unity", "Wireshark"},
			[]string{"BlueStacks", "Foxit reader", "Google Chrome", "Krita", "Mozilla Firefox", "OpenAL", "Roblox", "ShootersPool", "Steam", "Winrar"},
			"", ""},
		{29, "0A23460005250090317", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Cisco Packet Tracer", "Composer", "PHP + Xampp", "Postman", "Python", "Unity", "Wireshark"},
			[]string{"Brave Browser", "CPUID", "Delta Force", "Fast Node Manager", "Growtopia", "Rainmeter", "Roblox", "StarUML", "Tiktok Live Studio", "Winrar", "WPS Office"},
			"", ""},
		{30, "0A23170003635600046", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Android Studio", "Blender", "Cisco Packet Tracer", "Git", "Postman", "Python", "Unity", "Wireshark", "PHP + Xampp"},
			[]string{"Foxit reader", "Google Chrome", "Google Play", "Mozilla Firefox", "Rainmeter", "Roblox", "Spotify", "Winrar"},
			"", "Mouse DPI nya cepet dan kadang ngebug ngeblink² gitu gerakan nya"},
		{31, "0A23460005250070618", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Blender", "Cisco Packet Tracer", "Composer", "PHP + Xampp", "Postman", "Python", "Unity", "Wireshark"},
			[]string{"Bandicam", "Brave Browser", "Counter Strike Condition 0", "Denuvo anti cheat", "Fears to Fathom Episode 1", "Fears to Fathom Ironbark Lookout", "Growtopia", "Google Chrome", "Java", "Makehuman", "Internet Download Manager (IDM)", "Riot Client", "Roblox", "Steam", "Sleeping Dogs", "Thief Simulator", "Valorant", "Winrar", "VRoid Studio"},
			"", ""},
		{32, "0A23170003635700080", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Android Studio", "Blender", "Composer", "PHP + Xampp", "Node.js", "Postman", "Python", "Unity"},
			[]string{"Banana Hell: Mountain of Madness (Game)", "Foxit reader", "Google Chrome", "Overcooked 2", "Roblox", "Steam", "Winrar"},
			"", ""},
		{33, "", "", nil, nil, "", "Belum dicatat"},
		{34, "0A23470005309081218", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Android Studio", "Blender", "Cisco Packet Tracer", "Figma", "Git", "SQL Server", "Postman", "Python", "Unity", "Wireshark", "PHP + Xampp"},
			[]string{"Google Chrome", "Internet Download Manager (IDM)", "MLWapp", "Opera Browser", "Roblox", "Samsung Dex", "TestQCWin-RMA", "Winrar", "WPS Office", "Lively Wallpaper"},
			"", ""},
		{35, "0A23170003635600180", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Android Studio", "Blender", "Cisco Packet Tracer", "Composer", "PHP + Xampp", "Figma", "Git", "Postman", "Python", "Unity", "Wireshark"},
			[]string{"7zip", "Google Chrome", "Mozilla Firefox", "Opera Browser", "Phoenix Code", "SPSS", "WebAdvisor McAfee", "Winrar"},
			"warning", "ada bulatan hitam di sisi kanan atas monitor"},
		{36, "0A23460005250010421", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Android Studio", "Blender", "Cisco Packet Tracer", "Composer", "PHP + Xampp", "SQL Server", "Git", "Postman", "Python", "Unity", "Wireshark"},
			[]string{"7zip", "Bluestacks", "Google Chrome", "Google Play", "MySQL", "Roblox", "Steam", "Unsolved Case", "Winrar", "WPS Office"},
			"warning", "Ada 2 bulatan hitam di sisi kanan dan kiri atas monitor"},
		{37, "0A23170003635100305", "Windows 11 Pro 25H2",
			[]string{"Visual Studio Code", "Android Studio", "Blender", "Cisco Packet Tracer", "Composer", "PHP + Xampp", "Git", "Postman", "Python", "Unity", "Wireshark"},
			[]string{"Brave browser", "Eclipse Temurin JDK", "Epic Games Launcher", "Foxit reader", "GameLoop", "Google Chrome", "Google Play", "Java", "OBS Studio", "Roblox", "Webadvisor McAfee", "Winrar"},
			"warning", "ada 3 bulatan hitam, 1 di sisi kanan atas, 2 di sisi kiri atas"},
		{38, "0A23170003635500280", "", nil, nil, "broken", "PC Black Screen, tidak bisa load ke Windows, dan saat dimatikan lewat tombol di kanan monitor, PC malah loop nyala lagi"},
		{39, "0A23470005309041415", "Windows 10 Pro 22H2",
			[]string{"Visual Studio Code", "Android Studio", "Blender", "Cisco Packet Tracer", "Composer", "PHP + Xampp", "Figma", "Git", "Node.js", "Python", "Unity", "Wireshark"},
			[]string{"7zip", "Counter strike condition 0", "Discord", "Docker Desktop", "Google Chrome", "Internet Download Manager (IDM)", "Riot Client", "Roblox", "SPSS", "Stremio", "Valorant", "Windsurf", "Winrar"},
			"broken", "Layar retak dalam, retaknya hampir setengah layar"},
		{40, "0A23190003722018185", "", nil, nil, "broken", "PC Black Screen, tidak bisa load ke Windows, kalau yg ini tidak looping nyala lagi saat ditekan tombol power di kanan monitornya. Juga bagian laci keyboard susah dibuka (sepertinya agak stuck)"},
	}

	// Default values
	const (
		defDeviceType  = "PC All-in-one"
		defBrandModel  = "Axioo Mypc One Pro K7-24 (16N9)"
		defProcessor   = "Intel Core i7"
		defRAM         = "16GB DDR4"
		defStorage     = "1TB NVMe"
		defAccessories = "Keyboard & Mouse Axioo (Wired Set)"
		defStatus      = "normal"
		defCondition   = "baik"
	)

	rowFor := func(n int) int { return ((n - 1) / 8) + 1 }
	colFor := func(n int) int { return ((n - 1) % 8) + 1 }

	// Pre-resolve all software IDs from catalog
	swByName := map[string]int{}
	catalogRows, _ := db.Query(`SELECT id, name FROM software_catalog`)
	if catalogRows != nil {
		defer catalogRows.Close()
		for catalogRows.Next() {
			var id int; var name string
			catalogRows.Scan(&id, &name)
			swByName[name] = id
		}
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback()

	insertSW := func(tx *Tx, pcID int, names []string, skipMissing bool) {
		for _, name := range names {
			swID, ok := swByName[name]
			if !ok {
				if skipMissing { continue }
				pgErr := tx.QueryRow(`INSERT INTO software_catalog (name, category, description) VALUES (?, 'other', '') RETURNING id`, name).Scan(&swID)
				if pgErr != nil {
					tx.Exec(`INSERT INTO software_catalog (name, category, description) VALUES (?, 'other', '')`, name)
					tx.QueryRow(`SELECT id FROM software_catalog WHERE name = ?`, name).Scan(&swID)
				}
				if swID > 0 { swByName[name] = swID }
			}
			if swID > 0 {
				var exists int
				tx.QueryRow(`SELECT COUNT(*) FROM pc_software WHERE pc_id = ? AND software_id = ?`, pcID, swID).Scan(&exists)
				if exists == 0 {
					tx.Exec(`INSERT INTO pc_software (pc_id, software_id, installed) VALUES (?, ?, TRUE)`, pcID, swID)
				}
			}
		}
	}

	for _, pc := range pcs {
		pcStatus := pc.Status
		if pcStatus == "" {
			pcStatus = defStatus
		}

		_, execErr := tx.Exec(`INSERT INTO pcs (pc_number, "row", "column", status, processor, ram, storage,
			serial_number, operating_system, device_type, brand_model, accessories,
			physical_condition, notes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
			pc.Number, rowFor(pc.Number), colFor(pc.Number),
			pcStatus, defProcessor, defRAM, defStorage,
			pc.SN, pc.OS, defDeviceType, defBrandModel, defAccessories, defCondition, pc.Notes)
		if execErr != nil {
			tx.Rollback()
			return fmt.Errorf("failed to seed PC-%d: %w", pc.Number, execErr)
		}

		var pcID int
		tx.QueryRow(`SELECT id FROM pcs WHERE pc_number = ?`, pc.Number).Scan(&pcID)
		if pcID == 0 { continue }

		insertSW(tx, pcID, pc.RequiredSW, true)
		insertSW(tx, pcID, pc.OtherSW, false)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	fmt.Printf("Seeded %d PCs with software data\n", len(pcs))
	return nil
}
