package middleware

import (
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClientIPIgnoresSpoofedHeadersWithoutTrustedProxy(t *testing.T) {
	req := httptestRequest("203.0.113.9:1234")
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	req.Header.Set("CF-Connecting-IP", "5.6.7.8")
	require.Equal(t, "203.0.113.9", ClientIP(req, nil))
}

func TestClientIPUsesForwardedHeadersFromTrustedProxy(t *testing.T) {
	_, n, err := net.ParseCIDR("10.0.0.0/8")
	require.NoError(t, err)
	trusted := []*net.IPNet{n}

	req := httptestRequest("10.1.2.3:80")
	req.Header.Set("X-Forwarded-For", "198.51.100.7, 10.9.9.9")
	require.Equal(t, "198.51.100.7", ClientIP(req, trusted))

	req = httptestRequest("10.1.2.3:80")
	req.Header.Set("CF-Connecting-IP", "198.51.100.8")
	req.Header.Set("X-Forwarded-For", "1.1.1.1")
	require.Equal(t, "1.1.1.1", ClientIP(req, trusted))
}

func TestClientIPIgnoresClientSuppliedCFHeaderBehindTrustedProxy(t *testing.T) {
	_, n, err := net.ParseCIDR("10.0.0.0/8")
	require.NoError(t, err)
	req := httptestRequest("10.1.2.3:80")
	req.Header.Set("X-Forwarded-For", "203.0.113.42")
	req.Header.Set("CF-Connecting-IP", "198.51.100.8")
	require.Equal(t, "203.0.113.42", ClientIP(req, []*net.IPNet{n}))
}

func TestClientIPRightmostUntrustedHop(t *testing.T) {
	_, n, err := net.ParseCIDR("10.0.0.0/8")
	require.NoError(t, err)
	req := httptestRequest("10.0.0.1:80")
	req.Header.Set("X-Forwarded-For", "8.8.8.8, 203.0.113.10, 10.0.0.2")
	require.Equal(t, "203.0.113.10", ClientIP(req, []*net.IPNet{n}))
}

func httptestRequest(remote string) *http.Request {
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = remote
	return req
}
