package deployments

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/natet/honeygen/backend/internal/events"
)

type Manager struct {
	mu          sync.Mutex
	repo        *Repository
	listeners   map[string]*runningDeployment
	storageRoot string
	logger      *slog.Logger
	eventToken  string
	apiBaseURL  string
}

type runningDeployment struct {
	server *http.Server
	cancel context.CancelFunc
}

func NewManager(repo *Repository, storageRoot string, logger *slog.Logger, eventToken, apiBaseURL string) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{
		repo:        repo,
		listeners:   make(map[string]*runningDeployment),
		storageRoot: storageRoot,
		logger:      logger,
		eventToken:  eventToken,
		apiBaseURL:  strings.TrimRight(strings.TrimSpace(apiBaseURL), "/"),
	}
}

func (m *Manager) Start(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.listeners[id]; ok {
		return fmt.Errorf("deployment %q is already running", id)
	}

	d, err := m.repo.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("look up deployment: %w", err)
	}

	filePath := filepath.Join(m.storageRoot, "generated", d.WorldModelID, d.GenerationJobID)

	var handler http.Handler
	if d.RootPath != "/" && d.RootPath != "" {
		subDir := filepath.Join(filePath, filepath.FromSlash(strings.TrimPrefix(d.RootPath, "/")))
		handler = http.FileServer(http.Dir(subDir))
	} else {
		handler = http.FileServer(http.Dir(filePath))
	}

	handler = deploymentLoggingMiddleware(handler, d, m.apiBaseURL, m.eventToken, m.logger)

	addr := fmt.Sprintf(":%d", d.Port)
	server := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}

	srvCtx, cancel := context.WithCancel(context.Background())
	m.listeners[id] = &runningDeployment{server: server, cancel: cancel}

	go func() {
		m.logger.Info("deployment started", "id", id, "addr", addr, "world_model", d.WorldModelID)
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			m.logger.Error("deployment server error", "id", id, "error", err)
		}
		cancel()
	}()

	go func() {
		<-srvCtx.Done()
	}()

	if err := m.repo.UpdateStatus(ctx, id, "running"); err != nil {
		return fmt.Errorf("update deployment status: %w", err)
	}

	return nil
}

func (m *Manager) Stop(ctx context.Context, id string) error {
	m.mu.Lock()
	rd, ok := m.listeners[id]
	if ok {
		delete(m.listeners, id)
	}
	m.mu.Unlock()

	if ok {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := rd.server.Shutdown(shutdownCtx); err != nil {
			m.logger.Error("deployment shutdown error", "id", id, "error", err)
		}
		rd.cancel()
		m.logger.Info("deployment stopped", "id", id)
	}

	if err := m.repo.UpdateStatus(ctx, id, "stopped"); err != nil {
		return fmt.Errorf("update deployment status: %w", err)
	}

	return nil
}

func (m *Manager) StopAll(ctx context.Context) {
	m.mu.Lock()
	ids := make([]string, 0, len(m.listeners))
	for id := range m.listeners {
		ids = append(ids, id)
	}
	m.mu.Unlock()

	for _, id := range ids {
		if err := m.Stop(ctx, id); err != nil {
			m.logger.Error("stop deployment during shutdown", "id", id, "error", err)
		}
	}
}

func (m *Manager) RestoreRunning(ctx context.Context) {
	running, err := m.repo.ListByStatus(ctx, "running")
	if err != nil {
		m.logger.Error("restore running deployments", "error", err)
		return
	}
	for _, d := range running {
		m.logger.Info("restoring deployment", "id", d.ID, "port", d.Port)
		if err := m.Start(ctx, d.ID); err != nil {
			m.logger.Error("restore deployment failed", "id", d.ID, "error", err)
			// Mark as stopped since we couldn't restart it
			_ = m.repo.UpdateStatus(ctx, d.ID, "stopped")
		}
	}
}

func (m *Manager) IsRunning(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.listeners[id]
	return ok
}

// deploymentLoggingMiddleware wraps a handler with event forwarding, following the
// same pattern as decoy.LoggingMiddleware.
func deploymentLoggingMiddleware(next http.Handler, d Deployment, apiBaseURL, token string, logger *slog.Logger) http.Handler {
	client := &http.Client{Timeout: 3 * time.Second}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now().UTC()
		capture := &captureResponseWriter{ResponseWriter: w}
		next.ServeHTTP(capture, r)

		if apiBaseURL == "" || token == "" {
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
			Metadata: map[string]any{
				"deployment_id": d.ID,
				"world_model":   d.WorldModelID,
			},
		}

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			body, err := json.Marshal(payload)
			if err != nil {
				logger.Error("marshal deployment event", "error", err)
				return
			}

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBaseURL+"/internal/events", bytes.NewReader(body))
			if err != nil {
				logger.Error("create deployment event request", "error", err)
				return
			}
			req.Header.Set("Content-Type", "application/json; charset=utf-8")
			req.Header.Set(events.InternalIngestTokenHeader, token)

			resp, err := client.Do(req)
			if err != nil {
				logger.Error("post deployment event", "error", err, "path", payload.Path)
				return
			}
			defer resp.Body.Close()
			_, _ = io.Copy(io.Discard, resp.Body)
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

func sourceIPFromRequest(r *http.Request) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		host = strings.TrimSpace(r.RemoteAddr)
	}

	ip := net.ParseIP(strings.TrimSpace(host))
	if ip != nil && (ip.IsLoopback() || ip.IsPrivate()) {
		if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
			parts := strings.Split(forwarded, ",")
			if len(parts) > 0 {
				return strings.TrimSpace(parts[0])
			}
		}
	}

	return host
}
