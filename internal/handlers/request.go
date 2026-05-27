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
	Name         string `form:"name" binding:"required"`
	Brand        string `form:"brand"`
	Model        string `form:"model"`
	SerialNumber string `form:"serial_number"`
	ItemType     string `form:"item_type"`
	ItemMode     string `form:"item_mode"`
	Quantity     int    `form:"quantity_total" binding:"min=1"`
	Condition    string `form:"condition"`
	Location     string `form:"location"`
	PurchaseDate string `form:"purchase_date"`
	Notes        string `form:"notes"`
}

type EditDeviceRequest struct {
	DeviceTypeID     int    `form:"device_type_id" binding:"required"`
	Name             string `form:"name" binding:"required"`
	Brand            string `form:"brand"`
	Model            string `form:"model"`
	SerialNumber     string `form:"serial_number"`
	ItemType         string `form:"item_type"`
	ItemMode         string `form:"item_mode"`
	QuantityTotal    int    `form:"quantity_total" binding:"min=0"`
	QuantityAvailable int   `form:"quantity_available" binding:"min=0"`
	Condition        string `form:"condition"`
	Location         string `form:"location"`
	PurchaseDate     string `form:"purchase_date"`
	Notes            string `form:"notes"`
}

type CreateDeviceTypeRequest struct {
	Name            string `form:"name" binding:"required"`
	Category        string `form:"category" binding:"required"`
	Brand           string `form:"brand"`
	Model           string `form:"model"`
	ItemType        string `form:"item_type" binding:"required"`
	ItemMode        string `form:"item_mode"`
	AssetCodePrefix string `form:"asset_code_prefix"`
	DefaultLocation string `form:"default_location"`
	NotesTemplate   string `form:"notes_template"`
}

type EditDeviceTypeRequest struct {
	Name            string `form:"name" binding:"required"`
	Category        string `form:"category" binding:"required"`
	Brand           string `form:"brand"`
	Model           string `form:"model"`
	ItemType        string `form:"item_type" binding:"required"`
	ItemMode        string `form:"item_mode"`
	AssetCodePrefix string `form:"asset_code_prefix"`
	DefaultLocation string `form:"default_location"`
	NotesTemplate   string `form:"notes_template"`
}

type CreateDeviceLoanRequest struct {
	DeviceID            string `form:"device_id" binding:"required"`
	BorrowerName        string `form:"borrower_name" binding:"required"`
	BorrowerType        string `form:"borrower_type"`
	LoanDate            string `form:"loan_date" binding:"required"`
	ExpectedReturnDate  string `form:"expected_return_date"`
	Quantity            int    `form:"quantity" binding:"required,min=1"`
	Purpose             string `form:"purpose"`
}

type EditDeviceLoanRequest struct {
	BorrowerName       string `form:"borrower_name" binding:"required"`
	BorrowerType       string `form:"borrower_type"`
	LoanDate           string `form:"loan_date" binding:"required"`
	ExpectedReturnDate string `form:"expected_return_date"`
	ActualReturnDate   string `form:"actual_return_date"`
	Status             string `form:"status"`
	Purpose            string `form:"purpose"`
	Notes              string `form:"notes"`
}

type CreateDeviceUsageRequest struct {
	DeviceID    string `form:"device_id" binding:"required"`
	UserName    string `form:"user_name" binding:"required"`
	UserType    string `form:"user_type"`
	UsageDate   string `form:"usage_date" binding:"required"`
	Quantity    int    `form:"quantity" binding:"required,min=1"`
	IsAvailable string `form:"is_available"`
	Purpose     string `form:"purpose"`
}

type EditDeviceUsageRequest struct {
	UserName    string `form:"user_name"`
	UserType    string `form:"user_type"`
	UsageDate   string `form:"usage_date"`
	Quantity    int    `form:"quantity"`
	IsAvailable string `form:"is_available"`
	Purpose     string `form:"purpose"`
	Notes       string `form:"notes"`
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
	OldPassword string `form:"old_password" binding:"required"`
	NewPassword string `form:"new_password" binding:"required"`
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

type UpdateAvailabilityRequest struct {
	IsAvailable string `form:"is_available" binding:"required"`
}

type CreateLostItemRequest struct {
	DeviceID        string `form:"device_id"`
	ItemName        string `form:"item_name" binding:"required"`
	ItemDescription string `form:"item_description"`
	ReportedBy      string `form:"reported_by" binding:"required"`
	ReportedDate    string `form:"reported_date"`
	LastSeenAt      string `form:"last_seen_at"`
	Status          string `form:"status"`
	LocationLastSeen string `form:"location_last_seen"`
	Photo           string `form:"photo"`
}

type EditLostItemRequest struct {
	DeviceID        string `form:"device_id"`
	ItemName        string `form:"item_name" binding:"required"`
	ItemDescription string `form:"item_description"`
	ReportedBy      string `form:"reported_by" binding:"required"`
	ReportedDate    string `form:"reported_date"`
	LastSeenAt      string `form:"last_seen_at"`
	Status          string `form:"status"`
	LocationLastSeen string `form:"location_last_seen"`
	OwnerName       string `form:"owner_name"`
	OwnerClass      string `form:"owner_class"`
	OwnerNim        string `form:"owner_nim"`
	ReturnedDate    string `form:"returned_date"`
	Photo           string `form:"photo"`
}
