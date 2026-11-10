package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Environment string

const (
	EnvDevelopment Environment = "development"
	EnvStaging     Environment = "staging"
	EnvProduction  Environment = "production"
)

func (e Environment) IsProduction() bool { return e == EnvProduction }

type Config struct {
	Env      Environment
	Server   Server
	Database Database
	Auth     Auth
	Log      Log
}

type Server struct {
	Host              string
	Port              int
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	RequestTimeout    time.Duration
	ShutdownTimeout   time.Duration
	BodyLimit         string
	RateLimit         float64
	CORSAllowOrigins  []string
	TrustedProxies    []*net.IPNet
}

type Database struct {
	Host            string
	Port            int
	User            string
	Password        string
	Name            string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	ConnectTimeout  time.Duration
}

type Auth struct {
	Domain   string
	Audience string
	Disabled bool
}

type Log struct {
	Level  string
	Format string
}

func (d Database) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=%d",
		d.Host, d.Port, d.User, d.Password, d.Name, d.SSLMode, int(d.ConnectTimeout.Seconds()),
	)
}

func (s Server) Address() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

func Load() (*Config, error) {
	if path := os.Getenv("ENV_FILE"); path != "" {
		if err := godotenv.Load(path); err != nil {
			return nil, fmt.Errorf("load env file %q: %w", path, err)
		}
	} else if _, err := os.Stat(".env"); err == nil {
		if err := godotenv.Load(".env"); err != nil {
			return nil, fmt.Errorf("load .env: %w", err)
		}
	}

	var errs []error
	collect := func(err error) {
		if err != nil {
			errs = append(errs, err)
		}
	}

	cfg := &Config{
		Env: Environment(strings.ToLower(stringVar("APP_ENV", string(EnvDevelopment)))),
	}

	cfg.Server = Server{
		Host:              stringVar("APP_HOST", ""),
		ReadTimeout:       durationVar("SERVER_READ_TIMEOUT", 15*time.Second, collect),
		WriteTimeout:      durationVar("SERVER_WRITE_TIMEOUT", 15*time.Second, collect),
		IdleTimeout:       durationVar("SERVER_IDLE_TIMEOUT", 60*time.Second, collect),
		ReadHeaderTimeout: durationVar("SERVER_READ_HEADER_TIMEOUT", 5*time.Second, collect),
		RequestTimeout:    durationVar("SERVER_REQUEST_TIMEOUT", 10*time.Second, collect),
		ShutdownTimeout:   durationVar("SERVER_SHUTDOWN_TIMEOUT", 15*time.Second, collect),
		BodyLimit:         stringVar("SERVER_BODY_LIMIT", "1M"),
		RateLimit:         floatVar("SERVER_RATE_LIMIT", 20, collect),
		CORSAllowOrigins:  sliceVar("CORS_ALLOW_ORIGINS", []string{"*"}),
		TrustedProxies:    cidrVar("TRUSTED_PROXIES", collect),
	}
	cfg.Server.Port = intVar("APP_PORT", 8080, collect)

	cfg.Database = Database{
		Host:            stringVar("PG_HOST", "localhost"),
		User:            stringVar("PG_USER", ""),
		Password:        stringVar("PG_PASSWORD", ""),
		Name:            stringVar("PG_DB", ""),
		SSLMode:         stringVar("PG_SSL_MODE", "disable"),
		MaxOpenConns:    intVar("PG_MAX_OPEN_CONNS", 25, collect),
		MaxIdleConns:    intVar("PG_MAX_IDLE_CONNS", 10, collect),
		ConnMaxLifetime: durationVar("PG_CONN_MAX_LIFETIME", 30*time.Minute, collect),
		ConnMaxIdleTime: durationVar("PG_CONN_MAX_IDLE_TIME", 5*time.Minute, collect),
		ConnectTimeout:  durationVar("PG_CONNECT_TIMEOUT", 5*time.Second, collect),
	}
	cfg.Database.Port = intVar("PG_PORT", 5432, collect)

	cfg.Auth = Auth{
		Domain:   strings.TrimSuffix(strings.TrimPrefix(stringVar("AUTH0_DOMAIN", ""), "https://"), "/"),
		Audience: stringVar("AUTH0_AUDIENCE", ""),
		Disabled: boolVar("AUTH_DISABLED", false, collect),
	}

	cfg.Log = Log{
		Level:  strings.ToLower(stringVar("LOG_LEVEL", "info")),
		Format: strings.ToLower(stringVar("LOG_FORMAT", "json")),
	}

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	var errs []error

	switch c.Env {
	case EnvDevelopment, EnvStaging, EnvProduction:
	default:
		errs = append(errs, fmt.Errorf("APP_ENV: must be one of development, staging, production, got %q", c.Env))
	}

	if c.Server.Port < 1 || c.Server.Port > 65535 {
		errs = append(errs, fmt.Errorf("APP_PORT: must be between 1 and 65535, got %d", c.Server.Port))
	}
	if c.Server.RateLimit <= 0 {
		errs = append(errs, fmt.Errorf("SERVER_RATE_LIMIT: must be greater than 0, got %v", c.Server.RateLimit))
	}

	if c.Database.Host == "" {
		errs = append(errs, errors.New("PG_HOST: is required"))
	}
	if c.Database.User == "" {
		errs = append(errs, errors.New("PG_USER: is required"))
	}
	if c.Database.Name == "" {
		errs = append(errs, errors.New("PG_DB: is required"))
	}
	if c.Database.Port < 1 || c.Database.Port > 65535 {
		errs = append(errs, fmt.Errorf("PG_PORT: must be between 1 and 65535, got %d", c.Database.Port))
	}
	switch c.Database.SSLMode {
	case "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
	default:
		errs = append(errs, fmt.Errorf("PG_SSL_MODE: invalid value %q", c.Database.SSLMode))
	}
	if c.Database.MaxIdleConns > c.Database.MaxOpenConns {
		errs = append(errs, fmt.Errorf("PG_MAX_IDLE_CONNS: must not exceed PG_MAX_OPEN_CONNS (%d)", c.Database.MaxOpenConns))
	}

	if !c.Auth.Disabled {
		if c.Auth.Domain == "" {
			errs = append(errs, errors.New("AUTH0_DOMAIN: is required unless AUTH_DISABLED=true"))
		} else if _, err := url.Parse("https://" + c.Auth.Domain + "/"); err != nil {
			errs = append(errs, fmt.Errorf("AUTH0_DOMAIN: %w", err))
		}
		if c.Auth.Audience == "" {
			errs = append(errs, errors.New("AUTH0_AUDIENCE: is required unless AUTH_DISABLED=true"))
		}
	} else if c.Env.IsProduction() {
		errs = append(errs, errors.New("AUTH_DISABLED: must not be true when APP_ENV=production"))
	}

	switch c.Log.Level {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, fmt.Errorf("LOG_LEVEL: must be one of debug, info, warn, error, got %q", c.Log.Level))
	}
	switch c.Log.Format {
	case "json", "text":
	default:
		errs = append(errs, fmt.Errorf("LOG_FORMAT: must be one of json, text, got %q", c.Log.Format))
	}

	return errors.Join(errs...)
}

