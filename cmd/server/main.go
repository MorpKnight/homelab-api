package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"homelab-api/internal/beszel"
	"homelab-api/internal/config"
	"homelab-api/internal/httpapi"
	"homelab-api/internal/service"
	"homelab-api/internal/uptimekuma"
)

func main() {
	logger := log.New(os.Stdout, "homelab-api ", log.LstdFlags|log.LUTC)
	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("load configuration: %v", err)
	}

	upstreamClient := &http.Client{Timeout: cfg.UpstreamTimeout}
	beszelClient := beszel.NewClient(beszel.Config{
		BaseURL:                  cfg.BeszelURL,
		Identity:                 cfg.BeszelIdentity,
		Password:                 cfg.BeszelPassword,
		SystemsCollection:        cfg.BeszelSystemsCollection,
		SystemStatsCollection:    cfg.BeszelSystemStatsCollection,
		ContainersCollection:     cfg.BeszelContainersCollection,
		ContainerStatsCollection: cfg.BeszelContainerStats,
		RealtimeCollection:       cfg.BeszelRealtimeCollection,
		AlertsCollection:         cfg.BeszelAlertsCollection,
		HTTPClient:               upstreamClient,
	})
	kumaClient := uptimekuma.NewClient(cfg.UptimeKumaURL, cfg.UptimeKumaAPIKey, upstreamClient)
	store := service.NewStore(beszelClient, kumaClient)

	logger.Printf("beszel integration configured=%t uptime_kuma integration configured=%t", beszelClient.Configured(), kumaClient.Configured())

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go store.Run(ctx, cfg.PollInterval)

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           httpapi.NewHandler(store, cfg.APIToken),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Printf("listening on :%d", cfg.Port)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			logger.Fatalf("http server: %v", err)
		}
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			logger.Printf("shutdown: %v", err)
		}
	}
}
