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
	s.activityLogService.LogCreate(actorID, actorUsername, actorRole, "device", int(id), map[string]interface{}{"name": in.Name, "asset_code": code}, ipAddress, userAgent)
	return int(id), code, nil
}

func (s *DeviceService) UpdateDevice(id int, in UpdateDeviceInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	err := s.deviceRepo.Update(id, in.DeviceTypeID, in.Name, in.Brand, in.Model, in.SerialNumber, in.ItemType,
		in.ItemMode == "loanable", in.ItemMode == "consumable",
		in.QuantityTotal, in.QuantityAvailable, in.Condition, in.Location, in.PurchaseDate, in.Notes)
	if err != nil {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "device", 0, map[string]interface{}{"id": id}, nil, ipAddress, userAgent, err.Error())
		return err
	}
	s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "device", 0,
		map[string]interface{}{"id": id},
		map[string]interface{}{"name": in.Name}, ipAddress, userAgent)
	return nil
}

func (s *DeviceService) DeleteDevice(id, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	if err := s.deviceRepo.Delete(id); err != nil {
		s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "device", 0, map[string]interface{}{"id": id}, ipAddress, userAgent, err.Error())
		return err
	}
	s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "device", 0, map[string]interface{}{"id": id}, ipAddress, userAgent)
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
