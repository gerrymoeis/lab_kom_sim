package handlers

type LoginRequest struct {
	Username string `form:"username" binding:"required,max=50"`
	Password string `form:"password" binding:"required,max=255"`
}

type CreatePCRequest struct {
	Label           string `form:"label" binding:"omitempty,max=50"`
	Row             int    `form:"row"`
	Column          int    `form:"column"`
	Status          string `form:"status" binding:"omitempty,oneof=normal warning broken"`
	Placement       string `form:"placement" binding:"omitempty,oneof=dipakai cadangan"`
	IsMahasiswa     string `form:"is_mahasiswa"`
	SerialNumber    string `form:"serial_number" binding:"required,max=100"`
	OperatingSystem string `form:"operating_system" binding:"required,max=100"`
	PCType          string `form:"pc_type" binding:"omitempty,max=50"`
	BrandModel      string `form:"brand_model" binding:"omitempty,max=100"`
	Accessories     string `form:"accessories" binding:"omitempty,max=200"`
	Processor       string `form:"processor" binding:"omitempty,max=100"`
	RAM             string `form:"ram" binding:"omitempty,max=50"`
	Storage         string `form:"storage" binding:"omitempty,max=50"`
	SerialFileRef   string `form:"serial_file_ref"`
	FrontFileRef    string `form:"front_file_ref"`
	PurchaseDate    string `form:"purchase_date"`
	LastChecked     string `form:"last_checked"`
	Notes           string `form:"notes" binding:"omitempty,max=500"`
}

type EditPCRequest struct {
	Status          string   `form:"status" binding:"omitempty,oneof=normal warning broken"`
	Placement       string   `form:"placement" binding:"omitempty,oneof=dipakai cadangan"`
	SerialNumber    string   `form:"serial_number" binding:"required,max=100"`
	OperatingSystem string   `form:"operating_system" binding:"required,max=100"`
	PCType          string   `form:"pc_type" binding:"omitempty,max=50"`
	BrandModel      string   `form:"brand_model" binding:"omitempty,max=100"`
	Accessories     string   `form:"accessories" binding:"omitempty,max=200"`
	Processor       string   `form:"processor" binding:"omitempty,max=100"`
	RAM             string   `form:"ram" binding:"omitempty,max=50"`
	Storage         string   `form:"storage" binding:"omitempty,max=50"`
	Notes           string   `form:"notes" binding:"omitempty,max=500"`
	Label           string   `form:"label" binding:"omitempty,max=50"`
	Row             int      `form:"row"`
	Column          int      `form:"column"`
	PurchaseDate    string   `form:"purchase_date"`
	LastChecked     string   `form:"last_checked"`
	SerialFileRef   string   `form:"serial_file_ref"`
	FrontFileRef    string   `form:"front_file_ref"`
	RequiredSw      []string `form:"required_sw[]"`
	OtherName       []string `form:"other_name[]"`
	OtherDesc       []string `form:"other_desc[]"`
}

type CreateDeviceRequest struct {
	DeviceTypeID int    `form:"device_type_id" binding:"required"`
	SerialNumber string `form:"serial_number" binding:"omitempty,max=100"`
	Condition    string `form:"condition" binding:"omitempty,oneof=normal warning rusak"`
	Location     string `form:"location" binding:"omitempty,max=100"`
	PurchaseDate string `form:"purchase_date"`
	Notes        string `form:"notes" binding:"omitempty,max=500"`
}

type BatchDeviceItemRequest struct {
	SerialNumber string `json:"serial_number" binding:"omitempty,max=100"`
	Condition    string `json:"condition" binding:"omitempty,oneof=normal warning rusak"`
	Location     string `json:"location" binding:"omitempty,max=100"`
	PurchaseDate string `json:"purchase_date"`
	Notes        string `json:"notes" binding:"omitempty,max=500"`
}

type BatchCreateDeviceRequest struct {
	// Existing IDs (0 = create new inline)
	CategoryID   int `json:"category_id"`
	DeviceTypeID int `json:"device_type_id"`

	// Inline category creation (used when CategoryID == 0)
	NewCategoryName   string `json:"new_category_name" binding:"omitempty,max=100"`
	NewCategoryPrefix string `json:"new_category_prefix" binding:"omitempty,max=50"`

	// Inline device type creation (used when DeviceTypeID == 0)
	NewTypeName            string `json:"new_type_name" binding:"omitempty,max=100"`
	NewTypeBrand           string `json:"new_type_brand" binding:"omitempty,max=100"`
	NewTypeModel           string `json:"new_type_model" binding:"omitempty,max=100"`
	NewTypeAssetCodePrefix string `json:"new_type_asset_code_prefix" binding:"omitempty,max=50"`
	NewTypeUsageType       string `json:"new_type_usage_type" binding:"omitempty,oneof=loanable consumable installable"`
	NewTypeDefaultLocation string `json:"new_type_default_location" binding:"omitempty,max=100"`
	NewTypePhotoFileRef    string `json:"new_type_photo_file_ref"`

	Devices []BatchDeviceItemRequest `json:"devices" binding:"required,min=1,dive"`
}

