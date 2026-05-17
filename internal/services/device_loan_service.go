package services

import (
	"time"

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
	deviceLoanRepo     *repository.DeviceLoanRepository
	activityLogService *ActivityLogService
}

func NewDeviceLoanService(deviceLoanRepo *repository.DeviceLoanRepository, activityLogService *ActivityLogService) *DeviceLoanService {
	return &DeviceLoanService{deviceLoanRepo: deviceLoanRepo, activityLogService: activityLogService}
}

func (s *DeviceLoanService) CreateLoan(in CreateLoanInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) (int64, error) {
	loanDate, _ := time.Parse("2006-01-02", in.LoanDate)
	var expectedReturnDate *time.Time
	if in.ExpectedReturnDate != "" {
		if t, err := time.Parse("2006-01-02", in.ExpectedReturnDate); err == nil {
			expectedReturnDate = &t
		}
	}
	loanID, err := s.deviceLoanRepo.Create(in.DeviceID, in.BorrowerName, in.BorrowerType, loanDate, expectedReturnDate, in.Quantity, in.Purpose)
	if err != nil {
		s.activityLogService.LogCreate(actorID, actorUsername, actorRole, "device_loan", 0,
			map[string]interface{}{"borrower": in.BorrowerName}, ipAddress, userAgent, err.Error())
		return 0, err
	}
	s.activityLogService.LogCreate(actorID, actorUsername, actorRole, "device_loan", int(loanID),
		map[string]interface{}{"borrower": in.BorrowerName, "device_id": in.DeviceID}, ipAddress, userAgent)
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
			map[string]interface{}{"id": id}, nil, ipAddress, userAgent, err.Error())
		return err
	}
	s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "device_loan", id,
		map[string]interface{}{"id": id}, nil, ipAddress, userAgent)
	return nil
}

func (s *DeviceLoanService) DeleteLoan(id, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	if err := s.deviceLoanRepo.Delete(id); err != nil {
		s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "device_loan", id,
			map[string]interface{}{"id": id}, ipAddress, userAgent, err.Error())
		return err
	}
	s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "device_loan", id,
		map[string]interface{}{"id": id}, ipAddress, userAgent)
	return nil
}
