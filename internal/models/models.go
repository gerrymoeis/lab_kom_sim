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
	Label             string     `json:"label"`              // Display label (e.g. "PC-Dosen")
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
	ID          int       `json:"id"`
	DeviceID    int       `json:"device_id"`
	UserName    string    `json:"user_name"`
	UserType    string    `json:"user_type"`     // "dosen", "mahasiswa", "staff", "lainnya"
	UsageDate   time.Time `json:"usage_date"`
	Quantity    int       `json:"quantity"`
	IsAvailable string    `json:"is_available"` // "yes" (masih ada) or "no" (habis)
	Purpose     string    `json:"purpose"`
	Notes       string    `json:"notes"`
	CreatedAt   time.Time `json:"created_at"`
	// Display fields (not in database)
	DeviceAssetCode string `json:"device_asset_code,omitempty"`
	DeviceName      string `json:"device_name,omitempty"`
}


// LostItem represents a lost item report
type LostItem struct {
	ID               int        `json:"id"`
	DeviceID         *int       `json:"device_id"`
	ItemName         string     `json:"item_name"`
	ItemDescription  string     `json:"item_description"`
	ReportedBy       string     `json:"reported_by"`
	ReportedDate     time.Time  `json:"reported_date"`
	LastSeenAt       *time.Time `json:"last_seen_at"`
	LocationLastSeen string     `json:"location_last_seen"`
	Status           string     `json:"status"`
	OwnerName        string     `json:"owner_name"`
	OwnerClass       string     `json:"owner_class"`
	OwnerNim         string     `json:"owner_nim"`
	ReturnedDate     *time.Time `json:"returned_date"`
	Photo            string     `json:"photo"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// SoftwareCatalog represents a software entry in the master catalog
type SoftwareCatalog struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`     // UNIQUE
	Category    string    `json:"category"` // "required" or "other"
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// PCSoftware represents the many-to-many relation between PCs and software
type PCSoftware struct {
	PCID       int    `json:"pc_id"`
	SoftwareID int    `json:"software_id"`
	Installed  bool   `json:"installed"`
	// Joined fields (not in DB)
	SoftwareName string `json:"software_name,omitempty"`
	Category     string `json:"category,omitempty"`
	Description  string `json:"description,omitempty"`
}

// CourseSchedule represents a course schedule in the lab
type CourseSchedule struct {
	ID         int       `json:"id"`
	CourseName string    `json:"course_name"`
	Lecturer   string    `json:"lecturer"`
	Day        string    `json:"day"`
	Class      string    `json:"class"`
	TimeStart  string    `json:"time_start"`
	TimeEnd    string    `json:"time_end"`
	Notes      string    `json:"notes"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
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
