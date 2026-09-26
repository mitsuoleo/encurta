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

	_, err = NormalizeAndValidateURL("https://127.0.0.1/")
	require.ErrorIs(t, err, ErrInvalidURL)

	_, err = NormalizeAndValidateURL("http://192.168.0.10/admin")
	require.ErrorIs(t, err, ErrInvalidURL)

	_, err = NormalizeAndValidateURL("http://localhost/secret")
	require.ErrorIs(t, err, ErrInvalidURL)

	_, err = NormalizeAndValidateURL("http://169.254.169.254/latest/meta-data/")
	require.ErrorIs(t, err, ErrInvalidURL)

	_, err = NormalizeAndValidateURL("http://2130706433/")
	require.ErrorIs(t, err, ErrInvalidURL)

	_, err = NormalizeAndValidateURL("http://127.1/")
	require.ErrorIs(t, err, ErrInvalidURL)

	_, err = NormalizeAndValidateURL("http://0x7f000001/")
	require.ErrorIs(t, err, ErrInvalidURL)

	_, err = NormalizeAndValidateURL("http://0177.0.0.1/")
	require.ErrorIs(t, err, ErrInvalidURL)
}
