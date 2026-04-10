package provider

import (
	"context"
	"errors"
	"fmt"
)

type Provider interface {
	Generate(ctx context.Context, request GenerateRequest) (GenerateResponse, error)
	Test(ctx context.Context) error
}

type GenerateRequest struct {
	SystemPrompt string
	Prompt       string
	Metadata     map[string]string
}

type GenerateResponse struct {
	Content  string            `json:"content"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type ErrorKind string

const (
	KindConfig          ErrorKind = "config"
	KindUnauthorized    ErrorKind = "unauthorized"
	KindConnectivity    ErrorKind = "connectivity"
	KindUpstream        ErrorKind = "upstream"
	KindInvalidResponse ErrorKind = "invalid_response"
)

type Error struct {
	Kind       ErrorKind
	Message    string
	StatusCode int
	Err        error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("provider error (%s)", e.Kind)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func IsKind(err error, kind ErrorKind) bool {
	var providerErr *Error
	return errors.As(err, &providerErr) && providerErr.Kind == kind
}
