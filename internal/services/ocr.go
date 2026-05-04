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
	"time"
)

// OCRService handles OCR operations using Google Gemini API
type OCRService struct {
	apiKey string
	client *http.Client
}

// NewOCRService creates a new OCR service
func NewOCRService(apiKey string) *OCRService {
	return &OCRService{
		apiKey: apiKey,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// LogbookEntry represents extracted logbook data
type LogbookEntry struct {
	Date        string `json:"date"`
	StudentName string `json:"student_name"`
	NIM         string `json:"nim"`
	TimeIn      string `json:"time_in"`
	TimeOut     string `json:"time_out"`
	Notes       string `json:"notes"`
}

// OCRResult represents the result of OCR processing
type OCRResult struct {
	Success bool            `json:"success"`
	Entries []LogbookEntry  `json:"entries"`
	RawText string          `json:"raw_text"`
	Error   string          `json:"error,omitempty"`
}

// GeminiRequest represents request to Gemini API
type GeminiRequest struct {
	Contents []GeminiContent `json:"contents"`
}

type GeminiContent struct {
	Parts []GeminiPart `json:"parts"`
}

type GeminiPart struct {
	Text       string            `json:"text,omitempty"`
	InlineData *GeminiInlineData `json:"inlineData,omitempty"`
}

type GeminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

// GeminiResponse represents response from Gemini API
type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

// ExtractLogbookFromImage extracts logbook data from image using Gemini API
func (s *OCRService) ExtractLogbookFromImage(imagePath string) (*OCRResult, error) {
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

	// Create prompt for Gemini
	prompt := `Analyze this image of a handwritten logbook/attendance table. 
Extract the data and return it in JSON format with the following structure:

{
  "entries": [
    {
      "date": "YYYY-MM-DD",
      "student_name": "Full Name",
      "nim": "Student ID",
      "time_in": "HH:MM",
      "time_out": "HH:MM",
      "notes": "Any notes"
    }
  ]
}

Rules:
1. Extract ALL rows from the table
2. If a field is empty or unclear, use empty string ""
3. For date, try to parse to YYYY-MM-DD format
4. For time, use HH:MM format (24-hour)
5. Return ONLY valid JSON, no additional text
6. If you cannot read the handwriting clearly, make your best guess

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
	// Using gemini-3-flash-preview which works correctly
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

	// Parse extracted data
	var result struct {
		Entries []LogbookEntry `json:"entries"`
	}

	// Initialize with empty slice to avoid nil
	result.Entries = []LogbookEntry{}

	if err := json.Unmarshal([]byte(jsonText), &result); err != nil {
		// If JSON parsing fails, return raw text for manual processing
		return &OCRResult{
			Success: false,
			Entries: []LogbookEntry{}, // Empty array instead of nil
			RawText: responseText,
			Error:   fmt.Sprintf("Failed to parse JSON: %v. Raw JSON text length: %d chars. Please check the raw text.", err, len(jsonText)),
		}, nil
	}

	// Check if entries were actually parsed
	if len(result.Entries) == 0 {
		return &OCRResult{
			Success: false,
			Entries: []LogbookEntry{}, // Empty array
			RawText: responseText,
			Error:   "JSON parsed successfully but no entries found. Check if JSON structure matches expected format.",
		}, nil
	}

	return &OCRResult{
		Success: true,
		Entries: result.Entries,
		RawText: responseText,
	}, nil
}
