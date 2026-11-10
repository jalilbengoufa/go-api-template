package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

type stubPinger struct {
	err error
}

func (s stubPinger) Ping(context.Context) error { return s.err }

func TestHealthLive(t *testing.T) {
	e := echo.New()
	e.GET("/health/live", NewHealth(stubPinger{err: errors.New("down")}, "test").Live)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health/live", http.NoBody))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestHealthReady(t *testing.T) {
	tests := []struct {
		name       string
		pingErr    error
		wantStatus int
		wantCheck  string
	}{
		{"database reachable", nil, http.StatusOK, "ok"},
		{"database unreachable", errors.New("connection refused"), http.StatusServiceUnavailable, "unavailable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			e.GET("/health/ready", NewHealth(stubPinger{err: tt.pingErr}, "test").Ready)

			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health/ready", http.NoBody))

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tt.wantStatus, rec.Body)
			}

			var body healthResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("unmarshal body: %v", err)
			}
			if body.Checks["database"] != tt.wantCheck {
				t.Errorf("checks[database] = %q, want %q", body.Checks["database"], tt.wantCheck)
			}
			if body.Version != "test" {
				t.Errorf("version = %q, want %q", body.Version, "test")
			}
		})
	}
}
