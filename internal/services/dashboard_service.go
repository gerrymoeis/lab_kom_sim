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
	SpecialPCs    []models.PC
	PCLecturer    models.PC
	PCLaboran     models.PC
	PCCCTV        models.PC
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

	var specialPCs []models.PC
	data := &DashboardData{}

	for _, pc := range pcs {
		if pc.Row >= 1 && pc.Row <= 5 && pc.Column >= 1 && pc.Column <= 8 {
			grid[pc.Row-1][pc.Column-1] = pc
		} else if pc.Label != "" {
			specialPCs = append(specialPCs, pc)
			switch pc.Label {
			case "PC-Dosen":
				data.PCLecturer = pc
			case "PC-Laboran":
				data.PCLaboran = pc
			case "PC-CCTV":
				data.PCCCTV = pc
			}
		}
	}

	deviceCount, softwareCount, _ := s.dashboardRepo.CountAll()
	data.PCs = pcs
	data.Grid = grid
	data.StatusCounts = statusCounts
	data.DeviceCount = deviceCount
	data.SoftwareCount = softwareCount
	data.SpecialPCs = specialPCs
	return data, nil
}
