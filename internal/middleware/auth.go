package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	jwtmiddleware "github.com/auth0/go-jwt-middleware/v2"
	"github.com/auth0/go-jwt-middleware/v2/jwks"
	"github.com/auth0/go-jwt-middleware/v2/validator"
	"github.com/labstack/echo/v4"

	"github.com/jalilbengoufa/go-api-template/internal/config"
	"github.com/jalilbengoufa/go-api-template/internal/httpx"
)

const DevSubject = "dev-user"

type Claims struct {
	Scope       string   `json:"scope"`
	Permissions []string `json:"permissions"`
	Email       string   `json:"email"`
	Name        string   `json:"name"`
}

func (c Claims) Validate(context.Context) error { return nil }

func (c Claims) HasPermission(permission string) bool {
	for _, p := range c.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}

func JWT(cfg config.Auth, logger *slog.Logger) (echo.MiddlewareFunc, error) {
	if cfg.Disabled {
		logger.Warn("authentication is disabled, every request is attributed to a synthetic subject",
			slog.String("subject", DevSubject))
		return devBypass(), nil
	}

	issuerURL, err := url.Parse("https://" + cfg.Domain + "/")
	if err != nil {
		return nil, fmt.Errorf("parse issuer url: %w", err)
	}

	provider := jwks.NewCachingProvider(issuerURL, 15*time.Minute)

	jwtValidator, err := validator.New(
		provider.KeyFunc,
		validator.RS256,
		issuerURL.String(),
		[]string{cfg.Audience},
		validator.WithCustomClaims(func() validator.CustomClaims { return &Claims{} }),
		validator.WithAllowedClockSkew(time.Minute),
	)
	if err != nil {
		return nil, fmt.Errorf("build jwt validator: %w", err)
	}

	return echo.WrapMiddleware(jwtmiddleware.New(
		jwtValidator.ValidateToken,
		jwtmiddleware.WithErrorHandler(authErrorHandler(logger)),
		jwtmiddleware.WithCredentialsOptional(false),
	).CheckJWT), nil
}

func authErrorHandler(logger *slog.Logger) jwtmiddleware.ErrorHandler {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		message := "access token is invalid or expired"
		if errors.Is(err, jwtmiddleware.ErrJWTMissing) {
			message = "authorization header is missing or malformed"
		}

		logger.WarnContext(r.Context(), "rejected request without a valid access token",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Any("error", err),
		)

		body := httpx.ErrorResponse{Error: httpx.ErrorBody{
			Code:      "unauthorized",
			Message:   message,
			RequestID: requestIDOf(w, r),
		}}

		w.Header().Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		w.Header().Set(echo.HeaderWWWAuthenticate, `Bearer realm="api", error="invalid_token"`)
		w.WriteHeader(http.StatusUnauthorized)
		if err := json.NewEncoder(w).Encode(body); err != nil {
			logger.ErrorContext(r.Context(), "failed to write auth error response", slog.Any("error", err))
		}
	}
}

func requestIDOf(w http.ResponseWriter, r *http.Request) string {
	if id := r.Header.Get(echo.HeaderXRequestID); id != "" {
		return id
	}
	return w.Header().Get(echo.HeaderXRequestID)
}

func devBypass() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			claims := &validator.ValidatedClaims{
				RegisteredClaims: validator.RegisteredClaims{Subject: DevSubject},
				CustomClaims:     &Claims{},
			}
			ctx := context.WithValue(req.Context(), jwtmiddleware.ContextKey{}, claims)
			c.SetRequest(req.WithContext(ctx))
			return next(c)
		}
	}
}

func ValidatedClaims(c echo.Context) (*validator.ValidatedClaims, error) {
	claims, ok := c.Request().Context().Value(jwtmiddleware.ContextKey{}).(*validator.ValidatedClaims)
	if !ok || claims == nil {
		return nil, httpx.Unauthorized("missing or invalid access token")
	}
	return claims, nil
}

func Subject(c echo.Context) (string, error) {
	claims, err := ValidatedClaims(c)
	if err != nil {
		return "", err
	}
	if claims.RegisteredClaims.Subject == "" {
		return "", httpx.Unauthorized("access token has no subject")
	}
	return claims.RegisteredClaims.Subject, nil
}

func CustomClaims(c echo.Context) (*Claims, error) {
	claims, err := ValidatedClaims(c)
	if err != nil {
		return nil, err
	}
	custom, ok := claims.CustomClaims.(*Claims)
	if !ok || custom == nil {
		return nil, httpx.Unauthorized("access token has no custom claims")
	}
	return custom, nil
}

func RequirePermission(permission string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claims, err := CustomClaims(c)
			if err != nil {
				return err
			}
			if !claims.HasPermission(permission) {
				return httpx.Forbidden("missing required permission: " + permission)
			}
			return next(c)
		}
	}
}
