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
	in.Name = ToTitleCaseWithAbbr(in.Name)
	in.Brand = ToTitleCaseWithAbbr(in.Brand)
	in.Model = ToTitleCaseWithAbbr(in.Model)
	in.AssetCodePrefix = ToUpperTrim(in.AssetCodePrefix)
	in.DefaultLocation = ToTitleCaseWithAbbr(in.DefaultLocation)
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
	in.Name = ToTitleCaseWithAbbr(in.Name)
	in.Brand = ToTitleCaseWithAbbr(in.Brand)
	in.Model = ToTitleCaseWithAbbr(in.Model)
	in.AssetCodePrefix = ToUpperTrim(in.AssetCodePrefix)
	in.DefaultLocation = ToTitleCaseWithAbbr(in.DefaultLocation)

	oldDT, _ := s.repo.GetByID(id)
	oldVals := map[string]any{}
	newVals := map[string]any{}
	if oldDT != nil {
		if oldDT.Name != in.Name { oldVals["name"] = oldDT.Name; newVals["name"] = in.Name }
		if oldDT.Brand != in.Brand { oldVals["brand"] = oldDT.Brand; newVals["brand"] = in.Brand }
		if oldDT.Model != in.Model { oldVals["model"] = oldDT.Model; newVals["model"] = in.Model }
		if oldDT.CategoryID != in.CategoryID { oldVals["category_id"] = oldDT.CategoryID; newVals["category_id"] = in.CategoryID }
		if oldDT.AssetCodePrefix != in.AssetCodePrefix { oldVals["asset_code_prefix"] = oldDT.AssetCodePrefix; newVals["asset_code_prefix"] = in.AssetCodePrefix }
		if oldDT.UsageType != in.UsageType { oldVals["usage_type"] = oldDT.UsageType; newVals["usage_type"] = in.UsageType }
		if oldDT.DefaultLocation != in.DefaultLocation { oldVals["default_location"] = oldDT.DefaultLocation; newVals["default_location"] = in.DefaultLocation }
	}

	err := s.repo.Update(id, in.CategoryID, in.Name, in.Brand, in.Model, in.AssetCodePrefix, in.UsageType, in.DefaultLocation, in.Photo)
	if err != nil {
		s.log.LogUpdate(actorID, actorUsername, actorRole, "device_type", id,
			oldVals, nil, ipAddress, userAgent, err.Error())
		return sanitizeDBError(err)
	}
	s.log.LogUpdate(actorID, actorUsername, actorRole, "device_type", id,
		oldVals, newVals, ipAddress, userAgent)
	return nil
}

func (s *DeviceTypeService) Delete(id int, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	err := s.repo.Delete(id)
	if err != nil {
		s.log.LogDelete(actorID, actorUsername, actorRole, "device_type", id,
			map[string]any{"id": id}, ipAddress, userAgent, err.Error())
		return sanitizeDBError(err)
	}
	s.log.LogDelete(actorID, actorUsername, actorRole, "device_type", id,
		map[string]any{"id": id}, ipAddress, userAgent)
	return nil
}

func (s *DeviceTypeService) GetByID(id int) (*models.DeviceType, error) {
	return s.repo.GetByID(id)
}

func (s *DeviceTypeService) GetBySlug(slug string) (*models.DeviceType, error) {
	return s.repo.GetBySlug(slug)
}

func (s *DeviceTypeService) GetByPrefixSlug(slug string) (*models.DeviceType, error) {
	return s.repo.GetByPrefixSlug(slug)
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

func (s *DeviceTypeService) BatchDelete(ids []int, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	for _, id := range ids {
		if err := s.Delete(id, actorID, actorUsername, actorRole, ipAddress, userAgent); err != nil {
			return err
		}
	}
	return nil
}
