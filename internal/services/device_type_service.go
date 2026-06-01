package services

import (
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/repository"
)

type DeviceTypeCreateInput struct {
	CategoryID      int
	Name            string
	Brand           string
	Model           string
	AssetCodePrefix string
	UsageType       string
	DefaultLocation string
	Photo           string
}

type DeviceTypeUpdateInput struct {
	CategoryID      int
	Name            string
	Brand           string
	Model           string
	AssetCodePrefix string
	UsageType       string
	DefaultLocation string
	Photo           string
}

type DeviceTypeService struct {
	repo *repository.DeviceTypeRepository
	log  *ActivityLogService
}

func NewDeviceTypeService(repo *repository.DeviceTypeRepository, log *ActivityLogService) *DeviceTypeService {
	return &DeviceTypeService{repo: repo, log: log}
}

func (s *DeviceTypeService) Create(in DeviceTypeCreateInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) (int, error) {
	result, err := s.repo.Create(in.CategoryID, in.Name, in.Brand, in.Model, in.AssetCodePrefix, in.UsageType, in.DefaultLocation, in.Photo)
	if err != nil {
		s.log.LogCreate(actorID, actorUsername, actorRole, "device_type", 0,
			map[string]any{"name": in.Name}, ipAddress, userAgent, err.Error())
		return 0, sanitizeDBError(err)
	}
	id, _ := result.LastInsertId()
	s.log.LogCreate(actorID, actorUsername, actorRole, "device_type", int(id),
		map[string]any{"name": in.Name, "category_id": in.CategoryID, "usage_type": in.UsageType},
		ipAddress, userAgent)
	return int(id), nil
}

func (s *DeviceTypeService) Update(id int, in DeviceTypeUpdateInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	err := s.repo.Update(id, in.CategoryID, in.Name, in.Brand, in.Model, in.AssetCodePrefix, in.UsageType, in.DefaultLocation, in.Photo)
	if err != nil {
		s.log.LogUpdate(actorID, actorUsername, actorRole, "device_type", 0,
			map[string]any{"id": id}, nil, ipAddress, userAgent, err.Error())
		return sanitizeDBError(err)
	}
	s.log.LogUpdate(actorID, actorUsername, actorRole, "device_type", 0,
		map[string]any{"id": id},
		map[string]any{"name": in.Name, "category_id": in.CategoryID},
		ipAddress, userAgent)
	return nil
}

func (s *DeviceTypeService) Delete(id int, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	err := s.repo.Delete(id)
	if err != nil {
		s.log.LogDelete(actorID, actorUsername, actorRole, "device_type", 0,
			map[string]any{"id": id}, ipAddress, userAgent, err.Error())
		return sanitizeDBError(err)
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
	return s.repo.List(category, search)
}

func (s *DeviceTypeService) ListPaginated(category, search, sortBy string, page, pageSize int) ([]models.DeviceType, int, error) {
	return s.repo.ListPaginated(category, search, sortBy, page, pageSize)
}

func (s *DeviceTypeService) GetByCategoryID(categoryID int) ([]models.DeviceType, error) {
	return s.repo.GetByCategoryID(categoryID)
}

func (s *DeviceTypeService) CountByCategoryID(categoryID int) (int, error) {
	return s.repo.CountByCategoryID(categoryID)
}
