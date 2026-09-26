package main

import (
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
		"LUMIO_LOG_LEVEL": "info",
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
	t.Setenv("LUMIO_DB_CONN_LIFETIME", "invalid")
	if _, err := env.ParseEnv[config]("lumio"); err == nil {
		t.Fatal("invalid environment duration accepted")
	}
}
