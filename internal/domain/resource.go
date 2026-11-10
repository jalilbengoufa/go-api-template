package domain

import (
	"time"

	"github.com/google/uuid"
)

type Resource struct {
	ID        uuid.UUID  `gorm:"column:id;type:uuid;primaryKey" json:"id"`
	OwnerID   string     `gorm:"column:owner_id;size:255;index;not null" json:"owner_id"`
	Title     string     `gorm:"column:title;size:255;not null" json:"title"`
	CreatedAt time.Time  `gorm:"column:created_at;not null;autoCreateTime" json:"created_at"`
	UpdatedAt time.Time  `gorm:"column:updated_at;not null;autoUpdateTime" json:"updated_at"`
	DeletedAt *time.Time `gorm:"column:deleted_at;index" json:"-"`
}

func (Resource) TableName() string { return "resources" }

type CreateResource struct {
	OwnerID string
	Title   string
}

type UpdateResource struct {
	Title *string
}

type Page struct {
	Limit  int
	Offset int
}

const (
	DefaultPageLimit = 20
	MaxPageLimit     = 100
)

func (p Page) Normalized() Page {
	if p.Limit <= 0 {
		p.Limit = DefaultPageLimit
	}
	if p.Limit > MaxPageLimit {
		p.Limit = MaxPageLimit
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	return p
}
