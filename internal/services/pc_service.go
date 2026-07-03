package services

import (
	"fmt"
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/repository"
)

type CreatePCInput struct {
	Row, Column                         int
	Status, Placement                   string
	Processor, RAM, Storage             string
	SerialNumber, OperatingSystem       string
	PCType, BrandModel, Accessories     string
	PcBrand, MouseBrand, KeyboardBrand  string
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
	PcBrand, MouseBrand, KeyboardBrand  string
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
	in.PcBrand = SanitizeText(in.PcBrand)
	in.MouseBrand = SanitizeText(in.MouseBrand)
	in.KeyboardBrand = SanitizeText(in.KeyboardBrand)
	in.Notes = SanitizeText(in.Notes)
	if in.Label == "" {
		in.Label = s.pcRepo.NextLabel(in.Placement, in.IsMahasiswa)
	}

	if in.Placement == "dipakai" && !isNumericLabel(in.Label) && in.Row == 0 && in.Column == 0 {
		in.Column = s.pcRepo.NextSpecialCol()
	}

	result, err := s.pcRepo.Create(in.Row, in.Column, in.Status, in.Placement, in.Processor, in.RAM, in.Storage,
		in.SerialNumber, in.OperatingSystem, in.PCType, in.BrandModel, in.PcBrand, in.MouseBrand, in.KeyboardBrand, in.Accessories, in.PhotoSerial, in.PhotoFront, in.Label,
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
	in.PcBrand = SanitizeText(in.PcBrand)
	in.MouseBrand = SanitizeText(in.MouseBrand)
	in.KeyboardBrand = SanitizeText(in.KeyboardBrand)
	in.Notes = SanitizeText(in.Notes)

	pcData, _ := s.pcRepo.GetByLabel(label)

	err := s.pcRepo.Update(label, in.Row, in.Column, in.Status, in.Placement, in.PCType, in.SerialNumber, in.BrandModel, in.PcBrand, in.MouseBrand, in.KeyboardBrand, in.Accessories,
		in.Processor, in.RAM, in.Storage, in.OperatingSystem, in.Notes, in.PhotoSerial, in.PhotoFront, in.Label,
		in.PurchaseDate, in.LastChecked)
	if err != nil {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", 0,
			map[string]any{"label": label}, nil, ipAddress, userAgent, err.Error())
		return err
	}

	pcID := 0
	oldVals := map[string]any{}
	newVals := map[string]any{}
	if pcData != nil {
		pcID = pcData.ID
		if label != in.Label { oldVals["label"] = label; newVals["label"] = in.Label }
		if pcData.Status != in.Status { oldVals["status"] = pcData.Status; newVals["status"] = in.Status }
		if pcData.Placement != in.Placement { oldVals["placement"] = pcData.Placement; newVals["placement"] = in.Placement }
		if pcData.Processor != in.Processor { oldVals["processor"] = pcData.Processor; newVals["processor"] = in.Processor }
		if pcData.RAM != in.RAM { oldVals["ram"] = pcData.RAM; newVals["ram"] = in.RAM }
		if pcData.Storage != in.Storage { oldVals["storage"] = pcData.Storage; newVals["storage"] = in.Storage }
		if pcData.SerialNumber != in.SerialNumber { oldVals["serial_number"] = pcData.SerialNumber; newVals["serial_number"] = in.SerialNumber }
		if pcData.OperatingSystem != in.OperatingSystem { oldVals["operating_system"] = pcData.OperatingSystem; newVals["operating_system"] = in.OperatingSystem }
		if pcData.PCType != in.PCType { oldVals["pc_type"] = pcData.PCType; newVals["pc_type"] = in.PCType }
		if pcData.BrandModel != in.BrandModel { oldVals["brand_model"] = pcData.BrandModel; newVals["brand_model"] = in.BrandModel }
		if pcData.Accessories != in.Accessories { oldVals["accessories"] = pcData.Accessories; newVals["accessories"] = in.Accessories }
		if pcData.PcBrand != in.PcBrand { oldVals["pc_brand"] = pcData.PcBrand; newVals["pc_brand"] = in.PcBrand }
		if pcData.MouseBrand != in.MouseBrand { oldVals["mouse_brand"] = pcData.MouseBrand; newVals["mouse_brand"] = in.MouseBrand }
		if pcData.KeyboardBrand != in.KeyboardBrand { oldVals["keyboard_brand"] = pcData.KeyboardBrand; newVals["keyboard_brand"] = in.KeyboardBrand }
		if pcData.Row != in.Row { oldVals["row"] = pcData.Row; newVals["row"] = in.Row }
		if pcData.Column != in.Column { oldVals["column"] = pcData.Column; newVals["column"] = in.Column }
		if pcData.Notes != in.Notes { oldVals["notes"] = pcData.Notes; newVals["notes"] = in.Notes }
	}
	if len(oldVals) > 0 || len(newVals) > 0 {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", pcID,
			oldVals, newVals, ipAddress, userAgent)
	}
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
			map[string]any{"status": oldStatus}, nil, ipAddress, userAgent, err.Error())
		return err
	}

	s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", id,
		map[string]any{"status": oldStatus},
		map[string]any{"status": status},
		ipAddress, userAgent)
	return nil
}

