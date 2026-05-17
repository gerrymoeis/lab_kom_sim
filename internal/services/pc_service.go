package services

import (
	"inventaris-lab-kom/internal/repository"
)

type CreatePCInput struct {
	PCNumber, Row, Column               int
	Status, Processor, RAM, Storage     string
	SerialNumber, OperatingSystem       string
	DeviceType, BrandModel, Accessories string
	PhotoSerial, PhotoFront             string
}

type UpdatePCInput struct {
	Status, DeviceType, SerialNumber    string
	BrandModel, Accessories             string
	Processor, RAM, Storage             string
	OperatingSystem, Notes, ActionNotes string
	PhotoSerial, PhotoFront             string
	RequiredSW, OtherSW                 []int
}

type PCService struct {
	pcRepo             *repository.PCRepository
	activityLogService *ActivityLogService
}

func NewPCService(pcRepo *repository.PCRepository, activityLogService *ActivityLogService) *PCService {
	return &PCService{pcRepo: pcRepo, activityLogService: activityLogService}
}

func (s *PCService) CreatePC(in CreatePCInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) (int, error) {
	if in.Status == "" { in.Status = "normal" }
	if in.DeviceType == "" { in.DeviceType = "PC All-in-one" }
	if in.BrandModel == "" { in.BrandModel = "Axioo Mypc One Pro K7-24 (16N9)" }
	if in.Accessories == "" { in.Accessories = "Keyboard & Mouse Axioo (Wired Set)" }
	if in.Processor == "" { in.Processor = "Intel Core i7" }
	if in.RAM == "" { in.RAM = "16GB DDR4" }
	if in.Storage == "" { in.Storage = "1TB NVMe" }

	result, err := s.pcRepo.Create(in.PCNumber, in.Row, in.Column, in.Status, in.Processor, in.RAM, in.Storage,
		in.SerialNumber, in.OperatingSystem, in.DeviceType, in.BrandModel, in.Accessories, in.PhotoSerial, in.PhotoFront)
	if err != nil {
		s.activityLogService.LogCreate(actorID, actorUsername, actorRole, "pc", 0,
			map[string]interface{}{"pc_number": in.PCNumber, "serial_number": in.SerialNumber},
			ipAddress, userAgent, err.Error())
		return 0, err
	}
	pcID, _ := result.LastInsertId()
	if pcID > 0 {
		s.activityLogService.LogCreate(actorID, actorUsername, actorRole, "pc", int(pcID),
			map[string]interface{}{"pc_number": in.PCNumber, "serial_number": in.SerialNumber, "operating_system": in.OperatingSystem},
			ipAddress, userAgent)
		s.pcRepo.SeedRequiredSoftware(int(pcID))
	}
	return int(pcID), nil
}

func (s *PCService) UpdatePC(pcNumber int, in UpdatePCInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	err := s.pcRepo.Update(pcNumber, in.Status, in.DeviceType, in.SerialNumber, in.BrandModel, in.Accessories,
		in.Processor, in.RAM, in.Storage, in.OperatingSystem, in.Notes, in.ActionNotes, in.PhotoSerial, in.PhotoFront)
	if err != nil {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", pcNumber,
			map[string]interface{}{"pc_number": pcNumber}, nil, ipAddress, userAgent, err.Error())
		return err
	}
	s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "pc", pcNumber,
		map[string]interface{}{"pc_number": pcNumber},
		map[string]interface{}{"status": in.Status, "operating_system": in.OperatingSystem},
		ipAddress, userAgent)
	return nil
}

func (s *PCService) DeletePC(pcNumber, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	if err := s.pcRepo.DeleteByPCNumber(pcNumber); err != nil {
		s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "pc", 0,
			map[string]interface{}{"pc_number": pcNumber}, ipAddress, userAgent, err.Error())
		return err
	}
	s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "pc", 0,
		map[string]interface{}{"pc_number": pcNumber}, ipAddress, userAgent)
	return nil
}

func (s *PCService) UpdateStatus(id int, status string) error {
	return s.pcRepo.UpdateStatus(id, status)
}
