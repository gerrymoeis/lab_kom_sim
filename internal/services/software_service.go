package services

import (
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

func (s *SoftwareService) GetPCInstallStatus(id int) ([]repository.PCInstallStatus, error) {
	return s.repo.GetPCInstallStatus(id)
}

func (s *SoftwareService) Export() ([]repository.SoftwareStat, error) {
	return s.repo.Export()
}

func (s *SoftwareService) Create(in SoftwareCreateInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	if in.Category != "required" {
		in.Category = "other"
	}

	_, err := s.repo.Create(in.Name, in.Category, in.Description)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") {
			s.log.LogCreate(actorID, actorUsername, actorRole, "software", 0,
				map[string]any{"name": in.Name, "category": in.Category},
				ipAddress, userAgent, "Duplicate: "+in.Name)
			return err
		}
		s.log.LogCreate(actorID, actorUsername, actorRole, "software", 0,
			map[string]any{"name": in.Name, "category": in.Category},
			ipAddress, userAgent, err.Error())
		return err
	}
	s.log.LogCreate(actorID, actorUsername, actorRole, "software", 0,
		map[string]any{"name": in.Name, "category": in.Category, "description": in.Description},
		ipAddress, userAgent)
	return nil
}

func (s *SoftwareService) Update(id int, name, category, description string, pcIDs []string, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if category != "required" {
		category = "other"
	}

	if err := s.repo.UpdateMetadata(id, name, category, description); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") {
			s.log.LogUpdate(actorID, actorUsername, actorRole, "software", 0,
				map[string]any{"software_id": id}, nil, ipAddress, userAgent, "Duplicate: "+name)
			return err
		}
		s.log.LogUpdate(actorID, actorUsername, actorRole, "software", 0,
			map[string]any{"software_id": id}, nil, ipAddress, userAgent, err.Error())
		return err
	}

	var ids []int
	for _, pidStr := range pcIDs {
		pid := 0
		for _, c := range pidStr {
			if c >= '0' && c <= '9' {
				pid = pid*10 + int(c-'0')
			}
		}
		if pid > 0 {
			ids = append(ids, pid)
		}
	}
	if err := s.repo.UpdateSoftwarePCs(id, ids); err != nil {
		s.log.LogUpdate(actorID, actorUsername, actorRole, "software", 0,
			map[string]any{"name": name, "category": category}, nil, ipAddress, userAgent, err.Error())
		return err
	}
	s.log.LogUpdate(actorID, actorUsername, actorRole, "software", 0,
		map[string]any{"name": name, "category": category},
		map[string]any{"pc_ids": pcIDs}, ipAddress, userAgent)
	return nil
}

func (s *SoftwareService) Delete(id int, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	name, _ := s.repo.GetName(id)

	if err := s.repo.Delete(id); err != nil {
		s.log.LogDelete(actorID, actorUsername, actorRole, "software", 0,
			map[string]any{"name": name}, ipAddress, userAgent, err.Error())
		return err
	}
	s.log.LogDelete(actorID, actorUsername, actorRole, "software", 0,
		map[string]any{"name": name}, ipAddress, userAgent)
	return nil
}
