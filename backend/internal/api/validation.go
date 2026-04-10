package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

const maxJSONBodyBytes = 1 << 20

type requestValidationError struct {
	message string
}

func (e requestValidationError) Error() string {
	return e.message
}

func readJSONBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	body, err := readOptionalJSONBody(w, r)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, requestValidationError{message: "request body must not be empty"}
	}

	return body, nil
}

func readOptionalJSONBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxJSONBodyBytes))
	if err != nil {
		if strings.Contains(err.Error(), "http: request body too large") {
			return nil, requestValidationError{message: "request body must be 1 MiB or smaller"}
		}
		return nil, err
	}

	if len(strings.TrimSpace(string(body))) == 0 {
		return nil, nil
	}
	if !json.Valid(body) {
		return nil, requestValidationError{message: "request body must be valid JSON"}
	}

	return body, nil
}

func worldModelIDFromPath(path string) (string, error) {
	const prefix = "/api/world-models/"
	if !strings.HasPrefix(path, prefix) {
		return "", requestValidationError{message: "world model id is required"}
	}

	id := strings.TrimSpace(strings.TrimPrefix(path, prefix))
	if id == "" {
		return "", requestValidationError{message: "world model id is required"}
	}
	if strings.Contains(id, "/") {
		return "", requestValidationError{message: "world model id must not contain slashes"}
	}
	return id, nil
}
