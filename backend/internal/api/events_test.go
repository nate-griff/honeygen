package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/natet/honeygen/backend/internal/assets"
	"github.com/natet/honeygen/backend/internal/events"
	"github.com/natet/honeygen/backend/internal/models"
)

func TestInternalEventIngestionIsVisibleThroughPublicEventsAPI(t *testing.T) {
	application := newTestAPIApp(t)
	seedGenerationJob(t, application, "world-1", "job-1")

	if _, err := application.AssetRepo.Create(context.Background(), assets.Asset{
		ID:              "asset-1",
		GenerationJobID: "job-1",
		WorldModelID:    "world-1",
		SourceType:      "generated",
		RenderedType:    "html",
		Path:            "generated/world-1/job-1/public/report.html",
		MIMEType:        "text/html",
		SizeBytes:       42,
		Previewable:     true,
		Checksum:        "sum-1",
	}); err != nil {
		t.Fatalf("AssetRepo.Create() error = %v", err)
	}

	body := bytes.NewBufferString(`{
		"timestamp":"2024-06-01T12:00:00Z",
		"method":"GET",
		"path":"/generated/world-1/job-1/public/report.html",
		"query":"download=1",
		"source_ip":"203.0.113.10",
		"user_agent":"integration-test",
		"referer":"https://example.test/",
		"status_code":200,
		"bytes_sent":512
	}`)

	createReq := httptest.NewRequest(http.MethodPost, "/internal/events", body)
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set(events.InternalIngestTokenHeader, application.Config.InternalEventIngestToken)
	createRec := httptest.NewRecorder()

	NewRouter(application).ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("POST /internal/events status = %d, want %d, body=%s", createRec.Code, http.StatusCreated, createRec.Body.String())
	}

	var created events.Event
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("json.Unmarshal(created) error = %v", err)
	}
	if created.AssetID != "asset-1" {
		t.Fatalf("created.AssetID = %q, want %q", created.AssetID, "asset-1")
	}
	if created.WorldModelID != "world-1" {
		t.Fatalf("created.WorldModelID = %q, want %q", created.WorldModelID, "world-1")
	}
	if created.Path != "/generated/world-1/job-1/public/report.html" {
		t.Fatalf("created.Path = %q, want %q", created.Path, "/generated/world-1/job-1/public/report.html")
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/events?limit=1&offset=0&world_model_id=world-1&path=%2Fgenerated%2Fworld-1&source_ip=203.0.113.10&status_code=200", nil)
	listRec := httptest.NewRecorder()

	NewRouter(application).ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("GET /api/events status = %d, want %d, body=%s", listRec.Code, http.StatusOK, listRec.Body.String())
	}

	var listResponse struct {
		Items []events.Event `json:"items"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listResponse); err != nil {
		t.Fatalf("json.Unmarshal(list) error = %v", err)
	}
	if len(listResponse.Items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(listResponse.Items))
	}
	if listResponse.Items[0].ID != created.ID {
		t.Fatalf("list item ID = %q, want %q", listResponse.Items[0].ID, created.ID)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/events/"+created.ID, nil)
	getRec := httptest.NewRecorder()

	NewRouter(application).ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("GET /api/events/:id status = %d, want %d, body=%s", getRec.Code, http.StatusOK, getRec.Body.String())
	}

	var got events.Event
	if err := json.Unmarshal(getRec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal(detail) error = %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("detail ID = %q, want %q", got.ID, created.ID)
	}
	if got.BytesSent != 512 {
		t.Fatalf("detail.BytesSent = %d, want %d", got.BytesSent, 512)
	}
}

func TestInternalEventIngestionRejectsUnauthorizedRequests(t *testing.T) {
	application := newTestAPIApp(t)

	testCases := []struct {
		name        string
		token       string
		wantStatus  int
		wantMessage string
	}{
		{
			name:        "missing token",
			wantStatus:  http.StatusUnauthorized,
			wantMessage: "internal event token is invalid",
		},
		{
			name:        "invalid token",
			token:       "wrong-token",
			wantStatus:  http.StatusUnauthorized,
			wantMessage: "internal event token is invalid",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/internal/events", bytes.NewBufferString(`{
				"timestamp":"2024-06-01T12:00:00Z",
				"method":"GET",
				"path":"/generated/world-1/job-1/public/report.html",
				"source_ip":"203.0.113.10",
				"user_agent":"integration-test",
				"status_code":200,
				"bytes_sent":512
			}`))
			req.Header.Set("Content-Type", "application/json")
			if tc.token != "" {
				req.Header.Set(events.InternalIngestTokenHeader, tc.token)
			}

			rec := httptest.NewRecorder()
			NewRouter(application).ServeHTTP(rec, req)

			assertAPIErrorResponse(t, rec, tc.wantStatus, "", models.APIErrorResponse{
				Error: models.APIError{
					Code:    "unauthorized",
					Message: tc.wantMessage,
				},
			})
		})
	}
}
