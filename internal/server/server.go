package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"

	"github.com/jalilbengoufa/go-api-template/internal/config"
	"github.com/jalilbengoufa/go-api-template/internal/handler"
	"github.com/jalilbengoufa/go-api-template/internal/httpx"
	"github.com/jalilbengoufa/go-api-template/internal/middleware"
)

type Dependencies struct {
	Config   *config.Config
	Logger   *slog.Logger
	Database handler.Pinger
	Resource handler.ResourceService
	Version  string
}

type Server struct {
	echo   *echo.Echo
	cfg    *config.Config
	logger *slog.Logger
}

func New(deps Dependencies) (*Server, error) {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.IPExtractor = ipExtractor(deps.Config.Server.TrustedProxies)
	e.Validator = httpx.NewValidator()
	e.HTTPErrorHandler = httpx.ErrorHandler(deps.Logger)
	e.Server.ReadTimeout = deps.Config.Server.ReadTimeout
	e.Server.WriteTimeout = deps.Config.Server.WriteTimeout
	e.Server.IdleTimeout = deps.Config.Server.IdleTimeout
	e.Server.ReadHeaderTimeout = deps.Config.Server.ReadHeaderTimeout

	srv := &Server{echo: e, cfg: deps.Config, logger: deps.Logger}
	srv.useMiddleware()

	if err := srv.registerRoutes(deps); err != nil {
		return nil, err
	}
	return srv, nil
}

func ipExtractor(trusted []*net.IPNet) echo.IPExtractor {
	if len(trusted) == 0 {
		return echo.ExtractIPDirect()
	}

	opts := []echo.TrustOption{
		echo.TrustLoopback(false),
		echo.TrustLinkLocal(false),
		echo.TrustPrivateNet(false),
	}
	for _, network := range trusted {
		opts = append(opts, echo.TrustIPRange(network))
	}
	return echo.ExtractIPFromXFFHeader(opts...)
}

func (s *Server) useMiddleware() {
	s.echo.Pre(echomw.RemoveTrailingSlash())

	s.echo.Use(echomw.RequestIDWithConfig(echomw.RequestIDConfig{
		TargetHeader: echo.HeaderXRequestID,
		RequestIDHandler: func(c echo.Context, id string) {
			c.Request().Header.Set(echo.HeaderXRequestID, id)
		},
	}))
	s.echo.Use(middleware.ContextLogger(s.logger))
	s.echo.Use(middleware.RequestLogger(s.logger, "/health/live", "/health/ready"))
	s.echo.Use(echomw.RecoverWithConfig(echomw.RecoverConfig{
		StackSize:         4 << 10,
		DisablePrintStack: true,
		LogErrorFunc: func(c echo.Context, err error, stack []byte) error {
			s.logger.ErrorContext(c.Request().Context(), "recovered from panic",
				slog.Any("error", err),
				slog.String("stack", string(stack)),
			)
			return err
		},
	}))
	s.echo.Use(echomw.SecureWithConfig(echomw.SecureConfig{
		XFrameOptions:         "DENY",
		ContentTypeNosniff:    "nosniff",
		HSTSMaxAge:            31536000,
		HSTSExcludeSubdomains: false,
		ContentSecurityPolicy: "default-src 'none'; frame-ancestors 'none'",
		ReferrerPolicy:        "no-referrer",
	}))
	s.echo.Use(echomw.CORSWithConfig(echomw.CORSConfig{
		AllowOrigins: s.cfg.Server.CORSAllowOrigins,
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization, echo.HeaderXRequestID},
		MaxAge:       int((12 * time.Hour).Seconds()),
	}))
	s.echo.Use(echomw.BodyLimit(s.cfg.Server.BodyLimit))
	s.echo.Use(echomw.GzipWithConfig(echomw.GzipConfig{Level: 5, MinLength: 1024}))
	s.echo.Use(echomw.RateLimiterWithConfig(echomw.RateLimiterConfig{
		Store: echomw.NewRateLimiterMemoryStoreWithConfig(echomw.RateLimiterMemoryStoreConfig{
			Rate:      rate.Limit(s.cfg.Server.RateLimit),
			Burst:     int(s.cfg.Server.RateLimit) * 3,
			ExpiresIn: 3 * time.Minute,
		}),
	}))
	s.echo.Use(echomw.ContextTimeoutWithConfig(echomw.ContextTimeoutConfig{
		Timeout: s.cfg.Server.RequestTimeout,
		Skipper: func(c echo.Context) bool {
			return c.Request().Method == http.MethodGet && c.Path() == "/health/ready"
		},
		ErrorHandler: func(err error, _ echo.Context) error { return err },
	}))
}

func (s *Server) registerRoutes(deps Dependencies) error {
	health := handler.NewHealth(deps.Database, deps.Version)
	s.echo.GET("/health/live", health.Live)
	s.echo.GET("/health/ready", health.Ready)

	auth, err := middleware.JWT(s.cfg.Auth, s.logger)
	if err != nil {
		return fmt.Errorf("configure auth middleware: %w", err)
	}

	v1 := s.echo.Group("/v1")
	resources := v1.Group("/resources", auth)
	handler.NewResource(deps.Resource).Register(resources)

	return nil
}

func (s *Server) Handler() http.Handler { return s.echo }

func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		s.logger.Info("server listening", slog.String("address", s.cfg.Server.Address()), slog.String("env", string(s.cfg.Env)))
		if err := s.echo.Start(s.cfg.Server.Address()); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("start server: %w", err)
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		s.logger.Info("shutdown signal received", slog.Duration("timeout", s.cfg.Server.ShutdownTimeout))
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := s.echo.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown server: %w", err)
	}

	s.logger.Info("server stopped gracefully")
	return nil
}
