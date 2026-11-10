package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/jalilbengoufa/go-api-template/internal/domain"
)

type ResourceRepository interface {
	Create(ctx context.Context, resource *domain.Resource) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Resource, error)
	List(ctx context.Context, ownerID string, page domain.Page) ([]domain.Resource, int64, error)
	Update(ctx context.Context, id uuid.UUID, input domain.UpdateResource) (*domain.Resource, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type ResourceService struct {
	repo ResourceRepository
}

func NewResourceService(repo ResourceRepository) *ResourceService {
	return &ResourceService{repo: repo}
}

func (s *ResourceService) Create(ctx context.Context, input domain.CreateResource) (*domain.Resource, error) {
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return nil, fmt.Errorf("title must not be blank: %w", domain.ErrInvalidInput)
	}

	resource := &domain.Resource{
		ID:      uuid.New(),
		OwnerID: input.OwnerID,
		Title:   title,
	}
	if err := s.repo.Create(ctx, resource); err != nil {
		return nil, err
	}
	return resource, nil
}

func (s *ResourceService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Resource, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *ResourceService) List(ctx context.Context, page domain.Page) ([]domain.Resource, int64, error) {
	return s.repo.List(ctx, "", page.Normalized())
}

func (s *ResourceService) ListByOwner(ctx context.Context, ownerID string, page domain.Page) ([]domain.Resource, int64, error) {
	if ownerID == "" {
		return nil, 0, fmt.Errorf("owner is unknown: %w", domain.ErrUnauthorized)
	}
	return s.repo.List(ctx, ownerID, page.Normalized())
}

func (s *ResourceService) Update(ctx context.Context, id uuid.UUID, ownerID string, input domain.UpdateResource) (*domain.Resource, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.OwnerID != ownerID {
		return nil, fmt.Errorf("resource %s belongs to another owner: %w", id, domain.ErrForbidden)
	}

	if input.Title != nil {
		title := strings.TrimSpace(*input.Title)
		if title == "" {
			return nil, fmt.Errorf("title must not be blank: %w", domain.ErrInvalidInput)
		}
		input.Title = &title
	}

	return s.repo.Update(ctx, id, input)
}

func (s *ResourceService) Delete(ctx context.Context, id uuid.UUID, ownerID string) error {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if existing.OwnerID != ownerID {
		return fmt.Errorf("resource %s belongs to another owner: %w", id, domain.ErrForbidden)
	}
	return s.repo.Delete(ctx, id)
}
