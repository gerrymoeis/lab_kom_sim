package tests

import (
	"bytes"
	"compress/zlib"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func generatePrintURL(baseURL, prefix string, params map[string]string) string {
	u := baseURL + prefix + "/print/generate"
	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}
	return u + "?" + q.Encode()
}

func getBodyBytes(resp *http.Response) []byte {
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return body
}

func countPDFPages(data []byte) int {
	s := string(data)
	return strings.Count(s, "/Type /Page\n") + strings.Count(s, "/Type /Page\r")
}

func extractPDFContent(pdf []byte) []byte {
	var result bytes.Buffer
	s := string(pdf)

	pos := 0
	for {
		idx := strings.Index(s[pos:], "stream\n")
		if idx < 0 {
			idx = strings.Index(s[pos:], "stream\r\n")
		}
		if idx < 0 {
			break
		}
		idx += pos

		dataStart := idx + len("stream\n")
		if idx+len("stream\r\n") <= len(s) && s[idx:idx+len("stream\r\n")] == "stream\r\n" {
			dataStart = idx + len("stream\r\n")
		}

		endIdx := strings.Index(s[dataStart:], "endstream")
		if endIdx < 0 {
			break
		}
		endIdx += dataStart

		raw := bytes.TrimRight(pdf[dataStart:endIdx], " \t\r\n")
		if len(raw) > 0 {
			r, err := zlib.NewReader(bytes.NewReader(raw))
			if err == nil {
				d, _ := io.ReadAll(r)
				r.Close()
				result.Write(d)
				result.WriteString("\n")
			}
		}

		pos = endIdx + len("endstream")
	}
	return result.Bytes()
}

func labelInPDF(pdf []byte, label string) bool {
	if bytes.Contains(pdf, []byte("("+label+")")) || bytes.Contains(pdf, []byte(label)) {
		return true
	}
	content := extractPDFContent(pdf)
	return bytes.Contains(content, []byte("("+label+")")) || bytes.Contains(content, []byte(label))
}

func mapWith(base map[string]string, k, v string) map[string]string {
	m := make(map[string]string, len(base)+1)
	for kk, vv := range base {
		m[kk] = vv
	}
	m[k] = v
	return m
}

// ============================================
// Fase 2C: Print Stiker
// ============================================

