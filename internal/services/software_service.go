package services

import (
	"strings"

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
				map[string]interface{}{"name": in.Name, "category": in.Category},
				ipAddress, userAgent, "Duplicate: "+in.Name)
			return err
		}
		s.log.LogCreate(actorID, actorUsername, actorRole, "software", 0,
			map[string]interface{}{"name": in.Name, "category": in.Category},
			ipAddress, userAgent, err.Error())
		return err
	}
	s.log.LogCreate(actorID, actorUsername, actorRole, "software", 0,
		map[string]interface{}{"name": in.Name, "category": in.Category, "description": in.Description},
		ipAddress, userAgent)
	return nil
}

func (s *SoftwareService) Update(id int, pcIDs []string, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	if err := s.repo.UpdateSoftwarePCs(id, pcIDs); err != nil {
		s.log.LogUpdate(actorID, actorUsername, actorRole, "software", 0,
			map[string]interface{}{"software_id": id}, nil, ipAddress, userAgent, err.Error())
		return err
	}
	s.log.LogUpdate(actorID, actorUsername, actorRole, "software", 0,
		map[string]interface{}{"software_id": id},
		map[string]interface{}{"pc_ids": pcIDs}, ipAddress, userAgent)
	return nil
}

func (s *SoftwareService) Delete(id int, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	name, _ := s.repo.GetName(id)

	if err := s.repo.Delete(id); err != nil {
		s.log.LogDelete(actorID, actorUsername, actorRole, "software", 0,
			map[string]interface{}{"name": name}, ipAddress, userAgent, err.Error())
		return err
	}
	s.log.LogDelete(actorID, actorUsername, actorRole, "software", 0,
		map[string]interface{}{"name": name}, ipAddress, userAgent)
	return nil
}
