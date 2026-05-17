package services

import (
	"time"

	"inventaris-lab-kom/internal/repository"
)

type CreateUsageInput struct {
	DeviceID     int
	UserName     string
	UserType     string
	UsageDate    string
	Quantity     int
	IsAvailable  string
	Purpose      string
}

type UpdateUsageInput struct {
	UserName    string
	UserType    string
	UsageDate   string
	Quantity    int
	IsAvailable string
	Purpose     string
	Notes       string
}

type DeviceUsageService struct {
	deviceUsageRepo    *repository.DeviceUsageRepository
	activityLogService *ActivityLogService
}

func NewDeviceUsageService(deviceUsageRepo *repository.DeviceUsageRepository, activityLogService *ActivityLogService) *DeviceUsageService {
	return &DeviceUsageService{deviceUsageRepo: deviceUsageRepo, activityLogService: activityLogService}
}

func (s *DeviceUsageService) CreateUsage(in CreateUsageInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) (int64, error) {
	if in.IsAvailable != "no" { in.IsAvailable = "yes" }
	usageDate, _ := time.Parse("2006-01-02", in.UsageDate)
	id, err := s.deviceUsageRepo.Create(in.DeviceID, in.UserName, in.UserType, usageDate, in.Quantity, in.IsAvailable, in.Purpose)
	if err != nil {
		s.activityLogService.LogCreate(actorID, actorUsername, actorRole, "device_usage", 0,
			map[string]interface{}{"user": in.UserName}, ipAddress, userAgent, err.Error())
		return 0, err
	}
	s.activityLogService.LogCreate(actorID, actorUsername, actorRole, "device_usage", int(id),
		map[string]interface{}{"user": in.UserName, "device_id": in.DeviceID}, ipAddress, userAgent)
	return id, nil
}

func (s *DeviceUsageService) UpdateUsage(id int, in UpdateUsageInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	if in.IsAvailable != "no" { in.IsAvailable = "yes" }
	usageDate, _ := time.Parse("2006-01-02", in.UsageDate)
	err := s.deviceUsageRepo.Update(id, in.UserName, in.UserType, usageDate, in.Quantity, in.IsAvailable, in.Purpose, in.Notes)
	if err != nil {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "device_usage", id,
			map[string]interface{}{"id": id}, nil, ipAddress, userAgent, err.Error())
		return err
	}
	s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "device_usage", id,
		map[string]interface{}{"id": id}, nil, ipAddress, userAgent)
	return nil
}

func (s *DeviceUsageService) DeleteUsage(id, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	if err := s.deviceUsageRepo.Delete(id); err != nil {
		s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "device_usage", id,
			map[string]interface{}{"id": id}, ipAddress, userAgent, err.Error())
		return err
	}
	s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "device_usage", id,
		map[string]interface{}{"id": id}, ipAddress, userAgent)
	return nil
}

func (s *DeviceUsageService) UpdateAvailability(id int, isAvailable string) error {
	return s.deviceUsageRepo.UpdateAvailability(id, isAvailable)
}
