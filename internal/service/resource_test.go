package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/jalilbengoufa/go-api-template/internal/domain"
)

type stubRepo struct {
	resource *domain.Resource
	getErr   error

	created    *domain.Resource
	updatedID  uuid.UUID
	updated    domain.UpdateResource
	deletedID  uuid.UUID
	listOwner  string
	listPage   domain.Page
	deleteCall int
	updateCall int
}

func (s *stubRepo) Create(_ context.Context, resource *domain.Resource) error {
	s.created = resource
	return nil
}

func (s *stubRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Resource, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.resource != nil {
		return s.resource, nil
	}
	return &domain.Resource{ID: id}, nil
}

func (s *stubRepo) List(_ context.Context, ownerID string, page domain.Page) ([]domain.Resource, int64, error) {
	s.listOwner, s.listPage = ownerID, page
	return nil, 0, nil
}

func (s *stubRepo) Update(_ context.Context, id uuid.UUID, input domain.UpdateResource) (*domain.Resource, error) {
	s.updateCall++
	s.updatedID, s.updated = id, input
	return &domain.Resource{ID: id}, nil
}

func (s *stubRepo) Delete(_ context.Context, id uuid.UUID) error {
	s.deleteCall++
	s.deletedID = id
	return nil
}

func TestCreateTrimsTitle(t *testing.T) {
	repo := &stubRepo{}
	svc := NewResourceService(repo)

	got, err := svc.Create(context.Background(), domain.CreateResource{OwnerID: "owner-1", Title: "  spaced  "})
	if err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}
	if got.Title != "spaced" {
		t.Errorf("Title = %q, want %q", got.Title, "spaced")
	}
	if got.ID == uuid.Nil {
		t.Error("ID was not generated")
	}
	if repo.created == nil || repo.created.OwnerID != "owner-1" {
		t.Errorf("repository received %+v, want owner-1", repo.created)
	}
}

func TestCreateRejectsBlankTitle(t *testing.T) {
	svc := NewResourceService(&stubRepo{})

	_, err := svc.Create(context.Background(), domain.CreateResource{OwnerID: "owner-1", Title: "   "})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("Create() error = %v, want domain.ErrInvalidInput", err)
	}
}

func TestUpdateRejectsOtherOwner(t *testing.T) {
	repo := &stubRepo{resource: &domain.Resource{ID: uuid.New(), OwnerID: "owner-1"}}
	svc := NewResourceService(repo)

	title := "new title"
	_, err := svc.Update(context.Background(), repo.resource.ID, "owner-2", domain.UpdateResource{Title: &title})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("Update() error = %v, want domain.ErrForbidden", err)
	}
	if repo.updateCall != 0 {
		t.Error("repository Update was called despite the ownership check failing")
	}
}

func TestUpdatePropagatesNotFound(t *testing.T) {
	repo := &stubRepo{getErr: fmt.Errorf("lookup: %w", domain.ErrNotFound)}
	svc := NewResourceService(repo)

	_, err := svc.Update(context.Background(), uuid.New(), "owner-1", domain.UpdateResource{})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Update() error = %v, want domain.ErrNotFound", err)
	}
}

func TestUpdateTrimsAndRejectsBlankTitle(t *testing.T) {
	id := uuid.New()
	repo := &stubRepo{resource: &domain.Resource{ID: id, OwnerID: "owner-1"}}
	svc := NewResourceService(repo)

	blank := "   "
	if _, err := svc.Update(context.Background(), id, "owner-1", domain.UpdateResource{Title: &blank}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("Update() error = %v, want domain.ErrInvalidInput", err)
	}

	padded := "  trimmed  "
	if _, err := svc.Update(context.Background(), id, "owner-1", domain.UpdateResource{Title: &padded}); err != nil {
		t.Fatalf("Update() returned error: %v", err)
	}
	if repo.updated.Title == nil || *repo.updated.Title != "trimmed" {
		t.Errorf("repository received title %v, want \"trimmed\"", repo.updated.Title)
	}
}

func TestDeleteRejectsOtherOwner(t *testing.T) {
	id := uuid.New()
	repo := &stubRepo{resource: &domain.Resource{ID: id, OwnerID: "owner-1"}}
	svc := NewResourceService(repo)

	if err := svc.Delete(context.Background(), id, "owner-2"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("Delete() error = %v, want domain.ErrForbidden", err)
	}
	if repo.deleteCall != 0 {
		t.Error("repository Delete was called despite the ownership check failing")
	}
}

func TestListByOwnerRequiresOwner(t *testing.T) {
	svc := NewResourceService(&stubRepo{})

	_, _, err := svc.ListByOwner(context.Background(), "", domain.Page{})
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("ListByOwner() error = %v, want domain.ErrUnauthorized", err)
	}
}

func TestListNormalizesPage(t *testing.T) {
	repo := &stubRepo{}
	svc := NewResourceService(repo)

	if _, _, err := svc.List(context.Background(), domain.Page{Limit: 5000, Offset: -3}); err != nil {
		t.Fatalf("List() returned error: %v", err)
	}
	if repo.listPage.Limit != domain.MaxPageLimit || repo.listPage.Offset != 0 {
		t.Errorf("repository received page %+v, want limit %d offset 0", repo.listPage, domain.MaxPageLimit)
	}
	if repo.listOwner != "" {
		t.Errorf("repository received owner %q, want empty", repo.listOwner)
	}
}
