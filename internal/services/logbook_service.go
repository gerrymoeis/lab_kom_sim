package services

import (
	"strings"
	"time"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/repository"
)

type CreateLogbookInput struct {
	Date        string
	StudentName string
	NIM         string
	TimeIn      string
	TimeOut     string
	Purpose     string
}

type UpdateLogbookInput struct {
	Date        string
	StudentName string
	NIM         string
	TimeIn      string
	TimeOut     string
	Purpose     string
}

type LogbookService struct {
	logbookRepo        *repository.LogbookRepository
	activityLogService *ActivityLogService
}

func NewLogbookService(logbookRepo *repository.LogbookRepository, activityLogService *ActivityLogService) *LogbookService {
	return &LogbookService{logbookRepo: logbookRepo, activityLogService: activityLogService}
}

func (s *LogbookService) List(filters repository.LogbookFilters) ([]models.LogbookEntry, int, error) {
	return s.logbookRepo.List(filters)
}

func (s *LogbookService) ListCursor(filters repository.LogbookFilters) ([]models.LogbookEntry, bool, error) {
	return s.logbookRepo.ListCursor(filters)
}

func (s *LogbookService) GetByID(id int) (*models.LogbookEntry, error) {
	return s.logbookRepo.GetByID(id)
}

func (s *LogbookService) CreateEntry(in CreateLogbookInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) (int, error) {
	in.StudentName = ToTitleCaseWithAbbr(in.StudentName)
	in.NIM = strings.ToUpper(strings.TrimSpace(strings.ReplaceAll(in.NIM, " ", "")))
	in.Purpose = ToTitleCaseWithAbbr(in.Purpose)

	date := MustParseDate(in.Date)

	existing, _ := s.logbookRepo.GetDuplicateCheck(date, in.TimeIn)
	for _, e := range existing {
		if IsDuplicateEntry(date, e.Date, in.TimeIn, e.TimeIn, in.StudentName, e.StudentName, in.NIM, e.NIM, config.DefaultDuplicateConfig) {
			return 0, nil
		}
	}

	result, err := s.logbookRepo.Create(date, in.StudentName, in.NIM, in.TimeIn, in.TimeOut, in.Purpose)
	if err != nil {
		s.activityLogService.LogCreate(actorID, actorUsername, actorRole, "logbook", 0,
			map[string]any{"student_name": in.StudentName}, ipAddress, userAgent, err.Error())
		return 0, err
	}
	id, _ := result.LastInsertId()
	s.activityLogService.LogCreate(actorID, actorUsername, actorRole, "logbook", int(id),
		map[string]any{"student_name": in.StudentName, "nim": in.NIM}, ipAddress, userAgent)
	return int(id), nil
}

func (s *LogbookService) UpdateEntry(id int, in UpdateLogbookInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	in.StudentName = ToTitleCaseWithAbbr(in.StudentName)
	in.NIM = strings.ToUpper(strings.TrimSpace(strings.ReplaceAll(in.NIM, " ", "")))
	in.Purpose = ToTitleCaseWithAbbr(in.Purpose)

	date := MustParseDate(in.Date)

	old, _, _, _, _ := s.logbookRepo.GetDeleteInfo(id)

	err := s.logbookRepo.Update(id, date, in.StudentName, in.NIM, in.TimeIn, in.TimeOut, in.Purpose)
	if err != nil {
		s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "logbook", id,
			map[string]any{"id": id, "old_name": old}, nil, ipAddress, userAgent, err.Error())
		return err
	}
	s.activityLogService.LogUpdate(actorID, actorUsername, actorRole, "logbook", id,
		map[string]any{"old_name": old},
		map[string]any{"student_name": in.StudentName, "nim": in.NIM}, ipAddress, userAgent)
	return nil
}

func (s *LogbookService) DeleteEntry(id, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	eid, d, sn, nim, err := s.logbookRepo.GetDeleteInfo(id)
	if err != nil {
		return err
	}
	if err := s.logbookRepo.Delete(id); err != nil {
		s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "logbook", eid,
			map[string]any{"student_name": sn}, ipAddress, userAgent, err.Error())
		return err
	}
	s.activityLogService.LogDelete(actorID, actorUsername, actorRole, "logbook", eid,
		map[string]any{"student_name": sn, "nim": nim, "date": d.Format("2006-01-02")}, ipAddress, userAgent)
	return nil
}

func (s *LogbookService) BulkSave(entries []repository.BulkEntry, sourceFile string, actorID int, actorUsername, actorRole, ipAddress, userAgent string) (saved, dups int, err error) {
	cfg := config.DefaultDuplicateConfig
	var clean []repository.BulkEntry
	for _, e := range entries {
		existing, _ := s.logbookRepo.GetDuplicateCheck(e.Date, e.TimeIn)
		dup := false
		for _, ex := range existing {
			if IsDuplicateEntry(e.Date, ex.Date, e.TimeIn, ex.TimeIn, e.StudentName, ex.StudentName, e.NIM, ex.NIM, cfg) {
				dup = true
				dups++
				break
			}
		}
		if !dup {
			e.StudentName = ToTitleCaseWithAbbr(e.StudentName)
			e.NIM = strings.ToUpper(strings.TrimSpace(strings.ReplaceAll(e.NIM, " ", "")))
			e.Purpose = ToTitleCaseWithAbbr(e.Purpose)
			clean = append(clean, e)
			saved++
		}
	}
	if len(clean) == 0 {
		return 0, dups, nil
	}
	if err := s.logbookRepo.BulkImport(clean, sourceFile); err != nil {
		return 0, dups, err
	}
	s.activityLogService.LogCreate(actorID, actorUsername, actorRole, "logbook", 0,
		map[string]any{"saved": saved, "duplicates": dups}, ipAddress, userAgent)
	return saved, dups, nil
}

type LogbookDeleteInfo struct {
	ID          int
	Date        time.Time
	StudentName string
	NIM         string
}

func (s *LogbookService) GetDeleteInfo(id int) (*LogbookDeleteInfo, error) {
	eid, d, sn, nim, err := s.logbookRepo.GetDeleteInfo(id)
	if err != nil {
		return nil, err
	}
	return &LogbookDeleteInfo{ID: eid, Date: d, StudentName: sn, NIM: nim}, nil
}
