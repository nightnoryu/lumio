package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMetricsExposeRequestMediaAndRuntime(t *testing.T) {
	m := New()
	m.Request("Login", 429, time.Second)
	m.Media("upload_complete", "error", time.Second)
	m.Media("process", "success", time.Second)
	r := httptest.NewRecorder()
	m.Handler().ServeHTTP(r, httptest.NewRequest("GET", "/metrics", http.NoBody))
	for _, want := range []string{
		`lumio_http_requests_total{route="Login",status="429"} 1`,
		`lumio_media_operations_total{operation="upload_complete",outcome="error"} 1`,
		`lumio_media_operations_total{operation="process",outcome="success"} 1`,
		`lumio_http_request_duration_seconds_count{route="Login"} 1`,
		`lumio_media_operation_duration_seconds_count{operation="process"} 1`,
		"go_memstats_heap_alloc_bytes", "process_resident_memory_bytes",
	} {
		if !strings.Contains(r.Body.String(), want) {
			t.Errorf("missing %s", want)
		}
	}
	other := httptest.NewRecorder()
	New().Handler().ServeHTTP(other, httptest.NewRequest("GET", "/metrics", http.NoBody))
	if strings.Contains(other.Body.String(), `route="Login"`) {
		t.Fatal("registries shared")
	}
}
