package decoy

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/natet/honeygen/backend/internal/events"
)

func LoggingMiddleware(next http.Handler, recorder events.Recorder, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}

		startedAt := time.Now().UTC()
		capture := &captureResponseWriter{ResponseWriter: w}
		next.ServeHTTP(capture, r)

		if recorder == nil {
			return
		}

		payload := events.IngestRequest{
			Timestamp:  startedAt,
			Method:     r.Method,
			Path:       r.URL.Path,
			Query:      r.URL.RawQuery,
			SourceIP:   sourceIPFromRequest(r),
			UserAgent:  r.UserAgent(),
			Referer:    r.Referer(),
			StatusCode: capture.StatusCode(),
			BytesSent:  capture.bytesSent,
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			if err := recorder.Record(ctx, payload); err != nil {
				logger.Error("record decoy request event", "error", err, "path", payload.Path)
			}
		}()
	})
}

type captureResponseWriter struct {
	http.ResponseWriter
	statusCode int
	bytesSent  int
}

func (w *captureResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *captureResponseWriter) Write(data []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(data)
	w.bytesSent += n
	return n, err
}

func (w *captureResponseWriter) StatusCode() int {
	if w.statusCode == 0 {
		return http.StatusOK
	}
	return w.statusCode
}

func (w *captureResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func sourceIPFromRequest(r *http.Request) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		host = strings.TrimSpace(r.RemoteAddr)
	}

	if trustedProxyHost(host) {
		if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
			parts := strings.Split(forwarded, ",")
			if len(parts) > 0 {
				return strings.TrimSpace(parts[0])
			}
		}
	}

	return host
}

func trustedProxyHost(host string) bool {
	ip := net.ParseIP(strings.TrimSpace(host))
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate()
}
