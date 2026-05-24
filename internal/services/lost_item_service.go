package services

import (
	"time"

	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/repository"
)

type CreateLostItemInput struct {
	DeviceID        *int
	ItemName        string
	ItemDescription string
	ReportedBy      string
	ReportedDate    string
	LastSeenAt      string
	LocationLastSeen string
	Status          string
	Photo           string
}

type UpdateLostItemInput struct {
	DeviceID         *int
	ItemName         string
	ItemDescription  string
	ReportedBy       string
	ReportedDate     string
	LastSeenAt       string
	LocationLastSeen string
	Status           string
	OwnerName        string
	OwnerClass       string
	OwnerNim         string
	ReturnedDate     string
	Photo            string
}

type LostItemService struct {
	repo *repository.LostItemRepository
	log  *ActivityLogService
}

func NewLostItemService(repo *repository.LostItemRepository, log *ActivityLogService) *LostItemService {
	return &LostItemService{repo: repo, log: log}
}

func (s *LostItemService) List(filters repository.LostItemFilters) ([]models.LostItem, error) {
	return s.repo.List(filters)
}

func (s *LostItemService) ListPaginated(filters repository.LostItemFilters, page, pageSize int) ([]models.LostItem, int, error) {
	return s.repo.ListPaginated(filters, page, pageSize)
}

func (s *LostItemService) GetByID(id int) (*models.LostItem, error) {
	return s.repo.GetByID(id)
}

func parseDatetime(s string) *time.Time {
	if s == "" {
		return nil
	}
	formats := []string{
		"2006-01-02T15:04",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, f := range formats {
		t, err := time.Parse(f, s)
		if err == nil {
			return &t
		}
	}
	return nil
}

func parseDatePtr(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil
	}
	return &t
}

func (s *LostItemService) Create(in CreateLostItemInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) (int64, error) {
	id, err := s.repo.Create(in.DeviceID, in.ItemName, in.ItemDescription, in.ReportedBy, in.ReportedDate, parseDatetime(in.LastSeenAt), in.LocationLastSeen, in.Status, in.Photo)
	if err != nil {
		s.log.LogCreate(actorID, actorUsername, actorRole, "lost_item", 0,
			map[string]any{"item_name": in.ItemName}, ipAddress, userAgent, err.Error())
		return 0, err
	}
	s.log.LogCreate(actorID, actorUsername, actorRole, "lost_item", int(id),
		map[string]any{"item_name": in.ItemName, "status": in.Status},
		ipAddress, userAgent)
	return id, nil
}

func (s *LostItemService) Update(id int, in UpdateLostItemInput, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	err := s.repo.Update(id, repository.UpdateLostItemParams{
		DeviceID:         in.DeviceID,
		ItemName:         in.ItemName,
		ItemDescription:  in.ItemDescription,
		ReportedBy:       in.ReportedBy,
		ReportedDate:     in.ReportedDate,
		LastSeenAt:       parseDatetime(in.LastSeenAt),
		LocationLastSeen: in.LocationLastSeen,
		Status:           in.Status,
		OwnerName:        in.OwnerName,
		OwnerClass:       in.OwnerClass,
		OwnerNim:         in.OwnerNim,
		ReturnedDate:     parseDatePtr(in.ReturnedDate),
		Photo:            in.Photo,
	})
	if err != nil {
		s.log.LogUpdate(actorID, actorUsername, actorRole, "lost_item", id,
			map[string]any{"id": id}, nil, ipAddress, userAgent, err.Error())
		return err
	}
	s.log.LogUpdate(actorID, actorUsername, actorRole, "lost_item", id,
		map[string]any{"id": id},
		map[string]any{"item_name": in.ItemName, "status": in.Status},
		ipAddress, userAgent)
	return nil
}

func (s *LostItemService) Delete(id, actorID int, actorUsername, actorRole, ipAddress, userAgent string) error {
	err := s.repo.Delete(id)
	if err != nil {
		s.log.LogDelete(actorID, actorUsername, actorRole, "lost_item", id,
			map[string]any{"id": id}, ipAddress, userAgent, err.Error())
		return err
	}
	s.log.LogDelete(actorID, actorUsername, actorRole, "lost_item", id,
		map[string]any{"id": id}, ipAddress, userAgent)
	return nil
}
