package services

import (
	"time"

	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/repository"
)

type CreateInstallationInput struct {
	DeviceID               int
	LocationInstalled      string
	InstallationStartDate  string
	InstallationFinishDate string
	Photo                  string
	Notes                  string
}

type UpdateInstallationInput struct {
	LocationInstalled      string
	InstallationStartDate  string
	InstallationFinishDate string
	Photo                  string
	Notes                  string
}

type DeviceInstallationService struct {
	repo *repository.DeviceInstallationRepository
	log  *ActivityLogService
}

func NewDeviceInstallationService(repo *repository.DeviceInstallationRepository, log *ActivityLogService) *DeviceInstallationService {
	return &DeviceInstallationService{repo: repo, log: log}
}

func (s *DeviceInstallationService) GetInstallableDevices() ([]models.Device, error) {
	return s.repo.GetInstallableDevices()
}

func (s *DeviceInstallationService) ListPaginated(filters repository.InstallationFilters, page, pageSize int) ([]repository.InstallationRow, int, error) {
	return s.repo.ListPaginated(filters, page, pageSize)
}

func (s *DeviceInstallationService) GetByID(id int) (*repository.InstallationRow, error) {
	return s.repo.GetByID(id)
}

func (s *DeviceInstallationService) GetByDeviceID(deviceID int) (*models.DeviceInstallation, error) {
	return s.repo.GetByDeviceID(deviceID)
}

func (s *DeviceInstallationService) GetDistinctLocations() ([]string, error) {
	return s.repo.GetDistinctLocations()
}

func (s *DeviceInstallationService) ExportAll() ([]repository.InstallationRow, error) {
	return s.repo.ExportAll()
}

func parseNullableDate(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil
	}
	return &t
}

func (s *DeviceInstallationService) Create(in CreateInstallationInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) (int, error) {
	in.LocationInstalled = ToTitleCaseWithAbbr(in.LocationInstalled)
	in.Notes = SanitizeText(in.Notes)
	result, err := s.repo.Create(in.DeviceID, in.LocationInstalled,
		parseNullableDate(in.InstallationStartDate), parseNullableDate(in.InstallationFinishDate),
		in.Photo, in.Notes)
	if err != nil {
		s.log.LogCreate(actorID, actorUsername, actorRole, "device_installation", 0,
			map[string]any{"device_id": in.DeviceID}, ipAddress, userAgent, err.Error())
		return 0, err
	}
	id, _ := result.LastInsertId()
	s.log.LogCreate(actorID, actorUsername, actorRole, "device_installation", int(id),
		map[string]any{"device_id": in.DeviceID, "location": in.LocationInstalled}, ipAddress, userAgent)
	return int(id), nil
}

func (s *DeviceInstallationService) Update(id int, in UpdateInstallationInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	in.LocationInstalled = ToTitleCaseWithAbbr(in.LocationInstalled)
	in.Notes = SanitizeText(in.Notes)

	oldRow, _ := s.repo.GetByID(id)
	oldVals := map[string]any{}
	newVals := map[string]any{}
	if oldRow != nil {
		if oldRow.LocationInstalled != in.LocationInstalled { oldVals["location"] = oldRow.LocationInstalled; newVals["location"] = in.LocationInstalled }
		if oldRow.Notes != in.Notes { oldVals["notes"] = oldRow.Notes; newVals["notes"] = in.Notes }
	}

	err := s.repo.Update(id, in.LocationInstalled,
		parseNullableDate(in.InstallationStartDate), parseNullableDate(in.InstallationFinishDate),
		in.Photo, in.Notes)
	if err != nil {
		s.log.LogUpdate(actorID, actorUsername, actorRole, "device_installation", id,
			oldVals, nil, ipAddress, userAgent, err.Error())
		return err
	}
	s.log.LogUpdate(actorID, actorUsername, actorRole, "device_installation", id,
		oldVals, newVals, ipAddress, userAgent)
	return nil
}

func (s *DeviceInstallationService) Delete(id int, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	err := s.repo.Delete(id)
	if err != nil {
		s.log.LogDelete(actorID, actorUsername, actorRole, "device_installation", id,
			map[string]any{"id": id}, ipAddress, userAgent, err.Error())
		return err
	}
	s.log.LogDelete(actorID, actorUsername, actorRole, "device_installation", id,
		map[string]any{"id": id}, ipAddress, userAgent)
	return nil
}

func (s *DeviceInstallationService) BatchDelete(ids []int, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	for _, id := range ids {
		if err := s.repo.Delete(id); err != nil {
			s.log.LogDelete(actorID, actorUsername, actorRole, "device_installation", 0,
				map[string]any{"action": "batch_delete", "count": len(ids), "ids": ids},
				ipAddress, userAgent, err.Error())
			return err
		}
	}
	s.log.LogDelete(actorID, actorUsername, actorRole, "device_installation", 0,
		map[string]any{"action": "batch_delete", "count": len(ids), "ids": ids},
		ipAddress, userAgent)
	return nil
}
