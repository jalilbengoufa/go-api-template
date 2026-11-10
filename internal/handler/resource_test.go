package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/jalilbengoufa/go-api-template/internal/config"
	"github.com/jalilbengoufa/go-api-template/internal/domain"
	"github.com/jalilbengoufa/go-api-template/internal/httpx"
	"github.com/jalilbengoufa/go-api-template/internal/middleware"
)

type fakeResourceService struct {
	resources []domain.Resource
	total     int64
	err       error

	gotPage    domain.Page
	gotOwnerID string
	gotCreate  domain.CreateResource
	gotUpdate  domain.UpdateResource
	gotID      uuid.UUID
}

func (f *fakeResourceService) Create(_ context.Context, input domain.CreateResource) (*domain.Resource, error) {
	f.gotCreate = input
	if f.err != nil {
		return nil, f.err
	}
	return &domain.Resource{ID: uuid.New(), OwnerID: input.OwnerID, Title: input.Title}, nil
}

func (f *fakeResourceService) GetByID(_ context.Context, id uuid.UUID) (*domain.Resource, error) {
	f.gotID = id
	if f.err != nil {
		return nil, f.err
	}
	return &domain.Resource{ID: id, Title: "existing"}, nil
}

func (f *fakeResourceService) List(_ context.Context, page domain.Page) ([]domain.Resource, int64, error) {
	f.gotPage = page
	return f.resources, f.total, f.err
}

func (f *fakeResourceService) ListByOwner(_ context.Context, ownerID string, page domain.Page) ([]domain.Resource, int64, error) {
	f.gotOwnerID = ownerID
	f.gotPage = page
	return f.resources, f.total, f.err
}

func (f *fakeResourceService) Update(_ context.Context, id uuid.UUID, ownerID string, input domain.UpdateResource) (*domain.Resource, error) {
	f.gotID, f.gotOwnerID, f.gotUpdate = id, ownerID, input
	if f.err != nil {
		return nil, f.err
	}
	title := "existing"
	if input.Title != nil {
		title = *input.Title
	}
	return &domain.Resource{ID: id, OwnerID: ownerID, Title: title}, nil
}

func (f *fakeResourceService) Delete(_ context.Context, id uuid.UUID, ownerID string) error {
	f.gotID, f.gotOwnerID = id, ownerID
	return f.err
}

func newTestEcho(t *testing.T, svc ResourceService) *echo.Echo {
	t.Helper()

	e := echo.New()
	e.Validator = httpx.NewValidator()
	e.HTTPErrorHandler = httpx.ErrorHandler(slog.New(slog.NewTextHandler(io.Discard, nil)))

	auth, err := middleware.JWT(config.Auth{Disabled: true}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("middleware.JWT() returned error: %v", err)
	}

	NewResource(svc).Register(e.Group("/v1/resources", auth))
	return e
}

func do(t *testing.T, e *echo.Echo, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequestWithContext(t.Context(), method, target, reader)
	if body != "" {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) httpx.ErrorResponse {
	t.Helper()
	var out httpx.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal error body %q: %v", rec.Body.String(), err)
	}
	return out
}

