package services

import (
	"fmt"
	"time"

	"github.com/xuri/excelize/v2"
)

// ExcelService handles Excel file generation
type ExcelService struct{}

// NewExcelService creates a new Excel service instance
func NewExcelService() *ExcelService {
	return &ExcelService{}
}

// ExcelExportConfig defines configuration for Excel export
type ExcelExportConfig struct {
	SheetName          string
	Headers            []string
	Data               [][]interface{}
	ColumnWidths       map[string]float64
	ConditionalFormats []ConditionalFormat
}

// ConditionalFormat defines conditional formatting rules
type ConditionalFormat struct {
	Column    string // Column letter (e.g., "C")
	RowStart  int    // Starting row (usually 2, after header)
	RowEnd    int    // Ending row
	Condition string // Value to match (e.g., "normal", "warning")
	Color     string // Hex color (e.g., "#92D050" for green)
}

// GenerateExcel creates an Excel file based on the provided configuration
func (s *ExcelService) GenerateExcel(config ExcelExportConfig) (*excelize.File, error) {
	// Create new Excel file
	f := excelize.NewFile()

	// Create sheet
	index, err := f.NewSheet(config.SheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to create sheet: %w", err)
	}

	// Set headers
	if err := s.setHeaders(f, config.SheetName, config.Headers); err != nil {
		return nil, fmt.Errorf("failed to set headers: %w", err)
	}

	// Apply header style
	lastCol := string(rune('A' + len(config.Headers) - 1))
	if err := s.applyHeaderStyle(f, config.SheetName, lastCol); err != nil {
		return nil, fmt.Errorf("failed to apply header style: %w", err)
	}

	// Fill data
	if err := s.fillData(f, config.SheetName, config.Data); err != nil {
		return nil, fmt.Errorf("failed to fill data: %w", err)
	}

	// Set column widths
	if err := s.setColumnWidths(f, config.SheetName, config.ColumnWidths); err != nil {
		return nil, fmt.Errorf("failed to set column widths: %w", err)
	}

	// Apply conditional formatting
	if len(config.ConditionalFormats) > 0 {
		if err := s.applyConditionalFormatting(f, config.SheetName, config.ConditionalFormats); err != nil {
			return nil, fmt.Errorf("failed to apply conditional formatting: %w", err)
		}
	}

	// Set active sheet and delete default Sheet1
	f.SetActiveSheet(index)
	f.DeleteSheet("Sheet1")

	return f, nil
}

// setHeaders sets the header row
func (s *ExcelService) setHeaders(f *excelize.File, sheetName string, headers []string) error {
	for i, header := range headers {
		cell := fmt.Sprintf("%s1", string(rune('A'+i)))
		if err := f.SetCellValue(sheetName, cell, header); err != nil {
			return err
		}
	}
	return nil
}

// applyHeaderStyle applies styling to the header row
func (s *ExcelService) applyHeaderStyle(f *excelize.File, sheetName, lastCol string) error {
	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
			Color: "#FFFFFF",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#4472C4"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})
	if err != nil {
		return err
	}

	return f.SetCellStyle(sheetName, "A1", lastCol+"1", headerStyle)
}

// fillData fills the data rows
func (s *ExcelService) fillData(f *excelize.File, sheetName string, data [][]interface{}) error {
	for rowIdx, rowData := range data {
		row := rowIdx + 2 // Start from row 2 (after header)
		for colIdx, value := range rowData {
			cell := fmt.Sprintf("%s%d", string(rune('A'+colIdx)), row)
			if err := f.SetCellValue(sheetName, cell, value); err != nil {
				return err
			}
		}
	}
	return nil
}

// setColumnWidths sets the width for each column
func (s *ExcelService) setColumnWidths(f *excelize.File, sheetName string, widths map[string]float64) error {
	for col, width := range widths {
		if err := f.SetColWidth(sheetName, col, col, width); err != nil {
			return err
		}
	}
	return nil
}

// applyConditionalFormatting applies conditional formatting rules
func (s *ExcelService) applyConditionalFormatting(f *excelize.File, sheetName string, formats []ConditionalFormat) error {
	for _, format := range formats {
		// Create style for this condition
		style, err := f.NewStyle(&excelize.Style{
			Fill: excelize.Fill{
				Type:    "pattern",
				Color:   []string{format.Color},
				Pattern: 1,
			},
		})
		if err != nil {
			return err
		}

		// Apply to matching cells
		for row := format.RowStart; row <= format.RowEnd; row++ {
			cell := fmt.Sprintf("%s%d", format.Column, row)
			
			// Get cell value
			value, err := f.GetCellValue(sheetName, cell)
			if err != nil {
				continue
			}

			// Apply style if value matches condition
			if value == format.Condition {
				if err := f.SetCellStyle(sheetName, cell, cell, style); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// GenerateFilename generates a consistent filename with timestamp
// Format: {prefix}_HHMM_DDMMYYYY.xlsx
func (s *ExcelService) GenerateFilename(prefix string) string {
	timestamp := time.Now().Format("1504_02012006")
	return fmt.Sprintf("%s_%s.xlsx", prefix, timestamp)
}
