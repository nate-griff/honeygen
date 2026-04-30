package deployments

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/natet/honeygen/backend/internal/events"
)

const smbShareName = "honeygen"

func (m *Manager) startSMB(d Deployment, filePath string, srvCtx context.Context, cancel context.CancelFunc) (func(), error) {
	smbdPath, err := findSMBDPath()
	if err != nil {
		return nil, err
	}

	runDir, err := os.MkdirTemp("", "honeygen-smb-"+d.ID+"-")
	if err != nil {
		return nil, fmt.Errorf("create smb runtime dir: %w", err)
	}

	for _, dir := range []string{
		filepath.Join(runDir, "cache"),
		filepath.Join(runDir, "lock"),
		filepath.Join(runDir, "ncalrpc"),
		filepath.Join(runDir, "private"),
		filepath.Join(runDir, "state"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			_ = os.RemoveAll(runDir)
			return nil, fmt.Errorf("create smb runtime directory %q: %w", dir, err)
		}
	}

	configPath := filepath.Join(runDir, "smb.conf")
	if err := os.WriteFile(configPath, []byte(buildSMBConfig(d, filePath, runDir)), 0o600); err != nil {
		_ = os.RemoveAll(runDir)
		return nil, fmt.Errorf("write smb config: %w", err)
	}

	cmd := exec.CommandContext(
		srvCtx,
		smbdPath,
		"--foreground",
		"--no-process-group",
		"--debug-stdout",
		"--configfile="+configPath,
		"-p",
		strconv.Itoa(d.Port),
	)
	cmd.Stdout = &smbLogWriter{logger: m.logger, deployment: d, recorder: m.recorder, stream: "stdout"}
	cmd.Stderr = &smbLogWriter{logger: m.logger, deployment: d, recorder: m.recorder, stream: "stderr"}

	if err := cmd.Start(); err != nil {
		_ = os.RemoveAll(runDir)
		return nil, fmt.Errorf("start smbd: %w", err)
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
		cancel()
	}()

	select {
	case err := <-waitCh:
		_ = os.RemoveAll(runDir)
		if err != nil {
			return nil, fmt.Errorf("smbd exited during startup: %w", err)
		}
		return nil, errors.New("smbd exited during startup")
	case <-time.After(750 * time.Millisecond):
	}

	m.logger.Info("smb deployment started", "id", d.ID, "port", d.Port, "world_model", d.WorldModelID, "root_path", d.RootPath)

	stopFn := func() {
		cancel()

		select {
		case <-waitCh:
		case <-time.After(5 * time.Second):
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
				<-waitCh
			}
		}

		if err := os.RemoveAll(runDir); err != nil {
			m.logger.Warn("remove smb runtime dir", "id", d.ID, "dir", runDir, "error", err)
		}
	}

	return stopFn, nil
}

func findSMBDPath() (string, error) {
	if path, err := exec.LookPath("smbd"); err == nil {
		return path, nil
	}

	const fallback = "/usr/sbin/smbd"
	if _, err := os.Stat(fallback); err == nil {
		return fallback, nil
	}

	return "", errors.New("smbd is not installed in the runtime image")
}

func buildSMBConfig(d Deployment, sharePath, runDir string) string {
	sharePath = filepath.ToSlash(sharePath)
	runDir = filepath.ToSlash(runDir)

	return fmt.Sprintf(`[global]
	server role = standalone server
	workgroup = WORKGROUP
	netbios name = HONEYGEN
	map to guest = Bad User
	guest account = nobody
	server min protocol = SMB2
	smb ports = %d
	vfs objects = full_audit
	full_audit:prefix = %s|%%I
	full_audit:success = all
	full_audit:failure = none
	full_audit:syslog = false
	load printers = no
	printing = bsd
	printcap name = /dev/null
	disable spoolss = yes
	log file = %s/log.%%m
	max log size = 1000
	lock directory = %s/lock
	state directory = %s/state
	cache directory = %s/cache
	ncalrpc dir = %s/ncalrpc
	private dir = %s/private
	pid directory = %s/state

[%s]
	path = %s
	read only = yes
	guest ok = yes
	browseable = yes
`, d.Port, d.ID, runDir, runDir, runDir, runDir, runDir, runDir, runDir, smbShareName, sharePath)
}

type smbLogWriter struct {
	logger     *slog.Logger
	deployment Deployment
	recorder   events.Recorder
	stream     string
	mu         sync.Mutex
	pending    string
}

func (w *smbLogWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.pending += string(data)
	lines := strings.Split(w.pending, "\n")
	w.pending = lines[len(lines)-1]

	for _, line := range lines[:len(lines)-1] {
		w.processLine(line)
	}
	return len(data), nil
}

func (w *smbLogWriter) processLine(line string) {
	message := strings.TrimSpace(line)
	if message == "" {
		return
	}

	w.logger.Info("smbd log", "deployment", w.deployment.ID, "stream", w.stream, "message", message)
	if payload, ok := parseSMBAuditLine(w.deployment, message); ok {
		emitProtocolEvent(w.recorder, w.logger, payload)
	}
}

var _ io.Writer = (*smbLogWriter)(nil)
