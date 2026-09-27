package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/nightnoryu/go-kita/jsonlog"
)

const appID = "lumio"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if validationErr := cfg.validate(); validationErr != nil {
		return validationErr
	}
	logger, err := jsonlog.NewLogger(&jsonlog.Config{AppName: appID, Level: cfg.LogLevel})
	if err != nil {
		return err
	}
	defer func() { _ = logger.Sync() }()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return service(ctx, cfg, logger)
}
