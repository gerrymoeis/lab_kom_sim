package services

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// FinanceEntry represents extracted finance table data
type FinanceEntry struct {
	No          string `json:"no"`
	Jenis       string `json:"jenis"`
	Nama        string `json:"nama"`
	HargaDasar  string `json:"harga_dasar"`
	HargaJual   string `json:"harga_jual"`
	Untung      string `json:"untung"`
	TotalUntung string `json:"total_untung"`
	Hutang      string `json:"hutang"`
	TotalHutang string `json:"total_hutang"`
	Bayar       string `json:"bayar"`
	SaldoTunai  string `json:"saldo_tunai"`
	SaldoAkun   string `json:"saldo_akun"`
	Total       string `json:"total"`
}

// FinanceOCRResult represents the result of finance OCR processing
type FinanceOCRResult struct {
	Success bool           `json:"success"`
	Entries []FinanceEntry `json:"entries"`
	RawText string         `json:"raw_text"`
	Error   string         `json:"error,omitempty"`
}

// ExtractFinanceTableFromImage extracts finance table data from image using Gemini API
func (s *OCRService) ExtractFinanceTableFromImage(imagePath string) (*FinanceOCRResult, error) {
	// Read image file
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read image: %w", err)
	}

	// Encode image to base64
	base64Image := base64.StdEncoding.EncodeToString(imageData)

	// Determine mime type
	mimeType := "image/jpeg"
	if strings.HasSuffix(strings.ToLower(imagePath), ".png") {
		mimeType = "image/png"
	}

	// Create prompt for Gemini - Finance table specific
	prompt := `Analyze this image of a handwritten finance/accounting table.
Extract ALL the data and return it in JSON format with the following structure:

{
  "entries": [
    {
      "no": "Row number or ID",
      "jenis": "Type/Category",
      "nama": "Name/Description",
      "harga_dasar": "Base price (keep format: Rp. 25.000)",
      "harga_jual": "Selling price (keep format: Rp. 25.000)",
      "untung": "Profit (keep format: Rp. 2.875)",
      "total_untung": "Total profit (keep format: Rp. 14.612)",
      "hutang": "Debt (keep format: Rp. 25.000)",
      "total_hutang": "Total debt (keep format: Rp. 25.000)",
      "bayar": "Payment (keep format: Rp. 25.000)",
      "saldo_tunai": "Cash balance (keep format: Rp. 25.000)",
      "saldo_akun": "Account balance (keep format: Rp. 25.000)",
      "total": "Total (keep format: Rp. 25.000)"
    }
  ]
}

IMPORTANT RULES:
1. Extract ALL rows from the table (don't skip any rows with data)
2. If a cell is empty or unclear, use empty string ""
3. Keep number formatting EXACTLY as written (e.g., "Rp. 25.000", "Rp. 2.875")
4. Do NOT convert to plain numbers - preserve the original format
5. For unclear handwriting, make your best guess based on context
6. Return ONLY valid JSON, no additional text or explanation
7. The table has columns: No, Jenis, Nama, Harga Dasar, Harga Jual, Untung, Total Untung, Hutang, Total Hutang, Bayar, Saldo Tunai, Saldo Akun, Total

Please extract the data now:`

	// Create request
	reqBody := GeminiRequest{
		Contents: []GeminiContent{
			{
				Parts: []GeminiPart{
					{
						Text: prompt,
					},
					{
						InlineData: &GeminiInlineData{
							MimeType: mimeType,
							Data:     base64Image,
						},
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Call Gemini API
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-3-flash-preview:generateContent?key=%s", s.apiKey)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Gemini API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Gemini API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response from Gemini API")
	}

	// Extract text from response
	responseText := geminiResp.Candidates[0].Content.Parts[0].Text

	// Try to parse JSON from response
	// Sometimes Gemini wraps JSON in markdown code blocks
	jsonText := responseText

	// Remove markdown code blocks if present
	if strings.Contains(responseText, "```json") {
		// Find the start after ```json
		start := strings.Index(responseText, "```json")
		if start != -1 {
			start += 7 // length of "```json"
			// Find the closing ```
			end := strings.Index(responseText[start:], "```")
			if end != -1 {
				jsonText = responseText[start : start+end]
			}
		}
	} else if strings.Contains(responseText, "```") {
		// Generic code block
		start := strings.Index(responseText, "```")
		if start != -1 {
			start += 3 // length of "```"
			end := strings.Index(responseText[start:], "```")
			if end != -1 {
				jsonText = responseText[start : start+end]
			}
		}
	}

	jsonText = strings.TrimSpace(jsonText)

	// Debug: Print jsonText length and first 100 chars
	// fmt.Printf("DEBUG: jsonText length: %d\n", len(jsonText))
	// if len(jsonText) > 100 {
	// 	fmt.Printf("DEBUG: First 100 chars: %s\n", jsonText[:100])
	// }

	// Parse extracted data
	var result struct {
		Entries []FinanceEntry `json:"entries"`
	}

	// Initialize with empty slice to avoid nil
	result.Entries = []FinanceEntry{}

	if err := json.Unmarshal([]byte(jsonText), &result); err != nil {
		// If JSON parsing fails, return raw text for manual processing
		return &FinanceOCRResult{
			Success: false,
			Entries: []FinanceEntry{}, // Empty array instead of nil
			RawText: responseText,
			Error:   fmt.Sprintf("Failed to parse JSON: %v. Raw JSON text length: %d chars. Please check the raw text.", err, len(jsonText)),
		}, nil
	}

	// Check if entries were actually parsed
	if len(result.Entries) == 0 {
		return &FinanceOCRResult{
			Success: false,
			Entries: []FinanceEntry{}, // Empty array
			RawText: responseText,
			Error:   "JSON parsed successfully but no entries found. Check if JSON structure matches expected format.",
		}, nil
	}

	return &FinanceOCRResult{
		Success: true,
		Entries: result.Entries,
		RawText: responseText,
	}, nil
}
