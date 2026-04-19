package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHostGuard_AllowsLocalhostVariants(t *testing.T) {
	handler := HostGuard(nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, host := range []string{
		"localhost",
		"localhost:8080",
		"127.0.0.1",
		"127.0.0.1:8080",
		"[::1]:8080",
		"192.168.1.42",   // RFC1918
		"10.0.0.5",       // RFC1918
		"172.16.0.1",     // RFC1918
		"mymac.local",    // mDNS
	} {
		t.Run(host, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Host = host
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

func TestHostGuard_BlocksUnknownHost(t *testing.T) {
	handler := HostGuard(nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, host := range []string{
		"evil.example",
		"attacker.com",
		"8.8.8.8",         // public IP
		"example.org:80",
	} {
		t.Run(host, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Host = host
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			assert.Equal(t, http.StatusForbidden, w.Code,
				"host %q doit être rejeté par le DNS-rebinding guard", host)
		})
	}
}

func TestHostGuard_AcceptsExtraAllowlist(t *testing.T) {
	handler := HostGuard(
		[]string{"hive.example", "other.internal"},
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "hive.example"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
