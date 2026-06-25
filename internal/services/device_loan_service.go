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
	oldVals := map[string]any{}
	newVals := map[string]any{}
	if oldRow != nil {
		if oldRow.BorrowerName != in.BorrowerName { oldVals["borrower_name"] = oldRow.BorrowerName; newVals["borrower_name"] = in.BorrowerName }
		if oldRow.BorrowerType != in.BorrowerType { oldVals["borrower_type"] = oldRow.BorrowerType; newVals["borrower_type"] = in.BorrowerType }
		if oldRow.Purpose != in.Purpose { oldVals["purpose"] = oldRow.Purpose; newVals["purpose"] = in.Purpose }
		if oldRow.Notes != in.Notes { oldVals["notes"] = oldRow.Notes; newVals["notes"] = in.Notes }
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
	oldVals := map[string]any{}
	newVals := map[string]any{}
	if oldRow != nil {
		if oldRow.ActualReturnDate == nil && actualReturnDate != nil {
			oldVals["actual_return_date"] = nil
			newVals["actual_return_date"] = actualReturnDate
		} else if oldRow.ActualReturnDate != nil && actualReturnDate == nil {
			oldVals["actual_return_date"] = oldRow.ActualReturnDate
			newVals["actual_return_date"] = nil
		} else if oldRow.ActualReturnDate != nil && actualReturnDate != nil && !oldRow.ActualReturnDate.Equal(*actualReturnDate) {
			oldVals["actual_return_date"] = oldRow.ActualReturnDate
			newVals["actual_return_date"] = actualReturnDate
		}
		if oldRow.Notes != notes { oldVals["notes"] = oldRow.Notes; newVals["notes"] = notes }
	}
	err := s.loanRepo.UpdateReturn(id, actualReturnDate, notes)
	if err != nil {
		s.log.LogUpdate(actorID, actorUsername, actorRole, "device_loan", id,
			oldVals, nil, ipAddress, userAgent, err.Error())
		return err
	}
	s.log.LogUpdate(actorID, actorUsername, actorRole, "device_loan", id,
		oldVals, newVals, ipAddress, userAgent)
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

func (s *DeviceLoanService) BatchDelete(ids []int, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	items := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		info := map[string]any{"id": id}
		if row, err := s.loanRepo.GetByID(id); err == nil {
			info["borrower_name"] = row.BorrowerName
			info["device_label"] = row.DeviceLabel
		}
		if err := s.loanRepo.Delete(id); err != nil {
			s.log.LogDelete(actorID, actorUsername, actorRole, "device_loan", 0,
				map[string]any{"action": "batch_delete", "count": len(ids), "items": items},
				ipAddress, userAgent, err.Error())
			return err
		}
		items = append(items, info)
	}
	s.log.LogDelete(actorID, actorUsername, actorRole, "device_loan", 0,
		map[string]any{"action": "batch_delete", "count": len(ids), "items": items},
		ipAddress, userAgent)
	return nil
}
