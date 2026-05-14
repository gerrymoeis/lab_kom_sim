package models

import "time"

// User represents a system user
type User struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	Password  string    `json:"-"` // Never expose password in JSON
	FullName  string    `json:"full_name"`
	Role      string    `json:"role"` // "admin" or "dosen"
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PC represents a computer in the lab
type PC struct {
	ID                int        `json:"id"`
	PCNumber          int        `json:"pc_number"`      // 1-40
	Row               int        `json:"row"`            // 1-5
	Column            int        `json:"column"`         // 1-8
	Status            string     `json:"status"`         // "normal", "warning", "broken", "inactive"
	Processor         string     `json:"processor"`
	RAM               string     `json:"ram"`
	Storage           string     `json:"storage"`
	PurchaseDate      *time.Time `json:"purchase_date"`
	Notes             string     `json:"notes"`
	LastChecked       *time.Time `json:"last_checked"`
	// Asset management fields
	AssetID           string     `json:"asset_id"`
	SerialNumber      string     `json:"serial_number"`
	Brand             string     `json:"brand"`             // Deprecated, use BrandModel
	Model             string     `json:"model"`             // Deprecated, use BrandModel
	OperatingSystem   string     `json:"operating_system"`
	PhysicalCondition string     `json:"physical_condition"` // "baik", "cukup", "rusak"
	// New fields for PC refinement
	DeviceType        string     `json:"device_type"`       // "PC All-in-one", etc
	BrandModel        string     `json:"brand_model"`       // Combined brand + model
	Accessories       string     `json:"accessories"`       // "Keyboard & Mouse Axioo (Wired Set)"
	ActionNotes       string     `json:"action_notes"`      // Catatan tindakan perbaikan
	PhotoSerial       string     `json:"photo_serial"`      // Filename foto S/N + barcode
	PhotoFront        string     `json:"photo_front"`       // Filename foto tampilan depan
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// DeviceType represents a template/preset for device types
type DeviceType struct {
	ID               int       `json:"id"`
	Name             string    `json:"name"`
	Category         string    `json:"category"`
	Brand            string    `json:"brand"`
	Model            string    `json:"model"`
	ItemType         string    `json:"item_type"`          // "individual" or "consumable"
	IsLoanable       bool      `json:"is_loanable"`        // Can be borrowed?
	IsConsumable     bool      `json:"is_consumable"`      // Can be consumed (habis pakai)?
	AssetCodePrefix  string    `json:"asset_code_prefix"`  // "PENTAB", "SWITCH-RJ", etc
	DefaultLocation  string    `json:"default_location"`
	NotesTemplate    string    `json:"notes_template"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// Device represents other devices in the lab (UPDATED - Category removed, DeviceTypeID required)
type Device struct {
	ID                int        `json:"id"`
	DeviceTypeID      int        `json:"device_type_id"`      // Reference to DeviceType (REQUIRED)
	AssetCode         string     `json:"asset_code"`          // Unique code: "PENTAB-001"
	Name              string     `json:"name"`
	Brand             string     `json:"brand"`
	Model             string     `json:"model"`
	SerialNumber      string     `json:"serial_number"`
	ItemType          string     `json:"item_type"`           // "individual" or "consumable"
	IsLoanable        bool       `json:"is_loanable"`         // Can be borrowed?
	IsConsumable      bool       `json:"is_consumable"`       // Can be consumed?
	QuantityTotal     int        `json:"quantity_total"`      // Total quantity (for consumables)
	QuantityAvailable int        `json:"quantity_available"`  // Available quantity
	Condition         string     `json:"condition"`           // "baik", "rusak", "maintenance"
	Location          string     `json:"location"`
	PurchaseDate      *time.Time `json:"purchase_date"`
	Notes             string     `json:"notes"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// DeviceWithCategory represents Device with Category from DeviceType (for queries with JOIN)
type DeviceWithCategory struct {
	Device
	Category string `json:"category"` // From device_types table
}

// DeviceLoan represents a device loan/borrowing record
type DeviceLoan struct {
	ID                 int        `json:"id"`
	DeviceID           int        `json:"device_id"`
	BorrowerName       string     `json:"borrower_name"`
	BorrowerType       string     `json:"borrower_type"`       // "dosen", "mahasiswa", "staff", "lainnya"
	LoanDate           time.Time  `json:"loan_date"`
	ExpectedReturnDate *time.Time `json:"expected_return_date"`
	ActualReturnDate   *time.Time `json:"actual_return_date"`
	Quantity           int        `json:"quantity"`
	Status             string     `json:"status"`              // "active", "returned", "overdue"
	Purpose            string     `json:"purpose"`
	Notes              string     `json:"notes"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	// Display fields (not in database)
	DeviceAssetCode    string     `json:"device_asset_code,omitempty"`
	DeviceName         string     `json:"device_name,omitempty"`
	ComputedStatus     string     `json:"computed_status,omitempty"` // Real-time computed status
}

// DeviceUsage represents a device usage/consumption record
type DeviceUsage struct {
	ID        int       `json:"id"`
	DeviceID  int       `json:"device_id"`
	UserName  string    `json:"user_name"`
	UserType  string    `json:"user_type"`  // "dosen", "mahasiswa", "staff", "lainnya"
	UsageDate time.Time `json:"usage_date"`
	Quantity  int       `json:"quantity"`
	Purpose   string    `json:"purpose"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
	// Display fields (not in database)
	DeviceAssetCode string `json:"device_asset_code,omitempty"`
	DeviceName      string `json:"device_name,omitempty"`
}


// Software represents software installed on PCs
type Software struct {
	ID          int       `json:"id"`
	PCID        int       `json:"pc_id"`
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	License     string    `json:"license"`
	Category    string    `json:"category"`     // "required" (lab-installed) or "other" (student-installed)
	InstallDate *time.Time `json:"install_date"`
	Notes       string    `json:"notes"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// LogbookEntry represents an attendance logbook entry
type LogbookEntry struct {
	ID         int       `json:"id"`
	Date       time.Time `json:"date"`
	StudentName string   `json:"student_name"`
	NIM        string    `json:"nim"`
	TimeIn     string    `json:"time_in"`
	TimeOut    string    `json:"time_out"`
	Purpose    string    `json:"purpose"` // Changed from Notes to Purpose (keperluan)
	SourceFile string    `json:"source_file"` // Original uploaded file
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// MaintenanceLog represents maintenance history for PCs
type MaintenanceLog struct {
	ID          int       `json:"id"`
	PCID        int       `json:"pc_id"`
	Date        time.Time `json:"date"`
	Type        string    `json:"type"` // "repair", "upgrade", "cleaning", "check"
	Description string    `json:"description"`
	Technician  string    `json:"technician"`
	Cost        float64   `json:"cost"`
	CreatedAt   time.Time `json:"created_at"`
}

// StatusCount represents count of PCs by status
type StatusCount struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

// ActivityLog represents an audit trail entry
type ActivityLog struct {
	ID           int       `json:"id"`
	UserID       int       `json:"user_id"`
	Username     string    `json:"username"`
	UserRole     string    `json:"user_role"`
	Action       string    `json:"action"`        // "create", "update", "delete", "upload", "login", "logout"
	EntityType   string    `json:"entity_type"`   // "pc", "device", "software", "logbook", "user", "auth"
	EntityID     *int      `json:"entity_id"`     // Nullable for bulk operations or auth events
	Description  string    `json:"description"`   // Human-readable description
	OldValues    string    `json:"old_values"`    // JSON string of old values (for update/delete)
	NewValues    string    `json:"new_values"`    // JSON string of new values (for create/update)
	CreatedAt    time.Time `json:"created_at"`
	IPAddress    string    `json:"ip_address"`
	UserAgent    string    `json:"user_agent"`
	Status       string    `json:"status"`        // "success", "failed", "error"
	ErrorMessage string    `json:"error_message"` // If status = "failed"
}
