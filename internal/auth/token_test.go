package auth

import (
	"testing"
	"time"

	"github.com/mitsuoleo/encurta/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestIssueAndParse(t *testing.T) {
	toks := NewTokens("secret", time.Hour)
	hash, err := HashPassword("password1")
	require.NoError(t, err)
	signed, _, err := toks.Issue(domain.User{ID: 7, Email: "a@b.com", PasswordHash: hash})
	require.NoError(t, err)
	id, email, err := toks.Parse(signed)
	require.NoError(t, err)
	require.Equal(t, int64(7), id)
	require.Equal(t, "a@b.com", email)
	_, _, err = toks.Parse("nope")
	require.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestWeakPassword(t *testing.T) {
	_, err := HashPassword("short")
	require.ErrorIs(t, err, domain.ErrWeakPassword)
}

func TestPasswordTooLong(t *testing.T) {
	_, err := HashPassword(string(make([]byte, 73)))
	require.ErrorIs(t, err, domain.ErrPasswordTooLong)
}