func stringVar(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func intVar(key string, fallback int, collect func(error)) int {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		collect(fmt.Errorf("%s: invalid integer %q", key, raw))
		return fallback
	}
	return v
}

func floatVar(key string, fallback float64, collect func(error)) float64 {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return fallback
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		collect(fmt.Errorf("%s: invalid number %q", key, raw))
		return fallback
	}
	return v
}

func boolVar(key string, fallback bool, collect func(error)) bool {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return fallback
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		collect(fmt.Errorf("%s: invalid boolean %q", key, raw))
		return fallback
	}
	return v
}

func durationVar(key string, fallback time.Duration, collect func(error)) time.Duration {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return fallback
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		collect(fmt.Errorf("%s: invalid duration %q", key, raw))
		return fallback
	}
	if v <= 0 {
		collect(fmt.Errorf("%s: must be greater than 0, got %q", key, raw))
		return fallback
	}
	return v
}

func cidrVar(key string, collect func(error)) []*net.IPNet {
	raw := sliceVar(key, nil)
	if len(raw) == 0 {
		return nil
	}

	out := make([]*net.IPNet, 0, len(raw))
	for _, entry := range raw {
		_, network, err := net.ParseCIDR(entry)
		if err != nil {
			collect(fmt.Errorf("%s: %q is not a valid CIDR block", key, entry))
			continue
		}
		out = append(out, network)
	}
	return out
}

func sliceVar(key string, fallback []string) []string {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}
