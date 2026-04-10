package config

import "testing"

func TestEnvOrDefaultReturnsEnvironmentValue(t *testing.T) {
	t.Setenv("HONEYGEN_TEST_ENV", "configured")

	if got := EnvOrDefault("HONEYGEN_TEST_ENV", "fallback"); got != "configured" {
		t.Fatalf("EnvOrDefault() = %q, want %q", got, "configured")
	}
}

func TestEnvOrDefaultReturnsFallbackForUnsetValue(t *testing.T) {
	if got := EnvOrDefault("HONEYGEN_TEST_ENV_UNSET", "fallback"); got != "fallback" {
		t.Fatalf("EnvOrDefault() = %q, want %q", got, "fallback")
	}
}

func TestEnvOrDefaultReturnsFallbackForEmptyValue(t *testing.T) {
	t.Setenv("HONEYGEN_TEST_ENV_EMPTY", "")

	if got := EnvOrDefault("HONEYGEN_TEST_ENV_EMPTY", "fallback"); got != "fallback" {
		t.Fatalf("EnvOrDefault() = %q, want %q", got, "fallback")
	}
}
