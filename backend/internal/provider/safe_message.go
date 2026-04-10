package provider

import (
	"errors"
	"strings"
)

func SafeErrorMessage(err error) string {
	if err == nil {
		return ""
	}

	var providerErr *Error
	if !errors.As(err, &providerErr) || providerErr == nil {
		return "provider is temporarily unavailable"
	}

	switch providerErr.Kind {
	case KindConfig:
		if message := strings.TrimSpace(providerErr.Message); message != "" {
			return message
		}
		return "provider configuration is invalid"
	case KindUnauthorized:
		return "provider authentication failed"
	case KindConnectivity:
		return "provider request failed"
	case KindInvalidResponse:
		return "provider returned an invalid response"
	case KindUpstream:
		if message := strings.TrimSpace(providerErr.Message); message != "" {
			return message
		}
		return "provider is temporarily unavailable"
	default:
		if message := strings.TrimSpace(providerErr.Message); message != "" {
			return message
		}
		return "provider is temporarily unavailable"
	}
}
