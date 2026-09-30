package httptransport

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/nightnoryu/go-kita/jsonlog"
	"github.com/nightnoryu/go-kita/log"
)

func TestRoutes(t *testing.T) {
	t.Parallel()
	logger, err := jsonlog.NewLogger(&jsonlog.Config{AppName: "test", Level: jsonlog.ErrorLevel})
	if err != nil {
		t.Fatal(err)
	}
	assets := fstest.MapFS{
		"index.html":    {Data: []byte("<html>Lumio</html>")},
		"assets/app.js": {Data: []byte("console.log('lumio')")},
	}
	for _, available := range []bool{true, false} {
		handler, err := NewRouter(assets, func(context.Context) error {
			if !available {
				return errors.New("database password must never reach clients")
			}
			return nil
		}, logger)
		if err != nil {
			t.Fatal(err)
		}
		readyStatus := http.StatusOK
		if !available {
			readyStatus = http.StatusServiceUnavailable
		}
		cases := []struct {
			method, path string
			status       int
			body         string
		}{
			{"GET", "/", 200, "<html>Lumio</html>"},
			{"GET", "/assets/app.js", 200, "console.log('lumio')"},
			{"GET", "/assets/missing.js", 404, ""},
			{"GET", "/missing", 404, ""},
			{"GET", "/api/missing", 404, ""},
			{"GET", "/api", 404, ""},
			{"GET", "/api/status", 200, `"status":"ok"`},
			{"GET", "/livez", 200, "ok"},
			{"GET", "/healthz", readyStatus, ""},
			{"POST", "/", 405, ""},
		}
		for _, tc := range cases {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(tc.method, tc.path, http.NoBody))
			if response.Code != tc.status || !strings.Contains(response.Body.String(), tc.body) {
				t.Errorf("%s %s (database available=%v): got %d %q", tc.method, tc.path, available, response.Code, response.Body.String())
			}
			if strings.Contains(response.Body.String(), "password") {
				t.Error("dependency error exposed to client")
			}
		}
	}
}

func TestApexRedirect(t *testing.T) {
	for _, base := range []string{"lumio.test", "lumio.nightnoryu.com"} {
		origin := "https://app." + base
		handler, err := NewRouter(fstest.MapFS{}, func(context.Context) error { return nil }, log.NoopLogger{}, APIConfig{
			Origin: origin, BaseDomain: base,
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			method, target string
			status         int
			location       string
		}{
			{http.MethodGet, "https://" + base + "/", http.StatusFound, origin + "/"},
			{http.MethodHead, "https://" + base + "/", http.StatusFound, origin + "/"},
			{http.MethodGet, "https://" + base + "/api/status", http.StatusNotFound, ""},
			{http.MethodPost, "https://" + base + "/", http.StatusNotFound, ""},
			{http.MethodGet, "https://other.test/", http.StatusNotFound, ""},
		} {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(tc.method, tc.target, http.NoBody))
			if response.Code != tc.status || response.Header().Get("Location") != tc.location {
				t.Errorf("%s %s: got %d, location %q", tc.method, tc.target, response.Code, response.Header().Get("Location"))
			}
		}
	}
}
