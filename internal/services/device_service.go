package services

import (
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/repository"
)

type CreateDeviceInput struct {
	DeviceTypeID              int
	Name, Brand, Model        string
	SerialNumber, ItemType, ItemMode string
	Quantity                  int
	Condition, Location, PurchaseDate, Notes string
}

type UpdateDeviceInput struct {
	DeviceTypeID                     int
	Name, Brand, Model               string
	SerialNumber, ItemType, ItemMode string
	QuantityTotal, QuantityAvailable int
	Condition, Location, PurchaseDate, Notes string
}

type DeviceDetailData struct {
	Device         *models.DeviceWithCategory
	DeviceTypeName string
	Loans          []models.DeviceLoan
	Usages         []models.DeviceUsage
}

type DeviceService struct {
	deviceRepo         *repository.DeviceRepository
	deviceTypeRepo     *repository.DeviceTypeRepository
	activityLogService *ActivityLogService
}

func NewDeviceService(deviceRepo *repository.DeviceRepository, deviceTypeRepo *repository.DeviceTypeRepository, activityLogService *ActivityLogService) *DeviceService {
	return &DeviceService{deviceRepo: deviceRepo, deviceTypeRepo: deviceTypeRepo, activityLogService: activityLogService}
}

func (s *DeviceService) List(filters repository.DeviceFilters) ([]models.DeviceWithCategory, error) {
	return s.deviceRepo.List(filters)
}

func (s *DeviceService) ListLoans() ([]repository.DeviceLoanRow, error) {
	return s.deviceRepo.ListLoans()
}

func (s *DeviceService) ListUsages() ([]repository.DeviceUsageRow, error) {
	return s.deviceRepo.ListUsages()
}

func (s *DeviceService) GetByID(id int) (*models.DeviceWithCategory, error) {
	return s.deviceRepo.GetByID(id)
}

func (s *DeviceService) GetByIDSimple(id int) (*models.Device, error) {
	return s.deviceRepo.GetByIDSimple(id)
}

func (s *DeviceService) GetNextAssetCode(prefix string) string {
	return s.deviceRepo.GetNextAssetCode(prefix)
}

func (s *DeviceService) ExportAll() ([]repository.DeviceExportRow, error) {
	return s.deviceRepo.ExportAll()
}

func (s *DeviceService) ExportDeviceTypes() ([]repository.DeviceTypeExportRow, error) {
	return s.deviceRepo.ExportDeviceTypes()
}

func (s *DeviceService) ExportLoans() ([]repository.DeviceLoanRow, error) {
	return s.deviceRepo.ExportLoans()
}

func (s *DeviceService) ExportUsages() ([]repository.DeviceUsageRow, error) {
	return s.deviceRepo.ExportUsages()
}

func (s *DeviceService) CreateDevice(in CreateDeviceInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) (int, string, error) {
	prefix, err := s.deviceTypeRepo.GetPrefix(in.DeviceTypeID)
	if err != nil {
		return 0, "", err
	}
	code := s.deviceRepo.GetNextAssetCode(prefix)

	result, err := s.deviceRepo.Create(in.DeviceTypeID, code, in.Name, in.Brand, in.Model, in.SerialNumber, in.ItemType,
		in.ItemMode == "loanable", in.ItemMode == "consumable", in.Quantity, in.Condition, in.Location, in.PurchaseDate, in.Notes)
	if err != nil {
		return 0, "", err
	}
	id, _ := result.LastInsertId()
	s.activityLogService.LogCreate(actorID, actorUsername, actorRole, "device", int(id), map[string]any{"name": in.Name, "asset_code": code}, ipAddress, userAgent)
	return int(id), code, nil
}

func (s *DeviceService) UpdateDevice(id int, in UpdateDeviceInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	err := s.deviceRepo.Update(id, in.DeviceTypeID, in.Name, in.Brand, in.Model, in.SerialNumber, in.ItemType,
		in.ItemMode == "loanable", in.ItemMode == "consumable",
		in.QuantityTotal, in.QuantityAvailable, in.Condition, in.Location, in.PurchaseDate, in.Notes)
	if err != nil {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "device", 0, map[string]any{"id": id}, nil, ipAddress, userAgent, err.Error())
		return err
	}
	s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "device", 0,
		map[string]any{"id": id},
		map[string]any{"name": in.Name}, ipAddress, userAgent)
	return nil
}

func (s *DeviceService) DeleteDevice(id, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	if err := s.deviceRepo.Delete(id); err != nil {
		s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "device", 0, map[string]any{"id": id}, ipAddress, userAgent, err.Error())
		return err
	}
	s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "device", 0, map[string]any{"id": id}, ipAddress, userAgent)
	return nil
}

func (s *DeviceService) GetDetail(id int) (*DeviceDetailData, error) {
	d, err := s.deviceRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	dtName, _ := s.deviceTypeRepo.GetName(d.DeviceTypeID)
	loans, _ := s.deviceRepo.GetLoansByDevice(id, 10)
	usages, _ := s.deviceRepo.GetUsagesByDevice(id, 10)
	return &DeviceDetailData{Device: d, DeviceTypeName: dtName, Loans: loans, Usages: usages}, nil
}
