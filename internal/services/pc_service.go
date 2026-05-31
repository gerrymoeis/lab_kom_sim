package services

import (
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/repository"
)

type CreatePCInput struct {
	Row, Column                         int
	Status, Placement                   string
	Processor, RAM, Storage             string
	SerialNumber, OperatingSystem       string
	PCType, BrandModel, Accessories     string
	PhotoSerial, PhotoFront             string
	Label                               string
}

type UpdatePCInput struct {
	Status, Placement                   string
	SerialNumber                        string
	PCType, BrandModel, Accessories     string
	Processor, RAM, Storage             string
	OperatingSystem, Notes              string
	PhotoSerial, PhotoFront             string
	RequiredSW, OtherSW                 []int
	Label                               string
}

type PCService struct {
	pcRepo             *repository.PCRepository
	activityLogService *ActivityLogService
}

func NewPCService(pcRepo *repository.PCRepository, activityLogService *ActivityLogService) *PCService {
	return &PCService{pcRepo: pcRepo, activityLogService: activityLogService}
}

func (s *PCService) List(filters repository.PCFilters) ([]models.PC, error) {
	return s.pcRepo.List(filters)
}

func (s *PCService) ListPaginated(filters repository.PCFilters, page, pageSize int) ([]models.PC, int, error) {
	return s.pcRepo.ListPaginated(filters, page, pageSize)
}

func (s *PCService) GetByLabel(label string) (*models.PC, error) {
	return s.pcRepo.GetByLabel(label)
}

func (s *PCService) GetByLabelEdit(label string) (*models.PC, error) {
	return s.pcRepo.GetByLabelEdit(label)
}

func (s *PCService) GetSoftware(pcID int) (requiredSW, otherSW []models.PCSoftware, err error) {
	return s.pcRepo.GetSoftware(pcID)
}

func (s *PCService) ExportAll() ([]models.PC, error) {
	return s.pcRepo.ExportAll()
}

func (s *PCService) CreatePC(in CreatePCInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) (int, error) {
	if in.Status == "" { in.Status = "normal" }
	if in.Placement == "" { in.Placement = "dipakai" }
	if in.PCType == "" { in.PCType = "PC All-in-one" }
	if in.BrandModel == "" { in.BrandModel = "Axioo Mypc One Pro K7-24 (16N9)" }
	if in.Accessories == "" { in.Accessories = "Keyboard & Mouse Axioo (Wired Set)" }
	if in.Processor == "" { in.Processor = "Intel Core i7" }
	if in.RAM == "" { in.RAM = "16GB DDR4" }
	if in.Storage == "" { in.Storage = "1TB NVMe" }
	if in.Label == "" {
		in.Label = s.pcRepo.NextLabel(in.Placement, in.PCType)
	}

	result, err := s.pcRepo.Create(in.Row, in.Column, in.Status, in.Placement, in.Processor, in.RAM, in.Storage,
		in.SerialNumber, in.OperatingSystem, in.PCType, in.BrandModel, in.Accessories, in.PhotoSerial, in.PhotoFront, in.Label)
	if err != nil {
		s.activityLogService.LogCreate(actorID, actorUsername, actorRole, "pc", 0,
			map[string]any{"label": in.Label, "serial_number": in.SerialNumber},
			ipAddress, userAgent, err.Error())
		return 0, err
	}
	pcID, _ := result.LastInsertId()
	if pcID > 0 {
		s.activityLogService.LogCreate(actorID, actorUsername, actorRole, "pc", int(pcID),
			map[string]any{"label": in.Label, "serial_number": in.SerialNumber, "operating_system": in.OperatingSystem},
			ipAddress, userAgent)
		s.pcRepo.SeedRequiredSoftware(int(pcID))
	}
	return int(pcID), nil
}

func (s *PCService) UpdatePC(label string, in UpdatePCInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	if in.Label == "" {
		in.Label = label
	}
	err := s.pcRepo.Update(label, in.Status, in.Placement, in.PCType, in.SerialNumber, in.BrandModel, in.Accessories,
		in.Processor, in.RAM, in.Storage, in.OperatingSystem, in.Notes, in.PhotoSerial, in.PhotoFront, in.Label)
	if err != nil {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", 0,
			map[string]any{"label": label}, nil, ipAddress, userAgent, err.Error())
		return err
	}
	s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", 0,
		map[string]any{"label": label},
		map[string]any{"status": in.Status, "placement": in.Placement, "operating_system": in.OperatingSystem},
		ipAddress, userAgent)
	return nil
}

func (s *PCService) DeletePC(label string, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	if err := s.pcRepo.DeleteByLabel(label); err != nil {
		s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "pc", 0,
			map[string]any{"label": label}, ipAddress, userAgent, err.Error())
		return err
	}
	s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "pc", 0,
		map[string]any{"label": label}, ipAddress, userAgent)
	return nil
}

func (s *PCService) UpdateStatus(id int, status string, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	oldStatus, _ := s.pcRepo.GetStatus(id)

	if err := s.pcRepo.UpdateStatus(id, status); err != nil {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", id,
			map[string]any{"pc_id": id}, nil, ipAddress, userAgent, err.Error())
		return err
	}

	s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", id,
		map[string]any{"pc_id": id},
		map[string]any{"old_status": oldStatus, "new_status": status},
		ipAddress, userAgent)
	return nil
}

func (s *PCService) SyncSoftware(label string, requiredIDs []string, otherNames, otherDescs []string,
	actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {

	pc, _ := s.pcRepo.GetByLabel(label)
	pcID := 0
	if pc != nil { pcID = pc.ID }

	if err := s.pcRepo.SyncSoftware(pcID, requiredIDs, otherNames, otherDescs); err != nil {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "software", 0,
			map[string]any{"label": label}, nil, ipAddress, userAgent, err.Error())
		return err
	}

	s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "software", 0,
		map[string]any{"label": label},
		map[string]any{"required_ids": requiredIDs, "other_names": otherNames},
		ipAddress, userAgent)
	return nil
}
