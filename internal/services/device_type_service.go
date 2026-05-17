package services

import (
	"strings"

	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/repository"
)

type DeviceTypeCreateInput struct {
	Name, Category, Brand, Model, ItemType, ItemMode, AssetCodePrefix, DefaultLocation, NotesTemplate string
}

type DeviceTypeUpdateInput struct {
	Name, Category, Brand, Model, ItemType, ItemMode, AssetCodePrefix, DefaultLocation, NotesTemplate string
}

type DeviceTypeService struct {
	repo *repository.DeviceTypeRepository
	log  *ActivityLogService
}

func NewDeviceTypeService(repo *repository.DeviceTypeRepository, log *ActivityLogService) *DeviceTypeService {
	return &DeviceTypeService{repo: repo, log: log}
}

func (s *DeviceTypeService) Create(in DeviceTypeCreateInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) (int, error) {
	result, err := s.repo.Create(in.Name, in.Category, in.Brand, in.Model, in.ItemType, in.ItemMode == "loanable", in.ItemMode == "consumable", in.AssetCodePrefix, in.DefaultLocation, in.NotesTemplate)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return 0, err
		}
		s.log.LogCreate(actorID, actorUsername, actorRole, "device_type", 0,
			map[string]any{"name": in.Name}, ipAddress, userAgent, err.Error())
		return 0, err
	}
	id, _ := result.LastInsertId()
	s.log.LogCreate(actorID, actorUsername, actorRole, "device_type", int(id),
		map[string]any{"name": in.Name, "category": in.Category, "item_type": in.ItemType},
		ipAddress, userAgent)
	return int(id), nil
}

func (s *DeviceTypeService) Update(id int, in DeviceTypeUpdateInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	err := s.repo.Update(id, in.Name, in.Category, in.Brand, in.Model, in.ItemType, in.ItemMode == "loanable", in.ItemMode == "consumable", in.AssetCodePrefix, in.DefaultLocation, in.NotesTemplate)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return err
		}
		s.log.LogUpdate(actorID, actorUsername, actorRole, "device_type", 0,
			map[string]any{"id": id}, nil, ipAddress, userAgent, err.Error())
		return err
	}
	s.log.LogUpdate(actorID, actorUsername, actorRole, "device_type", 0,
		map[string]any{"id": id},
		map[string]any{"name": in.Name, "category": in.Category},
		ipAddress, userAgent)
	return nil
}

func (s *DeviceTypeService) Delete(id int, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	err := s.repo.Delete(id)
	if err != nil {
		if strings.Contains(err.Error(), "foreign key") {
			return err
		}
		s.log.LogDelete(actorID, actorUsername, actorRole, "device_type", 0,
			map[string]any{"id": id}, ipAddress, userAgent, err.Error())
		return err
	}
	s.log.LogDelete(actorID, actorUsername, actorRole, "device_type", 0,
		map[string]any{"id": id}, ipAddress, userAgent)
	return nil
}

func (s *DeviceTypeService) GetByID(id int) (*models.DeviceType, error) {
	return s.repo.GetByID(id)
}

func (s *DeviceTypeService) GetAllSimple() ([]models.DeviceType, error) {
	return s.repo.GetAllSimple()
}

func (s *DeviceTypeService) GetByIDSimple(id int) (*models.DeviceType, error) {
	return s.repo.GetByIDSimple(id)
}

func (s *DeviceTypeService) List(category, search string) ([]models.DeviceType, error) {
	types, err := s.repo.List(category)
	if err != nil {
		return nil, err
	}
	if search != "" {
		var filtered []models.DeviceType
		for _, dt := range types {
			if strings.Contains(strings.ToLower(dt.Name), strings.ToLower(search)) ||
				strings.Contains(strings.ToLower(dt.Category), strings.ToLower(search)) {
				filtered = append(filtered, dt)
			}
		}
		types = filtered
	}
	return types, nil
}