func (s *PCService) SwapPCs(labelA, labelB string, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	pcA, _ := s.pcRepo.GetByLabel(labelA)
	pcB, _ := s.pcRepo.GetByLabel(labelB)
	pcID := 0
	oldVals := map[string]any{}
	newVals := map[string]any{}
	if pcA != nil && pcB != nil {
		pcID = pcA.ID
		oldVals["a_label"] = labelA; oldVals["a_row"] = pcA.Row; oldVals["a_column"] = pcA.Column
		oldVals["b_label"] = labelB; oldVals["b_row"] = pcB.Row; oldVals["b_column"] = pcB.Column
		newVals["a_label"] = labelA; newVals["a_row"] = pcB.Row; newVals["a_column"] = pcB.Column
		newVals["b_label"] = labelB; newVals["b_row"] = pcA.Row; newVals["b_column"] = pcA.Column
	}
	if err := s.pcRepo.SwapLabels(labelA, labelB); err != nil {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", pcID,
			map[string]any{"operation": "swap"}, nil, ipAddress, userAgent, err.Error())
		return err
	}
	s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", pcID,
		oldVals, newVals, ipAddress, userAgent)
	return nil
}

func (s *PCService) ReplacePC(target, spare string, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	pcT, _ := s.pcRepo.GetByLabel(target)
	pcS, _ := s.pcRepo.GetByLabel(spare)
	pcID := 0
	oldVals := map[string]any{}
	newVals := map[string]any{}
	if pcT != nil && pcS != nil {
		pcID = pcT.ID
		oldVals["target_label"] = target; oldVals["target_row"] = pcT.Row; oldVals["target_column"] = pcT.Column
		oldVals["spare_label"] = spare; oldVals["spare_row"] = pcS.Row; oldVals["spare_column"] = pcS.Column
		newVals["target_label"] = target; newVals["target_row"] = 0; newVals["target_column"] = 0
		newVals["spare_label"] = spare; newVals["spare_row"] = pcT.Row; newVals["spare_column"] = pcT.Column
	}
	if err := s.pcRepo.ReplaceWithSpare(target, spare); err != nil {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", pcID,
			map[string]any{"operation": "replace"}, nil, ipAddress, userAgent, err.Error())
		return err
	}
	// Seed required software for the newly placed PC
	if pcT, _ := s.pcRepo.GetByLabel(target); pcT != nil {
		s.pcRepo.SeedMissingRequiredSoftware(pcT.ID)
		pcID = pcT.ID
	}
	s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", pcID,
		oldVals, newVals, ipAddress, userAgent)
	return nil
}

func (s *PCService) MoveRowToCadangan(row int, actorID int, actorUsername, actorRole, ipAddress, userAgent string) (map[string]string, error) {
	oldVals := map[string]any{"operation": "move_row", "row": row, "status": "active"}
	newVals := map[string]any{"operation": "move_row", "row": row, "status": "moved_to_cadangan"}
	labelMap, err := s.pcRepo.MoveRowToCadangan(row)
	if err != nil {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", 0,
			oldVals, nil, ipAddress, userAgent, err.Error())
		return nil, err
	}
	if labelMap != nil {
		newVals["labels"] = labelMap
	}
	s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", 0,
		oldVals, newVals, ipAddress, userAgent)
	return labelMap, nil
}

