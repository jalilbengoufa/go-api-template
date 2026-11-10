package httpx

import (
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/jalilbengoufa/go-api-template/internal/domain"
)

func RequestID(c echo.Context) string {
	if id := c.Request().Header.Get(echo.HeaderXRequestID); id != "" {
		return id
	}
	return c.Response().Header().Get(echo.HeaderXRequestID)
}

func BindAndValidate(c echo.Context, payload any) error {
	if err := c.Bind(payload); err != nil {
		return BadRequest("request body is malformed or uses unexpected types")
	}
	return c.Validate(payload)
}

func UUIDParam(c echo.Context, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		return uuid.Nil, BadRequest(name + " must be a valid UUID")
	}
	return id, nil
}

func PageQuery(c echo.Context) (domain.Page, error) {
	page := domain.Page{}

	if raw := c.QueryParam("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 {
			return page, BadRequest("limit must be a positive integer")
		}
		page.Limit = limit
	}

	if raw := c.QueryParam("offset"); raw != "" {
		offset, err := strconv.Atoi(raw)
		if err != nil || offset < 0 {
			return page, BadRequest("offset must be a non-negative integer")
		}
		page.Offset = offset
	}

	return page.Normalized(), nil
}

type ListResponse[T any] struct {
	Data []T  `json:"data"`
	Meta Meta `json:"meta"`
}

type Meta struct {
	Total  int64 `json:"total"`
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
}

type ItemResponse[T any] struct {
	Data T `json:"data"`
}

func List[T any](items []T, total int64, page domain.Page) ListResponse[T] {
	if items == nil {
		items = []T{}
	}
	return ListResponse[T]{Data: items, Meta: Meta{Total: total, Limit: page.Limit, Offset: page.Offset}}
}

func Item[T any](item T) ItemResponse[T] {
	return ItemResponse[T]{Data: item}
}
