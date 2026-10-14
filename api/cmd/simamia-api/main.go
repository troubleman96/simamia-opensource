package main

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"simamia/api"
)

func main() {
	level := new(slog.LevelVar)
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL"))) {
	case "debug":
		level.Set(slog.LevelDebug)
	case "warn", "warning":
		level.Set(slog.LevelWarn)
	case "error":
		level.Set(slog.LevelError)
	default:
		level.Set(slog.LevelInfo)
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	databasePath := os.Getenv("SIMAMIA_DB_PATH")
	if databasePath == "" {
		databasePath = os.Getenv("HUDUMADESK_DB_PATH") // Backward compatibility for existing deployments.
	}
	if databasePath == "" {
		databasePath = "data/simamia.db"
		if _, err := os.Stat(databasePath); errors.Is(err, os.ErrNotExist) {
			if _, legacyErr := os.Stat("data/hudumadesk.db"); legacyErr == nil {
				databasePath = "data/hudumadesk.db"
			}
		}
	}
	handler, closeStore, err := api.NewHTTPHandler(databasePath)
	if err != nil {
		slog.Error("startup_failed", "component", "storage", "error", err)
		os.Exit(1)
	}
	defer closeStore()
	listenAddress := strings.TrimSpace(os.Getenv("SIMAMIA_LISTEN_ADDRESS"))
	if listenAddress == "" {
		listenAddress = "0.0.0.0"
	}
	srv := &http.Server{Addr: net.JoinHostPort(listenAddress, port), Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	slog.Info("service_started", "service", "simamia-api", "address", srv.Addr, "database", databasePath)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("service_stopped_unexpectedly", "error", err)
		os.Exit(1)
	}
}
