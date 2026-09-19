package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeAndValidateURL(t *testing.T) {
	ok, err := NormalizeAndValidateURL("https://example.com/path")
	require.NoError(t, err)
	require.Equal(t, "https://example.com/path", ok)

	_, err = NormalizeAndValidateURL("javascript:alert(1)")
	require.ErrorIs(t, err, ErrInvalidURL)

	_, err = NormalizeAndValidateURL("ftp://files.example.com")
	require.ErrorIs(t, err, ErrInvalidURL)

	_, err = NormalizeAndValidateURL("not a url")
	require.ErrorIs(t, err, ErrInvalidURL)

	_, err = NormalizeAndValidateURL("")
	require.ErrorIs(t, err, ErrInvalidURL)

	_, err = NormalizeAndValidateURL("https://")
	require.ErrorIs(t, err, ErrInvalidURL)

	_, err = NormalizeAndValidateURL("data:text/html,hello")
	require.ErrorIs(t, err, ErrInvalidURL)
}
