package middleware

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"time"

	jwtmiddleware "github.com/auth0/go-jwt-middleware/v2"
	"github.com/auth0/go-jwt-middleware/v2/jwks"
	"github.com/auth0/go-jwt-middleware/v2/validator"
	"github.com/go-api-template/lib/viper"
	"github.com/labstack/echo/v4"
)

// CustomClaims contains custom data we want from the token.
type CustomClaims struct {
	Nickname      string `json:"nickname"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	UpdatedAt     string `json:"updated_at"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Issuer        string `json:"iss"`
	Audience      string `json:"aud"`
	IssuedAt      int64  `json:"iat"`
	ExpiresAt     int64  `json:"exp"`
	Subject       string `json:"sub"`
	SessionID     string `json:"sid"`
	Nonce         string `json:"nonce"`
}

func ParseClaims(c echo.Context) *CustomClaims {
	token := c.Request().Context().Value(jwtmiddleware.ContextKey{}).(*validator.ValidatedClaims)

	return token.CustomClaims.(*CustomClaims)
}

// Validate values from token
func (c CustomClaims) Validate(ctx context.Context) error {
	return nil
}

func EchoEnsureValidToken() echo.MiddlewareFunc {
	mw := echo.WrapMiddleware(EnsureValidToken())
	return mw
}

// EnsureValidToken is a middleware that will check the validity of our JWT.
func EnsureValidToken() func(next http.Handler) http.Handler {
	authDomain := viper.ViperEnvVariable("AUTH0_DOMAIN")
	authAudience := viper.ViperEnvVariable("AUTH0_AUDIENCE")

	issuerURL, err := url.Parse("https://" + authDomain + "/")
	if err != nil {
		log.Fatalf("Failed to parse the issuer url: %v", err)
	}

	provider := jwks.NewCachingProvider(issuerURL, 5*time.Minute)

	jwtValidator, err := validator.New(
		provider.KeyFunc,
		validator.RS256,
		issuerURL.String(),
		[]string{authAudience},
		validator.WithCustomClaims(
			func() validator.CustomClaims {
				return &CustomClaims{}
			},
		),
		validator.WithAllowedClockSkew(time.Minute),
	)
	if err != nil {
		log.Fatalf("Failed to set up the jwt validator")
	}

	middleware := jwtmiddleware.New(
		jwtValidator.ValidateToken,
		jwtmiddleware.WithErrorHandler(errorHandler),
	)
	return handleNextMiddleware(middleware)
}

func errorHandler(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("Encountered error while validating JWT: %v", err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"message":"Failed to validate JWT."}`))
}

func handleNextMiddleware(middleware *jwtmiddleware.JWTMiddleware) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return middleware.CheckJWT(next)
	}
}
