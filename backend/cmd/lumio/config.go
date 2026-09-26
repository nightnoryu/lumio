package main

import (
	"errors"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/nightnoryu/go-kita/jsonlog"
	"github.com/nightnoryu/go-kita/postgresql"
)

type config struct {
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
