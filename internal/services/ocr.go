package services

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

// OCRService handles OCR operations using AI vision APIs
type OCRService struct {
	geminiKey     string
	openRouterKey string
	client        *http.Client
}

// NewOCRService creates a new OCR service
// geminiKey: Google Gemini API key (used as fallback)
// openRouterKey: OpenRouter API key (used as primary via openrouter/free)
func NewOCRService(geminiKey, openRouterKey string) *OCRService {
	return &OCRService{
		geminiKey:     geminiKey,
		openRouterKey: openRouterKey,
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
	Success bool           `json:"success"`
	Entries []LogbookEntry `json:"entries"`
	RawText string         `json:"raw_text"`
	Error   string         `json:"error,omitempty"`
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

// ExtractLogbookFromImage extracts logbook data from image using AI vision API
// Strategy: OpenRouter (primary)  ->  Gemini (fallback)
func (s *OCRService) ExtractLogbookFromImage(imagePath string) (*OCRResult, error) {
	totalStart := time.Now()
	log.Printf("[OCR] Starting OCR for %s", imagePath)

	if s.geminiKey == "" && s.openRouterKey == "" {
		return nil, fmt.Errorf("OCR tidak dapat diproses: API key tidak dikonfigurasi")
	}

	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read image: %w", err)
	}

	base64Image := base64.StdEncoding.EncodeToString(imageData)

	mimeType := "image/jpeg"
	if strings.HasSuffix(strings.ToLower(imagePath), ".png") {
		mimeType = "image/png"
	}

	var responseText string

	if s.openRouterKey != "" {
		log.Printf("[OCR] Trying OpenRouter (openrouter/free) primary...")
		responseText, err = s.tryProvider(s.callOpenRouter, "OpenRouter", base64Image, mimeType, totalStart)
		if err == nil {
			return s.parseOCRResponse(responseText)
		}
		log.Printf("[OCR] OpenRouter failed, falling back to Gemini: %v", err)
	}

	if s.geminiKey != "" {
		responseText, err = s.tryProvider(s.callGemini, "Gemini", base64Image, mimeType, totalStart)
		if err == nil {
			return s.parseOCRResponse(responseText)
		}
	}

	return nil, fmt.Errorf("OCR failed after %v: %w", time.Since(totalStart), err)
}

// tryProvider runs the provided callFn in a retry loop
func (s *OCRService) tryProvider(callFn func(string, string) (string, error), name, base64Image, mimeType string, totalStart time.Time) (string, error) {
	maxRetries := 3
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			log.Printf("[OCR] %s retry %d/%d after %v (elapsed: %v)", name, attempt, maxRetries, backoff, time.Since(totalStart))
			time.Sleep(backoff)
		}

		callStart := time.Now()
		responseText, err := callFn(base64Image, mimeType)
		if err == nil {
			log.Printf("[OCR] %s success on attempt %d in %v (total: %v)", name, attempt+1, time.Since(callStart), time.Since(totalStart))
			return responseText, nil
		}

		if !isTransientError(err) {
			log.Printf("[OCR] %s non-transient error attempt %d in %v: %v", name, attempt+1, time.Since(callStart), err)
			return "", err
		}
		log.Printf("[OCR] %s transient error attempt %d in %v: %v", name, attempt+1, time.Since(callStart), err)
		lastErr = err
	}
	return "", lastErr
}

// parseOCRResponse parses Gemini/OpenRouter response text into OCRResult
func (s *OCRService) parseOCRResponse(responseText string) (*OCRResult, error) {
	jsonText := responseText
	if strings.Contains(responseText, "```json") {
		start := strings.Index(responseText, "```json")
		if start != -1 {
			start += 7
			end := strings.Index(responseText[start:], "```")
			if end != -1 {
				jsonText = responseText[start : start+end]
			}
		}
	} else if strings.Contains(responseText, "```") {
		start := strings.Index(responseText, "```")
		if start != -1 {
			start += 3
			end := strings.Index(responseText[start:], "```")
			if end != -1 {
				jsonText = responseText[start : start+end]
			}
		}
	}
	jsonText = strings.TrimSpace(jsonText)

	var result struct {
		Entries []LogbookEntry `json:"entries"`
	}
	result.Entries = []LogbookEntry{}

	if err := json.Unmarshal([]byte(jsonText), &result); err != nil {
		return &OCRResult{
			Success: false,
			Entries: []LogbookEntry{},
			RawText: responseText,
			Error:   fmt.Sprintf("Gagal parse JSON: %v", err),
		}, nil
	}

	if len(result.Entries) == 0 {
		return &OCRResult{
			Success: false,
			Entries: []LogbookEntry{},
			RawText: responseText,
			Error:   "JSON valid tapi tidak ada entry ditemukan.",
		}, nil
	}

	for i := range result.Entries {
		normalizeTimeEntry(&result.Entries[i])
		result.Entries[i].StudentName = ToTitleCaseWithAbbr(result.Entries[i].StudentName)
		result.Entries[i].Purpose = ToTitleCaseWithAbbr(result.Entries[i].Purpose)
		result.Entries[i].NIM = strings.ToUpper(strings.TrimSpace(result.Entries[i].NIM))
		result.Entries[i].NIM = strings.ReplaceAll(result.Entries[i].NIM, " ", "")
	}

	return &OCRResult{
		Success: true,
		Entries: result.Entries,
		RawText: responseText,
	}, nil
}