type EditDeviceRequest struct {
	DeviceTypeID int    `form:"device_type_id" binding:"required"`
	AssetCode    string `form:"asset_code"`
	SerialNumber string `form:"serial_number" binding:"omitempty,max=100"`
	Condition    string `form:"condition" binding:"omitempty,oneof=normal warning rusak"`
	Location     string `form:"location" binding:"omitempty,max=100"`
	PurchaseDate string `form:"purchase_date"`
	Notes        string `form:"notes" binding:"omitempty,max=500"`
	UsageType    string `form:"usage_type" binding:"omitempty,oneof=loanable consumable installable"`
}

type CreateDeviceTypeRequest struct {
	CategoryID      int    `form:"category_id" binding:"required"`
	Name            string `form:"name" binding:"required,max=100"`
	Brand           string `form:"brand" binding:"omitempty,max=100"`
	Model           string `form:"model" binding:"omitempty,max=100"`
	AssetCodePrefix string `form:"asset_code_prefix" binding:"omitempty,max=50"`
	UsageType       string `form:"usage_type" binding:"required,oneof=loanable consumable installable"`
	DefaultLocation string `form:"default_location" binding:"omitempty,max=100"`
}

type EditDeviceTypeRequest struct {
	CategoryID      int    `form:"category_id" binding:"required"`
	Name            string `form:"name" binding:"required,max=100"`
	Brand           string `form:"brand" binding:"omitempty,max=100"`
	Model           string `form:"model" binding:"omitempty,max=100"`
	AssetCodePrefix string `form:"asset_code_prefix" binding:"omitempty,max=50"`
	UsageType       string `form:"usage_type" binding:"required,oneof=loanable consumable installable"`
	DefaultLocation string `form:"default_location" binding:"omitempty,max=100"`
	PhotoFileRef    string `form:"photo_file_ref"`
}

type EditCategoryRequest struct {
	Name          string `form:"name" binding:"required,max=100"`
	DefaultPrefix string `form:"default_prefix" binding:"required,max=50"`
}

type CreateDeviceLoanRequest struct {
	DeviceID     string `form:"device_id" binding:"required"`
	BorrowerName string `form:"borrower_name" binding:"required,max=100"`
	BorrowerType string `form:"borrower_type" binding:"omitempty,oneof=dosen mahasiswa staff lainnya"`
	LoanDate     string `form:"loan_date" binding:"required"`
	ReturnDate   string `form:"return_date" binding:"required"`
	Purpose      string `form:"purpose" binding:"omitempty,max=500"`
}

type EditDeviceLoanRequest struct {
	BorrowerName     string `form:"borrower_name" binding:"required,max=100"`
	BorrowerType     string `form:"borrower_type" binding:"omitempty,oneof=dosen mahasiswa staff lainnya"`
	LoanDate         string `form:"loan_date" binding:"required"`
	Purpose          string `form:"purpose" binding:"omitempty,max=500"`
	Status           string `form:"status" binding:"required"`
	ActualReturnDate string `form:"actual_return_date"`
	Notes            string `form:"notes" binding:"omitempty,max=500"`
}

type CreateDeviceUsageRequest struct {
	DeviceID    string `form:"device_id" binding:"required"`
	UserName    string `form:"user_name" binding:"required,max=100"`
	UserType    string `form:"user_type" binding:"omitempty,oneof=dosen mahasiswa staff lainnya"`
	UsageDate   string `form:"usage_date" binding:"required"`
	IsAvailable string `form:"is_available" binding:"omitempty,oneof=yes no"`
	Purpose     string `form:"purpose" binding:"omitempty,max=500"`
}

type EditDeviceUsageRequest struct {
	UserName    string `form:"user_name" binding:"omitempty,max=100"`
	UserType    string `form:"user_type" binding:"omitempty,oneof=dosen mahasiswa staff lainnya"`
	UsageDate   string `form:"usage_date"`
	IsAvailable string `form:"is_available" binding:"omitempty,oneof=yes no"`
	Purpose     string `form:"purpose" binding:"omitempty,max=500"`
	Notes       string `form:"notes" binding:"omitempty,max=500"`
}

