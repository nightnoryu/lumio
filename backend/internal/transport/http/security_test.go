package httptransport

import (
	"context"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/nightnoryu/go-kita/log"

	"lumio/internal/app"
)

func TestAssetCachingAndRecovery(t *testing.T) {
	assets := fstest.MapFS{
		"index.html":             {Data: []byte(`<html><script src="/recovery.js"></script></html>`)},
		"assets/app-1234abcd.js": {Data: []byte("export default 1")},
		"recovery.js":            {Data: []byte("recovery")},
	}
	handler, err := NewRouter(assets, func(context.Context) error { return nil }, log.NoopLogger{}, APIConfig{Origin: "https://app.lumio.test", BaseDomain: "lumio.test", StorageOrigin: "https://storage.test"})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path, cache string
		status      int
	}{
		{"/", "no-cache", 200}, {"/recovery.js", "no-cache", 200},
		{"/assets/app-1234abcd.js", "public, max-age=31536000, immutable", 200},
		{"/assets/old-1234abcd.js", "no-store", 404}, {"/assets/", "no-store", 404},
		{"/api/unknown", "no-store", 404}, {"/unknown", "no-store", 404},
		{"/metrics", "no-store", 404}, {"/index.html", "no-store", 404},
	} {
		req := httptest.NewRequest("GET", "https://app.lumio.test"+tc.path, http.NoBody)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != tc.status || res.Header().Get("Cache-Control") != tc.cache {
			t.Errorf("%s: %d %v", tc.path, res.Code, res.Header())
		}
		csp := res.Header().Get("Content-Security-Policy")
		if !strings.Contains(csp, "script-src 'self'") || !strings.Contains(csp, "connect-src 'self' https://storage.test;") || res.Header().Get("X-Content-Type-Options") != "nosniff" || res.Header().Get("Strict-Transport-Security") == "" {
			t.Fatalf("missing security headers: %v", res.Header())
		}
		if tc.status == 200 {
			req.Header.Set("If-None-Match", res.Header().Get("ETag"))
			cached := httptest.NewRecorder()
			handler.ServeHTTP(cached, req)
			if cached.Code != 304 || cached.Body.Len() != 0 {
				t.Fatalf("%s not revalidated: %d", tc.path, cached.Code)
			}
		}
	}
}

func TestSessionCookieHostIsolation(t *testing.T) {
	res := httptest.NewRecorder()
	setSession(context.WithValue(t.Context(), requestKey{}, requestState{writer: res}), "secret")
	response := res.Result()
	defer response.Body.Close()
	cookies := response.Cookies()
	if len(cookies) != 1 {
		t.Fatal("missing cookie")
	}
	c := cookies[0]
	if c.Domain != "" || !c.Secure || !c.HttpOnly || c.Path != "/" || c.SameSite != http.SameSiteStrictMode {
		t.Fatalf("unsafe cookie: %v", c)
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	origin, _ := url.Parse("https://app.lumio.test/")
	jar.SetCookies(origin, cookies)
	if len(jar.Cookies(origin)) != 1 {
		t.Fatal("dashboard session missing")
	}
	for _, host := range []string{"https://anna.lumio.test", "https://lumio.test", "http://app.lumio.test"} {
		target, _ := url.Parse(host)
		if len(jar.Cookies(target)) != 0 {
			t.Fatalf("session leaked to %s", host)
		}
	}
}

func TestRateLimitsAndProxyTrust(t *testing.T) {
	limits := &rateLimits{entries: make(map[string]rateEntry)}
	now := time.Now()
	for i := 0; i < 10; i++ {
		if !limits.allow("auth:ip", 10, now) {
			t.Fatal("early limit")
		}
	}
	if limits.allow("auth:ip", 10, now) || !limits.allow("auth:other", 10, now) || !limits.allow("auth:ip", 10, now.Add(time.Minute)) {
		t.Fatal("incorrect rate window")
	}
	req := httptest.NewRequest("POST", "/", http.NoBody)
	req.RemoteAddr = "10.1.0.2:1234"
	req.Header.Set("X-Forwarded-For", "192.0.2.99, 198.51.100.1")
	if clientIP(req, nil) != "10.1.0.2" {
		t.Fatal("trusted spoofed header")
	}
	_, trusted, _ := net.ParseCIDR("10.1.0.0/24")
	if clientIP(req, []*net.IPNet{trusted}) != "198.51.100.1" {
		t.Fatal("incorrect trusted proxy chain")
	}
	for i := 0; i < 10000; i++ {
		limits.entries[time.Unix(int64(i), 0).String()] = rateEntry{expires: now.Add(time.Minute)}
	}
	if limits.allow("new", 10, now) {
		t.Fatal("rate map exceeded bound")
	}
}

func TestAuthenticationThrottledBeforePasswordWork(t *testing.T) {
	handler, err := NewRouter(fstest.MapFS{}, func(context.Context) error { return nil }, log.NoopLogger{}, APIConfig{Service: &app.Service{}, Origin: "https://app.lumio.test", BaseDomain: "lumio.test"})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 11; i++ {
		req := httptest.NewRequest("POST", "https://app.lumio.test/api/auth/login", strings.NewReader("{}"))
		req.Header.Set("Origin", "https://app.lumio.test")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", time.Unix(int64(i), 0).String())
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		expected := 400
		if i == 10 {
			expected = 429
			if res.Header().Get("Retry-After") != "60" {
				t.Fatal("missing retry advice")
			}
		}
		if res.Code != expected {
			t.Fatalf("attempt %d: got %d, want %d", i, res.Code, expected)
		}
	}
}
