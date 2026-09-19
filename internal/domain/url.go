package domain

import (
	"net/url"
	"strings"
)

func NormalizeAndValidateURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ErrInvalidURL
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "javascript:") || strings.HasPrefix(lower, "data:") || strings.HasPrefix(lower, "file:") {
		return "", ErrInvalidURL
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", ErrInvalidURL
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", ErrInvalidURL
	}
	if parsed.Host == "" {
		return "", ErrInvalidURL
	}
	if parsed.User != nil {
		return "", ErrInvalidURL
	}
	return parsed.String(), nil
}
