package tests

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"

	"inventaris-lab-kom/internal/middleware"
)

func TestCacheControl(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name   string
		path   string
		expect map[string]string
	}{
		{
			name: "api_path",
			path: "/api/v1/pcs",
			expect: map[string]string{
				"Cache-Control": "no-cache, no-store, must-revalidate",
				"Pragma":        "no-cache",
				"Expires":       "0",
			},
		},
		{
			name: "uploads_path",
			path: "/uploads/photo.jpg",
			expect: map[string]string{
				"Cache-Control": "public, max-age=86400",
			},
		},
		{
			name: "uploads_temp",
			path: "/uploads/lab1/temp/abc123",
			expect: map[string]string{
				"Cache-Control": "no-cache, no-store",
			},
		},
		{
			name: "static_versioned_css",
			path: "/static/style.a1b2c3d4.css",
			expect: map[string]string{
				"Cache-Control": "public, max-age=31536000, immutable",
			},
		},
		{
			name: "static_versioned_js",
			path: "/static/app.e5f6a7b8.js",
			expect: map[string]string{
				"Cache-Control": "public, max-age=31536000, immutable",
			},
		},
		{
			name: "static_unversioned",
			path: "/static/style.css",
			expect: map[string]string{
				"Cache-Control": "public, max-age=86400",
			},
		},
		{
			name: "default_path",
			path: "/some/page",
			expect: map[string]string{
				"Cache-Control": "no-cache, no-store, must-revalidate",
				"Pragma":        "no-cache",
				"Expires":       "0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, tt.path, nil)

			middleware.CacheControl()(c)

			for key, expected := range tt.expect {
				if got := w.Header().Get(key); got != expected {
					t.Errorf("header %q = %q, want %q", key, got, expected)
				}
			}
		})
	}
}

func TestSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	commonHeaders := map[string]string{
		"X-Frame-Options":           "DENY",
		"X-Content-Type-Options":    "nosniff",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
		"Permissions-Policy":        "geolocation=(), camera=(), microphone=()",
		"Content-Security-Policy":   "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; worker-src 'self' blob:; img-src 'self' data: blob:; frame-ancestors 'none'",
	}

	tests := []struct {
		name        string
		environment string
		expect      map[string]string
	}{
		{
			name:        "development",
			environment: "development",
			expect:      commonHeaders,
		},
		{
			name:        "production",
			environment: "production",
			expect: func() map[string]string {
				m := make(map[string]string, len(commonHeaders)+1)
				for k, v := range commonHeaders {
					m[k] = v
				}
				m["Strict-Transport-Security"] = "max-age=31536000; includeSubDomains"
				return m
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

			middleware.SecurityHeaders(tt.environment)(c)

			for key, expected := range tt.expect {
				if got := w.Header().Get(key); got != expected {
					t.Errorf("header %q = %q, want %q", key, got, expected)
				}
			}
		})
	}
}

func TestFlashReader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := cookie.NewStore([]byte("test-secret-flash"))
	router := gin.New()
	router.Use(sessions.Sessions("test-flash", store))
	router.Use(middleware.FlashReader())

	router.GET("/set-flash", func(c *gin.Context) {
		s := sessions.Default(c)
		s.AddFlash("error message", "error")
		s.AddFlash("success message", "success")
		s.Save()
		c.String(http.StatusOK, "OK")
	})

	router.GET("/read-flash", func(c *gin.Context) {
		e, _ := c.Get("_flash_error")
		s, _ := c.Get("_flash_success")
		c.String(http.StatusOK, "err=%v|succ=%v", e, s)
	})

	router.GET("/no-flash", func(c *gin.Context) {
		_, eok := c.Get("_flash_error")
		_, sok := c.Get("_flash_success")
		c.String(http.StatusOK, "eok=%v|sok=%v", eok, sok)
	})

	t.Run("reads_flash_messages", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/set-flash", nil)
		router.ServeHTTP(w, req)
		resp := w.Result()
		cookieHeader := resp.Header.Get("Set-Cookie")
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest(http.MethodGet, "/read-flash", nil)
		req2.Header.Set("Cookie", extractCookieValue(cookieHeader))
		router.ServeHTTP(w2, req2)

		body := w2.Body.String()
		resp2 := w2.Result()
		resp2.Body.Close()

		if !strings.Contains(body, "err=error message") {
			t.Errorf("expected error flash, got: %s", body)
		}
		if !strings.Contains(body, "succ=success message") {
			t.Errorf("expected success flash, got: %s", body)
		}
	})

	t.Run("no_flash_messages", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/no-flash", nil)
		router.ServeHTTP(w, req)
		body := w.Body.String()
		resp := w.Result()
		resp.Body.Close()

		if !strings.Contains(body, "eok=false|sok=false") {
			t.Errorf("expected no flash, got: %s", body)
		}
	})
}

func extractCookieValue(header string) string {
	if idx := strings.Index(header, ";"); idx >= 0 {
		return header[:idx]
	}
	return header
}
