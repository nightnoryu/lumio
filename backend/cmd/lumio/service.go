package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/nightnoryu/go-kita/log"
	"github.com/nightnoryu/go-kita/postgresql"

	"lumio/data/migrations"
	"lumio/internal/app"
	"lumio/internal/domain"
	"lumio/internal/infrastructure/imaging"
	"lumio/internal/infrastructure/observability"
	"lumio/internal/infrastructure/password"
	"lumio/internal/infrastructure/postgres"
	"lumio/internal/infrastructure/storage"
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
	migrator, err := db.Migrator(logger, migrations.UpFS)
	if err != nil {
		return err
	}
	if err = migrator.MigrateUp(ctx); err != nil {
		return err
	}
	if len(os.Args) == 2 && os.Args[1] == "migrate" {
		return nil
	}
	identity := &app.Service{Store: &postgres.Store{DB: db.TransactionalClient()}, Passwords: password.Argon{}}
	if len(os.Args) > 1 && os.Args[1] != "worker" && os.Args[1] != "storage-init" {
		return operatorCommand(ctx, identity, cfg, os.Args[1:])
	}
	objects, err := storage.New(ctx, cfg.S3Endpoint, cfg.S3PublicEndpoint, cfg.S3Region, cfg.S3Bucket, cfg.S3AccessKey, cfg.S3SecretKey)
	if err != nil {
		return err
	}
	if len(os.Args) > 1 && os.Args[1] == "storage-init" {
		return objects.Initialize(ctx)
	}
	if err = objects.VerifyLifecycle(ctx); err != nil {
		return err
	}
	metrics := observability.New()
	if cfg.MetricsAddress != "" {
		closeMetrics, metricsErr := startMetrics(ctx, cfg.MetricsAddress, metrics, logger)
		if metricsErr != nil {
			return fmt.Errorf("start metrics: %w", metricsErr)
		}
		defer closeMetrics()
	}
	media := &app.Media{Observe: func(operation, outcome string, duration time.Duration) {
		metrics.Media(operation, outcome, duration)
		logger.WithFields(log.Fields{"operation": operation, "outcome": outcome, "duration_ms": duration.Milliseconds()}).Info("media operation")
	}, Store: &postgres.Store{DB: db.TransactionalClient()}, Objects: objects, Limits: domain.MediaLimits{FileBytes: cfg.MediaFileBytes, StorageBytes: cfg.MediaStorageBytes, Photos: cfg.MediaPhotos}}
	if len(os.Args) > 1 && os.Args[1] == "worker" {
		if _, err = exec.LookPath("vips"); err != nil {
			return fmt.Errorf("worker requires libvips: %w", err)
		}
		return media.Work(ctx, imaging.Vips{}, func(err error) { logger.Error(err, "media worker") })
	}
	assets, err := webui.Assets()
	if err != nil {
		return err
	}
	router, err := httptransport.NewRouter(assets, db.Ping, logger, httptransport.APIConfig{Service: identity, Media: media, Portfolio: &app.Portfolio{Store: &postgres.Store{DB: db.TransactionalClient()}, Media: media}, Origin: cfg.DashboardOrigin, BaseDomain: cfg.BaseDomain, StorageOrigin: cfg.S3PublicEndpoint, TrustedProxies: cfg.trustedProxies(), Metrics: metrics})
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
