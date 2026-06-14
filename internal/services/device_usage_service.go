package services

import (
	"strconv"

	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/repository"
)

type CreateUsageInput struct {
	DeviceID    int
	UserName    string
	UserType    string
	UsageDate   string
	IsAvailable string
	Purpose     string
}

type UpdateUsageInput struct {
	UserName    string
	UserType    string
	UsageDate   string
	IsAvailable string
	Purpose     string
	Notes       string
}

type DeviceUsageService struct {
	repo *repository.DeviceUsageRepository
	log  *ActivityLogService
}

func NewDeviceUsageService(repo *repository.DeviceUsageRepository, log *ActivityLogService) *DeviceUsageService {
	return &DeviceUsageService{repo: repo, log: log}
}

func (s *DeviceUsageService) GetByID(id int) (*repository.DeviceUsageRow, error) {
	return s.repo.GetByID(id)
}

func (s *DeviceUsageService) GetConsumableDevices() ([]models.Device, error) {
	return s.repo.GetConsumableDevices()
}

func (s *DeviceUsageService) ListPaginated(filters repository.DeviceUsageFilters, page, pageSize int) ([]repository.DeviceUsageRow, int, error) {
	return s.repo.ListPaginated(filters, page, pageSize)
}

func (s *DeviceUsageService) ListByDeviceID(deviceID int) ([]repository.DeviceUsageRow, error) {
	return s.repo.List(repository.DeviceUsageFilters{DeviceID: strconv.Itoa(deviceID)})
}

func (s *DeviceUsageService) ExportAll() ([]repository.DeviceUsageRow, error) {
	return s.repo.ExportAll()
}

func (s *DeviceUsageService) CreateUsage(in CreateUsageInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) (int64, error) {
	if in.IsAvailable != "no" {
		in.IsAvailable = "yes"
	}
	usageDate := MustParseDate(in.UsageDate)
	in.UserName = ToTitleCaseWithAbbr(in.UserName)
	in.Purpose = SanitizeText(in.Purpose)

	id, err := s.repo.Create(in.DeviceID, in.UserName, in.UserType, usageDate, in.IsAvailable, in.Purpose)
	if err != nil {
		s.log.LogCreate(actorID, actorUsername, actorRole, "device_usage", 0,
			map[string]any{"user": in.UserName}, ipAddress, userAgent, err.Error())
		return 0, err
	}

	s.log.LogCreate(actorID, actorUsername, actorRole, "device_usage", int(id),
		map[string]any{"user": in.UserName, "device_id": in.DeviceID}, ipAddress, userAgent)
	return id, nil
}

func (s *DeviceUsageService) UpdateUsage(id int, in UpdateUsageInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	if in.IsAvailable != "no" {
		in.IsAvailable = "yes"
	}
	usageDate := MustParseDate(in.UsageDate)
	in.UserName = ToTitleCaseWithAbbr(in.UserName)
	in.Purpose = SanitizeText(in.Purpose)
	in.Notes = SanitizeText(in.Notes)

	oldRow, _ := s.repo.GetByID(id)
	oldVals := map[string]any{}
	newVals := map[string]any{}
	if oldRow != nil {
		if oldRow.UserName != in.UserName { oldVals["user_name"] = oldRow.UserName; newVals["user_name"] = in.UserName }
		if oldRow.UserType != in.UserType { oldVals["user_type"] = oldRow.UserType; newVals["user_type"] = in.UserType }
		if oldRow.Purpose != in.Purpose { oldVals["purpose"] = oldRow.Purpose; newVals["purpose"] = in.Purpose }
		if oldRow.IsAvailable != in.IsAvailable { oldVals["is_available"] = oldRow.IsAvailable; newVals["is_available"] = in.IsAvailable }
		if oldRow.Notes != in.Notes { oldVals["notes"] = oldRow.Notes; newVals["notes"] = in.Notes }
	}

	if err := s.repo.Update(id, in.UserName, in.UserType, usageDate, in.IsAvailable, in.Purpose, in.Notes); err != nil {
		s.log.LogUpdate(actorID, actorUsername, actorRole, "device_usage", id,
			oldVals, nil, ipAddress, userAgent, err.Error())
		return err
	}

	s.log.LogUpdate(actorID, actorUsername, actorRole, "device_usage", id,
		oldVals, newVals, ipAddress, userAgent)
	return nil
}

func (s *DeviceUsageService) DeleteUsage(id, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	if err := s.repo.Delete(id); err != nil {
		s.log.LogDelete(actorID, actorUsername, actorRole, "device_usage", id,
			map[string]any{"id": id}, ipAddress, userAgent, err.Error())
		return err
	}
	s.log.LogDelete(actorID, actorUsername, actorRole, "device_usage", id,
		map[string]any{"id": id}, ipAddress, userAgent)
	return nil
}

func (s *DeviceUsageService) BatchDelete(ids []int, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	items := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		info := map[string]any{"id": id}
		if row, err := s.repo.GetByID(id); err == nil {
			info["user_name"] = row.UserName
			info["device_asset_code"] = row.DeviceAssetCode
		}
		if err := s.repo.Delete(id); err != nil {
			s.log.LogDelete(actorID, actorUsername, actorRole, "device_usage", 0,
				map[string]any{"action": "batch_delete", "count": len(ids), "items": items},
				ipAddress, userAgent, err.Error())
			return err
		}
		items = append(items, info)
	}
	s.log.LogDelete(actorID, actorUsername, actorRole, "device_usage", 0,
		map[string]any{"action": "batch_delete", "count": len(ids), "items": items},
		ipAddress, userAgent)
	return nil
}
