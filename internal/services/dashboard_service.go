package services

import (
	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/repository"
)

type DashboardData struct {
	PCs           []models.PC
	Grid          [][]models.PC
	ExtraPCs      []models.PC
	StatusCounts  map[string]int
	SpareCount    int
	DeviceCount   int
	SoftwareCount int
	SpecialPCs    []models.PC
	PCLecturer    models.PC
	PCLaboran     models.PC
	PCCCTV        models.PC
	ColsPerRow    []int
	GapPos        int
	HasGap        bool
	RowGaps       [][]int
}

type DashboardService struct {
	dashboardRepo *repository.DashboardRepository
}

func NewDashboardService(dashboardRepo *repository.DashboardRepository) *DashboardService {
	return &DashboardService{dashboardRepo: dashboardRepo}
}

func (s *DashboardService) GetDashboardData(layout config.GridLayout) (*DashboardData, error) {
	pcs, err := s.dashboardRepo.ListPCs()
	if err != nil {
		return nil, err
	}

	statusCounts := make(map[string]int)
	var spareCount int
	for _, pc := range pcs {
		if pc.Placement == "cadangan" {
			spareCount++
		} else {
			statusCounts[pc.Status]++
		}
	}

	maxRow := len(layout.ColsPerRow)
	for _, pc := range pcs {
		if pc.Placement == "dipakai" && isNumericLabel(pc.Label) && pc.Row > maxRow {
			maxRow = pc.Row
		}
	}

	colsAtRow := func(rowIndex int) int {
		if rowIndex < 0 || rowIndex >= len(layout.ColsPerRow) {
			return 8
		}
		return layout.ColsPerRow[rowIndex]
	}

	grid := make([][]models.PC, maxRow)
	for i := range grid {
		grid[i] = make([]models.PC, colsAtRow(i))
	}

	var specialPCs []models.PC
	data := &DashboardData{}

	for _, pc := range pcs {
		if pc.Placement == "cadangan" {
			continue
		}
		maxCol := colsAtRow(pc.Row - 1)
		if isNumericLabel(pc.Label) && pc.Row >= 1 && pc.Column >= 1 && pc.Row <= maxRow && pc.Column <= maxCol {
			grid[pc.Row-1][pc.Column-1] = pc
		} else if pc.Label != "" {
			switch pc.Label {
			case "pc-dosen":
				data.PCLecturer = pc
			case "pc-laboran":
				data.PCLaboran = pc
			case "pc-cctv":
				data.PCCCTV = pc
			default:
				specialPCs = append(specialPCs, pc)
			}
		}
	}

	deviceCount, softwareCount, _ := s.dashboardRepo.CountAll()
	data.PCs = pcs
	data.Grid = grid
	data.StatusCounts = statusCounts
	data.SpareCount = spareCount
	data.DeviceCount = deviceCount
	data.SoftwareCount = softwareCount
	data.SpecialPCs = specialPCs
	data.ColsPerRow = layout.ColsPerRow
	data.GapPos = layout.GapPos
	data.HasGap = layout.HasGap
	data.RowGaps = layout.RowGaps
	return data, nil
}