// callGemini sends image to Gemini API and returns response text
func (s *OCRService) callGemini(base64Image, mimeType string) (string, error) {
	prompt := buildOCRPrompt()
	reqBody := GeminiRequest{
		Contents: []GeminiContent{{Parts: []GeminiPart{
			{Text: prompt},
			{InlineData: &GeminiInlineData{MimeType: mimeType, Data: base64Image}},
		}}},
	}
	jsonData, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-3-flash-preview:generateContent?key=%s", s.geminiKey)
	body, err := s.doAPIRequest("POST", url, jsonData, nil)
	if err != nil { return "", err }

	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil { return "", fmt.Errorf("failed to parse response: %w", err) }
	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response from Gemini API")
	}
	return geminiResp.Candidates[0].Content.Parts[0].Text, nil
}

// OpenRouterResponse represents response from OpenRouter API (OpenAI-compatible)
type OpenRouterResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// callOpenRouter sends image to OpenRouter API and returns response text
func (s *OCRService) callOpenRouter(base64Image, mimeType string) (string, error) {
	prompt := buildOCRPrompt()
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Image)
	reqBody := map[string]any{
		"model": "openrouter/free",
		"messages": []map[string]any{{
			"role": "user",
			"content": []map[string]any{
				{"type": "text", "text": prompt},
				{"type": "image_url", "image_url": map[string]string{"url": dataURL}},
			},
		}},
	}
	jsonData, _ := json.Marshal(reqBody)
	body, err := s.doAPIRequest("POST", "https://openrouter.ai/api/v1/chat/completions", jsonData, map[string]string{"Authorization": "Bearer " + s.openRouterKey})
	if err != nil { return "", err }

	var orResp OpenRouterResponse
	if err := json.Unmarshal(body, &orResp); err != nil { return "", fmt.Errorf("failed to parse response: %w", err) }
	if len(orResp.Choices) == 0 { return "", fmt.Errorf("no response from OpenRouter") }
	return orResp.Choices[0].Message.Content, nil
}

func (s *OCRService) doAPIRequest(method, url string, jsonData []byte, extraHeaders map[string]string) ([]byte, error) {
	req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonData))
	if err != nil { return nil, fmt.Errorf("failed to create request: %w", err) }
	req.Header.Set("Content-Type", "application/json")
	for k, v := range extraHeaders { req.Header.Set(k, v) }

	resp, err := s.client.Do(req)
	if err != nil { return nil, fmt.Errorf("failed to call API: %w", err) }
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil { return nil, fmt.Errorf("failed to read response: %w", err) }
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func buildOCRPrompt() string {
	return `Analyze this image of a handwritten logbook/attendance table.
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

3. NAME ABBREVIATIONS:
   - For middle abbreviations (initials), add dots between letters: "Herman SW"  ->  "Herman S.W"
   - NEVER add trailing dot at the end: "Herman S.W."  ->  "Herman S.W"
   - Apply this consistently to all names

4. DATE HANDLING:
   - Parse to YYYY-MM-DD format
   - Accept formats: DD/MM/YYYY, DD-MM-YYYY, D/M/YYYY

5. NIM (STUDENT ID) VALIDATION:
   - NIM format: 11 digits (example: 24091397XXX)
   - If same student name appears multiple times, NIM MUST be EXACTLY the same
   - Common OCR errors: 4 -> 9, 3 -> 8, 1 -> 7, 0 -> 6

6. TIME FIELDS:
   - If you see a combined time range like "13.00 - 14.40":
     * SPLIT it: time_in="13:00", time_out="14:40"
   - Convert dots (.) to colons (:)
   - Always use HH:MM format (24-hour)

7. TEXT QUALITY:
   - Fix obvious spelling mistakes
   - Standardize capitalization (Title Case)
   - Be intelligent about abbreviations (e.g., "Pemrog Web"  ->  "Pemrograman Web")

8. RETURN ONLY valid JSON, no additional text or explanations

Please extract the data now with smart context understanding:`
}

// isTransientError returns true if the error is retryable
func isTransientError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "status 429") ||
		strings.Contains(msg, "status 500") ||
		strings.Contains(msg, "status 502") ||
		strings.Contains(msg, "status 503") ||
		strings.Contains(msg, "status 504")
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
	// Handle formats like "9:00"  ->  "09:00"
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


