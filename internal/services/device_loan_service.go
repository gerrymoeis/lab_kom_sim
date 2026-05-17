package services

import (
	"time"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/repository"
)

type CreateLoanInput struct {
	DeviceID             int
	BorrowerName         string
	BorrowerType         string
	LoanDate             string
	ExpectedReturnDate   string
	Quantity             int
	Purpose              string
}

type UpdateLoanInput struct {
	BorrowerName       string
	BorrowerType       string
	LoanDate           string
	ExpectedReturnDate string
	ActualReturnDate   string
	Status             string
	Purpose            string
	Notes              string
}

type DeviceLoanService struct {
	db                 *database.DB
	deviceLoanRepo     *repository.DeviceLoanRepository
	deviceRepo         *repository.DeviceRepository
	activityLogService *ActivityLogService
}

func NewDeviceLoanService(db *database.DB, deviceLoanRepo *repository.DeviceLoanRepository, deviceRepo *repository.DeviceRepository, activityLogService *ActivityLogService) *DeviceLoanService {
	return &DeviceLoanService{db: db, deviceLoanRepo: deviceLoanRepo, deviceRepo: deviceRepo, activityLogService: activityLogService}
}

func (s *DeviceLoanService) CreateLoan(in CreateLoanInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) (int64, error) {
	loanDate, _ := time.Parse("2006-01-02", in.LoanDate)
	var expectedReturnDate *time.Time
	if in.ExpectedReturnDate != "" {
		if t, err := time.Parse("2006-01-02", in.ExpectedReturnDate); err == nil {
			expectedReturnDate = &t
		}
	}

	tx, err := s.db.Begin()
	if err != nil { return 0, err }
	defer tx.Rollback()

	deviceTx := s.deviceRepo.WithTx(tx)
	if err := deviceTx.DeductQuantity(in.DeviceID, in.Quantity); err != nil {
		s.activityLogService.LogCreate(actorID, actorUsername, actorRole, "device_loan", 0,
			map[string]any{"borrower": in.BorrowerName}, ipAddress, userAgent, err.Error())
		return 0, err
	}

	loanTx := s.deviceLoanRepo.WithTx(tx)
	loanID, err := loanTx.Create(in.DeviceID, in.BorrowerName, in.BorrowerType, loanDate, expectedReturnDate, in.Quantity, in.Purpose)
	if err != nil {
		s.activityLogService.LogCreate(actorID, actorUsername, actorRole, "device_loan", 0,
			map[string]any{"borrower": in.BorrowerName}, ipAddress, userAgent, err.Error())
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	s.activityLogService.LogCreate(actorID, actorUsername, actorRole, "device_loan", int(loanID),
		map[string]any{"borrower": in.BorrowerName, "device_id": in.DeviceID}, ipAddress, userAgent)
	return loanID, nil
}

func (s *DeviceLoanService) UpdateLoan(id int, in UpdateLoanInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	loanDate, _ := time.Parse("2006-01-02", in.LoanDate)
	var expectedReturnDate, actualReturnDate *time.Time
	if in.ExpectedReturnDate != "" {
		if t, err := time.Parse("2006-01-02", in.ExpectedReturnDate); err == nil { expectedReturnDate = &t }
	}
	if in.ActualReturnDate != "" {
		if t, err := time.Parse("2006-01-02", in.ActualReturnDate); err == nil { actualReturnDate = &t }
	}
	err := s.deviceLoanRepo.Update(id, in.BorrowerName, in.BorrowerType, loanDate, expectedReturnDate, actualReturnDate, in.Status, in.Purpose, in.Notes)
	if err != nil {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "device_loan", id,
			map[string]any{"id": id}, nil, ipAddress, userAgent, err.Error())
		return err
	}
	s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "device_loan", id,
		map[string]any{"id": id}, nil, ipAddress, userAgent)
	return nil
}

func (s *DeviceLoanService) DeleteLoan(id, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	deviceID, quantity, err := s.deviceLoanRepo.GetDeviceAndQuantity(id)
	if err != nil {
		s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "device_loan", id,
			map[string]any{"id": id}, ipAddress, userAgent, err.Error())
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "device_loan", id,
			map[string]any{"id": id}, ipAddress, userAgent, err.Error())
		return err
	}
	defer tx.Rollback()

	deviceTx := s.deviceRepo.WithTx(tx)
	if err := deviceTx.RestoreQuantity(deviceID, quantity); err != nil {
		s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "device_loan", id,
			map[string]any{"id": id}, ipAddress, userAgent, err.Error())
		return err
	}

	loanTx := s.deviceLoanRepo.WithTx(tx)
	if err := loanTx.Delete(id); err != nil {
		s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "device_loan", id,
			map[string]any{"id": id}, ipAddress, userAgent, err.Error())
		return err
	}

	if err := tx.Commit(); err != nil {
		s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "device_loan", id,
			map[string]any{"id": id}, ipAddress, userAgent, err.Error())
		return err
	}

	s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "device_loan", id,
		map[string]any{"id": id}, ipAddress, userAgent)
	return nil
}
