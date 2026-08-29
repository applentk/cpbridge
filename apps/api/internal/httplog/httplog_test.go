package httplog

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func TestRequestLogger(t *testing.T) {
	var buf bytes.Buffer
	origOutput := log.Writer()
	origFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(origOutput)
		log.SetFlags(origFlags)
	}()

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(RequestLogger)

	r.Get("/test/ok", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello world"))
	})

	r.Post("/test/created", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"status":"created"}`))
	})

	// Test 1: GET request
	req := httptest.NewRequest(http.MethodGet, "/test/ok?foo=bar", nil)
	req.RemoteAddr = "192.168.1.50:54321"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	out := buf.String()
	if !strings.Contains(out, "[HTTP] 200 GET /test/ok?foo=bar") {
		t.Errorf("log does not contain expected method/status/URI: %s", out)
	}
	if !strings.Contains(out, "IP: 192.168.1.50") {
		t.Errorf("log does not contain expected IP: %s", out)
	}

	buf.Reset()

	// Test 2: POST request with X-Forwarded-For
	req2 := httptest.NewRequest(http.MethodPost, "/test/created", strings.NewReader(`{}`))
	req2.Header.Set("X-Forwarded-For", "203.0.113.195, 70.41.3.18")
	req2.Header.Set("X-Request-Id", "custom-req-id-123")
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec2.Code)
	}

	out2 := buf.String()
	if !strings.Contains(out2, "[HTTP] 201 POST /test/created") {
		t.Errorf("log does not contain expected method/status: %s", out2)
	}
	if !strings.Contains(out2, "IP: 203.0.113.195") {
		t.Errorf("log does not contain expected forwarded IP: %s", out2)
	}
	if !strings.Contains(out2, "ReqID: custom-req-id-123") {
		t.Errorf("log does not contain custom req ID: %s", out2)
	}
}

func TestExtractIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		xrip       string
		expected   string
	}{
		{
			name:       "remote addr with port",
			remoteAddr: "10.0.0.1:12345",
			expected:   "10.0.0.1",
		},
		{
			name:       "remote addr without port",
			remoteAddr: "10.0.0.2",
			expected:   "10.0.0.2",
		},
		{
			name:       "x-forwarded-for single",
			remoteAddr: "10.0.0.1:12345",
			xff:        "198.51.100.1",
			expected:   "198.51.100.1",
		},
		{
			name:       "x-forwarded-for multiple",
			remoteAddr: "10.0.0.1:12345",
			xff:        " 198.51.100.2 , 10.0.0.1 ",
			expected:   "198.51.100.2",
		},
		{
			name:       "x-real-ip",
			remoteAddr: "10.0.0.1:12345",
			xrip:       "198.51.100.3",
			expected:   "198.51.100.3",
		},
		{
			name:     "nil request",
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.name != "nil request" {
				req = httptest.NewRequest(http.MethodGet, "/", nil)
				req.RemoteAddr = tt.remoteAddr
				if tt.xff != "" {
					req.Header.Set("X-Forwarded-For", tt.xff)
				}
				if tt.xrip != "" {
					req.Header.Set("X-Real-IP", tt.xrip)
				}
			}
			got := extractIP(req)
			if got != tt.expected {
				t.Errorf("extractIP() = %v, want %v", got, tt.expected)
			}
		})
	}
}
