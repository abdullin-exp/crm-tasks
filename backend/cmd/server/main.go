package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"crm-tasks/internal/config"
	"crm-tasks/internal/db"
	"crm-tasks/internal/httpapi"
	"crm-tasks/internal/kafka"
	"crm-tasks/internal/seed"
	"crm-tasks/internal/service"
	"crm-tasks/internal/store"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		return err
	}

	st, err := store.New(pool, cfg.CRMSchema)
	if err != nil {
		return err
	}
	if cfg.SeedDemo {
		if err := seed.Run(ctx, st); err != nil {
			return err
		}
	}
	svc := &service.Service{Store: st}

	var kafkaStatus httpapi.KafkaStatus = kafka.Disabled{}
	if cfg.KafkaEnabled {
		c := &kafka.Consumer{Brokers: cfg.KafkaBrokers, Topic: cfg.KafkaTopic, DLQTopic: cfg.KafkaDLQTopic, GroupID: cfg.KafkaGroupID, Service: svc}
		p := &kafka.Publisher{Brokers: cfg.KafkaBrokers, Topic: cfg.KafkaEventsTopic, Store: st}
		kafkaStatus = kafka.Combined{Consumer: c, Publisher: p}
		go c.Run(ctx)
		go p.Run(ctx)
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           (&httpapi.Server{Store: st, Service: svc, CRMURL: cfg.CRMURL, Kafka: kafkaStatus}).Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	slog.Info("http server started", "addr", cfg.HTTPAddr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
