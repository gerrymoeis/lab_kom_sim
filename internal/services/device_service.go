package services

import (
	"fmt"
	"strconv"
	"strings"

	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/repository"
)

func sanitizeDBError(err error) error {
	if err == nil {
		return nil
	}

	msg := err.Error()
	lower := strings.ToLower(msg)

	if strings.Contains(lower, "unique") || strings.Contains(lower, "duplicate") {
		if strings.Contains(lower, "name") {
			return fmt.Errorf("Nama sudah digunakan")
		}
		if strings.Contains(lower, "prefix") || strings.Contains(lower, "asset_code_prefix") {
			return fmt.Errorf("Prefix sudah digunakan")
		}
		return fmt.Errorf("Data sudah ada")
	}

	if strings.Contains(lower, "check") && strings.Contains(lower, "usage_type") {
		return fmt.Errorf("Tipe penggunaan tidak valid")
	}

	if strings.Contains(lower, "foreign key") {
		return fmt.Errorf("Data masih digunakan oleh data lain")
	}

	return err
}

type CreateDeviceInput struct {
	DeviceTypeID int
	SerialNumber string
	Condition    string
	Location     string
	PurchaseDate string
	Notes        string
}

type UpdateDeviceInput struct {
	DeviceTypeID int
	AssetCode    string
	SerialNumber string
	Condition    string
	Location     string
	PurchaseDate string
	Notes        string
	UsageType    string // empty = inherit from device type
}

type DeviceService struct {
	deviceRepo     *repository.DeviceRepository
	deviceTypeRepo *repository.DeviceTypeRepository
	log            *ActivityLogService
}

func NewDeviceService(deviceRepo *repository.DeviceRepository, deviceTypeRepo *repository.DeviceTypeRepository, log *ActivityLogService) *DeviceService {
	return &DeviceService{deviceRepo: deviceRepo, deviceTypeRepo: deviceTypeRepo, log: log}
}

func (s *DeviceService) List(filters repository.DeviceFilters) ([]models.Device, error) {
	return s.deviceRepo.List(filters)
}

func (s *DeviceService) ListPaginated(filters repository.DeviceFilters, page, pageSize int) ([]models.Device, int, error) {
	return s.deviceRepo.ListPaginated(filters, page, pageSize)
}

func (s *DeviceService) GetByID(id int) (*models.Device, error) {
	return s.deviceRepo.GetByID(id)
}

func (s *DeviceService) GetBySlug(slug string) (*models.Device, error) {
	return s.deviceRepo.GetBySlug(slug)
}

func (s *DeviceService) GetByAssetCodeSlug(slug string) (*models.Device, error) {
	return s.deviceRepo.GetByAssetCodeSlug(slug)
}

func (s *DeviceService) GetByAssetCode(code string) (*models.Device, error) {
	return s.deviceRepo.GetByAssetCode(code)
}

func (s *DeviceService) GetActiveLoanIDs() (map[int]bool, error) {
	return s.deviceRepo.GetActiveLoanDeviceIDs()
}

func (s *DeviceService) GetDepletedIDs() (map[int]bool, error) {
	return s.deviceRepo.GetDepletedDeviceIDs()
}

func (s *DeviceService) GetInstallationStatuses() (map[int]string, error) {
	return s.deviceRepo.GetInstallationStatuses()
}

func (s *DeviceService) GetNextAssetCode(prefix string) string {
	return s.deviceRepo.GetNextAssetCode(prefix)
}

func (s *DeviceService) CreateDevice(in CreateDeviceInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) (int, string, error) {
	in.Location = ToTitleCaseWithAbbr(in.Location)
	in.Notes = SanitizeText(in.Notes)
	in.SerialNumber = SanitizeText(in.SerialNumber)
	prefix, err := s.deviceTypeRepo.GetPrefix(in.DeviceTypeID)
	if err != nil {
		s.log.LogCreate(actorID, actorUsername, actorRole, "device", 0,
			map[string]any{"device_type_id": in.DeviceTypeID}, ipAddress, userAgent, err.Error())
		return 0, "", err
	}
	code := s.deviceRepo.GetNextAssetCode(prefix)
	result, err := s.deviceRepo.Create(in.DeviceTypeID, code, in.SerialNumber, in.Condition, in.Location, in.PurchaseDate, in.Notes)
	if err != nil {
		s.log.LogCreate(actorID, actorUsername, actorRole, "device", 0,
			map[string]any{"asset_code": code}, ipAddress, userAgent, err.Error())
		return 0, "", err
	}
	id, _ := result.LastInsertId()
	s.log.LogCreate(actorID, actorUsername, actorRole, "device", int(id),
		map[string]any{"asset_code": code}, ipAddress, userAgent)
	return int(id), code, nil
}

type BatchDeviceCreateInput struct {
	SerialNumber string
	Condition    string
	Location     string
	PurchaseDate string
	Notes        string
}

