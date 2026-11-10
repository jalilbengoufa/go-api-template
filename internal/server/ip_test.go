package server

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func mustCIDR(t *testing.T, raw string) *net.IPNet {
	t.Helper()
	_, network, err := net.ParseCIDR(raw)
	if err != nil {
		t.Fatalf("ParseCIDR(%q) returned error: %v", raw, err)
	}
	return network
}

func TestIPExtractorIgnoresSpoofedHeadersByDefault(t *testing.T) {
	e := echo.New()
	e.IPExtractor = ipExtractor(nil)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	req.RemoteAddr = "203.0.113.7:1234"
	req.Header.Set(echo.HeaderXForwardedFor, "1.2.3.4")
	req.Header.Set(echo.HeaderXRealIP, "5.6.7.8")

	c := e.NewContext(req, httptest.NewRecorder())
	if got := c.RealIP(); got != "203.0.113.7" {
		t.Errorf("RealIP() = %q, want the socket address 203.0.113.7, not a header value", got)
	}
}

func TestIPExtractorTrustsOnlyConfiguredProxies(t *testing.T) {
	trusted := []*net.IPNet{mustCIDR(t, "10.0.0.0/8")}

	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		want       string
	}{
		{"from trusted proxy", "10.0.0.1:1234", "1.2.3.4", "1.2.3.4"},
		{"from untrusted peer", "203.0.113.7:1234", "1.2.3.4", "203.0.113.7"},
		{"loopback is not implicitly trusted", "127.0.0.1:1234", "1.2.3.4", "127.0.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			e.IPExtractor = ipExtractor(trusted)

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
			req.RemoteAddr = tt.remoteAddr
			req.Header.Set(echo.HeaderXForwardedFor, tt.xff)

			c := e.NewContext(req, httptest.NewRecorder())
			if got := c.RealIP(); got != tt.want {
				t.Errorf("RealIP() = %q, want %q", got, tt.want)
			}
		})
	}
}