type CreateInstallationRequest struct {
	DeviceID               string `form:"device_id" binding:"required"`
	LocationInstalled      string `form:"location_installed" binding:"required,max=100"`
	InstallationStartDate  string `form:"installation_start_date"`
	InstallationFinishDate string `form:"installation_finish_date"`
	PhotoFileRef           string `form:"photo_file_ref"`
	Notes                  string `form:"notes" binding:"omitempty,max=500"`
}

type EditInstallationRequest struct {
	LocationInstalled      string `form:"location_installed" binding:"required,max=100"`
	InstallationStartDate  string `form:"installation_start_date"`
	InstallationFinishDate string `form:"installation_finish_date"`
	PhotoFileRef           string `form:"photo_file_ref"`
	Notes                  string `form:"notes" binding:"omitempty,max=500"`
}

type CreateLogbookRequest struct {
	Date        string `form:"date" binding:"required"`
	StudentName string `form:"student_name" binding:"required,max=100"`
	NIM         string `form:"nim" binding:"required,len=11"`
	TimeIn      string `form:"time_in" binding:"required,max=5"`
	TimeOut     string `form:"time_out" binding:"omitempty,max=5"`
	Purpose     string `form:"purpose" binding:"omitempty,max=500"`
}

type EditLogbookRequest struct {
	Date        string `form:"date" binding:"required"`
	StudentName string `form:"student_name" binding:"required,max=100"`
	NIM         string `form:"nim" binding:"required,len=11"`
	TimeIn      string `form:"time_in" binding:"required,max=5"`
	TimeOut     string `form:"time_out" binding:"omitempty,max=5"`
	Purpose     string `form:"purpose" binding:"omitempty,max=500"`
}

type CreateScheduleRequest struct {
	CourseName string `form:"course_name" binding:"required,max=100"`
	Lecturer   string `form:"lecturer" binding:"required,max=100"`
	Day        string `form:"day" binding:"required,max=10"`
	Class      string `form:"class" binding:"required,max=50"`
	TimeStart  string `form:"time_start" binding:"required,max=5"`
	TimeEnd    string `form:"time_end" binding:"required,max=5"`
	Notes      string `form:"notes" binding:"omitempty,max=500"`
}

type EditScheduleRequest struct {
	CourseName string `form:"course_name" binding:"required,max=100"`
	Lecturer   string `form:"lecturer" binding:"required,max=100"`
	Day        string `form:"day" binding:"required,max=10"`
	Class      string `form:"class" binding:"required,max=50"`
	TimeStart  string `form:"time_start" binding:"required,max=5"`
	TimeEnd    string `form:"time_end" binding:"required,max=5"`
	Notes      string `form:"notes" binding:"omitempty,max=500"`
}

type CreateSoftwareRequest struct {
	Name        string `form:"name" binding:"required,max=100"`
	Category    string `form:"category" binding:"omitempty,oneof=required other"`
	Description string `form:"description" binding:"omitempty,max=500"`
}

type CreateUserRequest struct {
	Username string `form:"username" binding:"required,max=50"`
	Password string `form:"password" binding:"required,max=255"`
	FullName string `form:"full_name" binding:"required,max=100"`
	Role     string `form:"role" binding:"required,oneof=admin dosen"`
}

type UpdateUserRequest struct {
	Username    string `form:"username" binding:"required,max=50"`
	FullName    string `form:"full_name" binding:"required,max=100"`
	Role        string `form:"role" binding:"required,oneof=admin dosen"`
	NewPassword string `form:"new_password" binding:"omitempty,max=255"`
}

type UpdateProfileRequest struct {
	Username string `form:"username" binding:"required,max=50"`
	FullName string `form:"full_name" binding:"required,max=100"`
}

type ChangePasswordRequest struct {
	OldPassword     string `form:"old_password" binding:"required,max=255"`
	NewPassword     string `form:"new_password" binding:"required,max=255"`
	ConfirmPassword string `form:"confirm_password" binding:"required,max=255"`
}

type LogbookSaveRequest struct {
	SourceFile  string   `form:"source_file"`
	Date        []string `form:"date[]"`
	TimeIn      []string `form:"time_in[]"`
	TimeOut     []string `form:"time_out[]"`
	StudentName []string `form:"student_name[]"`
	NIM         []string `form:"nim[]"`
	Purpose     []string `form:"purpose[]"`
	Verified    []string `form:"verified[]"`
}

type UploadImageRequest struct {
	Type  string `form:"type" binding:"omitempty,oneof=serial front device_type installation logbook"`
	Label string `form:"label" binding:"omitempty,max=50"`
}
