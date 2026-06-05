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
	IsMahasiswa                         bool
	PurchaseDate, LastChecked           string
	Notes                               string
}

type UpdatePCInput struct {
	Row, Column                         int
	Status, Placement                   string
	SerialNumber                        string
	PCType, BrandModel, Accessories     string
	Processor, RAM, Storage             string
	OperatingSystem, Notes              string
	PhotoSerial, PhotoFront             string
	RequiredSW, OtherSW                 []int
	Label                               string
	PurchaseDate, LastChecked           string
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

func (s *PCService) GetDistinctOS() ([]string, error) {
	return s.pcRepo.GetDistinctOS()
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
	in.Processor = SanitizeText(in.Processor)
	in.RAM = SanitizeText(in.RAM)
	in.Storage = SanitizeText(in.Storage)
	in.SerialNumber = SanitizeText(in.SerialNumber)
	in.OperatingSystem = SanitizeText(in.OperatingSystem)
	in.PCType = SanitizeText(in.PCType)
	in.BrandModel = SanitizeText(in.BrandModel)
	in.Accessories = SanitizeText(in.Accessories)
	in.Notes = SanitizeText(in.Notes)
	if in.Label == "" {
		in.Label = s.pcRepo.NextLabel(in.Placement, in.IsMahasiswa)
	}

	result, err := s.pcRepo.Create(in.Row, in.Column, in.Status, in.Placement, in.Processor, in.RAM, in.Storage,
		in.SerialNumber, in.OperatingSystem, in.PCType, in.BrandModel, in.Accessories, in.PhotoSerial, in.PhotoFront, in.Label,
		in.PurchaseDate, in.LastChecked, in.Notes)
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
		if in.Placement != "cadangan" {
			s.pcRepo.SeedRequiredSoftware(int(pcID))
		}
	}
	return int(pcID), nil
}

func (s *PCService) NextLabel(placement string, isMahasiswa bool) string {
	return s.pcRepo.NextLabel(placement, isMahasiswa)
}

func (s *PCService) UpdatePC(label string, in UpdatePCInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	if in.Label == "" {
		in.Label = label
	}
	in.Processor = SanitizeText(in.Processor)
	in.RAM = SanitizeText(in.RAM)
	in.Storage = SanitizeText(in.Storage)
	in.SerialNumber = SanitizeText(in.SerialNumber)
	in.OperatingSystem = SanitizeText(in.OperatingSystem)
	in.PCType = SanitizeText(in.PCType)
	in.BrandModel = SanitizeText(in.BrandModel)
	in.Accessories = SanitizeText(in.Accessories)
	in.Notes = SanitizeText(in.Notes)
	err := s.pcRepo.Update(label, in.Row, in.Column, in.Status, in.Placement, in.PCType, in.SerialNumber, in.BrandModel, in.Accessories,
		in.Processor, in.RAM, in.Storage, in.OperatingSystem, in.Notes, in.PhotoSerial, in.PhotoFront, in.Label,
		in.PurchaseDate, in.LastChecked)
	if err != nil {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", 0,
			map[string]any{"label": label}, nil, ipAddress, userAgent, err.Error())
		return err
	}
	pc, _ := s.pcRepo.GetByLabel(in.Label)
	pcID := 0
	if pc != nil { pcID = pc.ID }
	s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", pcID,
		map[string]any{"label": label},
		map[string]any{"status": in.Status, "placement": in.Placement, "operating_system": in.OperatingSystem},
		ipAddress, userAgent)
	return nil
}

func (s *PCService) DeletePC(label string, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	pc, _ := s.pcRepo.GetByLabel(label)
	pcID := 0
	if pc != nil { pcID = pc.ID }
	if err := s.pcRepo.DeleteByLabel(label); err != nil {
		s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "pc", pcID,
			map[string]any{"label": label}, ipAddress, userAgent, err.Error())
		return err
	}
	s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "pc", pcID,
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

