package services

import (
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/repository"
)

type ScheduleCreateInput struct {
	CourseName, Lecturer, Day, Class, TimeStart, TimeEnd, Notes string
}

type ScheduleUpdateInput struct {
	CourseName, Lecturer, Day, Class, TimeStart, TimeEnd, Notes string
}

type ScheduleService struct {
	repo *repository.ScheduleRepository
	log  *ActivityLogService
}

func NewScheduleService(repo *repository.ScheduleRepository, log *ActivityLogService) *ScheduleService {
	return &ScheduleService{repo: repo, log: log}
}

func (s *ScheduleService) Create(in ScheduleCreateInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	in.CourseName = ToTitleCaseWithAbbr(in.CourseName)
	in.Lecturer = ToTitleCaseWithAbbr(in.Lecturer)
	in.Class = ToTitleCaseWithAbbr(in.Class)
	in.Notes = SanitizeText(in.Notes)
	result, err := s.repo.Create(in.CourseName, in.Lecturer, in.Day, in.Class, in.TimeStart, in.TimeEnd, in.Notes)
	if err != nil {
		s.log.LogCreate(actorID, actorUsername, actorRole, "schedule", 0,
			map[string]any{"course_name": in.CourseName}, ipAddress, userAgent, err.Error())
		return err
	}
	id, _ := result.LastInsertId()
	s.log.LogCreate(actorID, actorUsername, actorRole, "schedule", int(id),
		map[string]any{
			"course_name": in.CourseName, "lecturer": in.Lecturer, "day": in.Day, "class": in.Class,
			"time": in.TimeStart + "-" + in.TimeEnd,
		}, ipAddress, userAgent)
	return nil
}

func (s *ScheduleService) Update(id int, in ScheduleUpdateInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	in.CourseName = ToTitleCaseWithAbbr(in.CourseName)
	in.Lecturer = ToTitleCaseWithAbbr(in.Lecturer)
	in.Class = ToTitleCaseWithAbbr(in.Class)
	in.Notes = SanitizeText(in.Notes)

	oldSchedule, _ := s.repo.GetByID(id)
	oldVals := map[string]any{}
	newVals := map[string]any{}
	if oldSchedule != nil {
		if oldSchedule.CourseName != in.CourseName { oldVals["course_name"] = oldSchedule.CourseName; newVals["course_name"] = in.CourseName }
		if oldSchedule.Lecturer != in.Lecturer { oldVals["lecturer"] = oldSchedule.Lecturer; newVals["lecturer"] = in.Lecturer }
		if oldSchedule.Day != in.Day { oldVals["day"] = oldSchedule.Day; newVals["day"] = in.Day }
		if oldSchedule.Class != in.Class { oldVals["class"] = oldSchedule.Class; newVals["class"] = in.Class }
		if oldSchedule.TimeStart != in.TimeStart { oldVals["time_start"] = oldSchedule.TimeStart; newVals["time_start"] = in.TimeStart }
		if oldSchedule.TimeEnd != in.TimeEnd { oldVals["time_end"] = oldSchedule.TimeEnd; newVals["time_end"] = in.TimeEnd }
		if oldSchedule.Notes != in.Notes { oldVals["notes"] = oldSchedule.Notes; newVals["notes"] = in.Notes }
	}

	err := s.repo.Update(id, in.CourseName, in.Lecturer, in.Day, in.Class, in.TimeStart, in.TimeEnd, in.Notes)
	if err != nil {
		s.log.LogUpdate(actorID, actorUsername, actorRole, "schedule", id,
			oldVals, nil, ipAddress, userAgent, err.Error())
		return err
	}
	s.log.LogUpdate(actorID, actorUsername, actorRole, "schedule", id,
		oldVals, newVals, ipAddress, userAgent)
	return nil
}

func (s *ScheduleService) Delete(id int, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	courseName, _ := s.repo.GetCourseName(id)

	err := s.repo.Delete(id)
	if err != nil {
		s.log.LogDelete(actorID, actorUsername, actorRole, "schedule", id,
			map[string]any{"course_name": courseName}, ipAddress, userAgent, err.Error())
		return err
	}
	s.log.LogDelete(actorID, actorUsername, actorRole, "schedule", id,
		map[string]any{"course_name": courseName}, ipAddress, userAgent)
	return nil
}

func (s *ScheduleService) GetByID(id int) (*models.CourseSchedule, error) {
	return s.repo.GetByID(id)
}

func (s *ScheduleService) List(search, dayFilter string) ([]models.CourseSchedule, error) {
	return s.repo.List(search, dayFilter)
}

func (s *ScheduleService) ListPaginated(search, dayFilter, sortBy string, page, pageSize int) ([]models.CourseSchedule, int, error) {
	return s.repo.ListPaginated(search, dayFilter, sortBy, page, pageSize)
}

func (s *ScheduleService) BatchDelete(ids []int, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	for _, id := range ids {
		if err := s.repo.Delete(id); err != nil {
			s.log.LogDelete(actorID, actorUsername, actorRole, "schedule", 0,
				map[string]any{"action": "batch_delete", "count": len(ids), "ids": ids},
				ipAddress, userAgent, err.Error())
			return err
		}
	}
	s.log.LogDelete(actorID, actorUsername, actorRole, "schedule", 0,
		map[string]any{"action": "batch_delete", "count": len(ids), "ids": ids},
		ipAddress, userAgent)
	return nil
}
