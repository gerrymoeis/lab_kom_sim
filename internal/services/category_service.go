package services

import (
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/repository"
)

type CategoryService struct {
	repo *repository.CategoryRepository
	log  *ActivityLogService
}

func NewCategoryService(repo *repository.CategoryRepository, log *ActivityLogService) *CategoryService {
	return &CategoryService{repo: repo, log: log}
}

func (s *CategoryService) List() ([]models.Category, error) {
	return s.repo.List()
}

func (s *CategoryService) ListByUsageType(usageType string) ([]models.Category, error) {
	return s.repo.ListByUsageType(usageType)
}

func (s *CategoryService) GetByID(id int) (*models.Category, error) {
	return s.repo.GetByID(id)
}

func (s *CategoryService) GetByPrefixSlug(slug string) (*models.Category, error) {
	return s.repo.GetByPrefixSlug(slug)
}

func (s *CategoryService) Create(name, prefix string, actorID int, actorUsername, actorRole, ipAddress, userAgent string) (int, error) {
	name = ToTitleCaseWithAbbr(name)
	prefix = ToUpperTrim(prefix)
	result, err := s.repo.Create(name, prefix)
	if err != nil {
		s.log.LogCreate(actorID, actorUsername, actorRole, "category", 0,
			map[string]any{"name": name}, ipAddress, userAgent, err.Error())
		return 0, sanitizeDBError(err)
	}
	id, _ := result.LastInsertId()
	s.log.LogCreate(actorID, actorUsername, actorRole, "category", int(id),
		map[string]any{"name": name, "prefix": prefix}, ipAddress, userAgent)
	return int(id), nil
}

func (s *CategoryService) Update(id int, name, prefix string, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	name = ToTitleCaseWithAbbr(name)
	prefix = ToUpperTrim(prefix)

	oldCat, _ := s.repo.GetByID(id)
	oldVals := map[string]any{}
	newVals := map[string]any{}
	if oldCat != nil {
		if oldCat.Name != name { oldVals["name"] = oldCat.Name; newVals["name"] = name }
		if oldCat.DefaultPrefix != prefix { oldVals["prefix"] = oldCat.DefaultPrefix; newVals["prefix"] = prefix }
	}

	err := s.repo.Update(id, name, prefix)
	if err != nil {
		s.log.LogUpdate(actorID, actorUsername, actorRole, "category", id,
			oldVals, nil, ipAddress, userAgent, err.Error())
		return sanitizeDBError(err)
	}
	s.log.LogUpdate(actorID, actorUsername, actorRole, "category", id,
		oldVals, newVals, ipAddress, userAgent)
	return nil
}

func (s *CategoryService) Delete(id int, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	err := s.repo.Delete(id)
	if err != nil {
		s.log.LogDelete(actorID, actorUsername, actorRole, "category", id,
			map[string]any{"id": id}, ipAddress, userAgent, err.Error())
		return sanitizeDBError(err)
	}
	s.log.LogDelete(actorID, actorUsername, actorRole, "category", id,
		map[string]any{"id": id}, ipAddress, userAgent)
	return nil
}

func (s *CategoryService) BatchDelete(ids []int, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	for _, id := range ids {
		if err := s.repo.Delete(id); err != nil {
			s.log.LogDelete(actorID, actorUsername, actorRole, "category", 0,
				map[string]any{"action": "batch_delete", "count": len(ids), "ids": ids},
				ipAddress, userAgent, err.Error())
			return sanitizeDBError(err)
		}
	}
	s.log.LogDelete(actorID, actorUsername, actorRole, "category", 0,
		map[string]any{"action": "batch_delete", "count": len(ids), "ids": ids},
		ipAddress, userAgent)
	return nil
}