func (s *PCService) MovePC(label string, row, col int, newLabel string, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	pc, _ := s.pcRepo.GetByLabel(label)
	pcID := 0
	oldVals := map[string]any{}
	newVals := map[string]any{}
	if pc != nil {
		pcID = pc.ID
		oldVals["label"] = label; oldVals["row"] = pc.Row; oldVals["column"] = pc.Column
	}
	newVals["label"] = newLabel; newVals["row"] = row; newVals["column"] = col
	if err := s.pcRepo.MoveToPosition(label, row, col, newLabel); err != nil {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", pcID,
			oldVals, nil, ipAddress, userAgent, err.Error())
		return err
	}
	s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", pcID,
		oldVals, newVals, ipAddress, userAgent)
	return nil
}

func (s *PCService) PlaceCadangan(label string, row, col int, newLabel string, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	pc, _ := s.pcRepo.GetByLabel(label)
	pcID := 0
	oldVals := map[string]any{}
	newVals := map[string]any{}
	if pc != nil {
		pcID = pc.ID
		oldVals["label"] = label; oldVals["row"] = pc.Row; oldVals["column"] = pc.Column
	}
	newVals["label"] = newLabel; newVals["row"] = row; newVals["column"] = col
	if err := s.pcRepo.PlaceCadangan(label, row, col, newLabel); err != nil {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", pcID,
			oldVals, nil, ipAddress, userAgent, err.Error())
		return err
	}
	s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", pcID,
		oldVals, newVals, ipAddress, userAgent)
	return nil
}

func (s *PCService) MoveToCadangan(label string, actorID int, actorUsername, actorRole, ipAddress, userAgent string) (string, error) {
	pc, _ := s.pcRepo.GetByLabel(label)
	pcID := 0
	oldVals := map[string]any{}
	newVals := map[string]any{}
	if pc != nil {
		pcID = pc.ID
		oldVals["label"] = label; oldVals["row"] = pc.Row; oldVals["column"] = pc.Column
		oldVals["status"] = pc.Status; oldVals["placement"] = pc.Placement
	}
	newVals["label"] = label; newVals["row"] = 0; newVals["column"] = 0
	newVals["status"] = "cadangan"; newVals["placement"] = "cadangan"
	newLabel, err := s.pcRepo.MoveToCadangan(label)
	if err != nil {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", pcID,
			oldVals, nil, ipAddress, userAgent, err.Error())
		return "", err
	}
	newVals["label"] = newLabel
	s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", pcID,
		oldVals, newVals, ipAddress, userAgent)
	return newLabel, nil
}

func (s *PCService) SyncSoftware(label string, requiredIDs []string, otherNames, otherDescs []string,
	actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {

	pc, _ := s.pcRepo.GetByLabel(label)
	pcID := 0
	if pc != nil { pcID = pc.ID }

	oldVals := map[string]any{"label": label, "action": "sync_software"}
	newVals := map[string]any{"label": label, "action": "sync_software", "required_ids": requiredIDs, "other_names": otherNames}
	if err := s.pcRepo.SyncSoftware(pcID, requiredIDs, otherNames, otherDescs); err != nil {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "software", pcID,
			oldVals, nil, ipAddress, userAgent, err.Error())
		return err
	}

	s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "software", pcID,
		oldVals, newVals, ipAddress, userAgent)
	return nil
}

func (s *PCService) BatchDeletePC(labels []string, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	for _, label := range labels {
		count, err := s.pcRepo.CountByLabel(label)
		if err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("PC %s tidak ditemukan", label)
		}
	}

	tx, err := s.pcRepo.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	items := make([]map[string]string, 0, len(labels))
	for _, label := range labels {
		info := map[string]string{"label": label}
		if err := s.pcRepo.DeleteByLabelTx(tx, label); err != nil {
			s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "pc", 0,
				map[string]any{"action": "batch_delete", "count": len(labels), "items": items},
				ipAddress, userAgent, err.Error())
			return err
		}
		items = append(items, info)
	}

	if err := tx.Commit(); err != nil {
		s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "pc", 0,
			map[string]any{"action": "batch_delete", "count": len(labels), "items": items},
			ipAddress, userAgent, err.Error())
		return err
	}

	s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "pc", 0,
		map[string]any{"action": "batch_delete", "count": len(labels), "items": items},
		ipAddress, userAgent)
	return nil
}
