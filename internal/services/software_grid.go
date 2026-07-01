package services

import (
	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/repository"
)

func BuildSoftwareGrid(pcList []repository.PCInstallStatus, layout config.GridLayout) [][]repository.PCInstallStatus {
	maxRow := len(layout.ColsPerRow)
	for _, p := range pcList {
		if p.Row > maxRow {
			maxRow = p.Row
		}
	}
	if maxRow < 1 {
		return nil
	}

	grid := make([][]repository.PCInstallStatus, maxRow)
	for i := range grid {
		grid[i] = make([]repository.PCInstallStatus, layout.ColsAtRow(i))
	}

	for _, p := range pcList {
		if p.Row >= 1 && p.Row <= maxRow {
			maxCol := layout.ColsAtRow(p.Row - 1)
			if p.Column >= 1 && p.Column <= maxCol {
				grid[p.Row-1][p.Column-1] = p
			}
		}
	}
	return grid
}
