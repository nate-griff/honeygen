package deployments

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/natet/honeygen/backend/internal/events"
	ftpserver "goftp.io/server/v2"
)

type recordingEventRecorder struct {
	payloads []events.IngestRequest
}

func (r *recordingEventRecorder) Record(_ context.Context, payload events.IngestRequest) error {
	r.payloads = append(r.payloads, payload)
	return nil
}

func TestDeploymentEventPathUsesCanonicalGeneratedPath(t *testing.T) {
	deployment := Deployment{
		WorldModelID:    "world-1",
		GenerationJobID: "job-1",
		RootPath:        "/shared",
	}

	got := deploymentEventPath(deployment, "/finance/budget.xlsx")
	want := "/generated/world-1/job-1/shared/finance/budget.xlsx"
	if got != want {
		t.Fatalf("deploymentEventPath() = %q, want %q", got, want)
	}
}

func TestTelemetryFTPDriverGetFileRecordsDownloadEvent(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "about.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	recorder := &recordingEventRecorder{}
	driver := &telemetryFTPDriver{
		driver: &ftpFileDriver{root: root},
		deployment: Deployment{
			ID:              "deployment-1",
			Protocol:        "ftp",
			WorldModelID:    "world-1",
			GenerationJobID: "job-1",
			RootPath:        "/",
		},
		recorder: recorder,
	}

	size, reader, err := driver.GetFile(&ftpserver.Context{Cmd: "RETR", Param: "/about.txt"}, "/about.txt", 0)
	if err != nil {
		t.Fatalf("GetFile() error = %v", err)
	}
	defer reader.Close()

	if size != 5 {
		t.Fatalf("GetFile() size = %d, want %d", size, 5)
	}
	if len(recorder.payloads) != 1 {
		t.Fatalf("len(payloads) = %d, want 1", len(recorder.payloads))
	}

	got := recorder.payloads[0]
	if got.EventType != "ftp_download" {
		t.Fatalf("payload.EventType = %q, want %q", got.EventType, "ftp_download")
	}
	if got.Method != "RETR" {
		t.Fatalf("payload.Method = %q, want %q", got.Method, "RETR")
	}
	if got.Path != "/generated/world-1/job-1/about.txt" {
		t.Fatalf("payload.Path = %q, want %q", got.Path, "/generated/world-1/job-1/about.txt")
	}
}

func TestTelemetryFilesystemOpenRecordsNFSReadEvent(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "public"), 0o755); err != nil {
		t.Fatalf("os.Mkdir() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "public", "about.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	recorder := &recordingEventRecorder{}
	fs := newTelemetryFilesystem(osfs.New(root), Deployment{
		ID:              "deployment-2",
		Protocol:        "nfs",
		WorldModelID:    "world-1",
		GenerationJobID: "job-1",
		RootPath:        "/",
	}, recorder, "10.0.0.9")

	file, err := fs.Open(filepath.Join("public", "about.txt"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	_ = file.Close()

	if len(recorder.payloads) != 1 {
		t.Fatalf("len(payloads) = %d, want 1", len(recorder.payloads))
	}

	got := recorder.payloads[0]
	if got.EventType != "nfs_read" {
		t.Fatalf("payload.EventType = %q, want %q", got.EventType, "nfs_read")
	}
	if got.Method != "READ" {
		t.Fatalf("payload.Method = %q, want %q", got.Method, "READ")
	}
	if got.SourceIP != "10.0.0.9" {
		t.Fatalf("payload.SourceIP = %q, want %q", got.SourceIP, "10.0.0.9")
	}
	if got.Path != "/generated/world-1/job-1/public/about.txt" {
		t.Fatalf("payload.Path = %q, want %q", got.Path, "/generated/world-1/job-1/public/about.txt")
	}
}

func TestParseSMBAuditLineBuildsAccessEvent(t *testing.T) {
	deployment := Deployment{
		ID:              "deployment-3",
		Protocol:        "smb",
		WorldModelID:    "world-1",
		GenerationJobID: "job-1",
		RootPath:        "/shared",
	}

	payload, ok := parseSMBAuditLine(deployment, "smbd_audit: deployment-3|192.0.2.44|open|ok|finance/budget.xlsx")
	if !ok {
		t.Fatalf("parseSMBAuditLine() ok = false, want true")
	}
	if payload.EventType != "smb_open" {
		t.Fatalf("payload.EventType = %q, want %q", payload.EventType, "smb_open")
	}
	if payload.Method != "OPEN" {
		t.Fatalf("payload.Method = %q, want %q", payload.Method, "OPEN")
	}
	if payload.SourceIP != "192.0.2.44" {
		t.Fatalf("payload.SourceIP = %q, want %q", payload.SourceIP, "192.0.2.44")
	}
	if payload.Path != "/generated/world-1/job-1/shared/finance/budget.xlsx" {
		t.Fatalf("payload.Path = %q, want %q", payload.Path, "/generated/world-1/job-1/shared/finance/budget.xlsx")
	}
}
