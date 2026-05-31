package handlers

type LoginRequest struct {
	Username string `form:"username" binding:"required"`
	Password string `form:"password" binding:"required"`
}

type CreatePCRequest struct {
	PCNumber        int    `form:"pc_number" binding:"required,min=1,max=43"`
	Label           string `form:"label"`
	Row             int    `form:"row"`
	Column          int    `form:"column"`
	Status          string `form:"status"`
	Placement       string `form:"placement"`
	SerialNumber    string `form:"serial_number" binding:"required"`
	OperatingSystem string `form:"operating_system" binding:"required"`
	PCType          string `form:"pc_type"`
	BrandModel      string `form:"brand_model"`
	Accessories     string `form:"accessories"`
	Processor       string `form:"processor"`
	RAM             string `form:"ram"`
	Storage         string `form:"storage"`
	SerialFileRef   string `form:"serial_file_ref"`
	FrontFileRef    string `form:"front_file_ref"`
}

type EditPCRequest struct {
	Status          string   `form:"status"`
	Placement       string   `form:"placement"`
	SerialNumber    string   `form:"serial_number" binding:"required"`
	OperatingSystem string   `form:"operating_system" binding:"required"`
	PCType          string   `form:"pc_type"`
	BrandModel      string   `form:"brand_model"`
	Accessories     string   `form:"accessories"`
	Processor       string   `form:"processor"`
	RAM             string   `form:"ram"`
	Storage         string   `form:"storage"`
	Notes           string   `form:"notes"`
	Label           string   `form:"label"`
	SerialFileRef   string   `form:"serial_file_ref"`
	FrontFileRef    string   `form:"front_file_ref"`
	RequiredSw      []string `form:"required_sw"`
	OtherName       []string `form:"other_name"`
	OtherDesc       []string `form:"other_desc"`
}

type CreateDeviceRequest struct {
	DeviceTypeID int    `form:"device_type_id" binding:"required"`
	SerialNumber string `form:"serial_number"`
	Condition    string `form:"condition"`
	Location     string `form:"location"`
	PurchaseDate string `form:"purchase_date"`
	Notes        string `form:"notes"`
}

type BatchDeviceItemRequest struct {
	SerialNumber string `json:"serial_number"`
	Condition    string `json:"condition"`
	Location     string `json:"location"`
	PurchaseDate string `json:"purchase_date"`
	Notes        string `json:"notes"`
}

type BatchCreateDeviceRequest struct {
	// Existing IDs (0 = create new inline)
	CategoryID   int `json:"category_id"`
	DeviceTypeID int `json:"device_type_id"`

	// Inline category creation (used when CategoryID == 0)
	NewCategoryName   string `json:"new_category_name"`
	NewCategoryPrefix string `json:"new_category_prefix"`

	// Inline device type creation (used when DeviceTypeID == 0)
	NewTypeName            string `json:"new_type_name"`
	NewTypeBrand           string `json:"new_type_brand"`
	NewTypeModel           string `json:"new_type_model"`
	NewTypeAssetCodePrefix string `json:"new_type_asset_code_prefix"`
	NewTypeUsageType       string `json:"new_type_usage_type"`
	NewTypeDefaultLocation string `json:"new_type_default_location"`

	Devices []BatchDeviceItemRequest `json:"devices" binding:"required,min=1,dive"`
}

type EditDeviceRequest struct {
	DeviceTypeID int    `form:"device_type_id" binding:"required"`
	AssetCode    string `form:"asset_code"`
	SerialNumber string `form:"serial_number"`
	Condition    string `form:"condition"`
	Location     string `form:"location"`
	PurchaseDate string `form:"purchase_date"`
	Notes        string `form:"notes"`
}

type CreateDeviceTypeRequest struct {
	CategoryID      int    `form:"category_id" binding:"required"`
	Name            string `form:"name" binding:"required"`
	Brand           string `form:"brand"`
	Model           string `form:"model"`
	AssetCodePrefix string `form:"asset_code_prefix"`
	UsageType       string `form:"usage_type" binding:"required"`
	DefaultLocation string `form:"default_location"`
}

type EditDeviceTypeRequest struct {
	CategoryID      int    `form:"category_id" binding:"required"`
	Name            string `form:"name" binding:"required"`
	Brand           string `form:"brand"`
	Model           string `form:"model"`
	AssetCodePrefix string `form:"asset_code_prefix"`
	UsageType       string `form:"usage_type" binding:"required"`
	DefaultLocation string `form:"default_location"`
}

type CreateDeviceLoanRequest struct {
	DeviceID     string `form:"device_id" binding:"required"`
	BorrowerName string `form:"borrower_name" binding:"required"`
	BorrowerType string `form:"borrower_type"`
	LoanDate     string `form:"loan_date" binding:"required"`
	ReturnDate   string `form:"return_date" binding:"required"`
	Purpose      string `form:"purpose"`
}

