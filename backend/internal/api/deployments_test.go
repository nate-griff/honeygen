package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/natet/honeygen/backend/internal/deployments"
	"github.com/natet/honeygen/backend/internal/models"
)

func TestDeploymentsCreateAcceptsSMBProtocol(t *testing.T) {
	application := newTestAPIApp(t)
	router := NewRouter(application)
	handler := deploymentsCreateHandler(application)
	job, err := application.JobStore.Create(context.Background(), "northbridge-financial")
	if err != nil {
		t.Fatalf("JobStore.Create() error = %v", err)
	}

	req := authenticatedRequest(t, router, http.MethodPost, "/api/deployments", strings.NewReader(`{
		"generation_job_id": "`+job.ID+`",
		"world_model_id": "northbridge-financial",
		"protocol": "smb",
		"port": 9001,
		"root_path": "/shared"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

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
	if deployment.Port != 9001 {
		t.Fatalf("port = %d, want %d", deployment.Port, 9001)
	}
	if deployment.RootPath != "/shared" {
		t.Fatalf("root_path = %q, want %q", deployment.RootPath, "/shared")
	}
}

func TestDeploymentsListIncludesConnectionDetails(t *testing.T) {
	application := newTestAPIApp(t)
	router := NewRouter(application)
	handler := deploymentsListHandler(application)
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

	req := authenticatedRequest(t, router, http.MethodGet, "/api/deployments", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

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

func TestDeploymentsCreateRejectsOversizedJSONBody(t *testing.T) {
	application := newTestAPIApp(t)
	router := NewRouter(application)
	handler := deploymentsCreateHandler(application)
	job, err := application.JobStore.Create(context.Background(), "northbridge-financial")
	if err != nil {
		t.Fatalf("JobStore.Create() error = %v", err)
	}

	var body bytes.Buffer
	body.WriteString(`{"generation_job_id":"`)
	body.WriteString(job.ID)
	body.WriteString(`","world_model_id":"northbridge-financial","protocol":"http","port":9000,"root_path":"/","padding":"`)
	body.Write(bytes.Repeat([]byte("a"), maxJSONBodyBytes))
	body.WriteString(`"}`)

	req := authenticatedRequest(t, router, http.MethodPost, "/api/deployments", &body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assertAPIErrorResponse(t, rec, http.StatusBadRequest, "", models.APIErrorResponse{
		Error: models.APIError{
			Code:    "validation_error",
			Message: "request body must be 1 MiB or smaller",
		},
	})
}

func TestDeploymentsCreateRejectsPortsOutsideDeploymentRange(t *testing.T) {
	application := newTestAPIApp(t)
	router := NewRouter(application)
	handler := deploymentsCreateHandler(application)
	job, err := application.JobStore.Create(context.Background(), "northbridge-financial")
	if err != nil {
		t.Fatalf("JobStore.Create() error = %v", err)
	}

	req := authenticatedRequest(t, router, http.MethodPost, "/api/deployments", strings.NewReader(`{
		"generation_job_id": "`+job.ID+`",
		"world_model_id": "northbridge-financial",
		"protocol": "http",
		"port": 8999,
		"root_path": "/"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assertAPIErrorResponse(t, rec, http.StatusBadRequest, "", models.APIErrorResponse{
		Error: models.APIError{
			Code:    "validation_error",
			Message: "port must be between 9000 and 9009",
		},
	})
}

func TestDeploymentsCreateRejectsPortsAtReservedBoundary(t *testing.T) {
	application := newTestAPIApp(t)
	router := NewRouter(application)
	handler := deploymentsCreateHandler(application)
	job, err := application.JobStore.Create(context.Background(), "northbridge-financial")
	if err != nil {
		t.Fatalf("JobStore.Create() error = %v", err)
	}

	req := authenticatedRequest(t, router, http.MethodPost, "/api/deployments", strings.NewReader(`{
		"generation_job_id": "`+job.ID+`",
		"world_model_id": "northbridge-financial",
		"protocol": "ftp",
		"port": 9010,
		"root_path": "/shared"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assertAPIErrorResponse(t, rec, http.StatusBadRequest, "", models.APIErrorResponse{
		Error: models.APIError{
			Code:    "validation_error",
			Message: "port must be between 9000 and 9009",
		},
	})
}

func TestDeploymentsCreateRejectsPortConflictsWithExistingDeployment(t *testing.T) {
	application := newTestAPIApp(t)
	router := NewRouter(application)
	handler := deploymentsCreateHandler(application)
	job, err := application.JobStore.Create(context.Background(), "northbridge-financial")
	if err != nil {
		t.Fatalf("JobStore.Create() error = %v", err)
	}

	_, err = application.DeploymentRepo.Create(context.Background(), deployments.Deployment{
		GenerationJobID: job.ID,
		WorldModelID:    "northbridge-financial",
		Protocol:        "http",
		Port:            9001,
		RootPath:        "/",
	})
	if err != nil {
		t.Fatalf("DeploymentRepo.Create() error = %v", err)
	}

	req := authenticatedRequest(t, router, http.MethodPost, "/api/deployments", strings.NewReader(`{
		"generation_job_id": "`+job.ID+`",
		"world_model_id": "northbridge-financial",
		"protocol": "ftp",
		"port": 9001,
		"root_path": "/shared"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assertAPIErrorResponse(t, rec, http.StatusConflict, "", models.APIErrorResponse{
		Error: models.APIError{
			Code:    "port_conflict",
			Message: "port 9001 is already assigned to another deployment",
		},
	})
}

func TestDeploymentsCreateReturnsConflictWhenPortBecomesDuplicateAtInsertTime(t *testing.T) {
	application := newTestAPIApp(t)
	router := NewRouter(application)
	handler := deploymentsCreateHandler(application)
	job, err := application.JobStore.Create(context.Background(), "northbridge-financial")
	if err != nil {
		t.Fatalf("JobStore.Create() error = %v", err)
	}

	_, err = application.DB.ExecContext(context.Background(), `
		CREATE TRIGGER deployments_simulate_port_race
		BEFORE INSERT ON deployments
		WHEN NEW.port = 9002 AND NOT EXISTS (SELECT 1 FROM deployments WHERE id = 'race-insert')
		BEGIN
			INSERT INTO deployments (id, generation_job_id, world_model_id, protocol, port, root_path, status)
			VALUES ('race-insert', NEW.generation_job_id, NEW.world_model_id, 'http', NEW.port, '/', 'stopped');
		END;
	`)
	if err != nil {
		t.Fatalf("create trigger error = %v", err)
	}

	req := authenticatedRequest(t, router, http.MethodPost, "/api/deployments", strings.NewReader(`{
		"generation_job_id": "`+job.ID+`",
		"world_model_id": "northbridge-financial",
		"protocol": "ftp",
		"port": 9002,
		"root_path": "/shared"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assertAPIErrorResponse(t, rec, http.StatusConflict, "", models.APIErrorResponse{
		Error: models.APIError{
			Code:    "port_conflict",
			Message: "port 9002 is already assigned to another deployment",
		},
	})
}
