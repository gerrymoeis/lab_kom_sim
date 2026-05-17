package services

import (
	"time"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/repository"
)

type CreateUsageInput struct {
	DeviceID     int
	UserName     string
	UserType     string
	UsageDate    string
	Quantity     int
	IsAvailable  string
	Purpose      string
}

type UpdateUsageInput struct {
	UserName    string
	UserType    string
	UsageDate   string
	Quantity    int
	IsAvailable string
	Purpose     string
	Notes       string
}

type DeviceUsageService struct {
	db                 *database.DB
	deviceUsageRepo    *repository.DeviceUsageRepository
	deviceRepo         *repository.DeviceRepository
	activityLogService *ActivityLogService
}

func NewDeviceUsageService(db *database.DB, deviceUsageRepo *repository.DeviceUsageRepository, deviceRepo *repository.DeviceRepository, activityLogService *ActivityLogService) *DeviceUsageService {
	return &DeviceUsageService{db: db, deviceUsageRepo: deviceUsageRepo, deviceRepo: deviceRepo, activityLogService: activityLogService}
}

func (s *DeviceUsageService) GetByID(id int) (*repository.DeviceUsageRow, error) {
	return s.deviceUsageRepo.GetByID(id)
}

func (s *DeviceUsageService) CreateUsage(in CreateUsageInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) (int64, error) {
	if in.IsAvailable != "no" { in.IsAvailable = "yes" }
	usageDate, _ := time.Parse("2006-01-02", in.UsageDate)

	tx, err := s.db.Begin()
	if err != nil { return 0, err }
	defer tx.Rollback()

	deviceTx := s.deviceRepo.WithTx(tx)
	if in.IsAvailable == "no" {
		if err := deviceTx.DeductQuantity(in.DeviceID, in.Quantity); err != nil {
			s.activityLogService.LogCreate(actorID, actorUsername, actorRole, "device_usage", 0,
				map[string]any{"user": in.UserName}, ipAddress, userAgent, err.Error())
			return 0, err
		}
	}

	usageTx := s.deviceUsageRepo.WithTx(tx)
	id, err := usageTx.Create(in.DeviceID, in.UserName, in.UserType, usageDate, in.Quantity, in.IsAvailable, in.Purpose)
	if err != nil {
		s.activityLogService.LogCreate(actorID, actorUsername, actorRole, "device_usage", 0,
			map[string]any{"user": in.UserName}, ipAddress, userAgent, err.Error())
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	s.activityLogService.LogCreate(actorID, actorUsername, actorRole, "device_usage", int(id),
		map[string]any{"user": in.UserName, "device_id": in.DeviceID}, ipAddress, userAgent)
	return id, nil
}

func (s *DeviceUsageService) UpdateUsage(id int, in UpdateUsageInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	if in.IsAvailable != "no" { in.IsAvailable = "yes" }
	usageDate, _ := time.Parse("2006-01-02", in.UsageDate)

	tx, err := s.db.Begin()
	if err != nil { return err }
	defer tx.Rollback()

	usageTx := s.deviceUsageRepo.WithTx(tx)
	deviceID, oldQty, oldAvail, err := usageTx.GetDeviceAndQuantity(id)
	if err != nil {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "device_usage", id,
			map[string]any{"id": id}, nil, ipAddress, userAgent, err.Error())
		return err
	}

	if err := usageTx.Update(id, in.UserName, in.UserType, usageDate, in.Quantity, in.IsAvailable, in.Purpose, in.Notes); err != nil {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "device_usage", id,
			map[string]any{"id": id}, nil, ipAddress, userAgent, err.Error())
		return err
	}

	deviceTx := s.deviceRepo.WithTx(tx)
	switch {
	case oldAvail != in.IsAvailable:
		if in.IsAvailable == "yes" {
			if err := deviceTx.RestoreQuantity(deviceID, in.Quantity); err != nil {
				s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "device_usage", id, map[string]any{"id": id}, nil, ipAddress, userAgent, err.Error())
				return err
			}
		} else {
			if err := deviceTx.DeductQuantity(deviceID, in.Quantity); err != nil {
				s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "device_usage", id, map[string]any{"id": id}, nil, ipAddress, userAgent, err.Error())
				return err
			}
		}
	case in.Quantity != oldQty:
		if err := deviceTx.SetQuantity(deviceID, oldQty-in.Quantity); err != nil {
			s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "device_usage", id, map[string]any{"id": id}, nil, ipAddress, userAgent, err.Error())
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "device_usage", id,
			map[string]any{"id": id}, nil, ipAddress, userAgent, err.Error())
		return err
	}

	s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "device_usage", id,
		map[string]any{"id": id}, nil, ipAddress, userAgent)
	return nil
}

func (s *DeviceUsageService) DeleteUsage(id, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	tx, err := s.db.Begin()
	if err != nil {
		s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "device_usage", id,
			map[string]any{"id": id}, ipAddress, userAgent, err.Error())
		return err
	}
	defer tx.Rollback()

	usageTx := s.deviceUsageRepo.WithTx(tx)
	devID, qty, avail, err := usageTx.GetDeviceAndQuantity(id)
	if err != nil {
		s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "device_usage", id,
			map[string]any{"id": id}, ipAddress, userAgent, err.Error())
		return err
	}

	if err := usageTx.Delete(id); err != nil {
		s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "device_usage", id,
			map[string]any{"id": id}, ipAddress, userAgent, err.Error())
		return err
	}

	if avail == "no" {
		deviceTx := s.deviceRepo.WithTx(tx)
		if err := deviceTx.RestoreQuantity(devID, qty); err != nil {
			s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "device_usage", id,
				map[string]any{"id": id}, ipAddress, userAgent, err.Error())
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "device_usage", id,
			map[string]any{"id": id}, ipAddress, userAgent, err.Error())
		return err
	}

	s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "device_usage", id,
		map[string]any{"id": id}, ipAddress, userAgent)
	return nil
}

func (s *DeviceUsageService) UpdateAvailability(id int, isAvailable string, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	tx, err := s.db.Begin()
	if err != nil {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "device_usage", id,
			map[string]any{"id": id}, nil, ipAddress, userAgent, err.Error())
		return err
	}
	defer tx.Rollback()

	usageTx := s.deviceUsageRepo.WithTx(tx)
	devID, quantity, oldAvail, err := usageTx.GetDeviceAndQuantity(id)
	if err != nil {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "device_usage", id,
			map[string]any{"id": id}, nil, ipAddress, userAgent, err.Error())
		return err
	}
	if oldAvail == isAvailable {
		return tx.Commit()
	}

	if err := usageTx.UpdateAvailability(id, isAvailable); err != nil {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "device_usage", id,
			map[string]any{"id": id}, nil, ipAddress, userAgent, err.Error())
		return err
	}

	deviceTx := s.deviceRepo.WithTx(tx)
	if isAvailable == "yes" {
		if err := deviceTx.RestoreQuantity(devID, quantity); err != nil {
			s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "device_usage", id,
				map[string]any{"id": id}, nil, ipAddress, userAgent, err.Error())
			return err
		}
	} else {
		if err := deviceTx.DeductQuantity(devID, quantity); err != nil {
			s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "device_usage", id,
				map[string]any{"id": id}, nil, ipAddress, userAgent, err.Error())
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "device_usage", id,
			map[string]any{"id": id}, nil, ipAddress, userAgent, err.Error())
		return err
	}

	s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "device_usage", id,
		map[string]any{"id": id}, nil, ipAddress, userAgent)
	return nil
}
