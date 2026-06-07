package services

import (
	"strconv"
	"strings"

	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/repository"
)

type SoftwareCreateInput struct {
	Name, Category, Description string
}

type SoftwareService struct {
	repo *repository.SoftwareRepository
	log  *ActivityLogService
}

func NewSoftwareService(repo *repository.SoftwareRepository, log *ActivityLogService) *SoftwareService {
	return &SoftwareService{repo: repo, log: log}
}

func (s *SoftwareService) List(search, filterCategory string) ([]repository.SoftwareStat, error) {
	return s.repo.List(search, filterCategory)
}

func (s *SoftwareService) ListPaginated(search, filterCategory, sortBy string, page, pageSize int) ([]repository.SoftwareStat, int, error) {
	return s.repo.ListPaginated(search, filterCategory, sortBy, page, pageSize)
}

func (s *SoftwareService) GetOtherCatalog() ([]repository.SoftwareItem, error) {
	return s.repo.GetOtherCatalog()
}

func (s *SoftwareService) GetByID(id int) (*models.SoftwareCatalog, error) {
	return s.repo.GetByID(id)
}

func (s *SoftwareService) GetBySlug(slug string) (*models.SoftwareCatalog, error) {
	return s.repo.GetBySlug(slug)
}

func (s *SoftwareService) GetPCInstallStatus(id int) ([]repository.PCInstallStatus, error) {
	return s.repo.GetPCInstallStatus(id)
}

func (s *SoftwareService) Export() ([]repository.SoftwareStat, error) {
	return s.repo.Export()
}

func (s *SoftwareService) Create(in SoftwareCreateInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	in.Name = ToTitleCaseWithAbbr(in.Name)
	in.Description = SanitizeText(in.Description)
	if in.Category != "required" {
		in.Category = "other"
	}

	result, err := s.repo.Create(in.Name, in.Category, in.Description)
	if err != nil {
		s.log.LogCreate(actorID, actorUsername, actorRole, "software", 0,
			map[string]any{"name": in.Name, "category": in.Category},
			ipAddress, userAgent, err.Error())
		return sanitizeDBError(err)
	}
	id, _ := result.LastInsertId()
	s.log.LogCreate(actorID, actorUsername, actorRole, "software", int(id),
		map[string]any{"name": in.Name, "category": in.Category, "description": in.Description},
		ipAddress, userAgent)
	return nil
}

func (s *SoftwareService) Update(id int, name, category, description string, pcIDs []string, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	name = ToTitleCaseWithAbbr(name)
	description = SanitizeText(description)
	if category != "required" {
		category = "other"
	}

	old, _ := s.repo.GetByID(id)
	oldPCIDs, _ := s.repo.GetPCIDs(id)
	oldVals := map[string]any{}
	newVals := map[string]any{}
	if old != nil {
		if old.Name != name { oldVals["name"] = old.Name; newVals["name"] = name }
		if old.Category != category { oldVals["category"] = old.Category; newVals["category"] = category }
		if old.Description != description { oldVals["description"] = old.Description; newVals["description"] = description }
	}

	var newIDs []int
	for _, pidStr := range pcIDs {
		pid := 0
		for _, c := range pidStr {
			if c >= '0' && c <= '9' {
				pid = pid*10 + int(c-'0')
			}
		}
		if pid > 0 {
			newIDs = append(newIDs, pid)
		}
	}
	pcIDsChanged := len(oldPCIDs) != len(newIDs)
	if !pcIDsChanged {
		for i := range oldPCIDs {
			if oldPCIDs[i] != newIDs[i] {
				pcIDsChanged = true
				break
			}
		}
	}
	if pcIDsChanged {
		oldPCStrs := make([]string, len(oldPCIDs))
		for i, pid := range oldPCIDs {
			oldPCStrs[i] = strconv.Itoa(pid)
		}
		oldVals["pc_ids"] = oldPCStrs
		newVals["pc_ids"] = pcIDs
	}

	if err := s.repo.UpdateMetadata(id, name, category, description); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") {
			s.log.LogUpdate(actorID, actorUsername, actorRole, "software", id,
				oldVals, nil, ipAddress, userAgent, "Duplicate: "+name)
			return err
		}
		s.log.LogUpdate(actorID, actorUsername, actorRole, "software", id,
			oldVals, nil, ipAddress, userAgent, err.Error())
		return err
	}

	if err := s.repo.UpdateSoftwarePCs(id, newIDs); err != nil {
		s.log.LogUpdate(actorID, actorUsername, actorRole, "software", id,
			oldVals, nil, ipAddress, userAgent, err.Error())
		return err
	}

	s.log.LogUpdate(actorID, actorUsername, actorRole, "software", id,
		oldVals, newVals, ipAddress, userAgent)
	return nil
}

func (s *SoftwareService) Delete(id int, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	name, _ := s.repo.GetName(id)

	if err := s.repo.Delete(id); err != nil {
		s.log.LogDelete(actorID, actorUsername, actorRole, "software", id,
			map[string]any{"name": name}, ipAddress, userAgent, err.Error())
		return err
	}
	s.log.LogDelete(actorID, actorUsername, actorRole, "software", id,
		map[string]any{"name": name}, ipAddress, userAgent)
	return nil
}

func (s *SoftwareService) BatchDelete(ids []int, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	for _, id := range ids {
		if err := s.Delete(id, actorID, actorUsername, actorRole, ipAddress, userAgent); err != nil {
			return err
		}
	}
	return nil
}

