package deployments

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	ftpserver "goftp.io/server/v2"
)

func (m *Manager) startFTP(d Deployment, filePath string, srvCtx context.Context, cancel context.CancelFunc) (func(), error) {
	addr := fmt.Sprintf(":%d", d.Port)

	driver := &ftpFileDriver{root: filePath}
	opts := &ftpserver.Options{
		Name:   fmt.Sprintf("honeygen-ftp-%s", d.ID),
		Driver: driver,
		Auth:   &ftpAnonymousAuth{},
		Perm:   ftpserver.NewSimplePerm("honeygen", "honeygen"),
		Port:   d.Port,
		Logger: &ftpLogger{m: m, deploymentID: d.ID},
	}

	server, err := ftpserver.NewServer(opts)
	if err != nil {
		return nil, fmt.Errorf("create ftp server: %w", err)
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("ftp listen on %s: %w", addr, err)
	}

	go func() {
		m.logger.Info("ftp deployment started", "id", d.ID, "addr", addr, "world_model", d.WorldModelID)
		if err := server.Serve(listener); err != nil {
			if srvCtx.Err() == nil {
				m.logger.Error("ftp deployment server error", "id", d.ID, "error", err)
			}
		}
		cancel()
	}()

	stopFn := func() {
		_ = server.Shutdown()
	}

	return stopFn, nil
}

// ftpFileDriver implements goftp.io/server/v2 Driver for read-only file serving.
type ftpFileDriver struct {
	root string
}

func (d *ftpFileDriver) Stat(ctx *ftpserver.Context, path string) (os.FileInfo, error) {
	fullPath := d.resolve(path)
	return os.Stat(fullPath)
}

func (d *ftpFileDriver) ListDir(ctx *ftpserver.Context, path string, callback func(os.FileInfo) error) error {
	fullPath := d.resolve(path)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if err := callback(info); err != nil {
			return err
		}
	}
	return nil
}

func (d *ftpFileDriver) GetFile(ctx *ftpserver.Context, path string, offset int64) (int64, io.ReadCloser, error) {
	fullPath := d.resolve(path)
	f, err := os.Open(fullPath)
	if err != nil {
		return 0, nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return 0, nil, err
	}
	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			f.Close()
			return 0, nil, err
		}
	}
	return info.Size() - offset, f, nil
}

func (d *ftpFileDriver) PutFile(ctx *ftpserver.Context, path string, r io.Reader, offset int64) (int64, error) {
	return 0, fmt.Errorf("read-only filesystem")
}

func (d *ftpFileDriver) DeleteDir(ctx *ftpserver.Context, path string) error {
	return fmt.Errorf("read-only filesystem")
}

func (d *ftpFileDriver) DeleteFile(ctx *ftpserver.Context, path string) error {
	return fmt.Errorf("read-only filesystem")
}

func (d *ftpFileDriver) Rename(ctx *ftpserver.Context, fromPath string, toPath string) error {
	return fmt.Errorf("read-only filesystem")
}

func (d *ftpFileDriver) MakeDir(ctx *ftpserver.Context, path string) error {
	return fmt.Errorf("read-only filesystem")
}

func (d *ftpFileDriver) resolve(path string) string {
	clean := filepath.FromSlash(filepath.Clean("/" + path))
	clean = strings.TrimPrefix(clean, string(filepath.Separator))
	if clean == "." || clean == "" {
		return d.root
	}
	result := filepath.Join(d.root, clean)
	// Prevent path traversal
	if !strings.HasPrefix(result, d.root) {
		return d.root
	}
	return result
}

// ftpAnonymousAuth accepts any login credentials (anonymous FTP).
type ftpAnonymousAuth struct{}

func (a *ftpAnonymousAuth) CheckPasswd(ctx *ftpserver.Context, user, pass string) (bool, error) {
	return true, nil
}

// ftpLogger adapts slog to the goftp logger interface.
type ftpLogger struct {
	m            *Manager
	deploymentID string
}

func (l *ftpLogger) Print(sessionID string, message interface{}) {
	l.m.logger.Debug("ftp", "deployment", l.deploymentID, "session", sessionID, "msg", message)
}

func (l *ftpLogger) Printf(sessionID string, format string, v ...interface{}) {
	l.m.logger.Debug("ftp", "deployment", l.deploymentID, "session", sessionID, "msg", fmt.Sprintf(format, v...))
}

func (l *ftpLogger) PrintCommand(sessionID string, command string, params string) {
	l.m.logger.Debug("ftp command", "deployment", l.deploymentID, "session", sessionID, "command", command, "params", params)
}

func (l *ftpLogger) PrintResponse(sessionID string, code int, message string) {
	l.m.logger.Debug("ftp response", "deployment", l.deploymentID, "session", sessionID, "code", code, "msg", message)
}


