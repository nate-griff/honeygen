package provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/natet/honeygen/backend/internal/config"
)

func TestOpenAIProviderTestSuccess(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodGet)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q, want %q", got, "Bearer test-key")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	provider := NewOpenAI(config.ProviderConfig{
		BaseURL: server.URL + "/v1",
		APIKey:  "test-key",
		Model:   "gpt-4.1-mini",
	}, server.Client())

	if err := provider.Test(context.Background()); err != nil {
		t.Fatalf("Test() error = %v", err)
	}
}

func TestOpenAIProviderTestConfigValidation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		cfg  config.ProviderConfig
		want string
	}{
		{
			name: "missing base url",
			cfg: config.ProviderConfig{
				APIKey: "test-key",
				Model:  "gpt-4.1-mini",
			},
			want: "provider base URL is required",
		},
		{
			name: "missing api key",
			cfg: config.ProviderConfig{
				BaseURL: "https://provider.example/v1",
				Model:   "gpt-4.1-mini",
			},
			want: "provider API key is required",
		},
		{
			name: "missing model",
			cfg: config.ProviderConfig{
				BaseURL: "https://provider.example/v1",
				APIKey:  "test-key",
			},
			want: "provider model is required",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			provider := NewOpenAI(tc.cfg, nil)
			err := provider.Test(context.Background())
			if err == nil {
				t.Fatal("Test() error = nil, want config error")
			}
			if !IsKind(err, KindConfig) {
				t.Fatalf("error kind = %v, want %v", err, KindConfig)
			}
			if err.Error() != tc.want {
				t.Fatalf("error = %q, want %q", err.Error(), tc.want)
			}
		})
	}
}

func TestOpenAIProviderTestUnauthorized(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	provider := NewOpenAI(config.ProviderConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "gpt-4.1-mini",
	}, server.Client())

	err := provider.Test(context.Background())
	if err == nil {
		t.Fatal("Test() error = nil, want unauthorized error")
	}
	if !IsKind(err, KindUnauthorized) {
		t.Fatalf("error kind = %v, want %v", err, KindUnauthorized)
	}
	if err.Error() != "provider authentication failed" {
		t.Fatalf("error = %q, want %q", err.Error(), "provider authentication failed")
	}
}

func TestOpenAIProviderTestMalformedResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list"}`))
	}))
	defer server.Close()

	provider := NewOpenAI(config.ProviderConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "gpt-4.1-mini",
	}, server.Client())

	err := provider.Test(context.Background())
	if err == nil {
		t.Fatal("Test() error = nil, want invalid response error")
	}
	if !IsKind(err, KindInvalidResponse) {
		t.Fatalf("error kind = %v, want %v", err, KindInvalidResponse)
	}
	if err.Error() != "provider returned an invalid response" {
		t.Fatalf("error = %q, want %q", err.Error(), "provider returned an invalid response")
	}
}

func TestOpenAIProviderGenerateSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/v1/chat/completions")
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q, want %q", got, "Bearer test-key")
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("io.ReadAll() error = %v", err)
		}
		payload := string(body)
		if !strings.Contains(payload, `"model":"gpt-4.1-mini"`) {
			t.Fatalf("request body = %s, want model", payload)
		}
		if !strings.Contains(payload, `"content":"Follow the policy."`) {
			t.Fatalf("request body = %s, want system prompt", payload)
		}
		if !strings.Contains(payload, `"content":"Write a memo."`) {
			t.Fatalf("request body = %s, want prompt", payload)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"model":"gpt-4.1-mini",
			"choices":[{"message":{"content":"Generated memo"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":18}
		}`))
	}))
	defer server.Close()

	provider := NewOpenAI(config.ProviderConfig{
		BaseURL: server.URL + "/v1",
		APIKey:  "test-key",
		Model:   "gpt-4.1-mini",
	}, server.Client())

	response, err := provider.Generate(context.Background(), GenerateRequest{
		SystemPrompt: "Follow the policy.",
		Prompt:       "Write a memo.",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if response.Content != "Generated memo" {
		t.Fatalf("response.Content = %q, want %q", response.Content, "Generated memo")
	}
	if response.Metadata["model"] != "gpt-4.1-mini" {
		t.Fatalf("response.Metadata[model] = %q, want %q", response.Metadata["model"], "gpt-4.1-mini")
	}
	if response.Metadata["finish_reason"] != "stop" {
		t.Fatalf("response.Metadata[finish_reason] = %q, want %q", response.Metadata["finish_reason"], "stop")
	}
	if response.Metadata["prompt_tokens"] != "11" || response.Metadata["completion_tokens"] != "7" || response.Metadata["total_tokens"] != "18" {
		t.Fatalf("response.Metadata token counts = %+v, want prompt/completion/total tokens", response.Metadata)
	}
}

func TestOpenAIProviderGeneratePreservesProviderMetadata(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"model":"gpt-4.1-mini",
			"choices":[{"message":{"content":"Generated memo"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":18}
		}`))
	}))
	defer server.Close()

	provider := NewOpenAI(config.ProviderConfig{
		BaseURL: server.URL + "/v1",
		APIKey:  "test-key",
		Model:   "gpt-4.1-mini",
	}, server.Client())

	response, err := provider.Generate(context.Background(), GenerateRequest{
		SystemPrompt: "Follow the policy.",
		Prompt:       "Write a memo.",
		Metadata: map[string]string{
			"model":             "caller-model",
			"finish_reason":     "caller-finish",
			"prompt_tokens":     "999",
			"completion_tokens": "999",
			"total_tokens":      "999",
			"request_id":        "req-123",
		},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if response.Metadata["model"] != "gpt-4.1-mini" {
		t.Fatalf("response.Metadata[model] = %q, want %q", response.Metadata["model"], "gpt-4.1-mini")
	}
	if response.Metadata["finish_reason"] != "stop" {
		t.Fatalf("response.Metadata[finish_reason] = %q, want %q", response.Metadata["finish_reason"], "stop")
	}
	if response.Metadata["prompt_tokens"] != "11" || response.Metadata["completion_tokens"] != "7" || response.Metadata["total_tokens"] != "18" {
		t.Fatalf("response.Metadata token counts = %+v, want prompt/completion/total tokens", response.Metadata)
	}
	if response.Metadata["request_id"] != "req-123" {
		t.Fatalf("response.Metadata[request_id] = %q, want %q", response.Metadata["request_id"], "req-123")
	}
}

func TestOpenAIProviderTestUsesShortRequestTimeout(t *testing.T) {
	t.Parallel()

	transport := &timeoutCaptureRoundTripper{
		t:            t,
		responseBody: `{"data":[]}`,
	}
	client := &http.Client{Transport: transport}

	provider := NewOpenAI(config.ProviderConfig{
		BaseURL: "https://provider.example/v1",
		APIKey:  "test-key",
		Model:   "gpt-4.1-mini",
	}, client)

	if err := provider.Test(context.Background()); err != nil {
		t.Fatalf("Test() error = %v", err)
	}

	timeout := transport.timeout()
	if timeout < 10*time.Second || timeout > 20*time.Second {
		t.Fatalf("request timeout = %v, want test timeout to stay short", timeout)
	}
}

func TestOpenAIProviderGenerateUsesExtendedRequestTimeout(t *testing.T) {
	t.Parallel()

	transport := &timeoutCaptureRoundTripper{
		t: t,
		responseBody: `{
			"model":"gpt-4.1-mini",
			"choices":[{"message":{"content":"Generated memo"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":18}
		}`,
	}
	client := &http.Client{Transport: transport}

	provider := NewOpenAI(config.ProviderConfig{
		BaseURL: "https://provider.example/v1",
		APIKey:  "test-key",
		Model:   "gpt-4.1-mini",
	}, client)

	if _, err := provider.Generate(context.Background(), GenerateRequest{
		SystemPrompt: "Follow the policy.",
		Prompt:       "Write a memo.",
	}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	timeout := transport.timeout()
	if timeout < 4*time.Minute {
		t.Fatalf("request timeout = %v, want multi-minute generation timeout", timeout)
	}
}

type timeoutCaptureRoundTripper struct {
	t            *testing.T
	responseBody string
	deadline     time.Time
	startedAt    time.Time
}

func (rt *timeoutCaptureRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.startedAt = time.Now()
	deadline, ok := req.Context().Deadline()
	if !ok {
		rt.t.Fatal("request context missing deadline")
	}
	rt.deadline = deadline

	response := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(rt.responseBody)),
		Request:    req,
	}
	response.Header.Set("Content-Type", "application/json")
	return response, nil
}

func (rt *timeoutCaptureRoundTripper) timeout() time.Duration {
	return rt.deadline.Sub(rt.startedAt)
}
