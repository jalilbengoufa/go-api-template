package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/jalilbengoufa/go-api-template/internal/domain"
)

type ResourceRepository struct {
	db *gorm.DB
}

func NewResourceRepository(db *gorm.DB) *ResourceRepository {
	return &ResourceRepository{db: db}
}

func (r *ResourceRepository) Create(ctx context.Context, resource *domain.Resource) error {
	if err := r.db.WithContext(ctx).Create(resource).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return fmt.Errorf("create resource: %w", domain.ErrConflict)
		}
		return fmt.Errorf("create resource: %w", err)
	}
	return nil
}

func (r *ResourceRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Resource, error) {
	var resource domain.Resource
	err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&resource).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("get resource %s: %w", id, domain.ErrNotFound)
		}
		return nil, fmt.Errorf("get resource %s: %w", id, err)
	}
	return &resource, nil
}

func (r *ResourceRepository) List(ctx context.Context, ownerID string, page domain.Page) ([]domain.Resource, int64, error) {
	query := r.db.WithContext(ctx).Model(&domain.Resource{}).Where("deleted_at IS NULL")
	if ownerID != "" {
		query = query.Where("owner_id = ?", ownerID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count resources: %w", err)
	}

	resources := make([]domain.Resource, 0, page.Limit)
	err := query.
		Order("created_at DESC, id DESC").
		Limit(page.Limit).
		Offset(page.Offset).
		Find(&resources).Error
	if err != nil {
		return nil, 0, fmt.Errorf("list resources: %w", err)
	}

	return resources, total, nil
}

func (r *ResourceRepository) Update(ctx context.Context, id uuid.UUID, input domain.UpdateResource) (*domain.Resource, error) {
	fields := map[string]any{}
	if input.Title != nil {
		fields["title"] = *input.Title
	}
	if len(fields) == 0 {
		return r.GetByID(ctx, id)
	}

	result := r.db.WithContext(ctx).
		Model(&domain.Resource{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(fields)
	if result.Error != nil {
		return nil, fmt.Errorf("update resource %s: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("update resource %s: %w", id, domain.ErrNotFound)
	}

	return r.GetByID(ctx, id)
}

func (r *ResourceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Model(&domain.Resource{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", gorm.Expr("NOW()"))
	if result.Error != nil {
		return fmt.Errorf("delete resource %s: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("delete resource %s: %w", id, domain.ErrNotFound)
	}
	return nil
}
