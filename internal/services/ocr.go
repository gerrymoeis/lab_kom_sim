package services

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
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
	Purpose     string `json:"purpose"` // Changed from Notes to Purpose (keperluan)
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
      "purpose": "Purpose/reason for attendance"
    }
  ]
}

CRITICAL RULES - READ CAREFULLY:

1. EXTRACT ALL ROWS from the table

2. SMART CONTEXT UNDERSTANDING:
   - If a field is empty or shows ditto marks ("~~~", "\\", "''", or similar), COPY the value from the row above
   - If date is empty but other rows have dates, INFER the date from context (usually same date for consecutive entries)
   - If you see spelling errors or typos, CORRECT them intelligently based on context
   - Examples:
     * "Pemrograman Web lanjut" with typo → Correct to "Pemrograman Web Lanjut"
     * Empty date but previous row is "05/05/2026" → Use "05/05/2026"
     * "~~~" in purpose field → Copy purpose from row above
     * "Rian Dwi Hermawan" with inconsistent capitalization → Standardize to proper Title Case

3. NAME ABBREVIATIONS:
   - For middle abbreviations (initials), add dots between letters: "Herman SW" → "Herman S.W"
   - NEVER add trailing dot at the end: "Herman S.W." → "Herman S.W"
   - Examples:
     * "Rian DH" → "Rian D.H"
     * "Herman SW" → "Herman S.W"
     * "Najwa AS" → "Najwa A.S"
   - Apply this consistently to all names

4. DATE HANDLING:
   - Parse to YYYY-MM-DD format
   - Accept formats: DD/MM/YYYY, DD-MM-YYYY, D/M/YYYY
   - If date is missing but can be inferred from context, fill it in
   - If completely unclear, use empty string ""

4. TIME FIELDS:
   - If you see a COMBINED time range like "13.00 - 14.40" or "13:00 - 14:40":
     * SPLIT it into two separate times
     * Put the START time in "time_in" field
     * Put the END time in "time_out" field
   - If you see dots (.) in time format, CONVERT them to colons (:)
   - Examples:
     * Input: "13.00 - 14.40" → Output: time_in="13:00", time_out="14:40"
     * Input: "09.00 - 10.20" → Output: time_in="09:00", time_out="10:20"
     * Input: "~~~" → Copy from row above
   - Always use HH:MM format (24-hour)

5. TEXT QUALITY:
   - Fix obvious spelling mistakes
   - Standardize capitalization (proper names should be Title Case)
   - Remove extra spaces
   - Be intelligent about abbreviations (e.g., "Pemrog Web" → "Pemrograman Web")

6. RETURN ONLY valid JSON, no additional text or explanations

Please extract the data now with smart context understanding:`

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

	// Normalize time format for all entries (post-processing fallback)
	for i := range result.Entries {
		normalizeTimeEntry(&result.Entries[i])
		
		// Apply text normalization
		result.Entries[i].StudentName = toTitleCase(result.Entries[i].StudentName)
		result.Entries[i].Purpose = toTitleCase(result.Entries[i].Purpose)
		result.Entries[i].NIM = strings.ToUpper(strings.TrimSpace(result.Entries[i].NIM))
		
		// Remove extra whitespace from NIM
		result.Entries[i].NIM = strings.ReplaceAll(result.Entries[i].NIM, " ", "")
	}

	return &OCRResult{
		Success: true,
		Entries: result.Entries,
		RawText: responseText,
	}, nil
}

// normalizeTimeEntry normalizes time format in logbook entry
// Handles combined time ranges and dot-to-colon conversion
func normalizeTimeEntry(entry *LogbookEntry) {
	// Handle combined time range in time_in field
	// Pattern: "HH.MM - HH.MM" or "HH:MM - HH:MM"
	if strings.Contains(entry.TimeIn, "-") {
		parts := strings.Split(entry.TimeIn, "-")
		if len(parts) == 2 {
			entry.TimeIn = strings.TrimSpace(parts[0])
			// Only set time_out if it's empty
			if entry.TimeOut == "" {
				entry.TimeOut = strings.TrimSpace(parts[1])
			}
		}
	}
	
	// Convert dots to colons in both fields
	entry.TimeIn = strings.ReplaceAll(entry.TimeIn, ".", ":")
	entry.TimeOut = strings.ReplaceAll(entry.TimeOut, ".", ":")
	
	// Validate and pad time format (ensure HH:MM)
	entry.TimeIn = normalizeTimeFormat(entry.TimeIn)
	entry.TimeOut = normalizeTimeFormat(entry.TimeOut)
}

// normalizeTimeFormat ensures time is in HH:MM format
func normalizeTimeFormat(timeStr string) string {
	if timeStr == "" {
		return ""
	}
	
	// Remove any extra spaces
	timeStr = strings.TrimSpace(timeStr)
	
	// If already in HH:MM format, return as is
	if matched, _ := regexp.MatchString(`^\d{2}:\d{2}$`, timeStr); matched {
		return timeStr
	}
	
	// Try to parse and reformat
	// Handle formats like "9:00" → "09:00"
	parts := strings.Split(timeStr, ":")
	if len(parts) == 2 {
		hour := strings.TrimSpace(parts[0])
		minute := strings.TrimSpace(parts[1])
		
		// Pad with zero if needed
		if len(hour) == 1 {
			hour = "0" + hour
		}
		if len(minute) == 1 {
			minute = "0" + minute
		}
		
		// Validate hour and minute are numeric
		if matched, _ := regexp.MatchString(`^\d+$`, hour); matched {
			if matched, _ := regexp.MatchString(`^\d+$`, minute); matched {
				return hour + ":" + minute
			}
		}
	}
	
	// If cannot parse, return as is
	return timeStr
}

// normalizeText normalizes text by:
// - Trimming leading/trailing whitespace
// - Removing double spaces
// - Converting to Title Case for proper names
func normalizeText(text string) string {
	if text == "" {
		return ""
	}
	
	// Trim leading and trailing whitespace
	text = strings.TrimSpace(text)
	
	// Replace multiple spaces with single space
	re := regexp.MustCompile(`\s+`)
	text = re.ReplaceAllString(text, " ")
	
	return text
}

// toTitleCase converts text to Title Case (proper capitalization)
func toTitleCase(text string) string {
	if text == "" {
		return ""
	}
	
	// Normalize first
	text = normalizeText(text)
	
	// Split by space and capitalize each word
	words := strings.Fields(text)
	for i, word := range words {
		if len(word) > 0 {
			// Convert to lowercase first, then capitalize first letter
			words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
		}
	}
	
	result := strings.Join(words, " ")
	
	// Normalize abbreviations (singkatan)
	result = normalizeAbbreviations(result)
	
	return result
}

// normalizeAbbreviations normalizes abbreviations in names
// Rules:
// - Middle abbreviations get dots: "Herman SW" → "Herman S.W"
// - No trailing dot at end: "Herman S.W." → "Herman S.W"
func normalizeAbbreviations(text string) string {
	if text == "" {
		return ""
	}
	
	// Pattern: single uppercase letter followed by space or another uppercase letter
	// This handles cases like "SW", "SH", "A", etc.
	re := regexp.MustCompile(`\b([A-Z])([A-Z])\b`)
	
	// Add dots between consecutive uppercase letters
	// "SW" → "S.W", "SH" → "S.H"
	text = re.ReplaceAllString(text, "$1.$2")
	
	// Remove trailing dot at the end of text
	text = strings.TrimSuffix(text, ".")
	
	return text
}
