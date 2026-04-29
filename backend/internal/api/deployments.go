package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/natet/honeygen/backend/internal/app"
	"github.com/natet/honeygen/backend/internal/deployments"
)

const (
	deploymentPortMin = 9000
	deploymentPortMax = 9009
)

type createDeploymentRequest struct {
	GenerationJobID string `json:"generation_job_id"`
	WorldModelID    string `json:"world_model_id"`
	Protocol        string `json:"protocol"`
	Port            int    `json:"port"`
	RootPath        string `json:"root_path"`
}

func deploymentsCollectionHandler(application *app.APIApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			deploymentsListHandler(application)(w, r)
		case http.MethodPost:
			deploymentsCreateHandler(application)(w, r)
		default:
			w.Header().Set("Allow", "GET, POST")
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func deploymentsListHandler(application *app.APIApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := application.DeploymentRepo.List(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to list deployments")
			return
		}

		type response struct {
			Items   []deployments.Deployment `json:"items"`
			Running map[string]bool          `json:"running"`
		}

		running := make(map[string]bool, len(items))
		for _, d := range items {
			running[d.ID] = application.DeploymentManager.IsRunning(d.ID)
		}

		writeJSON(w, http.StatusOK, response{Items: items, Running: running})
	}
}

func deploymentsCreateHandler(application *app.APIApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := readJSONBody(w, r)
		if err != nil {
			writeValidationFailure(w, err)
			return
		}

		var req createDeploymentRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeValidationFailure(w, requestValidationError{message: "request body must be a JSON object"})
			return
		}

		if strings.TrimSpace(req.GenerationJobID) == "" {
			writeError(w, http.StatusBadRequest, "validation_error", "generation_job_id is required")
			return
		}
		if strings.TrimSpace(req.WorldModelID) == "" {
			writeError(w, http.StatusBadRequest, "validation_error", "world_model_id is required")
			return
		}
		if err := validateDeploymentPort(req.Port); err != nil {
			writeValidationFailure(w, err)
			return
		}
		if req.Protocol == "" {
			req.Protocol = "http"
		}
		validProtocols := map[string]bool{"http": true, "ftp": true, "nfs": true, "smb": true}
		if !validProtocols[req.Protocol] {
			writeError(w, http.StatusBadRequest, "validation_error", "protocol must be \"http\", \"ftp\", \"nfs\", or \"smb\"")
			return
		}
		if req.RootPath == "" {
			req.RootPath = "/"
		}
		portInUse, err := application.DeploymentRepo.ExistsPort(r.Context(), req.Port)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to validate deployment port availability")
			return
		}
		if portInUse {
			writeError(w, http.StatusConflict, "port_conflict", fmt.Sprintf("port %d is already assigned to another deployment", req.Port))
			return
		}

		d := deployments.Deployment{
			GenerationJobID: strings.TrimSpace(req.GenerationJobID),
			WorldModelID:    strings.TrimSpace(req.WorldModelID),
			Protocol:        req.Protocol,
			Port:            req.Port,
			RootPath:        req.RootPath,
		}

		created, err := application.DeploymentRepo.Create(r.Context(), d)
		if err != nil {
			if errors.Is(err, deployments.ErrPortConflict) {
				writeError(w, http.StatusConflict, "port_conflict", fmt.Sprintf("port %d is already assigned to another deployment", req.Port))
				return
			}
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to create deployment")
			return
		}

		writeJSON(w, http.StatusCreated, created)
	}
}

func validateDeploymentPort(port int) error {
	if port < deploymentPortMin || port > deploymentPortMax {
		return requestValidationError{message: fmt.Sprintf("port must be between %d and %d", deploymentPortMin, deploymentPortMax)}
	}
	return nil
}

func deploymentRoutingHandler(application *app.APIApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/deployments/")

		if strings.HasSuffix(path, "/start") {
			deploymentStartHandler(application)(w, r)
			return
		}
		if strings.HasSuffix(path, "/stop") {
			deploymentStopHandler(application)(w, r)
			return
		}

		deploymentItemHandler(application)(w, r)
	}
}

func deploymentItemHandler(application *app.APIApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/deployments/")
		id = strings.TrimSuffix(id, "/")

		switch r.Method {
		case http.MethodGet:
			d, err := application.DeploymentRepo.Get(r.Context(), id)
			if errors.Is(err, deployments.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "deployment not found")
				return
			}
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "failed to get deployment")
				return
			}
			writeJSON(w, http.StatusOK, d)

		case http.MethodDelete:
			if application.DeploymentManager.IsRunning(id) {
				if err := application.DeploymentManager.Stop(r.Context(), id); err != nil {
					writeError(w, http.StatusInternalServerError, "internal_error", "failed to stop deployment before delete")
					return
				}
			}
			if err := application.DeploymentRepo.Delete(r.Context(), id); err != nil {
				if errors.Is(err, deployments.ErrNotFound) {
					writeError(w, http.StatusNotFound, "not_found", "deployment not found")
					return
				}
				writeError(w, http.StatusInternalServerError, "internal_error", "failed to delete deployment")
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

		default:
			w.Header().Set("Allow", "GET, DELETE")
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func deploymentStartHandler(application *app.APIApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/api/deployments/")
		id := strings.TrimSuffix(path, "/start")

		if err := application.DeploymentManager.Start(r.Context(), id); err != nil {
			if errors.Is(err, deployments.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "deployment not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "start_failed", err.Error())
			return
		}

		d, err := application.DeploymentRepo.Get(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "deployment started but failed to read back")
			return
		}
		writeJSON(w, http.StatusOK, d)
	}
}

func deploymentStopHandler(application *app.APIApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/api/deployments/")
		id := strings.TrimSuffix(path, "/stop")

		if err := application.DeploymentManager.Stop(r.Context(), id); err != nil {
			if errors.Is(err, deployments.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "deployment not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "stop_failed", err.Error())
			return
		}

		d, err := application.DeploymentRepo.Get(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "deployment stopped but failed to read back")
			return
		}
		writeJSON(w, http.StatusOK, d)
	}
}
