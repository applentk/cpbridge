package httplog

import (
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// RequestLogger is an HTTP middleware that logs every request with method, URI,
// response status, duration, response size, client IP, and request ID.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		defer func() {
			duration := time.Since(start)
			status := ww.Status()
			if status == 0 {
				status = http.StatusOK
			}

			reqID := middleware.GetReqID(r.Context())
			if reqID == "" {
				reqID = r.Header.Get("X-Request-Id")
			}
			if reqID == "" {
				reqID = "-"
			}

			clientIP := extractIP(r)

			uri := r.URL.RequestURI()
			if uri == "" {
				uri = r.URL.Path
			}
			if uri == "" {
				uri = "/"
			}

			log.Printf("[HTTP] %d %s %s | %v | %dB | IP: %s | ReqID: %s",
				status,
				r.Method,
				uri,
				duration.Round(time.Microsecond),
				ww.BytesWritten(),
				clientIP,
				reqID,
			)
		}()

		next.ServeHTTP(ww, r)
	})
}

// extractIP determines the client IP address prioritizing X-Forwarded-For and X-Real-IP,
// falling back to RemoteAddr.
func extractIP(r *http.Request) string {
	if r == nil {
		return "unknown"
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}
	if xrip := strings.TrimSpace(r.Header.Get("X-Real-IP")); xrip != "" {
		return xrip
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	if addr := strings.TrimSpace(r.RemoteAddr); addr != "" {
		return addr
	}
	return "unknown"
}
