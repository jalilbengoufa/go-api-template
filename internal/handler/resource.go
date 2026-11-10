package handler

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/jalilbengoufa/go-api-template/internal/domain"
	"github.com/jalilbengoufa/go-api-template/internal/httpx"
	"github.com/jalilbengoufa/go-api-template/internal/middleware"
)

type ResourceService interface {
	Create(ctx context.Context, input domain.CreateResource) (*domain.Resource, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Resource, error)
	List(ctx context.Context, page domain.Page) ([]domain.Resource, int64, error)
	ListByOwner(ctx context.Context, ownerID string, page domain.Page) ([]domain.Resource, int64, error)
	Update(ctx context.Context, id uuid.UUID, ownerID string, input domain.UpdateResource) (*domain.Resource, error)
	Delete(ctx context.Context, id uuid.UUID, ownerID string) error
}

type Resource struct {
	service ResourceService
}

func NewResource(service ResourceService) *Resource {
	return &Resource{service: service}
}

type createResourceRequest struct {
	Title string `json:"title" validate:"required,min=1,max=255"`
}

type updateResourceRequest struct {
	Title *string `json:"title" validate:"omitempty,min=1,max=255"`
}

func (h *Resource) Register(g *echo.Group) {
	g.GET("", h.List)
	g.POST("", h.Create)
	g.GET("/me", h.ListMine)
	g.GET("/:id", h.Get)
	g.PATCH("/:id", h.Update)
	g.DELETE("/:id", h.Delete)
}

func (h *Resource) Create(c echo.Context) error {
	var req createResourceRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}

	subject, err := middleware.Subject(c)
	if err != nil {
		return err
	}

	resource, err := h.service.Create(c.Request().Context(), domain.CreateResource{
		OwnerID: subject,
		Title:   req.Title,
	})
	if err != nil {
		return err
	}

	c.Response().Header().Set(echo.HeaderLocation, c.Request().URL.Path+"/"+resource.ID.String())
	return c.JSON(http.StatusCreated, httpx.Item(resource))
}

func (h *Resource) List(c echo.Context) error {
	page, err := httpx.PageQuery(c)
	if err != nil {
		return err
	}

	resources, total, err := h.service.List(c.Request().Context(), page)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpx.List(resources, total, page))
}

func (h *Resource) ListMine(c echo.Context) error {
	page, err := httpx.PageQuery(c)
	if err != nil {
		return err
	}

	subject, err := middleware.Subject(c)
	if err != nil {
		return err
	}

	resources, total, err := h.service.ListByOwner(c.Request().Context(), subject, page)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpx.List(resources, total, page))
}

func (h *Resource) Get(c echo.Context) error {
	id, err := httpx.UUIDParam(c, "id")
	if err != nil {
		return err
	}

	resource, err := h.service.GetByID(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpx.Item(resource))
}

func (h *Resource) Update(c echo.Context) error {
	id, err := httpx.UUIDParam(c, "id")
	if err != nil {
		return err
	}

	var req updateResourceRequest
	if err := httpx.BindAndValidate(c, &req); err != nil {
		return err
	}

	subject, err := middleware.Subject(c)
	if err != nil {
		return err
	}

	resource, err := h.service.Update(c.Request().Context(), id, subject, domain.UpdateResource{Title: req.Title})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpx.Item(resource))
}

func (h *Resource) Delete(c echo.Context) error {
	id, err := httpx.UUIDParam(c, "id")
	if err != nil {
		return err
	}

	subject, err := middleware.Subject(c)
	if err != nil {
		return err
	}

	if err := h.service.Delete(c.Request().Context(), id, subject); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
