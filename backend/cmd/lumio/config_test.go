package main

import (
	"os"
	"testing"
	"time"

	"github.com/nightnoryu/go-kita/env"
)

func TestConfigFromEnvironment(t *testing.T) {
	for key, value := range map[string]string{
		"LUMIO_SERVE_REST_ADDRESS": "127.0.0.1:8080",
		"LUMIO_DB_HOST":            "localhost", "LUMIO_DB_PORT": "5432",
		"LUMIO_DB_NAME": "lumio", "LUMIO_DB_USER": "lumio", "LUMIO_DB_PASSWORD": "test",
		"LUMIO_DB_MAX_CONN": "10", "LUMIO_DB_CONN_LIFETIME": "60s",
		"LUMIO_LOG_LEVEL":        "info",
		"LUMIO_DASHBOARD_ORIGIN": "http://localhost:3000", "LUMIO_BASE_DOMAIN": "localhost",
		"LUMIO_S3_ENDPOINT": "http://localhost:3900", "LUMIO_S3_PUBLIC_ENDPOINT": "http://localhost:3900",
		"LUMIO_S3_REGION": "garage", "LUMIO_S3_BUCKET": "lumio",
		"LUMIO_S3_ACCESS_KEY": "GKtest", "LUMIO_S3_SECRET_KEY": "test-secret",
	} {
		t.Setenv(key, value)
	}
	cfg, err := env.ParseEnv[config]("lumio")
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.validate(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*config)
	}{
		{"insecure remote origin", func(c *config) { c.DashboardOrigin = "http://app.example.com" }},
		{"origin with path", func(c *config) { c.DashboardOrigin = "https://app.example.com/" }},
		{"unrelated dashboard", func(c *config) { c.DashboardOrigin = "https://evil.example.com"; c.BaseDomain = "example.com" }},
		{"invalid base domain", func(c *config) { c.BaseDomain = "*.example.com" }},
		{"invalid address", func(c *config) { c.ServeRESTAddress = "localhost" }},
		{"invalid HTTP port", func(c *config) { c.ServeRESTAddress = ":70000" }},
		{"empty database", func(c *config) { c.DBName = " " }},
		{"empty password", func(c *config) { c.DBPassword = "" }},
		{"invalid database port", func(c *config) { c.DBPort = 0 }},
		{"invalid pool size", func(c *config) { c.DBMaxConn = 0 }},
		{"invalid lifetime", func(c *config) { c.DBConnLifetime = -time.Second }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			invalid := *cfg
			tc.change(&invalid)
			if invalid.validate() == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
	for _, tc := range []struct {
		name, value, want string
		unset             bool
	}{
		{name: "metrics default", want: "127.0.0.1:9090", unset: true},
		{name: "metrics disabled", value: "", want: ""},
		{name: "metrics explicit address", value: "127.0.0.1:9091", want: "127.0.0.1:9091"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("LUMIO_METRICS_ADDRESS", tc.value)
			if tc.unset {
				if err := os.Unsetenv("LUMIO_METRICS_ADDRESS"); err != nil {
					t.Fatal(err)
				}
			}
			parsed, err := loadConfig()
			if err != nil {
				t.Fatal(err)
			}
			if parsed.MetricsAddress != tc.want {
				t.Fatalf("metrics address = %q, want %q", parsed.MetricsAddress, tc.want)
			}
			if err := parsed.validate(); err != nil {
				t.Fatal(err)
			}
		})
	}
	t.Setenv("LUMIO_DB_CONN_LIFETIME", "invalid")
	if _, err := env.ParseEnv[config]("lumio"); err == nil {
		t.Fatal("invalid environment duration accepted")
	}
}

func TestS3EnvironmentRequired(t *testing.T) {
	for _, name := range []string{"S3_ENDPOINT", "S3_PUBLIC_ENDPOINT", "S3_REGION", "S3_BUCKET", "S3_ACCESS_KEY", "S3_SECRET_KEY"} {
		t.Run(name, func(t *testing.T) {
			for _, field := range []string{"S3_ENDPOINT", "S3_PUBLIC_ENDPOINT", "S3_REGION", "S3_BUCKET", "S3_ACCESS_KEY", "S3_SECRET_KEY"} {
				t.Setenv("LUMIO_"+field, "value")
			}
			t.Setenv("LUMIO_DB_HOST", "localhost")
			t.Setenv("LUMIO_DB_NAME", "lumio")
			t.Setenv("LUMIO_DB_USER", "lumio")
			t.Setenv("LUMIO_DB_PASSWORD", "test")
			if _, err := env.ParseEnv[config]("lumio"); err != nil {
				t.Fatal(err)
			}
			if err := os.Unsetenv("LUMIO_" + name); err != nil {
				t.Fatal(err)
			}
			if _, err := env.ParseEnv[config]("lumio"); err == nil {
				t.Fatalf("missing LUMIO_%s accepted", name)
			}
		})
	}
}
