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
	Data               [][]any
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
	f := excelize.NewFile()
	if err := f.SetSheetName("Sheet1", config.SheetName); err != nil {
		return nil, fmt.Errorf("failed to rename default sheet: %w", err)
	}
	if err := s.populateSheet(f, config); err != nil { return nil, err }
	f.SetActiveSheet(0)
	return f, nil
}

func (s *ExcelService) populateSheet(f *excelize.File, config ExcelExportConfig) error {
	if err := s.setHeaders(f, config.SheetName, config.Headers); err != nil {
		return fmt.Errorf("failed to set headers: %w", err)
	}
	lastCol := string(rune('A' + len(config.Headers) - 1))
	if err := s.applyHeaderStyle(f, config.SheetName, lastCol); err != nil {
		return fmt.Errorf("failed to apply header style: %w", err)
	}
	if err := s.fillData(f, config.SheetName, config.Data); err != nil {
		return fmt.Errorf("failed to fill data: %w", err)
	}
	if err := s.setColumnWidths(f, config.SheetName, config.ColumnWidths); err != nil {
		return fmt.Errorf("failed to set column widths: %w", err)
	}
	if len(config.ConditionalFormats) > 0 {
		if err := s.applyConditionalFormatting(f, config.SheetName, config.ConditionalFormats); err != nil {
			return fmt.Errorf("failed to apply conditional formatting: %w", err)
		}
	}
	return nil
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
func (s *ExcelService) fillData(f *excelize.File, sheetName string, data [][]any) error {
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

// applyConditionalFormatting applies conditional formatting rules using native excelize SetConditionalFormat
func (s *ExcelService) applyConditionalFormatting(f *excelize.File, sheetName string, formats []ConditionalFormat) error {
	for _, format := range formats {
		styleIdx, err := f.NewConditionalStyle(&excelize.Style{
			Fill: excelize.Fill{
				Type:    "pattern",
				Color:   []string{format.Color},
				Pattern: 1,
			},
		})
		if err != nil {
			return err
		}
		cellRef := fmt.Sprintf("%s%d:%s%d", format.Column, format.RowStart, format.Column, format.RowEnd)
		if err := f.SetConditionalFormat(sheetName, cellRef, []excelize.ConditionalFormatOptions{
			{Type: "cell", Criteria: "==", Value: format.Condition, Format: &styleIdx},
		}); err != nil {
			return err
		}
	}
	return nil
}

// GenerateFilename generates a consistent filename with timestamp
// Format: {prefix}_YYYYMMDD_HHMMSS.xlsx
func (s *ExcelService) GenerateFilename(prefix string) string {
	timestamp := time.Now().Format("20060102_150405")
	return fmt.Sprintf("%s_%s.xlsx", prefix, timestamp)
}

// GenerateMultiSheetExcel creates an Excel file with multiple sheets
func (s *ExcelService) GenerateMultiSheetExcel(configs []ExcelExportConfig) (*excelize.File, error) {
	if len(configs) == 0 { return nil, fmt.Errorf("no sheet configurations provided") }

	f := excelize.NewFile()
	deleteDefaultSheet := true
	firstSheetIndex := -1

	for i, config := range configs {
		var sheetIndex int; var err error
		if i == 0 {
			if err := f.SetSheetName("Sheet1", config.SheetName); err != nil { return nil, fmt.Errorf("failed to rename default sheet: %w", err) }
			sheetIndex, deleteDefaultSheet = 0, false
		} else {
			sheetIndex, err = f.NewSheet(config.SheetName)
			if err != nil { return nil, fmt.Errorf("failed to create sheet %s: %w", config.SheetName, err) }
		}
		if firstSheetIndex == -1 { firstSheetIndex = sheetIndex }
		if err := s.populateSheet(f, config); err != nil { return nil, err }
	}

	if firstSheetIndex >= 0 { f.SetActiveSheet(firstSheetIndex) }
	if deleteDefaultSheet { f.DeleteSheet("Sheet1") }
	return f, nil
}
