package decoy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/natet/honeygen/backend/internal/events"
)

type eventRecorder interface {
	Record(context.Context, events.IngestRequest) error
}

type httpEventRecorder struct {
	baseURL string
	client  *http.Client
}

func newHTTPEventRecorder(baseURL string, client *http.Client) eventRecorder {
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	return &httpEventRecorder{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		client:  client,
	}
}

func (r *httpEventRecorder) Record(ctx context.Context, payload events.IngestRequest) error {
	if r == nil || r.baseURL == "" {
		return nil
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal event payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+"/internal/events", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create ingestion request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("post event payload: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("event ingestion returned status %d", resp.StatusCode)
	}
	return nil
}

func LoggingMiddleware(next http.Handler, recorder eventRecorder, logger *slog.Logger) http.Handler {
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

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := recorder.Record(ctx, events.IngestRequest{
			Timestamp:  startedAt,
			Method:     r.Method,
			Path:       r.URL.Path,
			Query:      r.URL.RawQuery,
			SourceIP:   sourceIPFromRequest(r),
			UserAgent:  r.UserAgent(),
			Referer:    r.Referer(),
			StatusCode: capture.StatusCode(),
			BytesSent:  capture.bytesSent,
		})
		if err != nil {
			logger.Error("record decoy request event", "error", err, "path", r.URL.Path)
		}
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
