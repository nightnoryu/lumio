package main

import (
	"errors"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nightnoryu/go-kita/jsonlog"
	"github.com/nightnoryu/go-kita/postgresql"
)

const localhost = "localhost"

type config struct {
	DashboardOrigin  string        `env:"DASHBOARD_ORIGIN" envDefault:"http://localhost:3000"`
	BaseDomain       string        `env:"BASE_DOMAIN" envDefault:"localhost"`
	ServeRESTAddress string        `env:"SERVE_REST_ADDRESS" envDefault:":8080"`
	LogLevel         jsonlog.Level `env:"LOG_LEVEL" envDefault:"info"`
	DBHost           string        `env:"DB_HOST,required"`
	DBPort           int           `env:"DB_PORT" envDefault:"5432"`
	DBName           string        `env:"DB_NAME,required"`
	DBUser           string        `env:"DB_USER,required"`
	DBPassword       string        `env:"DB_PASSWORD,required"`
	DBMaxConn        int           `env:"DB_MAX_CONN" envDefault:"10"`
	DBConnLifetime   time.Duration `env:"DB_CONN_LIFETIME" envDefault:"60s"`
}

func (c *config) validate() error {
	origin, originErr := url.Parse(c.DashboardOrigin)
	if originErr != nil || origin.Host == "" || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" || origin.ForceQuery ||
		(origin.Scheme != "https" && (origin.Scheme != "http" || origin.Hostname() != localhost)) {
		return errors.New("LUMIO_DASHBOARD_ORIGIN must be an HTTPS origin (HTTP is allowed only for localhost)")
	}
	if len(c.BaseDomain) > 189 || !regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)*$`).MatchString(c.BaseDomain) {
		return errors.New("LUMIO_BASE_DOMAIN must be a lowercase DNS name")
	}
	for _, label := range strings.Split(c.BaseDomain, ".") {
		if len(label) > 63 {
			return errors.New("LUMIO_BASE_DOMAIN labels must not exceed 63 characters")
		}
	}
	if origin.Hostname() != localhost && origin.Hostname() != "app."+c.BaseDomain {
		return errors.New("LUMIO_DASHBOARD_ORIGIN must use app.LUMIO_BASE_DOMAIN")
	}

	_, port, err := net.SplitHostPort(c.ServeRESTAddress)
	if err != nil {
		return errors.New("LUMIO_SERVE_REST_ADDRESS must be a host:port address")
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return errors.New("LUMIO_SERVE_REST_ADDRESS port must be between 1 and 65535")
	}
	if strings.TrimSpace(c.DBHost) == "" || strings.TrimSpace(c.DBName) == "" ||
		strings.TrimSpace(c.DBUser) == "" || c.DBPassword == "" {
		return errors.New("LUMIO_DB_HOST, LUMIO_DB_NAME, LUMIO_DB_USER and LUMIO_DB_PASSWORD must be nonempty")
	}
	if c.DBPort < 1 || c.DBPort > 65535 {
		return errors.New("LUMIO_DB_PORT must be between 1 and 65535")
	}
	if c.DBMaxConn < 1 || c.DBConnLifetime <= 0 {
		return errors.New("LUMIO_DB_MAX_CONN and LUMIO_DB_CONN_LIFETIME must be positive")
	}
	return nil
}

func (c *config) postgresDSN() postgresql.DSN {
	return postgresql.DSN{
		Host: c.DBHost, Port: c.DBPort, Database: c.DBName,
		User: c.DBUser, Password: c.DBPassword,
	}
}
