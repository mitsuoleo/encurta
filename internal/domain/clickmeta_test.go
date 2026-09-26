package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitizeClickMeta(t *testing.T) {
	ua, ref := SanitizeClickMeta("Mozilla", "https://ref.example/path?token=secret")
	require.Equal(t, "Mozilla", ua)
	require.Equal(t, "https://ref.example/path", ref)

	long := strings.Repeat("a", 600)
	ua, _ = SanitizeClickMeta(long, "")
	require.Equal(t, 512, len([]rune(ua)))
}
