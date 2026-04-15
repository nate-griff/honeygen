package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/natet/honeygen/backend/internal/deployments"
)

func TestDeploymentsCreateAcceptsSMBProtocol(t *testing.T) {
	application := newTestAPIApp(t)
	job, err := application.JobStore.Create(context.Background(), "northbridge-financial")
	if err != nil {
		t.Fatalf("JobStore.Create() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/deployments", strings.NewReader(`{
		"generation_job_id": "`+job.ID+`",
		"world_model_id": "northbridge-financial",
		"protocol": "smb",
		"port": 1445,
		"root_path": "/shared"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	NewRouter(application).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status code = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var deployment struct {
		Protocol string `json:"protocol"`
		Port     int    `json:"port"`
		RootPath string `json:"root_path"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &deployment); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if deployment.Protocol != "smb" {
		t.Fatalf("protocol = %q, want %q", deployment.Protocol, "smb")
	}
	if deployment.Port != 1445 {
		t.Fatalf("port = %d, want %d", deployment.Port, 1445)
	}
	if deployment.RootPath != "/shared" {
		t.Fatalf("root_path = %q, want %q", deployment.RootPath, "/shared")
	}
}

func TestDeploymentsListIncludesConnectionDetails(t *testing.T) {
	application := newTestAPIApp(t)
	job, err := application.JobStore.Create(context.Background(), "northbridge-financial")
	if err != nil {
		t.Fatalf("JobStore.Create() error = %v", err)
	}

	for _, deployment := range []struct {
		protocol string
		port     int
	}{
		{protocol: "smb", port: 9001},
		{protocol: "nfs", port: 9003},
	} {
		_, err := application.DeploymentRepo.Create(context.Background(), deployments.Deployment{
			GenerationJobID: job.ID,
			WorldModelID:    "northbridge-financial",
			Protocol:        deployment.protocol,
			Port:            deployment.port,
			RootPath:        "/",
		})
		if err != nil {
			t.Fatalf("DeploymentRepo.Create(%q) error = %v", deployment.protocol, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/deployments", nil)
	rec := httptest.NewRecorder()

	NewRouter(application).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var response struct {
		Items []struct {
			Protocol  string `json:"protocol"`
			ShareName string `json:"share_name"`
			MountPath string `json:"mount_path"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	var (
		smbShareName string
		nfsMountPath string
	)
	for _, item := range response.Items {
		switch item.Protocol {
		case "smb":
			smbShareName = item.ShareName
		case "nfs":
			nfsMountPath = item.MountPath
		}
	}

	if smbShareName != "honeygen" {
		t.Fatalf("smb share_name = %q, want %q", smbShareName, "honeygen")
	}
	if nfsMountPath != "/mount" {
		t.Fatalf("nfs mount_path = %q, want %q", nfsMountPath, "/mount")
	}
}
