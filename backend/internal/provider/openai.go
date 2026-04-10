package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/natet/honeygen/backend/internal/config"
)

const defaultTimeout = 15 * time.Second

type OpenAIProvider struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewOpenAI(cfg config.ProviderConfig, client *http.Client) *OpenAIProvider {
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}

	return &OpenAIProvider{
		baseURL:    strings.TrimSpace(cfg.BaseURL),
		apiKey:     strings.TrimSpace(cfg.APIKey),
		model:      strings.TrimSpace(cfg.Model),
		httpClient: client,
	}
}

func (p *OpenAIProvider) Test(ctx context.Context) error {
	if err := p.validateConfig(); err != nil {
		return err
	}

	req, err := p.newRequest(ctx, http.MethodGet, "/models", nil)
	if err != nil {
		return err
	}

	var response struct {
		Data json.RawMessage `json:"data"`
	}
	if err := p.do(req, &response); err != nil {
		return err
	}
	if len(response.Data) == 0 || !json.Valid(response.Data) || bytes.TrimSpace(response.Data)[0] != '[' {
		return &Error{Kind: KindInvalidResponse, Message: "provider returned an invalid response"}
	}

	return nil
}

func (p *OpenAIProvider) Generate(ctx context.Context, request GenerateRequest) (GenerateResponse, error) {
	if err := p.validateConfig(); err != nil {
		return GenerateResponse{}, err
	}

	payload := struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}{
		Model: p.model,
		Messages: []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{
			{Role: "system", Content: request.SystemPrompt},
			{Role: "user", Content: request.Prompt},
		},
	}

	req, err := p.newRequest(ctx, http.MethodPost, "/chat/completions", payload)
	if err != nil {
		return GenerateResponse{}, err
	}

	var response struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := p.do(req, &response); err != nil {
		return GenerateResponse{}, err
	}
	if len(response.Choices) == 0 || strings.TrimSpace(response.Choices[0].Message.Content) == "" {
		return GenerateResponse{}, &Error{Kind: KindInvalidResponse, Message: "provider returned an invalid response"}
	}

	metadata := map[string]string{
		"model":             response.Model,
		"finish_reason":     response.Choices[0].FinishReason,
		"prompt_tokens":     strconv.Itoa(response.Usage.PromptTokens),
		"completion_tokens": strconv.Itoa(response.Usage.CompletionTokens),
		"total_tokens":      strconv.Itoa(response.Usage.TotalTokens),
	}
	for key, value := range request.Metadata {
		metadata[key] = value
	}

	return GenerateResponse{
		Content:  response.Choices[0].Message.Content,
		Metadata: metadata,
	}, nil
}

func (p *OpenAIProvider) validateConfig() error {
	switch {
	case p.baseURL == "":
		return &Error{Kind: KindConfig, Message: "provider base URL is required"}
	case p.apiKey == "":
		return &Error{Kind: KindConfig, Message: "provider API key is required"}
	case p.model == "":
		return &Error{Kind: KindConfig, Message: "provider model is required"}
	default:
		return nil
	}
}

func (p *OpenAIProvider) newRequest(ctx context.Context, method, endpoint string, payload any) (*http.Request, error) {
	url := strings.TrimRight(p.baseURL, "/") + endpoint

	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal provider request: %w", err)
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, &Error{Kind: KindConfig, Message: "provider base URL is invalid", Err: err}
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	return req, nil
}

func (p *OpenAIProvider) do(req *http.Request, out any) error {
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return &Error{Kind: KindConnectivity, Message: "provider request failed", Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return &Error{Kind: KindUnauthorized, Message: "provider authentication failed", StatusCode: resp.StatusCode}
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return &Error{
			Kind:       KindUpstream,
			Message:    fmt.Sprintf("provider returned status %d", resp.StatusCode),
			StatusCode: resp.StatusCode,
		}
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return &Error{Kind: KindInvalidResponse, Message: "provider returned an invalid response", Err: err}
	}
	return nil
}
