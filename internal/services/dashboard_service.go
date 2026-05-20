package services

import (
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/repository"
)

type DashboardData struct {
	PCs           []models.PC
	Grid          [][]models.PC
	StatusCounts  map[string]int
	DeviceCount   int
	SoftwareCount int
}

type DashboardService struct {
	dashboardRepo *repository.DashboardRepository
}

func NewDashboardService(dashboardRepo *repository.DashboardRepository) *DashboardService {
	return &DashboardService{dashboardRepo: dashboardRepo}
}

func (s *DashboardService) GetDashboardData() (*DashboardData, error) {
	pcs, err := s.dashboardRepo.ListPCs()
	if err != nil {
		return nil, err
	}

	statusCounts := make(map[string]int)
	for _, pc := range pcs {
		statusCounts[pc.Status]++
	}

	grid := make([][]models.PC, 5)
	for i := range grid {
		grid[i] = make([]models.PC, 8)
	}
	for _, pc := range pcs {
		if pc.Row >= 1 && pc.Row <= 5 && pc.Column >= 1 && pc.Column <= 8 {
			grid[pc.Row-1][pc.Column-1] = pc
		}
	}

	deviceCount, softwareCount, _ := s.dashboardRepo.CountAll()
	return &DashboardData{
		PCs: pcs, Grid: grid, StatusCounts: statusCounts,
		DeviceCount: deviceCount, SoftwareCount: softwareCount,
	}, nil
}
