package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMetrics_PassesThrough(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := Metrics("/test", next)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if !called {
		t.Error("expected next handler to be called")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestMetrics_CapturesNonOKStatus(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	handler := Metrics("/test", next)
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestMetrics_DefaultStatusIsOK(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	handler := Metrics("/test", next)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestWriteHeader_CapturesStatus(t *testing.T) {
	base := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: base, status: http.StatusOK}

	rw.WriteHeader(http.StatusCreated)

	if rw.status != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rw.status)
	}
	if base.Code != http.StatusCreated {
		t.Errorf("expected underlying status 201, got %d", base.Code)
	}
}
