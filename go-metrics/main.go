package main

import (
	"context"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg := LoadConfig()
	log.Printf("config: %s", cfg.String())

	rootCtx := context.Background()

	pool, err := pgxpool.New(rootCtx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to create pg pool: %v", err)
	}
	defer pool.Close()

	startupCtx, cancel := context.WithTimeout(rootCtx, cfg.StartupTimeout)
	defer cancel()

	log.Printf("waiting for db...")
	if err := waitForDB(startupCtx, pool, cfg.StartupInterval, cfg.DBPingTimeout); err != nil {
		log.Fatalf("db is not ready: %v", err)
	}
	log.Printf("db is ready")

	mux := http.NewServeMux()
	registerRoutes(mux, pool, cfg)

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
	}

	if err := runHTTPServer(rootCtx, server, cfg.ShutdownTimeout); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
