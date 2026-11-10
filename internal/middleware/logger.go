package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/jalilbengoufa/go-api-template/internal/httpx"
)

type contextKey struct{}

func RequestLogger(logger *slog.Logger, skipPaths ...string) echo.MiddlewareFunc {
	skip := make(map[string]struct{}, len(skipPaths))
	for _, p := range skipPaths {
		skip[p] = struct{}{}
	}

	return middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogLatency:       true,
		LogRemoteIP:      true,
		LogHost:          true,
		LogMethod:        true,
		LogURI:           true,
		LogUserAgent:     false,
		LogStatus:        true,
		LogError:         true,
		LogRequestID:     true,
		HandleError:      true,
		LogProtocol:      true,
		LogRoutePath:     true,
		LogContentLength: true,
		LogResponseSize:  true,
		Skipper: func(c echo.Context) bool {
			_, ok := skip[c.Request().URL.Path]
			return ok
		},
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			attrs := []slog.Attr{
				slog.String("request_id", v.RequestID),
				slog.String("method", v.Method),
				slog.String("uri", v.URI),
				slog.String("route", v.RoutePath),
				slog.Int("status", v.Status),
				slog.Duration("latency", v.Latency),
				slog.String("remote_ip", v.RemoteIP),
				slog.String("host", v.Host),
				slog.String("protocol", v.Protocol),
				slog.Int64("response_size", v.ResponseSize),
			}

			level := slog.LevelInfo
			switch {
			case v.Status >= http.StatusInternalServerError:
				level = slog.LevelError
			case v.Status >= http.StatusBadRequest:
				level = slog.LevelWarn
			}
			if v.Error != nil {
				attrs = append(attrs, slog.String("error", v.Error.Error()))
			}

			logger.LogAttrs(c.Request().Context(), level, "request handled", attrs...)
			return nil
		},
	})
}

func ContextLogger(logger *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			scoped := logger.With(slog.String("request_id", httpx.RequestID(c)))
			c.SetRequest(req.WithContext(context.WithValue(req.Context(), contextKey{}, scoped)))
			return next(c)
		}
	}
}

func LoggerFrom(ctx context.Context, fallback *slog.Logger) *slog.Logger {
	if logger, ok := ctx.Value(contextKey{}).(*slog.Logger); ok && logger != nil {
		return logger
	}
	return fallback
}
