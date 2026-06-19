package tests

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func TestAPIStickerTemplates(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA

	if !loginAndRefresh(lab, "labA_only", "test123") {
		t.Fatal("login failed")
	}

	var createdID int
	const stickerType = "pc"
	const fontSize = 1.0
	const padH = 0.5
	const padV = 0.8

	t.Run("list_empty", func(t *testing.T) {
		resp, err := lab.get("/api/sticker-templates?type=pc")
		if err != nil {
			t.Fatalf("GET /api/sticker-templates: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		var list []map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(list) != 0 {
			t.Errorf("expected empty list, got %d items", len(list))
		}
	})

	t.Run("create", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		body := `{"name":"Template Test PC","sticker_type":"pc","font_size_cm":1.0,"padding_h_cm":0.5,"padding_v_cm":0.8}`
		resp, err := lab.postJSON("/api/sticker-templates", body)
		if err != nil {
			t.Fatalf("POST /api/sticker-templates: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 201 {
			t.Errorf("expected 201, got %d", resp.StatusCode)
		}
		var createRes struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&createRes); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !createRes.Success {
			t.Error("create success=false")
		}
	})

	t.Run("list_after_create", func(t *testing.T) {
		resp, err := lab.get("/api/sticker-templates?type=pc")
		if err != nil {
			t.Fatalf("GET /api/sticker-templates: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		var list []struct {
			ID          int     `json:"id"`
			Name        string  `json:"name"`
			StickerType string  `json:"sticker_type"`
			FontSizeCM  float64 `json:"font_size_cm"`
			PaddingHCM  float64 `json:"padding_h_cm"`
			PaddingVCM  float64 `json:"padding_v_cm"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(list) == 0 {
			t.Fatal("expected 1 template, got 0")
		}
		var found bool
		for _, tmpl := range list {
			if tmpl.Name == "Template Test PC" && tmpl.StickerType == "pc" {
				createdID = tmpl.ID
				found = true
				break
			}
		}
		if !found {
			t.Error("created template not found in list")
		}
	})

	t.Run("update", func(t *testing.T) {
		if createdID == 0 {
			t.Skip("no template ID from list_after_create")
		}
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		body := `{"name":"Template Updated","sticker_type":"pc","font_size_cm":1.5,"padding_h_cm":0.6,"padding_v_cm":1.0}`
		req, _ := http.NewRequest("PUT", env.TS.URL+lab.prefix+"/api/sticker-templates/"+strconv.Itoa(createdID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", lab.csrf)
		lab.addCookies(req)
		resp, err := lab.client.Do(req)
		if err != nil {
			t.Fatalf("PUT /api/sticker-templates/%d: %v", createdID, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		var updateRes struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&updateRes); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !updateRes.Success {
			t.Error("update success=false")
		}
	})

	t.Run("list_after_update", func(t *testing.T) {
		resp, err := lab.get("/api/sticker-templates?type=pc")
		if err != nil {
			t.Fatalf("GET /api/sticker-templates: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		var list []struct {
			ID          int     `json:"id"`
			Name        string  `json:"name"`
			StickerType string  `json:"sticker_type"`
			FontSizeCM  float64 `json:"font_size_cm"`
			PaddingHCM  float64 `json:"padding_h_cm"`
			PaddingVCM  float64 `json:"padding_v_cm"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
			t.Fatalf("decode: %v", err)
		}
		var found bool
		for _, tmpl := range list {
			if tmpl.ID == createdID && tmpl.Name == "Template Updated" {
				found = true
				break
			}
		}
		if !found {
			t.Error("updated template not found in list")
		}
	})

	t.Run("delete", func(t *testing.T) {
		if createdID == 0 {
			t.Skip("no template ID from list_after_create")
		}
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		req, _ := http.NewRequest("DELETE", env.TS.URL+lab.prefix+"/api/sticker-templates/"+strconv.Itoa(createdID), nil)
		req.Header.Set("X-CSRF-Token", lab.csrf)
		lab.addCookies(req)
		resp, err := lab.client.Do(req)
		if err != nil {
			t.Fatalf("DELETE /api/sticker-templates/%d: %v", createdID, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		var deleteRes struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&deleteRes); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !deleteRes.Success {
			t.Error("delete success=false")
		}
	})

	t.Run("list_after_delete", func(t *testing.T) {
		resp, err := lab.get("/api/sticker-templates?type=pc")
		if err != nil {
			t.Fatalf("GET /api/sticker-templates: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		var list []map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(list) != 0 {
			t.Errorf("expected empty list, got %d items", len(list))
		}
	})

	t.Run("fail_invalid_type", func(t *testing.T) {
		resp, err := lab.get("/api/sticker-templates?type=invalid")
		if err != nil {
			t.Fatalf("GET /api/sticker-templates?type=invalid: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Errorf("expected 400, got %d", resp.StatusCode)
		}
		var errRes struct {
			Error string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errRes); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if errRes.Error != "type harus 'pc' atau 'device'" {
			t.Errorf("unexpected error message: %s", errRes.Error)
		}
	})

	t.Run("fail_empty_body", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		body := `{}`
		resp, err := lab.postJSON("/api/sticker-templates", body)
		if err != nil {
			t.Fatalf("POST /api/sticker-templates empty: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Errorf("expected 400, got %d", resp.StatusCode)
		}
		var errRes struct {
			Error string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errRes); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !strings.Contains(errRes.Error, "Data tidak valid") {
			t.Errorf("unexpected error message: %s", errRes.Error)
		}
	})

	t.Run("create_device_type", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		body := `{"name":"Template Device","sticker_type":"device","font_size_cm":0.8,"padding_h_cm":0.4,"padding_v_cm":0.6}`
		resp, err := lab.postJSON("/api/sticker-templates", body)
		if err != nil {
			t.Fatalf("POST /api/sticker-templates device: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 201 {
			t.Errorf("expected 201, got %d", resp.StatusCode)
		}
		var createRes struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&createRes); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !createRes.Success {
			t.Error("create success=false")
		}
	})

	t.Run("list_filter_device", func(t *testing.T) {
		resp, err := lab.get("/api/sticker-templates?type=device")
		if err != nil {
			t.Fatalf("GET /api/sticker-templates?type=device: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		var list []struct {
			ID          int    `json:"id"`
			Name        string `json:"name"`
			StickerType string `json:"sticker_type"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(list) == 0 {
			t.Fatal("expected at least 1 device template, got 0")
		}
		for _, tmpl := range list {
			if tmpl.StickerType != "device" {
				t.Errorf("expected sticker_type=device, got %s for template %d", tmpl.StickerType, tmpl.ID)
			}
		}
	})
}


