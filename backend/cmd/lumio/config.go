package main

import (
	"errors"
	"net"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nightnoryu/go-kita/env"
	"github.com/nightnoryu/go-kita/jsonlog"
	"github.com/nightnoryu/go-kita/postgresql"
)

const localhost = "localhost"

type config struct {
	MetricsAddress    string   `env:"METRICS_ADDRESS"`
	TrustedProxyCIDRs []string `env:"TRUSTED_PROXY_CIDRS" envSeparator:","`
	S3Endpoint        string   `env:"S3_ENDPOINT" envDefault:"http://localhost:9000"`
	S3PublicEndpoint  string   `env:"S3_PUBLIC_ENDPOINT" envDefault:"http://localhost:9000"`
	S3Region          string   `env:"S3_REGION" envDefault:"us-east-1"`
	S3Bucket          string   `env:"S3_BUCKET" envDefault:"lumio"`
	S3AccessKey       string   `env:"S3_ACCESS_KEY" envDefault:"lumio-local"`
	S3SecretKey       string   `env:"S3_SECRET_KEY" envDefault:"lumio-local-only"`
	MediaFileBytes    int64    `env:"MEDIA_FILE_BYTES" envDefault:"52428800"`
	MediaStorageBytes int64    `env:"MEDIA_STORAGE_BYTES" envDefault:"2147483648"`
	MediaPhotos       int      `env:"MEDIA_PHOTOS" envDefault:"100"`

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

func loadConfig() (*config, error) {
	cfg, err := env.ParseEnv[config](appID)
	if err != nil {
		return nil, err
	}
	// envDefault also replaces explicitly empty values, which disable metrics.
	if _, present := os.LookupEnv("LUMIO_METRICS_ADDRESS"); !present {
		cfg.MetricsAddress = "127.0.0.1:9090"
	}
	return cfg, nil
}

func (c *config) validate() error {
	if err := c.validateServices(); err != nil {
		return err
	}
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

func (c *config) validateMedia() error {
	if c.MediaFileBytes <= 0 || c.MediaFileBytes > 524288000 || c.MediaStorageBytes < c.MediaFileBytes || c.MediaPhotos <= 0 {
		return errors.New("invalid media limits")
	}
	for _, endpoint := range []string{c.S3Endpoint, c.S3PublicEndpoint} {
		u, err := url.Parse(endpoint)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
			return errors.New("invalid S3 endpoint")
		}
	}
	if c.S3Bucket == "" || c.S3Region == "" || c.S3AccessKey == "" || c.S3SecretKey == "" {
		return errors.New("S3 bucket, region and credentials are required")
	}

	return nil
}

func (c *config) trustedProxies() []*net.IPNet {
	ranges := make([]*net.IPNet, 0, len(c.TrustedProxyCIDRs))
	for _, value := range c.TrustedProxyCIDRs {
		_, network, err := net.ParseCIDR(value)
		if err == nil {
			ranges = append(ranges, network)
		}
	}
	return ranges
}

func (c *config) validateServices() error {
	if c.MetricsAddress != "" {
		_, port, err := net.SplitHostPort(c.MetricsAddress)
		number, parseErr := strconv.Atoi(port)
		if err != nil || parseErr != nil || number < 1 || number > 65535 {
			return errors.New("LUMIO_METRICS_ADDRESS must be a host:port address")
		}
	}
	for _, value := range c.TrustedProxyCIDRs {
		if _, _, err := net.ParseCIDR(value); err != nil {
			return errors.New("invalid LUMIO_TRUSTED_PROXY_CIDRS")
		}
	}
	return c.validateMedia()
}
