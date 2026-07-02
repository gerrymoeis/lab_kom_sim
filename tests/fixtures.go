package tests

import (
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

func createTempJPEG(t *testing.T) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{255, 255, 255, 255})
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jpg")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return path
}

func geminiResp(ocrText string) string {
	b, _ := json.Marshal(map[string]any{
		"candidates": []any{map[string]any{
			"content": map[string]any{
				"parts": []any{map[string]any{"text": ocrText}},
			},
		}},
	})
	return string(b)
}

func openRouterResp(ocrText string) string {
	b, _ := json.Marshal(map[string]any{
		"choices": []any{map[string]any{
			"message": map[string]any{"content": ocrText},
		}},
	})
	return string(b)
}

func validOCRJSON() string {
	return `{"entries":[{"date":"2026-06-23","student_name":"Budi Santoso","nim":"24091397001","time_in":"08:00","time_out":"10:00","purpose":"Praktikum"}]}`
}


