package services

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/repository"
)

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

func (s *DeviceService) ListPaginated(filters repository.DeviceFilters, page, pageSize int) ([]models.Device, int, error) {
	return s.deviceRepo.ListPaginated(filters, page, pageSize)
}

func (s *DeviceService) GetByID(id int) (*models.Device, error) {
	return s.deviceRepo.GetByID(id)
}

func (s *DeviceService) GetByAssetCode(code string) (*models.Device, error) {
	return s.deviceRepo.GetByAssetCode(code)
}

func (s *DeviceService) GetGrouped() (*models.DeviceGroupedData, error) {
	rows, err := s.deviceRepo.GetAllGrouped()
	if err != nil {
		return nil, err
	}

	activeLoanIDs, err := s.deviceRepo.GetActiveLoanDeviceIDs()
	if err != nil {
		activeLoanIDs = make(map[int]bool)
	}

	depletedIDs, err := s.deviceRepo.GetDepletedDeviceIDs()
	if err != nil {
		depletedIDs = make(map[int]bool)
	}

	catMap := make(map[int]*models.CategoryGroup)
	typeMap := make(map[int]*models.DeviceTypeGroup)
	var catOrder []int

	for _, row := range rows {
		cat, ok := catMap[row.CategoryID]
		if !ok {
			cat = &models.CategoryGroup{
				CategoryID:     row.CategoryID,
				CategoryName:   row.CategoryName,
				CategoryPrefix: row.CategoryPrefix,
			}
			catMap[row.CategoryID] = cat
			catOrder = append(catOrder, row.CategoryID)
		}

		tg, ok2 := typeMap[row.TypeID]
		if !ok2 {
			tg = &models.DeviceTypeGroup{
				TypeID:     row.TypeID,
				TypeName:   row.TypeName,
				TypePrefix: row.TypePrefix,
				UsageType:  row.TypeUsageType,
				TypePhoto:  row.TypePhoto,
			}
			typeMap[row.TypeID] = tg
			cat.Types = append(cat.Types, *tg)
		}

		if row.DeviceID != nil {
			var pDate *time.Time
			if *row.PurchaseDate != "" {
				if t, err := time.Parse("2006-01-02", *row.PurchaseDate); err == nil {
					pDate = &t
				}
			}
			effectiveUsageType := row.TypeUsageType
			if row.DeviceUsageType != nil && *row.DeviceUsageType != "" {
				effectiveUsageType = *row.DeviceUsageType
			}
			dev := models.Device{
				ID:         *row.DeviceID,
				DeviceTypeID: row.TypeID,
				AssetCode:  *row.AssetCode,
				SerialNumber: *row.SerialNumber,
				Condition:  *row.Condition,
				Location:   *row.Location,
				PurchaseDate: pDate,
				Notes:      *row.Notes,
				CategoryName:   row.CategoryName,
				CategoryPrefix: row.CategoryPrefix,
				DeviceTypeName: row.TypeName,
				DeviceTypePrefix: row.TypePrefix,
				UsageType:  effectiveUsageType,
				DeviceTypePhoto: row.TypePhoto,
			}
			// Find the type group in the cat and add device
			for i := range cat.Types {
				if cat.Types[i].TypeID == row.TypeID {
					cat.Types[i].Devices = append(cat.Types[i].Devices, dev)
					break
				}
			}
		}
	}

	categories := make([]models.CategoryGroup, len(catOrder))
	for i, id := range catOrder {
		categories[i] = *catMap[id]
	}

	return &models.DeviceGroupedData{
		Categories:    categories,
		ActiveLoanIDs: activeLoanIDs,
		DepletedIDs:   depletedIDs,
	}, nil
}

func (s *DeviceService) GetNextAssetCode(prefix string) string {
	return s.deviceRepo.GetNextAssetCode(prefix)
}

func (s *DeviceService) CreateDevice(in CreateDeviceInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) (int, string, error) {
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

	startCode := s.deviceRepo.GetNextAssetCode(prefix)
	parts := strings.Split(startCode, "-")
	startNum, _ := strconv.Atoi(parts[len(parts)-1])
	if startNum < 1 {
		startNum = 1
	}

	var inputs []repository.BatchCreateInput
	var codes []string
	for i, dev := range devices {
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
		for _, code := range codes {
			s.log.LogCreate(actorID, actorUsername, actorRole, "device", 0,
				map[string]any{"asset_code": code}, ipAddress, userAgent, err.Error())
		}
		return nil, err
	}

	for _, code := range codes {
		s.log.LogCreate(actorID, actorUsername, actorRole, "device", 0,
			map[string]any{"asset_code": code}, ipAddress, userAgent)
	}
	return codes, nil
}

func (s *DeviceService) UpdateDevice(id int, in UpdateDeviceInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	err := s.deviceRepo.Update(id, in.DeviceTypeID, in.AssetCode, in.SerialNumber, in.Condition, in.Location, in.PurchaseDate, in.Notes, in.UsageType)
	if err != nil {
		s.log.LogUpdate(actorID, actorUsername, actorRole, "device", id,
			map[string]any{"id": id}, nil, ipAddress, userAgent, err.Error())
		return err
	}
	s.log.LogUpdate(actorID, actorUsername, actorRole, "device", id,
		map[string]any{"id": id}, map[string]any{"asset_code": in.AssetCode}, ipAddress, userAgent)
	return nil
}

func (s *DeviceService) DeleteDevice(id int, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	if err := s.deviceRepo.Delete(id); err != nil {
		s.log.LogDelete(actorID, actorUsername, actorRole, "device", id,
			map[string]any{"id": id}, ipAddress, userAgent, err.Error())
		return err
	}
	s.log.LogDelete(actorID, actorUsername, actorRole, "device", id,
		map[string]any{"id": id}, ipAddress, userAgent)
	return nil
}
