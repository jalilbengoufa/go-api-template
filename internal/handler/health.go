package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

type Health struct {
	db      Pinger
	version string
	timeout time.Duration
}

func NewHealth(db Pinger, version string) *Health {
	return &Health{db: db, version: version, timeout: 2 * time.Second}
}

type healthResponse struct {
	Status   string            `json:"status"`
	Version  string            `json:"version"`
	Checks   map[string]string `json:"checks,omitempty"`
	Duration string            `json:"duration,omitempty"`
}

func (h *Health) Live(c echo.Context) error {
	return c.JSON(http.StatusOK, healthResponse{Status: "ok", Version: h.version})
}

func (h *Health) Ready(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), h.timeout)
	defer cancel()

	start := time.Now()
	status := http.StatusOK
	body := healthResponse{Status: "ok", Version: h.version, Checks: map[string]string{}}

	if err := h.db.Ping(ctx); err != nil {
		status = http.StatusServiceUnavailable
		body.Status = "unavailable"
		body.Checks["database"] = "unavailable"
	} else {
		body.Checks["database"] = "ok"
	}

	body.Duration = time.Since(start).String()
	return c.JSON(status, body)
}
