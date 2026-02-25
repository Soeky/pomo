package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthCheckAndWaitForHealthy(t *testing.T) {
	t.Parallel()

	okSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer okSrv.Close()

	if err := HealthCheck(okSrv.URL); err != nil {
		t.Fatalf("HealthCheck expected ok: %v", err)
	}
	if !WaitForHealthy(okSrv.URL, time.Second) {
		t.Fatalf("WaitForHealthy expected true")
	}

	badSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer badSrv.Close()

	if err := HealthCheck(badSrv.URL); err == nil {
		t.Fatalf("HealthCheck expected non-200 error")
	}
	if WaitForHealthy(badSrv.URL, 100*time.Millisecond) {
		t.Fatalf("WaitForHealthy expected false for unhealthy server")
	}
}

func TestFindAvailablePortAndHostMapping(t *testing.T) {
	t.Parallel()

	p, err := FindAvailablePort(0)
	if err != nil {
		t.Fatalf("FindAvailablePort failed: %v", err)
	}
	if p < 3210 || p > 3299 {
		t.Fatalf("unexpected port range: %d", p)
	}

	ok, err := HasHostMapping("localhost")
	if err != nil {
		t.Fatalf("HasHostMapping failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected localhost mapping on 127.0.0.1")
	}
}

func TestRunWithSignalsInvalidHost(t *testing.T) {
	// Invalid host should make ListenAndServe fail fast.
	if err := RunWithSignals(ServerConfig{Host: "bad host name", Port: 3210}); err == nil {
		t.Fatalf("expected RunWithSignals to fail for invalid host")
	}
}
