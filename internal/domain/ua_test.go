package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClassifyUserAgent(t *testing.T) {
	device, browser := ClassifyUserAgent("Mozilla/5.0 (iPhone; CPU iPhone OS 17_0) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1")
	require.Equal(t, "mobile", device)
	require.Equal(t, "safari", browser)

	device, browser = ClassifyUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	require.Equal(t, "desktop", device)
	require.Equal(t, "chrome", browser)

	device, browser = ClassifyUserAgent("")
	require.Equal(t, "unknown", device)
	require.Equal(t, "unknown", browser)
}
