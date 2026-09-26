package config

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProductionRejectsDefaults(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("JWT_SECRET", "dev-jwt-change-me")
	t.Setenv("IP_HASH_SALT", "not-default-salt")
	t.Setenv("METRICS_TOKEN", "tok")
	t.Setenv("PUBLIC_BASE_URL", "https://short.example")
	_, err := Load()
	require.Error(t, err)

	t.Setenv("JWT_SECRET", "a-real-secret-value-32-chars-min!")
	cfg, err := Load()
	require.NoError(t, err)
	require.True(t, cfg.Production())
	require.Equal(t, "tok", cfg.MetricsToken)
}

func TestProductionRejectsHTTPPublicURL(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("JWT_SECRET", "a-real-secret-value-32-chars-min!")
	t.Setenv("IP_HASH_SALT", "not-default-salt")
	t.Setenv("METRICS_TOKEN", "tok")
	t.Setenv("PUBLIC_BASE_URL", "http://short.example")
	_, err := Load()
	require.Error(t, err)
}

func TestProductionRejectsShortJWT(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("JWT_SECRET", "short-secret")
	t.Setenv("IP_HASH_SALT", "not-default-salt")
	t.Setenv("METRICS_TOKEN", "tok")
	t.Setenv("PUBLIC_BASE_URL", "https://short.example")
	_, err := Load()
	require.Error(t, err)
}

func TestDevelopmentAllowsDefaults(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("JWT_SECRET", "")
	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "dev-jwt-change-me", cfg.JWTSecret)
	require.NotEmpty(t, cfg.ClickConsumer)
}

func TestTrustedProxiesParse(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("TRUSTED_PROXIES", "10.0.0.1, 192.168.0.0/16")
	cfg, err := Load()
	require.NoError(t, err)
	require.Len(t, cfg.TrustedProxies, 2)
	require.True(t, cfg.TrustedProxies[0].Contains(mustParseIP("10.0.0.1")))
}

func mustParseIP(s string) net.IP {
	ip := net.ParseIP(s)
	if ip == nil {
		panic(s)
	}
	return ip
}

func TestLoadRejectsNonPositiveRateLimits(t *testing.T) {
	for _, tc := range []struct {
		name  string
		key   string
		value string
	}{
		{"create zero", "RATE_LIMIT_CREATE_PER_HOUR", "0"},
		{"auth negative", "RATE_LIMIT_AUTH_PER_MINUTE", "-1"},
		{"redirect zero", "RATE_LIMIT_REDIRECT_PER_MINUTE", "0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ENV", "development")
			t.Setenv(tc.key, tc.value)
			_, err := Load()
			require.ErrorContains(t, err, tc.key)
		})
	}
}