func (s *PCService) SwapPCs(labelA, labelB string, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	pcA, _ := s.pcRepo.GetByLabel(labelA)
	pcID := 0
	if pcA != nil { pcID = pcA.ID }
	if err := s.pcRepo.SwapLabels(labelA, labelB); err != nil {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", pcID,
			map[string]any{"operation": "swap"}, nil, ipAddress, userAgent, err.Error())
		return err
	}
	s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", pcID,
		map[string]any{"operation": "swap", "a": labelA, "b": labelB},
		map[string]any{"status": "swapped"}, ipAddress, userAgent)
	return nil
}

func (s *PCService) ReplacePC(target, spare string, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	pc, _ := s.pcRepo.GetByLabel(target)
	pcID := 0
	if pc != nil { pcID = pc.ID }
	if err := s.pcRepo.ReplaceWithSpare(target, spare); err != nil {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", pcID,
			map[string]any{"operation": "replace"}, nil, ipAddress, userAgent, err.Error())
		return err
	}
	// Seed required software for the newly placed PC
	if pc, _ := s.pcRepo.GetByLabel(target); pc != nil {
		_ = s.pcRepo.SeedRequiredSoftware(pc.ID)
		pcID = pc.ID
	}
	s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", pcID,
		map[string]any{"operation": "replace", "target": target, "spare": spare},
		map[string]any{"status": "replaced"}, ipAddress, userAgent)
	return nil
}

func (s *PCService) MoveRowToCadangan(row int, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	if err := s.pcRepo.MoveRowToCadangan(row); err != nil {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", 0,
			map[string]any{"operation": "move_row", "row": row}, nil, ipAddress, userAgent, err.Error())
		return err
	}
	s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", 0,
		map[string]any{"operation": "move_row", "row": row},
		map[string]any{"status": "moved"}, ipAddress, userAgent)
	return nil
}

func (s *PCService) MovePC(label string, row, col int, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	pc, _ := s.pcRepo.GetByLabel(label)
	pcID := 0
	if pc != nil { pcID = pc.ID }
	if err := s.pcRepo.MoveToPosition(label, row, col); err != nil {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", pcID,
			map[string]any{"operation": "move"}, nil, ipAddress, userAgent, err.Error())
		return err
	}
	s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", pcID,
		map[string]any{"operation": "move", "label": label, "row": row, "col": col},
		map[string]any{"status": "moved"}, ipAddress, userAgent)
	return nil
}

func (s *PCService) PlaceCadangan(label string, row, col int, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	pc, _ := s.pcRepo.GetByLabel(label)
	pcID := 0
	if pc != nil { pcID = pc.ID }
	if err := s.pcRepo.PlaceCadangan(label, row, col); err != nil {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", pcID,
			map[string]any{"operation": "place"}, nil, ipAddress, userAgent, err.Error())
		return err
	}
	s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", pcID,
		map[string]any{"operation": "place", "label": label, "row": row, "col": col},
		map[string]any{"status": "placed"}, ipAddress, userAgent)
	return nil
}

func (s *PCService) MoveToCadangan(label string, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	pc, _ := s.pcRepo.GetByLabel(label)
	pcID := 0
	if pc != nil { pcID = pc.ID }
	if err := s.pcRepo.MoveToCadangan(label); err != nil {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", pcID,
			map[string]any{"operation": "move-to-cadangan"}, nil, ipAddress, userAgent, err.Error())
		return err
	}
	s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", pcID,
		map[string]any{"operation": "move-to-cadangan", "label": label},
		map[string]any{"status": "cadangan"}, ipAddress, userAgent)
	return nil
}

func (s *PCService) SyncSoftware(label string, requiredIDs []string, otherNames, otherDescs []string,
	actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {

	pc, _ := s.pcRepo.GetByLabel(label)
	pcID := 0
	if pc != nil { pcID = pc.ID }

	if err := s.pcRepo.SyncSoftware(pcID, requiredIDs, otherNames, otherDescs); err != nil {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "software", pcID,
			map[string]any{"label": label}, nil, ipAddress, userAgent, err.Error())
		return err
	}

	s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "software", pcID,
		map[string]any{"label": label},
		map[string]any{"required_ids": requiredIDs, "other_names": otherNames},
		ipAddress, userAgent)
	return nil
}
