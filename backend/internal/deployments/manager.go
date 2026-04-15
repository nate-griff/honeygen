package deployments

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/natet/honeygen/backend/internal/events"
)

type Manager struct {
	mu              sync.Mutex
	repo            *Repository
	listeners       map[string]*runningDeployment
	storageRoot     string
	ftpPublicHost   string
	ftpPassivePorts string
	logger          *slog.Logger
	recorder        events.Recorder
}

type runningDeployment struct {
	stop   func() // protocol-specific shutdown function
	cancel context.CancelFunc
}

func NewManager(repo *Repository, storageRoot string, logger *slog.Logger, eventToken, apiBaseURL, ftpPublicHost, ftpPassivePorts string) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{
		repo:            repo,
		listeners:       make(map[string]*runningDeployment),
		storageRoot:     storageRoot,
		ftpPublicHost:   strings.TrimSpace(ftpPublicHost),
		ftpPassivePorts: strings.TrimSpace(ftpPassivePorts),
		logger:          logger,
		recorder:        events.NewHTTPRecorder(apiBaseURL, eventToken, nil),
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

	filePath := m.resolveFilePath(d)

	// Verify the directory exists.
	if _, err := os.Stat(filePath); err != nil {
		return fmt.Errorf("generated files directory not found: %w", err)
	}

	srvCtx, cancel := context.WithCancel(context.Background())
	var stopFn func()

	switch d.Protocol {
	case "http":
		stopFn, err = m.startHTTP(d, filePath, srvCtx, cancel)
	case "ftp":
		stopFn, err = m.startFTP(d, filePath, srvCtx, cancel)
	case "nfs":
		stopFn, err = m.startNFS(d, filePath, srvCtx, cancel)
	case "smb":
		stopFn, err = m.startSMB(d, filePath, srvCtx, cancel)
	default:
		cancel()
		return fmt.Errorf("unsupported protocol: %s", d.Protocol)
	}

	if err != nil {
		cancel()
		return err
	}

	m.listeners[id] = &runningDeployment{stop: stopFn, cancel: cancel}

	if err := m.repo.UpdateStatus(ctx, id, "running"); err != nil {
		return fmt.Errorf("update deployment status: %w", err)
	}

	return nil
}

func (m *Manager) resolveFilePath(d Deployment) string {
	filePath := filepath.Join(m.storageRoot, "generated", d.WorldModelID, d.GenerationJobID)
	rootPath := normalizeDeploymentRootPath(d)
	if rootPath != "/" {
		filePath = filepath.Join(filePath, filepath.FromSlash(strings.TrimPrefix(rootPath, "/")))
	}
	return filePath
}

func normalizeDeploymentRootPath(d Deployment) string {
	rootPath := strings.TrimSpace(filepath.ToSlash(d.RootPath))
	if rootPath == "" || rootPath == "/" {
		return "/"
	}

	cleanRoot := path.Clean("/" + strings.TrimPrefix(rootPath, "/"))
	fullPrefix := path.Clean("/generated/" + d.WorldModelID + "/" + d.GenerationJobID)

	if cleanRoot == fullPrefix {
		return "/"
	}
	if strings.HasPrefix(cleanRoot, fullPrefix+"/") {
		return strings.TrimPrefix(cleanRoot, fullPrefix)
	}

	return cleanRoot
}

func (m *Manager) startHTTP(d Deployment, filePath string, srvCtx context.Context, cancel context.CancelFunc) (func(), error) {
	handler := http.Handler(http.FileServer(http.Dir(filePath)))
	handler = deploymentLoggingMiddleware(handler, d, m.recorder, m.logger)

	addr := fmt.Sprintf(":%d", d.Port)
	server := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", addr, err)
	}

	go func() {
		m.logger.Info("http deployment started", "id", d.ID, "addr", addr, "world_model", d.WorldModelID)
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			m.logger.Error("http deployment server error", "id", d.ID, "error", err)
		}
		cancel()
	}()

	stopFn := func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownCtx)
	}
	_ = srvCtx // keep the context alive via the cancel in runningDeployment

	return stopFn, nil
}

func (m *Manager) Stop(ctx context.Context, id string) error {
	m.mu.Lock()
	rd, ok := m.listeners[id]
	if ok {
		delete(m.listeners, id)
	}
	m.mu.Unlock()

	if ok {
		rd.stop()
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
		m.logger.Info("restoring deployment", "id", d.ID, "port", d.Port, "protocol", d.Protocol)
		if err := m.Start(ctx, d.ID); err != nil {
			m.logger.Error("restore deployment failed", "id", d.ID, "error", err)
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
func deploymentLoggingMiddleware(next http.Handler, d Deployment, recorder events.Recorder, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now().UTC()
		capture := &captureResponseWriter{ResponseWriter: w}
		next.ServeHTTP(capture, r)

		if recorder == nil {
			return
		}

		payload := events.IngestRequest{
			Timestamp:  startedAt,
			EventType:  "http_request",
			Method:     r.Method,
			Path:       deploymentEventPath(d, r.URL.Path),
			Query:      r.URL.RawQuery,
			SourceIP:   sourceIPFromRequest(r),
			UserAgent:  r.UserAgent(),
			Referer:    r.Referer(),
			StatusCode: capture.StatusCode(),
			BytesSent:  capture.bytesSent,
			Metadata: map[string]any{
				"deployment_id": d.ID,
				"world_model":   d.WorldModelID,
				"protocol":      d.Protocol,
			},
		}

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			if err := recorder.Record(ctx, payload); err != nil {
				logger.Error("post deployment event", "error", err, "path", payload.Path)
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
