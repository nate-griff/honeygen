package deployments

import (
	"context"
	"io"
	"log/slog"
	"net"
	"os"
	"path"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/natet/honeygen/backend/internal/events"
	nfs "github.com/willscott/go-nfs"
	ftpserver "goftp.io/server/v2"
)

func deploymentEventPath(d Deployment, requestedPath string) string {
	base := path.Clean("/generated/" + d.WorldModelID + "/" + d.GenerationJobID)
	root := normalizeDeploymentRootPath(d)
	if root != "/" {
		base = path.Join(base, strings.TrimPrefix(root, "/"))
	}

	clean := path.Clean("/" + strings.TrimPrefix(strings.TrimSpace(strings.ReplaceAll(requestedPath, "\\", "/")), "/"))
	if clean == "/" || clean == "." {
		return base
	}

	return path.Join(base, strings.TrimPrefix(clean, "/"))
}

func emitProtocolEvent(recorder events.Recorder, logger *slog.Logger, payload events.IngestRequest) {
	if recorder == nil {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := recorder.Record(ctx, payload); err != nil {
		logger.Error("record protocol event", "error", err, "event_type", payload.EventType, "path", payload.Path)
	}
}

func newProtocolEvent(d Deployment, eventType, method, requestedPath, sourceIP string, bytesSent int, metadata map[string]any) events.IngestRequest {
	mergedMetadata := map[string]any{
		"deployment_id": d.ID,
		"world_model":   d.WorldModelID,
		"protocol":      d.Protocol,
	}
	for key, value := range metadata {
		mergedMetadata[key] = value
	}

	return events.IngestRequest{
		Timestamp: time.Now().UTC(),
		EventType: strings.TrimSpace(eventType),
		Method:    strings.TrimSpace(method),
		Path:      deploymentEventPath(d, requestedPath),
		SourceIP:  strings.TrimSpace(sourceIP),
		BytesSent: max(0, bytesSent),
		Metadata:  mergedMetadata,
	}
}

func sourceIPFromAddr(addr net.Addr) string {
	if addr == nil {
		return ""
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(addr.String()))
	if err != nil {
		host = strings.TrimSpace(addr.String())
	}
	return host
}

type telemetryFTPDriver struct {
	driver     ftpserver.Driver
	deployment Deployment
	recorder   events.Recorder
	logger     *slog.Logger
}

func (d *telemetryFTPDriver) Stat(ctx *ftpserver.Context, name string) (os.FileInfo, error) {
	return d.driver.Stat(ctx, name)
}

func (d *telemetryFTPDriver) ListDir(ctx *ftpserver.Context, name string, callback func(os.FileInfo) error) error {
	if err := d.driver.ListDir(ctx, name, callback); err != nil {
		return err
	}

	method := "LIST"
	if ctx != nil && strings.TrimSpace(ctx.Cmd) != "" {
		method = strings.ToUpper(strings.TrimSpace(ctx.Cmd))
	}
	emitProtocolEvent(d.recorder, d.logger, newProtocolEvent(d.deployment, "ftp_list", method, name, sourceIPFromFTPCtx(ctx), 0, map[string]any{
		"operation": "list",
	}))
	return nil
}

func (d *telemetryFTPDriver) DeleteDir(ctx *ftpserver.Context, name string) error {
	return d.driver.DeleteDir(ctx, name)
}

func (d *telemetryFTPDriver) DeleteFile(ctx *ftpserver.Context, name string) error {
	return d.driver.DeleteFile(ctx, name)
}

func (d *telemetryFTPDriver) Rename(ctx *ftpserver.Context, fromPath string, toPath string) error {
	return d.driver.Rename(ctx, fromPath, toPath)
}

func (d *telemetryFTPDriver) MakeDir(ctx *ftpserver.Context, name string) error {
	return d.driver.MakeDir(ctx, name)
}

func (d *telemetryFTPDriver) GetFile(ctx *ftpserver.Context, name string, offset int64) (int64, io.ReadCloser, error) {
	size, reader, err := d.driver.GetFile(ctx, name, offset)
	if err != nil {
		return 0, nil, err
	}

	method := "RETR"
	if ctx != nil && strings.TrimSpace(ctx.Cmd) != "" {
		method = strings.ToUpper(strings.TrimSpace(ctx.Cmd))
	}
	emitProtocolEvent(d.recorder, d.logger, newProtocolEvent(d.deployment, "ftp_download", method, name, sourceIPFromFTPCtx(ctx), int(size), map[string]any{
		"operation": "download",
	}))
	return size, reader, nil
}

func (d *telemetryFTPDriver) PutFile(ctx *ftpserver.Context, destPath string, data io.Reader, offset int64) (int64, error) {
	return d.driver.PutFile(ctx, destPath, data, offset)
}

func sourceIPFromFTPCtx(ctx *ftpserver.Context) string {
	if ctx == nil || ctx.Sess == nil {
		return ""
	}
	return sourceIPFromAddr(ctx.Sess.RemoteAddr())
}

type telemetryFilesystem struct {
	billy.Filesystem
	deployment Deployment
	recorder   events.Recorder
	logger     *slog.Logger
	sourceIP   string
}

func newTelemetryFilesystem(fs billy.Filesystem, deployment Deployment, recorder events.Recorder, sourceIP string) *telemetryFilesystem {
	return &telemetryFilesystem{
		Filesystem: fs,
		deployment: deployment,
		recorder:   recorder,
		sourceIP:   sourceIP,
	}
}

func (fs *telemetryFilesystem) Open(filename string) (billy.File, error) {
	file, err := fs.Filesystem.Open(filename)
	if err != nil {
		return nil, err
	}
	fs.recordRead(filename)
	return file, nil
}

func (fs *telemetryFilesystem) OpenFile(filename string, flag int, perm os.FileMode) (billy.File, error) {
	file, err := fs.Filesystem.OpenFile(filename, flag, perm)
	if err != nil {
		return nil, err
	}
	if flag&(os.O_WRONLY|os.O_RDWR) == 0 {
		fs.recordRead(filename)
	}
	return file, nil
}

func (fs *telemetryFilesystem) ReadDir(dirname string) ([]os.FileInfo, error) {
	entries, err := fs.Filesystem.ReadDir(dirname)
	if err != nil {
		return nil, err
	}

	emitProtocolEvent(fs.recorder, fs.logger, newProtocolEvent(fs.deployment, "nfs_list", "READDIR", dirname, fs.sourceIP, 0, map[string]any{
		"operation": "list",
		"entries":   len(entries),
	}))
	return entries, nil
}

func (fs *telemetryFilesystem) Chroot(dirname string) (billy.Filesystem, error) {
	child, err := fs.Filesystem.Chroot(dirname)
	if err != nil {
		return nil, err
	}
	return newTelemetryFilesystem(child, fs.deployment, fs.recorder, fs.sourceIP), nil
}

func (fs *telemetryFilesystem) recordRead(filename string) {
	size := 0
	if info, err := fs.Filesystem.Stat(filename); err == nil {
		size = int(info.Size())
	}

	emitProtocolEvent(fs.recorder, fs.logger, newProtocolEvent(fs.deployment, "nfs_read", "READ", filename, fs.sourceIP, size, map[string]any{
		"operation": "read",
	}))
}

type telemetryNFSHandler struct {
	fs         billy.Filesystem
	deployment Deployment
	recorder   events.Recorder
	logger     *slog.Logger
}

func newTelemetryNFSHandler(fs billy.Filesystem, deployment Deployment, recorder events.Recorder, logger *slog.Logger) nfs.Handler {
	return &telemetryNFSHandler{
		fs:         fs,
		deployment: deployment,
		recorder:   recorder,
		logger:     logger,
	}
}

func (h *telemetryNFSHandler) Mount(_ context.Context, conn net.Conn, req nfs.MountRequest) (nfs.MountStatus, billy.Filesystem, []nfs.AuthFlavor) {
	sourceIP := sourceIPFromAddr(conn.RemoteAddr())
	emitProtocolEvent(h.recorder, h.logger, newProtocolEvent(h.deployment, "nfs_mount", "MOUNT", "/", sourceIP, 0, map[string]any{
		"operation": "mount",
		"export":    strings.TrimSpace(string(req.Dirpath)),
	}))
	return nfs.MountStatusOk, newTelemetryFilesystem(h.fs, h.deployment, h.recorder, sourceIP), []nfs.AuthFlavor{nfs.AuthFlavorNull}
}

func (h *telemetryNFSHandler) Change(fs billy.Filesystem) billy.Change {
	if changer, ok := fs.(billy.Change); ok {
		return changer
	}
	return nil
}

func (h *telemetryNFSHandler) FSStat(_ context.Context, _ billy.Filesystem, _ *nfs.FSStat) error {
	return nil
}

func (h *telemetryNFSHandler) ToHandle(billy.Filesystem, []string) []byte {
	return nil
}

func (h *telemetryNFSHandler) FromHandle([]byte) (billy.Filesystem, []string, error) {
	return nil, nil, nil
}

func (h *telemetryNFSHandler) InvalidateHandle(billy.Filesystem, []byte) error {
	return nil
}

func (h *telemetryNFSHandler) HandleLimit() int {
	return -1
}

func parseSMBAuditLine(deployment Deployment, line string) (events.IngestRequest, bool) {
	const marker = "smbd_audit:"
	index := strings.Index(line, marker)
	if index < 0 {
		return events.IngestRequest{}, false
	}

	fields := strings.Split(strings.TrimSpace(line[index+len(marker):]), "|")
	if len(fields) != 5 {
		return events.IngestRequest{}, false
	}
	if strings.TrimSpace(fields[0]) != deployment.ID {
		return events.IngestRequest{}, false
	}
	if !strings.EqualFold(strings.TrimSpace(fields[3]), "ok") {
		return events.IngestRequest{}, false
	}

	operation := strings.ToLower(strings.TrimSpace(fields[2]))
	eventType := "smb_" + operation
	method := strings.ToUpper(operation)

	return newProtocolEvent(deployment, eventType, method, fields[4], fields[1], 0, map[string]any{
		"operation": operation,
		"result":    strings.TrimSpace(fields[3]),
	}), true
}
