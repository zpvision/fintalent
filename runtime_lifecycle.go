package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func configureDatabasePool() error {
	maxOpen := 16
	if raw := os.Getenv("DB_MAX_OPEN_CONNS"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 8 || value > 128 {
			return errors.New("DB_MAX_OPEN_CONNS должен быть от 8 до 128")
		}
		maxOpen = value
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxOpen / 2)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)
	return nil
}

func serveUntilShutdown(server *http.Server) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	workerCtx, cancelWorkers := context.WithCancel(context.Background())
	workers := startNotificationWorkers(workerCtx)
	serverErr := make(chan error, 1)
	go func() { serverErr <- server.ListenAndServe() }()
	var result error
	select {
	case result = <-serverErr:
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Print("HTTP shutdown deadline exceeded")
		_ = server.Close()
	}
	cancel()
	cancelWorkers()
	workers.Wait()
	if db != nil {
		_ = db.Close()
	}
	if errors.Is(result, http.ErrServerClosed) {
		return nil
	}
	return result
}

func healthReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(405)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	w.Header().Set("Cache-Control", "no-store")
	if db == nil || db.PingContext(ctx) != nil {
		w.WriteHeader(503)
		return
	}
	w.WriteHeader(204)
}