func TestCreateResource(t *testing.T) {
	svc := &fakeResourceService{}
	e := newTestEcho(t, svc)

	rec := do(t, e, http.MethodPost, "/v1/resources", `{"title":"hello"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusCreated, rec.Body)
	}
	if svc.gotCreate.Title != "hello" {
		t.Errorf("service received title %q, want %q", svc.gotCreate.Title, "hello")
	}
	if svc.gotCreate.OwnerID != middleware.DevSubject {
		t.Errorf("service received owner %q, want %q", svc.gotCreate.OwnerID, middleware.DevSubject)
	}
	if loc := rec.Header().Get(echo.HeaderLocation); !strings.HasPrefix(loc, "/v1/resources/") {
		t.Errorf("Location = %q, want a /v1/resources/{id} prefix", loc)
	}
}

func TestCreateResourceValidation(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{"missing title", `{}`, http.StatusUnprocessableEntity, "validation_failed"},
		{"blank title", `{"title":""}`, http.StatusUnprocessableEntity, "validation_failed"},
		{"wrong type", `{"title":123}`, http.StatusBadRequest, "bad_request"},
		{"malformed json", `{"title":`, http.StatusBadRequest, "bad_request"},
		{"title too long", fmt.Sprintf(`{"title":%q}`, strings.Repeat("a", 256)), http.StatusUnprocessableEntity, "validation_failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestEcho(t, &fakeResourceService{})

			rec := do(t, e, http.MethodPost, "/v1/resources", tt.body)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if got := decodeError(t, rec).Error.Code; got != tt.wantCode {
				t.Errorf("error.code = %q, want %q", got, tt.wantCode)
			}
		})
	}
}

func TestListResourcesPagination(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantLimit  int
		wantOffset int
	}{
		{"defaults", "", http.StatusOK, domain.DefaultPageLimit, 0},
		{"explicit", "?limit=5&offset=10", http.StatusOK, 5, 10},
		{"limit clamped to max", "?limit=1000", http.StatusOK, domain.MaxPageLimit, 0},
		{"zero limit rejected", "?limit=0", http.StatusBadRequest, 0, 0},
		{"negative offset rejected", "?offset=-1", http.StatusBadRequest, 0, 0},
		{"non numeric limit rejected", "?limit=abc", http.StatusBadRequest, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeResourceService{resources: []domain.Resource{{ID: uuid.New()}}, total: 1}
			e := newTestEcho(t, svc)

			rec := do(t, e, http.MethodGet, "/v1/resources"+tt.query, "")

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantStatus != http.StatusOK {
				return
			}
			if svc.gotPage.Limit != tt.wantLimit || svc.gotPage.Offset != tt.wantOffset {
				t.Errorf("page = %+v, want limit %d offset %d", svc.gotPage, tt.wantLimit, tt.wantOffset)
			}

			var out httpx.ListResponse[domain.Resource]
			if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
				t.Fatalf("unmarshal body: %v", err)
			}
			if out.Meta.Total != 1 || out.Meta.Limit != tt.wantLimit {
				t.Errorf("meta = %+v, want total 1 and limit %d", out.Meta, tt.wantLimit)
			}
		})
	}
}

func TestListResourcesEmptyReturnsArray(t *testing.T) {
	e := newTestEcho(t, &fakeResourceService{})

	rec := do(t, e, http.MethodGet, "/v1/resources", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"data":[]`) {
		t.Errorf("body = %s, want an empty JSON array for data", rec.Body)
	}
}

func TestGetResourceInvalidUUID(t *testing.T) {
	e := newTestEcho(t, &fakeResourceService{})

	rec := do(t, e, http.MethodGet, "/v1/resources/not-a-uuid", "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body)
	}
}

func TestDomainErrorsMapToStatus(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"not found", fmt.Errorf("wrapped: %w", domain.ErrNotFound), http.StatusNotFound, "not_found"},
		{"forbidden", fmt.Errorf("wrapped: %w", domain.ErrForbidden), http.StatusForbidden, "forbidden"},
		{"conflict", fmt.Errorf("wrapped: %w", domain.ErrConflict), http.StatusConflict, "conflict"},
		{"invalid input", fmt.Errorf("wrapped: %w", domain.ErrInvalidInput), http.StatusBadRequest, "bad_request"},
		{"unexpected", fmt.Errorf("boom"), http.StatusInternalServerError, "internal_error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestEcho(t, &fakeResourceService{err: tt.err})

			rec := do(t, e, http.MethodGet, "/v1/resources/"+uuid.New().String(), "")

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tt.wantStatus, rec.Body)
			}
			body := decodeError(t, rec)
			if body.Error.Code != tt.wantCode {
				t.Errorf("error.code = %q, want %q", body.Error.Code, tt.wantCode)
			}
			if tt.wantStatus == http.StatusInternalServerError && strings.Contains(body.Error.Message, "boom") {
				t.Error("internal error message leaked the underlying error to the client")
			}
		})
	}
}

func TestUpdateResource(t *testing.T) {
	svc := &fakeResourceService{}
	e := newTestEcho(t, svc)
	id := uuid.New()

	rec := do(t, e, http.MethodPatch, "/v1/resources/"+id.String(), `{"title":"updated"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body)
	}
	if svc.gotID != id {
		t.Errorf("service received id %s, want %s", svc.gotID, id)
	}
	if svc.gotUpdate.Title == nil || *svc.gotUpdate.Title != "updated" {
		t.Errorf("service received title %v, want \"updated\"", svc.gotUpdate.Title)
	}
	if svc.gotOwnerID != middleware.DevSubject {
		t.Errorf("service received owner %q, want %q", svc.gotOwnerID, middleware.DevSubject)
	}
}

func TestUpdateResourceOmittedTitleIsAllowed(t *testing.T) {
	svc := &fakeResourceService{}
	e := newTestEcho(t, svc)

	rec := do(t, e, http.MethodPatch, "/v1/resources/"+uuid.New().String(), `{}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body)
	}
	if svc.gotUpdate.Title != nil {
		t.Errorf("service received title %v, want nil", svc.gotUpdate.Title)
	}
}

func TestDeleteResource(t *testing.T) {
	svc := &fakeResourceService{}
	e := newTestEcho(t, svc)
	id := uuid.New()

	rec := do(t, e, http.MethodDelete, "/v1/resources/"+id.String(), "")

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (body: %s)", rec.Code, rec.Body)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %s, want empty", rec.Body)
	}
	if svc.gotID != id {
		t.Errorf("service received id %s, want %s", svc.gotID, id)
	}
}

func TestListMineScopesToSubject(t *testing.T) {
	svc := &fakeResourceService{}
	e := newTestEcho(t, svc)

	rec := do(t, e, http.MethodGet, "/v1/resources/me", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body)
	}
	if svc.gotOwnerID != middleware.DevSubject {
		t.Errorf("service received owner %q, want %q", svc.gotOwnerID, middleware.DevSubject)
	}
}
