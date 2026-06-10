package integration

import (
	"net"
	"net/url"
	"os"
	"testing"
	"time"
)

// dsnOrSkip returns the env value or skips the test, keeping the suite
// green when the backing service is not available.
func dsnOrSkip(t *testing.T, env string) string {
	t.Helper()
	dsn := os.Getenv(env)
	if dsn == "" {
		t.Skipf("set %s to run this integration test", env)
	}
	return dsn
}

// reachableOrSkip skips unless the endpoint accepts TCP connections.
func reachableOrSkip(t *testing.T, endpoint string) {
	t.Helper()
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" {
		t.Skipf("invalid endpoint %q", endpoint)
	}
	conn, err := net.DialTimeout("tcp", parsed.Host, 500*time.Millisecond)
	if err != nil {
		t.Skipf("endpoint %s unreachable: %v", endpoint, err)
	}
	conn.Close()
}
