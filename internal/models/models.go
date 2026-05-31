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
	ID              int        `json:"id"`
	PCNumber        int        `json:"pc_number"`        // 1-43
	Row             int        `json:"row"`              // 1-5 (0 for special/cadangan)
	Column          int        `json:"column"`           // 1-8 (0 for special/cadangan)
	Status          string     `json:"status"`           // "normal", "warning", "broken"
	Placement       string     `json:"placement"`        // "dipakai", "cadangan"
	Processor       string     `json:"processor"`
	RAM             string     `json:"ram"`
	Storage         string     `json:"storage"`
	PurchaseDate    *time.Time `json:"purchase_date"`
	Notes           string     `json:"notes"`
	LastChecked     *time.Time `json:"last_checked"`
	AssetID         string     `json:"asset_id"`
	SerialNumber    string     `json:"serial_number"`
	OperatingSystem string     `json:"operating_system"`
	PCType          string     `json:"pc_type"`          // "PC All-in-one", etc
	Label           string     `json:"label"`            // Display label (e.g. "PC-Dosen")
	BrandModel      string     `json:"brand_model"`      // Combined brand + model
	Accessories     string     `json:"accessories"`      // "Keyboard & Mouse Axioo (Wired Set)"
	PhotoSerial     string     `json:"photo_serial"`     // Filename foto S/N + barcode
	PhotoFront      string     `json:"photo_front"`      // Filename foto tampilan depan
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// Category represents a device category/group
type Category struct {
	ID            int       `json:"id"`
	Name          string    `json:"name"`           // TitleCase: "Mouse", "CCTV"
	DefaultPrefix string    `json:"default_prefix"` // UPPERCASE: "MOUSE", "CCTV"
	CreatedAt     time.Time `json:"created_at"`
}

// DeviceType represents a variant/template for a category of devices
type DeviceType struct {
	ID              int       `json:"id"`
	CategoryID      int       `json:"category_id"`
	Name            string    `json:"name"`             // TitleCase — variant: "Axioo"
	Brand           string    `json:"brand"`
	Model           string    `json:"model"`
	AssetCodePrefix string    `json:"asset_code_prefix"` // UPPERCASE UNIQUE: "MOUSE-AXIOO"
	UsageType       string    `json:"usage_type"`        // 'loanable'|'consumable'|'installable'
	DefaultLocation string    `json:"default_location"`
	Photo           string    `json:"photo"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	// Joined fields
	CategoryName string `json:"category_name,omitempty"`
}

// Device represents a unique physical device with its own asset code
type Device struct {
	ID           int        `json:"id"`
	DeviceTypeID int        `json:"device_type_id"`
	AssetCode    string     `json:"asset_code"`     // UNIQUE: "MOUSE-AXIOO-001"
	SerialNumber string     `json:"serial_number"`
	Condition    string     `json:"condition"`      // 'baik'|'rusak'|'maintenance'
	Location     string     `json:"location"`
	PurchaseDate *time.Time `json:"purchase_date"`
	Notes        string     `json:"notes"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	// Joined fields
	CategoryName     string `json:"category_name,omitempty"`
	CategoryPrefix   string `json:"category_prefix,omitempty"`
	DeviceTypeName   string `json:"device_type_name,omitempty"`
	DeviceTypePrefix string `json:"device_type_prefix,omitempty"`
	UsageType        string `json:"usage_type,omitempty"`          // EFFECTIVE: device override > device type
	UsageTypeOverride string `json:"usage_type_override,omitempty"` // raw d.usage_type (empty = inherit from type)
	DeviceTypePhoto  string `json:"device_type_photo,omitempty"`
}

// DeviceLoan represents a device loan/borrowing record
type DeviceLoan struct {
	ID               int        `json:"id"`
	DeviceID         int        `json:"device_id"`
	BorrowerName     string     `json:"borrower_name"`
	BorrowerType     string     `json:"borrower_type"`      // 'dosen'|'mahasiswa'|'staff'|'lainnya'
	LoanDate         time.Time  `json:"loan_date"`
	ReturnDate       time.Time  `json:"return_date"`        // Deadline harus kembali
	ActualReturnDate *time.Time `json:"actual_return_date"` // Diisi saat benar-benar kembali
	Purpose          string     `json:"purpose"`
	Notes            string     `json:"notes"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	// Joined fields
	DeviceAssetCode string `json:"device_asset_code,omitempty"`
	DeviceTypeName  string `json:"device_type_name,omitempty"`
	CategoryName    string `json:"category_name,omitempty"`
	ExtensionCount  int    `json:"extension_count,omitempty"`
}

// LoanExtension tracks return date extension history (backend-only)
type LoanExtension struct {
	ID                 int       `json:"id"`
	LoanID             int       `json:"loan_id"`
	PreviousReturnDate time.Time `json:"previous_return_date"`
	NewReturnDate      time.Time `json:"new_return_date"`
	ExtendedAt         time.Time `json:"extended_at"`
}

// DeviceUsage represents a device usage/consumption record
type DeviceUsage struct {
	ID          int       `json:"id"`
	DeviceID    int       `json:"device_id"`
	UserName    string    `json:"user_name"`
	UserType    string    `json:"user_type"`     // 'dosen'|'mahasiswa'|'staff'|'lainnya'
	UsageDate   time.Time `json:"usage_date"`
	IsAvailable string    `json:"is_available"`  // 'yes' (masih ada) or 'no' (habis)
	Purpose     string    `json:"purpose"`
	Notes       string    `json:"notes"`
	CreatedAt   time.Time `json:"created_at"`
	// Joined fields
	DeviceAssetCode string `json:"device_asset_code,omitempty"`
	DeviceTypeName  string `json:"device_type_name,omitempty"`
	CategoryName    string `json:"category_name,omitempty"`
}

// DeviceInstallation represents where a device is installed
type DeviceInstallation struct {
	ID                    int        `json:"id"`
	DeviceID              int        `json:"device_id"`               // UNIQUE
	LocationInstalled     string     `json:"location_installed"`      // Sentence case
	InstallationStartDate *time.Time `json:"installation_start_date"`
	InstallationFinishDate *time.Time `json:"installation_finish_date"`
	Photo                 string     `json:"photo"`
	Notes                 string     `json:"notes"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
	// Joined fields
	DeviceAssetCode string `json:"device_asset_code,omitempty"`
	DeviceTypeName  string `json:"device_type_name,omitempty"`
	CategoryName    string `json:"category_name,omitempty"`
}

// DeviceStatus represents the real-time computed status for UI display
type DeviceStatus struct {
	DeviceID int
	Status   string // 'available'|'loaned'|'depleted'|'installed'
	Detail   string // Human-readable detail
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

// Grouped view for device list page
type DeviceTypeGroup struct {
	TypeID     int
	TypeName   string
	TypePrefix string
	UsageType  string
	TypePhoto  string
	Devices    []Device
}

type CategoryGroup struct {
	CategoryID     int
	CategoryName   string
	CategoryPrefix string
	Types          []DeviceTypeGroup
}

type DeviceGroupedData struct {
	Categories    []CategoryGroup
	ActiveLoanIDs map[int]bool
	DepletedIDs   map[int]bool
}
