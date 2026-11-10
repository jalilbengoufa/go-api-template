package config

import (
	"strings"
	"testing"
	"time"
)

func baseEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ENV_FILE", "")
	t.Setenv("PG_HOST", "localhost")
	t.Setenv("PG_USER", "api")
	t.Setenv("PG_DB", "api")
	t.Setenv("AUTH_DISABLED", "true")
}

func TestLoadDefaults(t *testing.T) {
	baseEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.Env != EnvDevelopment {
		t.Errorf("Env = %q, want %q", cfg.Env, EnvDevelopment)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want 8080", cfg.Server.Port)
	}
	if cfg.Database.Port != 5432 {
		t.Errorf("Database.Port = %d, want 5432", cfg.Database.Port)
	}
	if cfg.Server.ShutdownTimeout != 15*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 15s", cfg.Server.ShutdownTimeout)
	}
	if got, want := strings.Join(cfg.Server.CORSAllowOrigins, ","), "*"; got != want {
		t.Errorf("CORSAllowOrigins = %q, want %q", got, want)
	}
}

func TestLoadOverrides(t *testing.T) {
	baseEnv(t)
	t.Setenv("APP_ENV", "staging")
	t.Setenv("APP_PORT", "9000")
	t.Setenv("SERVER_READ_TIMEOUT", "30s")
	t.Setenv("CORS_ALLOW_ORIGINS", "https://a.example , https://b.example,")
	t.Setenv("LOG_FORMAT", "text")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.Env != EnvStaging {
		t.Errorf("Env = %q, want staging", cfg.Env)
	}
	if cfg.Server.Port != 9000 {
		t.Errorf("Server.Port = %d, want 9000", cfg.Server.Port)
	}
	if cfg.Server.ReadTimeout != 30*time.Second {
		t.Errorf("ReadTimeout = %v, want 30s", cfg.Server.ReadTimeout)
	}
	if len(cfg.Server.CORSAllowOrigins) != 2 {
		t.Fatalf("CORSAllowOrigins = %v, want 2 entries", cfg.Server.CORSAllowOrigins)
	}
	if cfg.Server.Address() != ":9000" {
		t.Errorf("Address() = %q, want \":9000\"", cfg.Server.Address())
	}
}

func TestLoadErrors(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{
			name:    "missing database name",
			env:     map[string]string{"PG_DB": ""},
			wantErr: "PG_DB: is required",
		},
		{
			name:    "invalid port",
			env:     map[string]string{"APP_PORT": "not-a-number"},
			wantErr: "APP_PORT: invalid integer",
		},
		{
			name:    "port out of range",
			env:     map[string]string{"APP_PORT": "70000"},
			wantErr: "APP_PORT: must be between 1 and 65535",
		},
		{
			name:    "invalid duration",
			env:     map[string]string{"SERVER_READ_TIMEOUT": "ten-seconds"},
			wantErr: "SERVER_READ_TIMEOUT: invalid duration",
		},
		{
			name:    "negative duration",
			env:     map[string]string{"SERVER_READ_TIMEOUT": "-5s"},
			wantErr: "SERVER_READ_TIMEOUT: must be greater than 0",
		},
		{
			name:    "invalid ssl mode",
			env:     map[string]string{"PG_SSL_MODE": "maybe"},
			wantErr: "PG_SSL_MODE: invalid value",
		},
		{
			name:    "idle exceeds open conns",
			env:     map[string]string{"PG_MAX_OPEN_CONNS": "5", "PG_MAX_IDLE_CONNS": "10"},
			wantErr: "PG_MAX_IDLE_CONNS: must not exceed",
		},
		{
			name:    "auth required when enabled",
			env:     map[string]string{"AUTH_DISABLED": "false"},
			wantErr: "AUTH0_DOMAIN: is required",
		},
		{
			name:    "auth cannot be disabled in production",
			env:     map[string]string{"APP_ENV": "production"},
			wantErr: "AUTH_DISABLED: must not be true",
		},
		{
			name:    "unknown environment",
			env:     map[string]string{"APP_ENV": "qa"},
			wantErr: "APP_ENV: must be one of",
		},
		{
			name:    "invalid trusted proxy",
			env:     map[string]string{"TRUSTED_PROXIES": "10.0.0.0/8,not-a-cidr"},
			wantErr: "TRUSTED_PROXIES: \"not-a-cidr\" is not a valid CIDR block",
		},
		{
			name:    "invalid log level",
			env:     map[string]string{"LOG_LEVEL": "trace"},
			wantErr: "LOG_LEVEL: must be one of",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseEnv(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() = nil error, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Load() error = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestTrustedProxies(t *testing.T) {
	baseEnv(t)
	t.Setenv("TRUSTED_PROXIES", "10.0.0.0/8, 192.168.0.0/16")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if len(cfg.Server.TrustedProxies) != 2 {
		t.Fatalf("TrustedProxies = %v, want 2 entries", cfg.Server.TrustedProxies)
	}
	if got := cfg.Server.TrustedProxies[0].String(); got != "10.0.0.0/8" {
		t.Errorf("TrustedProxies[0] = %q, want %q", got, "10.0.0.0/8")
	}
}

func TestTrustedProxiesDefaultsToEmpty(t *testing.T) {
	baseEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if len(cfg.Server.TrustedProxies) != 0 {
		t.Errorf("TrustedProxies = %v, want empty so proxy headers are ignored", cfg.Server.TrustedProxies)
	}
}

func TestDatabaseDSN(t *testing.T) {
	db := Database{
		Host:           "db",
		Port:           5432,
		User:           "api",
		Password:       "secret",
		Name:           "apidb",
		SSLMode:        "require",
		ConnectTimeout: 5 * time.Second,
	}

	want := "host=db port=5432 user=api password=secret dbname=apidb sslmode=require connect_timeout=5"
	if got := db.DSN(); got != want {
		t.Errorf("DSN() = %q, want %q", got, want)
	}
}

func TestAuthDomainNormalization(t *testing.T) {
	baseEnv(t)
	t.Setenv("AUTH_DISABLED", "false")
	t.Setenv("AUTH0_DOMAIN", "https://tenant.eu.auth0.com/")
	t.Setenv("AUTH0_AUDIENCE", "https://api.example")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.Auth.Domain != "tenant.eu.auth0.com" {
		t.Errorf("Auth.Domain = %q, want %q", cfg.Auth.Domain, "tenant.eu.auth0.com")
	}
}
