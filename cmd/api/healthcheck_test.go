package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jalilbengoufa/go-api-template/internal/config"
)

func TestReadinessURL(t *testing.T) {
	tests := []struct {
		name string
		srv  config.Server
		want string
	}{
		{name: "all interfaces", srv: config.Server{Port: 8080}, want: "http://127.0.0.1:8080/health/ready"},
		{name: "ipv4 wildcard", srv: config.Server{Host: "0.0.0.0", Port: 9000}, want: "http://127.0.0.1:9000/health/ready"},
		{name: "ipv6 wildcard", srv: config.Server{Host: "::", Port: 8080}, want: "http://127.0.0.1:8080/health/ready"},
		{name: "specific host", srv: config.Server{Host: "10.0.0.5", Port: 8080}, want: "http://10.0.0.5:8080/health/ready"},
		{name: "ipv6 host", srv: config.Server{Host: "::1", Port: 8080}, want: "http://[::1]:8080/health/ready"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := readinessURL(tt.srv); got != tt.want {
				t.Errorf("readinessURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProbe(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{name: "ready", status: http.StatusOK, wantErr: false},
		{name: "not ready", status: http.StatusServiceUnavailable, wantErr: true},
		{name: "not found", status: http.StatusNotFound, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()

			err := probe(t.Context(), srv.URL)
			if (err != nil) != tt.wantErr {
				t.Fatalf("probe() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestProbeUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()

	if err := probe(t.Context(), url); err == nil {
		t.Fatal("probe() error = nil, want connection error")
	}
}
