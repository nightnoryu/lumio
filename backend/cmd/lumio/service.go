package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/nightnoryu/go-kita/log"
	"github.com/nightnoryu/go-kita/postgresql"

	httptransport "lumio/internal/transport/http"
	"lumio/internal/webui"
)

func service(ctx context.Context, cfg *config, logger log.Logger) error {
	db := postgresql.NewConnector()
	if err := db.Open(ctx, cfg.postgresDSN(), postgresql.Config{
		MaxOpenConnections:    cfg.DBMaxConn,
		MaxIdleConnections:    cfg.DBMaxConn,
		ConnectionMaxLifetime: cfg.DBConnLifetime,
		ConnectTimeout:        5 * time.Second,
	}); err != nil {
		return fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error(err, "close PostgreSQL")
		}
	}()
	assets, err := webui.Assets()
	if err != nil {
		return err
	}
	router, err := httptransport.NewRouter(assets, db.Ping, logger)
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr: cfg.ServeRESTAddress, Handler: router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second, WriteTimeout: 30 * time.Second,
		IdleTimeout: 60 * time.Second,
	}
	result := make(chan error, 1)
	go func() { result <- server.ListenAndServe() }()
	logger.Info("HTTP server listening on " + cfg.ServeRESTAddress)
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		// The signal context is canceled; shutdown needs its own deadline.
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return errors.Join(err, server.Close())
		}
		if err := <-result; !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}