type EditDeviceLoanRequest struct {
	BorrowerName     string `form:"borrower_name" binding:"required"`
	BorrowerType     string `form:"borrower_type"`
	LoanDate         string `form:"loan_date" binding:"required"`
	ReturnDate       string `form:"return_date"`
	ActualReturnDate string `form:"actual_return_date"`
	Purpose          string `form:"purpose"`
	Notes            string `form:"notes"`
}

type CreateDeviceUsageRequest struct {
	DeviceID    string `form:"device_id" binding:"required"`
	UserName    string `form:"user_name" binding:"required"`
	UserType    string `form:"user_type"`
	UsageDate   string `form:"usage_date" binding:"required"`
	IsAvailable string `form:"is_available"`
	Purpose     string `form:"purpose"`
}

type EditDeviceUsageRequest struct {
	UserName    string `form:"user_name"`
	UserType    string `form:"user_type"`
	UsageDate   string `form:"usage_date"`
	IsAvailable string `form:"is_available"`
	Purpose     string `form:"purpose"`
	Notes       string `form:"notes"`
}

type CreateInstallationRequest struct {
	DeviceID               string `form:"device_id" binding:"required"`
	LocationInstalled      string `form:"location_installed" binding:"required"`
	InstallationStartDate  string `form:"installation_start_date"`
	InstallationFinishDate string `form:"installation_finish_date"`
	Notes                  string `form:"notes"`
}

type EditInstallationRequest struct {
	LocationInstalled      string `form:"location_installed" binding:"required"`
	InstallationStartDate  string `form:"installation_start_date"`
	InstallationFinishDate string `form:"installation_finish_date"`
	Notes                  string `form:"notes"`
}

type CreateLogbookRequest struct {
	Date        string `form:"date" binding:"required"`
	StudentName string `form:"student_name" binding:"required"`
	NIM         string `form:"nim" binding:"required"`
	TimeIn      string `form:"time_in" binding:"required"`
	TimeOut     string `form:"time_out"`
	Purpose     string `form:"purpose"`
}

type EditLogbookRequest struct {
	Date        string `form:"date" binding:"required"`
	StudentName string `form:"student_name" binding:"required"`
	NIM         string `form:"nim" binding:"required"`
	TimeIn      string `form:"time_in" binding:"required"`
	TimeOut     string `form:"time_out"`
	Purpose     string `form:"purpose"`
}

type CreateScheduleRequest struct {
	CourseName string `form:"course_name" binding:"required"`
	Lecturer   string `form:"lecturer" binding:"required"`
	Day        string `form:"day" binding:"required"`
	Class      string `form:"class" binding:"required"`
	TimeStart  string `form:"time_start" binding:"required"`
	TimeEnd    string `form:"time_end" binding:"required"`
	Notes      string `form:"notes"`
}

type EditScheduleRequest struct {
	CourseName string `form:"course_name" binding:"required"`
	Lecturer   string `form:"lecturer" binding:"required"`
	Day        string `form:"day" binding:"required"`
	Class      string `form:"class" binding:"required"`
	TimeStart  string `form:"time_start" binding:"required"`
	TimeEnd    string `form:"time_end" binding:"required"`
	Notes      string `form:"notes"`
}

type CreateSoftwareRequest struct {
	Name        string `form:"name" binding:"required"`
	Category    string `form:"category"`
	Description string `form:"description"`
}

type CreateUserRequest struct {
	Username string `form:"username" binding:"required"`
	Password string `form:"password" binding:"required"`
	FullName string `form:"full_name" binding:"required"`
	Role     string `form:"role" binding:"required"`
}

type UpdateUserRequest struct {
	Username    string `form:"username" binding:"required"`
	FullName    string `form:"full_name" binding:"required"`
	Role        string `form:"role" binding:"required"`
	NewPassword string `form:"new_password"`
}

type UpdateProfileRequest struct {
	Username string `form:"username" binding:"required"`
	FullName string `form:"full_name" binding:"required"`
}

type ChangePasswordRequest struct {
	OldPassword     string `form:"old_password" binding:"required"`
	NewPassword     string `form:"new_password" binding:"required"`
	ConfirmPassword string `form:"confirm_password" binding:"required"`
}

type LogbookSaveRequest struct {
	SourceFile  string   `form:"source_file"`
	Date        []string `form:"date[]"`
	TimeIn      []string `form:"time_in[]"`
	TimeOut     []string `form:"time_out[]"`
	StudentName []string `form:"student_name[]"`
	NIM         []string `form:"nim[]"`
	Purpose     []string `form:"purpose[]"`
}

type UploadImageRequest struct {
	Type     string `form:"type"`
	PCNumber string `form:"pc_number"`
}
