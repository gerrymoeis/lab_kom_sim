package services

import (
	"time"

	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/repository"
)

type CreateLoanInput struct {
	DeviceID     int
	BorrowerName string
	BorrowerType string
	LoanDate     string
	ReturnDate   string
	Purpose      string
}

type UpdateLoanInput struct {
	BorrowerName     string
	BorrowerType     string
	LoanDate         string
	ReturnDate       *time.Time
	ActualReturnDate string
	Purpose          string
	Notes            string
}

type DeviceLoanService struct {
	loanRepo       *repository.DeviceLoanRepository
	extensionRepo  *repository.LoanExtensionRepository
	log            *ActivityLogService
}

func NewDeviceLoanService(loanRepo *repository.DeviceLoanRepository, extensionRepo *repository.LoanExtensionRepository, log *ActivityLogService) *DeviceLoanService {
	return &DeviceLoanService{loanRepo: loanRepo, extensionRepo: extensionRepo, log: log}
}

func (s *DeviceLoanService) GetLoanableDevices() ([]models.Device, error) {
	return s.loanRepo.GetLoanableDevices()
}

func (s *DeviceLoanService) GetByID(id int) (*repository.DeviceLoanRow, error) {
	return s.loanRepo.GetByID(id)
}

func (s *DeviceLoanService) ListPaginated(filters repository.DeviceLoanFilters, page, pageSize int) ([]repository.DeviceLoanRow, int, error) {
	return s.loanRepo.ListPaginated(filters, page, pageSize)
}

func (s *DeviceLoanService) ListByDeviceID(deviceID int) ([]repository.DeviceLoanRow, error) {
	return s.loanRepo.ListByDeviceID(deviceID)
}

func (s *DeviceLoanService) ExportAll() ([]repository.DeviceLoanRow, error) {
	return s.loanRepo.ExportAll()
}

func (s *DeviceLoanService) CreateLoan(in CreateLoanInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) (int64, error) {
	loanDate := MustParseDate(in.LoanDate)
	returnDate := MustParseDate(in.ReturnDate)
	in.BorrowerName = ToTitleCaseWithAbbr(in.BorrowerName)
	in.Purpose = SanitizeText(in.Purpose)

	loanID, err := s.loanRepo.Create(in.DeviceID, in.BorrowerName, in.BorrowerType, loanDate, returnDate, in.Purpose)
	if err != nil {
		s.log.LogCreate(actorID, actorUsername, actorRole, "device_loan", 0,
			map[string]any{"borrower": in.BorrowerName}, ipAddress, userAgent, err.Error())
		return 0, err
	}

	s.log.LogCreate(actorID, actorUsername, actorRole, "device_loan", int(loanID),
		map[string]any{"borrower": in.BorrowerName, "device_id": in.DeviceID}, ipAddress, userAgent)
	return loanID, nil
}

func (s *DeviceLoanService) UpdateLoan(id int, in UpdateLoanInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	loanDate := MustParseDate(in.LoanDate)
	var actualReturnDate *time.Time
	if in.ActualReturnDate != "" {
		if t, err := ParseDate(in.ActualReturnDate); err == nil {
			actualReturnDate = &t
		}
	}
	in.BorrowerName = ToTitleCaseWithAbbr(in.BorrowerName)
	in.Purpose = SanitizeText(in.Purpose)
	in.Notes = SanitizeText(in.Notes)

	oldRow, _ := s.loanRepo.GetByID(id)
	oldVals := map[string]any{"id": id}
	newVals := map[string]any{"id": id}
	if oldRow != nil {
		if oldRow.BorrowerName != in.BorrowerName { oldVals["borrower_name"] = oldRow.BorrowerName; newVals["borrower_name"] = in.BorrowerName }
		if oldRow.BorrowerType != in.BorrowerType { oldVals["borrower_type"] = oldRow.BorrowerType; newVals["borrower_type"] = in.BorrowerType }
		if oldRow.Purpose != in.Purpose { oldVals["purpose"] = oldRow.Purpose; newVals["purpose"] = in.Purpose }
	}

	err := s.loanRepo.Update(id, in.BorrowerName, in.BorrowerType, loanDate, in.ReturnDate, actualReturnDate, in.Purpose, in.Notes)
	if err != nil {
		s.log.LogUpdate(actorID, actorUsername, actorRole, "device_loan", id,
			oldVals, nil, ipAddress, userAgent, err.Error())
		return err
	}
	s.log.LogUpdate(actorID, actorUsername, actorRole, "device_loan", id,
		oldVals, newVals, ipAddress, userAgent)
	return nil
}

func (s *DeviceLoanService) ExtendLoan(loanID int, newReturnDate string, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	oldReturnDate, err := s.loanRepo.GetReturnDate(loanID)
	if err != nil {
		s.log.LogUpdate(actorID, actorUsername, actorRole, "device_loan", loanID,
			map[string]any{"id": loanID}, nil, ipAddress, userAgent, err.Error())
		return err
	}

	// Update return_date on loan
	if err := s.loanRepo.ExtendReturnDate(loanID, newReturnDate); err != nil {
		s.log.LogUpdate(actorID, actorUsername, actorRole, "device_loan", loanID,
			map[string]any{"id": loanID}, nil, ipAddress, userAgent, err.Error())
		return err
	}

	// Record extension history
	if _, err := s.extensionRepo.Create(loanID, oldReturnDate, newReturnDate); err != nil {
		s.log.LogUpdate(actorID, actorUsername, actorRole, "device_loan", loanID,
			map[string]any{"id": loanID}, nil, ipAddress, userAgent, err.Error())
		return err
	}

	s.log.LogUpdate(actorID, actorUsername, actorRole, "device_loan", loanID,
		map[string]any{"id": loanID, "return_date": oldReturnDate},
		map[string]any{"id": loanID, "return_date": newReturnDate},
		ipAddress, userAgent)
	return nil
}

func (s *DeviceLoanService) UpdateReturn(id int, actualReturnDate *time.Time, notes string, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	oldRow, _ := s.loanRepo.GetByID(id)
	oldVals := map[string]any{"id": id}
	if oldRow != nil {
		oldVals["borrower_name"] = oldRow.BorrowerName
		oldVals["actual_return_date"] = oldRow.ActualReturnDate
		oldVals["notes"] = oldRow.Notes
	}
	err := s.loanRepo.UpdateReturn(id, actualReturnDate, notes)
	if err != nil {
		s.log.LogUpdate(actorID, actorUsername, actorRole, "device_loan", id,
			oldVals, nil, ipAddress, userAgent, err.Error())
		return err
	}
	s.log.LogUpdate(actorID, actorUsername, actorRole, "device_loan", id,
		oldVals, map[string]any{"id": id, "actual_return_date": actualReturnDate, "notes": notes},
		ipAddress, userAgent)
	return nil
}

func (s *DeviceLoanService) GetExtensionsByLoanID(loanID int) ([]models.LoanExtension, error) {
	return s.extensionRepo.ListByLoanID(loanID)
}

func (s *DeviceLoanService) DeleteLoan(id, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	if err := s.loanRepo.Delete(id); err != nil {
		s.log.LogDelete(actorID, actorUsername, actorRole, "device_loan", id,
			map[string]any{"id": id}, ipAddress, userAgent, err.Error())
		return err
	}
	s.log.LogDelete(actorID, actorUsername, actorRole, "device_loan", id,
		map[string]any{"id": id}, ipAddress, userAgent)
	return nil
}
