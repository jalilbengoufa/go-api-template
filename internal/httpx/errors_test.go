package httpx

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestErrorHandlerMapsContextErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"deadline exceeded", fmt.Errorf("query: %w", context.DeadlineExceeded), http.StatusGatewayTimeout, "request_timeout"},
		{"client canceled", fmt.Errorf("query: %w", context.Canceled), StatusClientClosedRequest, "client_closed_request"},
		{"echo http error", echo.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "method_not_allowed"},
		{"api error", NotFound("gone"), http.StatusNotFound, "not_found"},
		{"unknown", fmt.Errorf("boom"), http.StatusInternalServerError, "internal_error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			e.HTTPErrorHandler = ErrorHandler(slog.New(slog.NewTextHandler(io.Discard, nil)))
			e.GET("/x", func(echo.Context) error { return tt.err })

			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", http.NoBody))

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tt.wantStatus, rec.Body)
			}

			var body ErrorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("unmarshal body %q: %v", rec.Body.String(), err)
			}
			if body.Error.Code != tt.wantCode {
				t.Errorf("error.code = %q, want %q", body.Error.Code, tt.wantCode)
			}
		})
	}
}
