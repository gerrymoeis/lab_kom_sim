package tests

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func mockGemini(t *testing.T, respBody string, statusCode int, counter *atomic.Int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if counter != nil {
			counter.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		w.Write([]byte(respBody))
	}))
}

func mockOpenRouter(t *testing.T, respBody string, statusCode int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		w.Write([]byte(respBody))
	}))
}
