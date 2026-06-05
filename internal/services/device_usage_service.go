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

	if err := s.repo.Update(id, in.UserName, in.UserType, usageDate, in.IsAvailable, in.Purpose, in.Notes); err != nil {
		s.log.LogUpdate(actorID, actorUsername, actorRole, "device_usage", id,
			map[string]any{"id": id}, nil, ipAddress, userAgent, err.Error())
		return err
	}

	s.log.LogUpdate(actorID, actorUsername, actorRole, "device_usage", id,
		map[string]any{"id": id}, nil, ipAddress, userAgent)
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
