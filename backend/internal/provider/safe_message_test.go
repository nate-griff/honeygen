package provider

import (
	"errors"
	"strings"
	"testing"
)

func TestSafeErrorMessage(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		err         error
		want        string
		notContains string
	}{
		{
			name: "config error keeps explicit message",
			err: &Error{
				Kind:    KindConfig,
				Message: "provider API key is required",
			},
			want: "provider API key is required",
		},
		{
			name: "connectivity error hides wrapped details",
			err: &Error{
				Kind: KindConnectivity,
				Err:  errors.New("dial tcp secret.internal:443: connect: connection refused"),
			},
			want:        "provider request failed",
			notContains: "secret.internal",
		},
		{
			name: "unknown error uses generic fallback",
			err:  errors.New("raw upstream failure"),
			want: "provider is temporarily unavailable",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := SafeErrorMessage(tc.err)
			if got != tc.want {
				t.Fatalf("SafeErrorMessage() = %q, want %q", got, tc.want)
			}
			if tc.notContains != "" && strings.Contains(got, tc.notContains) {
				t.Fatalf("SafeErrorMessage() = %q, should not contain %q", got, tc.notContains)
			}
		})
	}
}