func (s *DeviceService) BatchCreate(deviceTypeID int, devices []BatchDeviceCreateInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) ([]string, error) {
	prefix, err := s.deviceTypeRepo.GetPrefix(deviceTypeID)
	if err != nil {
		s.log.LogCreate(actorID, actorUsername, actorRole, "device", 0,
			map[string]any{"device_type_id": deviceTypeID}, ipAddress, userAgent, err.Error())
		return nil, err
	}

	startCode := s.deviceRepo.GetNextAssetCodeSequence(prefix)
	parts := strings.Split(startCode, "-")
	startNum, _ := strconv.Atoi(parts[len(parts)-1])
	if startNum < 1 {
		startNum = 1
	}

	var inputs []repository.BatchCreateInput
	var codes []string
	for i, dev := range devices {
		dev.Location = ToTitleCaseWithAbbr(dev.Location)
		dev.Notes = SanitizeText(dev.Notes)
		dev.SerialNumber = SanitizeText(dev.SerialNumber)
		code := fmt.Sprintf("%s-%03d", prefix, startNum+i)
		inputs = append(inputs, repository.BatchCreateInput{
			DeviceTypeID: deviceTypeID,
			AssetCode:    code,
			SerialNumber: dev.SerialNumber,
			Condition:    dev.Condition,
			Location:     dev.Location,
			PurchaseDate: dev.PurchaseDate,
			Notes:        dev.Notes,
		})
		codes = append(codes, code)
	}

	if err := s.deviceRepo.BatchCreate(inputs); err != nil {
		s.log.LogCreate(actorID, actorUsername, actorRole, "device", 0,
			map[string]any{"action": "batch_create", "count": len(codes), "codes": codes},
			ipAddress, userAgent, err.Error())
		return nil, err
	}

	s.log.LogCreate(actorID, actorUsername, actorRole, "device", 0,
		map[string]any{"action": "batch_create", "count": len(codes), "codes": codes},
		ipAddress, userAgent)
	return codes, nil
}

func (s *DeviceService) UpdateDevice(id int, in UpdateDeviceInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	in.Location = ToTitleCaseWithAbbr(in.Location)
	in.Notes = SanitizeText(in.Notes)
	in.SerialNumber = SanitizeText(in.SerialNumber)
	old, _ := s.deviceRepo.GetByID(id)
	err := s.deviceRepo.Update(id, in.DeviceTypeID, in.AssetCode, in.SerialNumber, in.Condition, in.Location, in.PurchaseDate, in.Notes, in.UsageType)
	if err != nil {
		s.log.LogUpdate(actorID, actorUsername, actorRole, "device", id,
			map[string]any{"id": id}, nil, ipAddress, userAgent, err.Error())
		return err
	}
	oldVals := map[string]any{}
	newVals := map[string]any{}
	if old != nil {
		if old.AssetCode != in.AssetCode { oldVals["asset_code"] = old.AssetCode; newVals["asset_code"] = in.AssetCode }
		if old.SerialNumber != in.SerialNumber { oldVals["serial_number"] = old.SerialNumber; newVals["serial_number"] = in.SerialNumber }
		if old.Condition != in.Condition { oldVals["condition"] = old.Condition; newVals["condition"] = in.Condition }
		if old.Location != in.Location { oldVals["location"] = old.Location; newVals["location"] = in.Location }
		if old.Notes != in.Notes { oldVals["notes"] = old.Notes; newVals["notes"] = in.Notes }
		if old.DeviceTypeID != in.DeviceTypeID { oldVals["device_type_id"] = old.DeviceTypeID; newVals["device_type_id"] = in.DeviceTypeID }
	}
	s.log.LogUpdate(actorID, actorUsername, actorRole, "device", id,
		oldVals, newVals, ipAddress, userAgent)
	return nil
}

func (s *DeviceService) CountByDeviceTypeID(deviceTypeID int) (int, error) {
	return s.deviceRepo.CountByDeviceTypeID(deviceTypeID)
}

func (s *DeviceService) CountByCategoryID(categoryID int) (int, error) {
	return s.deviceRepo.CountByCategoryID(categoryID)
}

func (s *DeviceService) DeleteDevice(id int, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	d, _ := s.deviceRepo.GetByID(id)
	oldVals := map[string]any{"id": id}
	if d != nil {
		oldVals["asset_code"] = d.AssetCode
		oldVals["serial_number"] = d.SerialNumber
	}
	if err := s.deviceRepo.Delete(id); err != nil {
		s.log.LogDelete(actorID, actorUsername, actorRole, "device", id,
			oldVals, ipAddress, userAgent, err.Error())
		return err
	}
	s.log.LogDelete(actorID, actorUsername, actorRole, "device", id,
		oldVals, ipAddress, userAgent)
	return nil
}

func (s *DeviceService) BatchDelete(ids []int, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	items := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		info := map[string]any{"id": id}
		if d, err := s.deviceRepo.GetByID(id); err == nil {
			info["asset_code"] = d.AssetCode
			info["serial_number"] = d.SerialNumber
		}
		if err := s.deviceRepo.Delete(id); err != nil {
			s.log.LogDelete(actorID, actorUsername, actorRole, "device", 0,
				map[string]any{"action": "batch_delete", "count": len(ids), "items": items},
				ipAddress, userAgent, err.Error())
			return err
		}
		items = append(items, info)
	}
	s.log.LogDelete(actorID, actorUsername, actorRole, "device", 0,
		map[string]any{"action": "batch_delete", "count": len(ids), "items": items},
		ipAddress, userAgent)
	return nil
}