func TestPrintSticker(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	db := env.DB_A

	if !loginAndRefresh(lab, "labA_only", "test123") {
		t.Fatal("login failed")
	}

	// ============================================
	// 2C.1: PC label PDF — konten mengandung label yang diminta
	// ============================================
	t.Run("2C.1_pc_label_pdf", func(t *testing.T) {
		url := generatePrintURL(env.TS.URL, lab.prefix, map[string]string{
			"type":       "pc",
			"paper_size": "A4",
		})

		req, _ := http.NewRequest("GET", url, nil)
		lab.addCookies(req)
		resp, err := lab.client.Do(req)
		if err != nil {
			t.Fatalf("GET /print/generate: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		ct := resp.Header.Get("Content-Type")
		if ct != "application/pdf" {
			t.Fatalf("expected application/pdf, got %s", ct)
		}

		pdfBytes := getBodyBytes(resp)
		if !bytes.HasPrefix(pdfBytes, []byte("%PDF-")) {
			t.Fatal("expected PDF header")
		}
		if !labelInPDF(pdfBytes, "PC-1") {
			t.Errorf("expected PC-1 label in PDF")
		}
		if !labelInPDF(pdfBytes, "PC-8") {
			t.Errorf("expected PC-8 label in PDF")
		}
	})

	// ============================================
	// 2C.2: Device label PDF — perlu seed device type & devices first
	// ============================================
	t.Run("2C.2_device_label_pdf", func(t *testing.T) {
		_, err := db.Exec("INSERT OR IGNORE INTO categories (name, label_prefix) VALUES (?, ?)", "Test Category", "TESTCT")
		if err != nil {
			t.Fatalf("insert category: %v", err)
		}
		var catID int
		db.QueryRow("SELECT id FROM categories WHERE label_prefix='TESTCT'").Scan(&catID)
		if catID == 0 {
			t.Fatal("category ID not found")
		}

		_, err = db.Exec(`INSERT OR IGNORE INTO device_types (category_id, name, brand, model, label_prefix, usage_type) VALUES (?, ?, ?, ?, ?, ?)`,
			catID, "Test Device", "TestBrand", "ModelX", "TESTDV", "loanable")
		if err != nil {
			t.Fatalf("insert device type: %v", err)
		}
		var dtID int
		db.QueryRow("SELECT id FROM device_types WHERE label_prefix='TESTDV'").Scan(&dtID)

		for _, label := range []string{"TESTDV-001", "TESTDV-002", "TESTDV-003"} {
			_, err := db.Exec(`INSERT OR IGNORE INTO devices (device_type_id, label, condition) VALUES (?, ?, ?)`,
				dtID, label, "normal")
			if err != nil {
				t.Fatalf("insert device %s: %v", label, err)
			}
		}

		url := generatePrintURL(env.TS.URL, lab.prefix, map[string]string{
			"type":         "device",
			"device_type":  "TESTDV",
			"paper_size":   "A4",
			"font_size":    "0.5",
			"padding_h":    "0.3",
			"padding_v":    "0.3",
		})

		req, _ := http.NewRequest("GET", url, nil)
		lab.addCookies(req)
		resp, err := lab.client.Do(req)
		if err != nil {
			t.Fatalf("GET /print/generate: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		ct := resp.Header.Get("Content-Type")
		if ct != "application/pdf" {
			t.Fatalf("expected application/pdf, got %s", ct)
		}

		pdfBytes := getBodyBytes(resp)
		if !bytes.HasPrefix(pdfBytes, []byte("%PDF-")) {
			t.Fatal("expected PDF header")
		}
		if !labelInPDF(pdfBytes, "TESTDV-001") {
			t.Errorf("expected TESTDV-001 label in PDF")
		}
		if !labelInPDF(pdfBytes, "TESTDV-003") {
			t.Errorf("expected TESTDV-003 label in PDF")
		}
	})

	// ============================================
	// 2C.3: Paper sizes A4, F4, A3 — semua menghasilkan PDF valid
	// ============================================
	t.Run("2C.3_paper_sizes", func(t *testing.T) {
		sizes := []string{"A4", "F4", "A3"}
		var prevSize int
		for i, size := range sizes {
			url := generatePrintURL(env.TS.URL, lab.prefix, map[string]string{
				"type":       "pc",
				"paper_size": size,
			})

			req, _ := http.NewRequest("GET", url, nil)
			lab.addCookies(req)
			resp, err := lab.client.Do(req)
			if err != nil {
				t.Fatalf("GET /print/generate paper=%s: %v", size, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 {
				t.Errorf("paper %s: expected 200, got %d", size, resp.StatusCode)
				continue
			}
			ct := resp.Header.Get("Content-Type")
			if ct != "application/pdf" {
				t.Errorf("paper %s: expected application/pdf, got %s", size, ct)
				continue
			}

			pdfBytes := getBodyBytes(resp)
			if !bytes.HasPrefix(pdfBytes, []byte("%PDF-")) {
				t.Errorf("paper %s: expected PDF header", size)
				continue
			}

			if !labelInPDF(pdfBytes, "PC-1") {
				t.Errorf("paper %s: expected PC-1 label", size)
			}

			if i > 0 && len(pdfBytes) == prevSize {
				t.Errorf("paper %s: PDF size same as previous, expected different", size)
			}
			prevSize = len(pdfBytes)
		}
	})

	// ============================================
	// 2C.4: Multiple sheets (num_sheets=3) — output 3x lipat
	// ============================================
	t.Run("2C.4_multiple_sheets", func(t *testing.T) {
		params := map[string]string{
			"type":       "pc",
			"paper_size": "A4",
		}

		url1 := generatePrintURL(env.TS.URL, lab.prefix, mapWith(params, "num_sheets", "1"))
		req1, _ := http.NewRequest("GET", url1, nil)
		lab.addCookies(req1)
		resp1, err := lab.client.Do(req1)
		if err != nil {
			t.Fatalf("num_sheets=1: %v", err)
		}
		defer resp1.Body.Close()
		if resp1.StatusCode != 200 {
			t.Fatalf("num_sheets=1: expected 200, got %d", resp1.StatusCode)
		}
		pdf1 := getBodyBytes(resp1)
		pages1 := countPDFPages(pdf1)

		url3 := generatePrintURL(env.TS.URL, lab.prefix, mapWith(params, "num_sheets", "3"))
		req3, _ := http.NewRequest("GET", url3, nil)
		lab.addCookies(req3)
		resp3, err := lab.client.Do(req3)
		if err != nil {
			t.Fatalf("num_sheets=3: %v", err)
		}
		defer resp3.Body.Close()
		if resp3.StatusCode != 200 {
			t.Fatalf("num_sheets=3: expected 200, got %d", resp3.StatusCode)
		}
		pdf3 := getBodyBytes(resp3)
		pages3 := countPDFPages(pdf3)

		if pages1 == 0 {
			t.Fatal("expected at least 1 page for num_sheets=1")
		}
		if pages3 != pages1*3 {
			t.Errorf("expected %d pages for num_sheets=3 (3×%d), got %d", pages1*3, pages1, pages3)
		}
		if len(pdf3) <= len(pdf1) {
			t.Errorf("expected num_sheets=3 PDF to be larger than num_sheets=1")
		}
	})

	// ============================================
	// 2C.5: Empty data — error "tidak ada data untuk di-print"
	// ============================================
	t.Run("2C.5_empty_data", func(t *testing.T) {
		url := generatePrintURL(env.TS.URL, lab.prefix, map[string]string{
			"type":       "pc",
			"paper_size": "A4",
			"pc_labels":  "NONEXISTENT_LABEL_XYZ",
		})

		req, _ := http.NewRequest("GET", url, nil)
		lab.addCookies(req)
		resp, err := lab.client.Do(req)
		if err != nil {
			t.Fatalf("GET /print/generate empty: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 500 {
			t.Errorf("expected 500, got %d", resp.StatusCode)
		}
		body := getBodyBytes(resp)
		if !strings.Contains(string(body), "tidak ada data") {
			t.Errorf("expected 'tidak ada data' in error response, got: %s", string(body))
		}
	})

	// ============================================
	// 2C.6: Sticker oversized — error "stiker terlalu besar"
	// ============================================
	t.Run("2C.6_sticker_oversized", func(t *testing.T) {
		url := generatePrintURL(env.TS.URL, lab.prefix, map[string]string{
			"type":       "pc",
			"paper_size": "A4",
			"font_size":  "5.0",
			"padding_h":  "5.0",
			"padding_v":  "5.0",
		})

		req, _ := http.NewRequest("GET", url, nil)
		lab.addCookies(req)
		resp, err := lab.client.Do(req)
		if err != nil {
			t.Fatalf("GET /print/generate oversized: %v", err)
		}
		defer resp.Body.Close()

		body := getBodyBytes(resp)
		if resp.StatusCode != 500 {
			t.Errorf("expected 500 for oversized, got %d", resp.StatusCode)
		}
		if !strings.Contains(string(body), "terlalu besar") {
			t.Errorf("expected 'terlalu besar' in error response, got: %s", string(body))
		}
	})

	// ============================================
	// 2C.7: Invalid params — font 0.2cm (below min), padding 6cm (above max)
	// Note: errHTML returns 500
	// ============================================
	t.Run("2C.7_invalid_params", func(t *testing.T) {
		tests := []struct {
			name string
			params map[string]string
			wantMsg string
		}{
			{
				name: "font_below_min",
				params: map[string]string{
					"type": "pc", "paper_size": "A4",
					"font_size": "0.2",
				},
				wantMsg: "Font size harus antara 0.3 - 5.0 cm",
			},
			{
				name: "font_above_max",
				params: map[string]string{
					"type": "pc", "paper_size": "A4",
					"font_size": "6.0",
				},
				wantMsg: "Font size harus antara 0.3 - 5.0 cm",
			},
			{
				name: "padding_h_below_min",
				params: map[string]string{
					"type": "pc", "paper_size": "A4",
					"padding_h": "0.05",
				},
				wantMsg: "Padding horizontal harus antara 0.1 - 5.0 cm",
			},
			{
				name: "padding_h_above_max",
				params: map[string]string{
					"type": "pc", "paper_size": "A4",
					"padding_h": "6.0",
				},
				wantMsg: "Padding horizontal harus antara 0.1 - 5.0 cm",
			},
			{
				name: "padding_v_below_min",
				params: map[string]string{
					"type": "pc", "paper_size": "A4",
					"padding_v": "0.05",
				},
				wantMsg: "Padding vertical harus antara 0.1 - 5.0 cm",
			},
			{
				name: "padding_v_above_max",
				params: map[string]string{
					"type": "pc", "paper_size": "A4",
					"padding_v": "6.0",
				},
				wantMsg: "Padding vertical harus antara 0.1 - 5.0 cm",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				url := generatePrintURL(env.TS.URL, lab.prefix, tt.params)
				req, _ := http.NewRequest("GET", url, nil)
				lab.addCookies(req)
				resp, err := lab.client.Do(req)
				if err != nil {
					t.Fatalf("GET /print/generate: %v", err)
				}
				defer resp.Body.Close()

				if resp.StatusCode != 500 {
					t.Errorf("expected 500, got %d", resp.StatusCode)
				}
				body, _ := io.ReadAll(resp.Body)
				if !strings.Contains(string(body), tt.wantMsg) {
					t.Errorf("expected message containing %q, got: %s", tt.wantMsg, string(body))
				}
			})
		}
	})

	// ============================================
	// 2C.8: Natural sorting — pc_labels in non-natural order
	// ============================================
	t.Run("2C.8_natural_sorting", func(t *testing.T) {
		// Insert test PCs at row=99 with labels that sort naturally as 9,10,11
		testLabels := []string{"pc-sort-9", "pc-sort-10", "pc-sort-11"}
		for i, label := range testLabels {
			_, err := db.Exec(`INSERT INTO pcs (label, row, column, placement, status, serial_number, operating_system, pc_type, brand_model) VALUES (?, 99, ?, 'dipakai', 'normal', ?, 'Win11', 'PC', 'Dell')`,
				label, i+1, "SN-"+label)
			if err != nil {
				t.Fatalf("insert pc %s: %v", label, err)
			}
		}
		t.Cleanup(func() {
			db.Exec("DELETE FROM pc_software WHERE pc_id IN (SELECT id FROM pcs WHERE row >= 99)")
			db.Exec("DELETE FROM pcs WHERE row >= 99")
		})

		// Request labels in non-natural order: pc-sort-10, pc-sort-9, pc-sort-11
		url := generatePrintURL(env.TS.URL, lab.prefix, map[string]string{
			"type":      "pc",
			"paper_size": "A4",
			"pc_labels": "pc-sort-10,pc-sort-9,pc-sort-11",
		})

		req, _ := http.NewRequest("GET", url, nil)
		lab.addCookies(req)
		resp, err := lab.client.Do(req)
		if err != nil {
			t.Fatalf("GET /print/generate: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		pdfBytes := getBodyBytes(resp)
		if !bytes.HasPrefix(pdfBytes, []byte("%PDF-")) {
			t.Fatal("expected PDF header")
		}

		// Extract decompressed content for order check
		content := extractPDFContent(pdfBytes)
		contentStr := string(content)

		idx9 := strings.Index(contentStr, "(PC-SORT-9)")
		idx10 := strings.Index(contentStr, "(PC-SORT-10)")
		idx11 := strings.Index(contentStr, "(PC-SORT-11)")

		if idx9 < 0 {
			t.Error("PC-SORT-9 not found in PDF content")
		}
		if idx10 < 0 {
			t.Error("PC-SORT-10 not found in PDF content")
		}
		if idx11 < 0 {
			t.Error("PC-SORT-11 not found in PDF content")
		}

		if idx9 >= 0 && idx10 >= 0 && idx9 > idx10 {
			t.Error("PC-SORT-9 should appear before PC-SORT-10 (natural sorting)")
		}
		if idx10 >= 0 && idx11 >= 0 && idx10 > idx11 {
			t.Error("PC-SORT-10 should appear before PC-SORT-11 (natural sorting)")
		}
	})
}
