package middleware

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	jwtmiddleware "github.com/auth0/go-jwt-middleware/v2"
	"github.com/auth0/go-jwt-middleware/v2/validator"
	"github.com/labstack/echo/v4"

	"github.com/jalilbengoufa/go-api-template/internal/config"
	"github.com/jalilbengoufa/go-api-template/internal/httpx"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestJWTRejectsRequestsWithEnvelope(t *testing.T) {
	auth, err := JWT(config.Auth{
		Domain:   "tenant.example.auth0.com",
		Audience: "https://api.example",
	}, discardLogger())
	if err != nil {
		t.Fatalf("JWT() returned error: %v", err)
	}

	e := echo.New()
	e.GET("/guarded", func(c echo.Context) error { return c.NoContent(http.StatusOK) }, auth)

	tests := []struct {
		name   string
		header string
	}{
		{"missing header", ""},
		{"malformed header", "Token abc"},
		{"malformed token", "Bearer not.a.jwt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/guarded", http.NoBody)
			if tt.header != "" {
				req.Header.Set(echo.HeaderAuthorization, tt.header)
			}
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401 (body: %s)", rec.Code, rec.Body)
			}
			if got := rec.Header().Get(echo.HeaderWWWAuthenticate); got == "" {
				t.Error("WWW-Authenticate header is missing")
			}

			var body httpx.ErrorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("unmarshal body %q: %v", rec.Body.String(), err)
			}
			if body.Error.Code != "unauthorized" {
				t.Errorf("error.code = %q, want %q", body.Error.Code, "unauthorized")
			}
		})
	}
}

func TestJWTDisabledInjectsDevSubject(t *testing.T) {
	auth, err := JWT(config.Auth{Disabled: true}, discardLogger())
	if err != nil {
		t.Fatalf("JWT() returned error: %v", err)
	}

	e := echo.New()
	e.GET("/guarded", func(c echo.Context) error {
		subject, err := Subject(c)
		if err != nil {
			return err
		}
		return c.String(http.StatusOK, subject)
	}, auth)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/guarded", http.NoBody))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body)
	}
	if rec.Body.String() != DevSubject {
		t.Errorf("subject = %q, want %q", rec.Body.String(), DevSubject)
	}
}

func TestSubjectWithoutClaims(t *testing.T) {
	e := echo.New()
	c := e.NewContext(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody), httptest.NewRecorder())

	if _, err := Subject(c); err == nil {
		t.Fatal("Subject() = nil error, want unauthorized error")
	}
}

func TestRequirePermission(t *testing.T) {
	tests := []struct {
		name       string
		granted    []string
		required   string
		wantStatus int
	}{
		{"granted", []string{"read:resources"}, "read:resources", http.StatusOK},
		{"missing", []string{"write:resources"}, "read:resources", http.StatusForbidden},
		{"none", nil, "read:resources", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			e.HTTPErrorHandler = httpx.ErrorHandler(discardLogger())
			e.GET("/guarded",
				func(c echo.Context) error { return c.NoContent(http.StatusOK) },
				injectClaims(tt.granted),
				RequirePermission(tt.required),
			)

			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/guarded", http.NoBody))

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tt.wantStatus, rec.Body)
			}
		})
	}
}

func injectClaims(permissions []string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			claims := &validator.ValidatedClaims{
				RegisteredClaims: validator.RegisteredClaims{Subject: "owner-1"},
				CustomClaims:     &Claims{Permissions: permissions},
			}
			ctx := context.WithValue(req.Context(), jwtmiddleware.ContextKey{}, claims)
			c.SetRequest(req.WithContext(ctx))
			return next(c)
		}
	}
}
