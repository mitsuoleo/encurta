package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGenerateShortCode(t *testing.T) {
	code, err := GenerateShortCode()
	require.NoError(t, err)
	require.Len(t, code, ShortCodeLength)
	require.True(t, ValidShortCode(code))
}

func TestGenerateShortCodeUniqueEnough(t *testing.T) {
	seen := map[string]struct{}{}
	for i := 0; i < 200; i++ {
		code, err := GenerateShortCode()
		require.NoError(t, err)
		_, exists := seen[code]
		require.False(t, exists)
		seen[code] = struct{}{}
	}
}

func TestValidShortCode(t *testing.T) {
	require.True(t, ValidShortCode("abc123A"))
	require.False(t, ValidShortCode(""))
	require.False(t, ValidShortCode("has space"))
	require.False(t, ValidShortCode("bad/code"))
	require.False(t, ValidShortCode(string(make([]byte, 21))))
	require.True(t, ValidAlias("myalias"))
	require.True(t, ValidAlias("my-link"))
	require.True(t, ValidShortCode("my-link"))
	require.False(t, ValidAlias("-link"))
	require.False(t, ValidAlias("link-"))
	require.False(t, ValidAlias("my--link"))
	require.False(t, ValidAlias("ab"))
	require.False(t, ValidAlias("auth"))
}

func TestLinkRedirectable(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	require.NoError(t, Link{IsActive: true}.Redirectable(now))
	require.ErrorIs(t, Link{IsActive: false}.Redirectable(now), ErrInactive)
	require.ErrorIs(t, Link{IsActive: true, ExpiresAt: &past}.Redirectable(now), ErrExpired)
	require.NoError(t, Link{IsActive: true, ExpiresAt: &future}.Redirectable(now))
}

func TestHashIP(t *testing.T) {
	a := HashIP("salt", "1.2.3.4")
	b := HashIP("salt", "1.2.3.4")
	c := HashIP("salt", "1.2.3.5")
	require.Equal(t, a, b)
	require.NotEqual(t, a, c)
	require.Len(t, a, 64)
}
