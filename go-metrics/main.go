package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func parseTickers(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	if strings.HasPrefix(raw, "[") {
		var arr []string
		if err := json.Unmarshal([]byte(raw), &arr); err == nil {
			out := make([]string, 0, len(arr))
			for _, t := range arr {
				t = strings.TrimSpace(t)
				if t != "" {
					out = append(out, t)
				}
			}
			return out
		}
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func parseInt64Env(key string, def int64) int64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return def
	}
	return n
}

type stat struct {
	lastTs int64
	cnt    int64
}

func fetchTickerStats(
	ctx context.Context,
	pool *pgxpool.Pool,
	tickers []string,
	fromTs int64,
) (map[string]stat, int64, error) {
	query := `
			WITH wanted AS (
				SELECT unnest($1::text[]) AS ticker
			)
			SELECT
				w.ticker,
				COALESCE(MAX(p.ts), 0) AS last_ts,
				COUNT(p.*) FILTER (WHERE p.ts >= $2) AS cnt_window
			FROM wanted w
			LEFT JOIN price_ticks p ON p.ticker = w.ticker
			GROUP BY w.ticker
			ORDER BY w.ticker;
			`

	rows, err := pool.Query(ctx, query, tickers, fromTs)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	stats := make(map[string]stat, len(tickers))
	var lastOverall int64 = 0

	for rows.Next() {
		var t string
		var lastTs int64
		var cnt int64

		if err := rows.Scan(&t, &lastTs, &cnt); err != nil {
			return nil, 0, err
		}

		stats[t] = stat{lastTs: lastTs, cnt: cnt}
		if lastTs > lastOverall {
			lastOverall = lastTs
		}
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return stats, lastOverall, nil
}

func waitForDB(ctx context.Context, pool *pgxpool.Pool, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := pool.Ping(pingCtx)
		cancel()

		if err == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	tickers := parseTickers(os.Getenv("TICKERS"))
	if len(tickers) == 0 {
		log.Fatal("TICKERS is required, example: TICKERS=btc_usd,eth_usd")
	}

	windowSeconds := parseInt64Env("WINDOW_SECONDS", 300)
	maxLagSeconds := parseInt64Env("MAX_LAG_SECONDS", 120)
	minTicksInWindow := parseInt64Env("MIN_TICKS_IN_WINDOW", 4)

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("failed to create pg pool: %v", err)
	}
	defer pool.Close()

	startupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Printf("waiting for db...")
	if err := waitForDB(startupCtx, pool, 500*time.Millisecond); err != nil {
		log.Fatalf("db is not ready: %v", err)
	}
	log.Printf("db is ready")

	log.Printf("tickers=%v", tickers)

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := pool.Ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"db_unavailable"}`))
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		now := time.Now().Unix()

		dbUp := 0

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := pool.Ping(ctx); err == nil {
			dbUp = 1
		}

		fmt.Fprintf(w, "crypto_client_db_up %d\n", dbUp)

		if dbUp != 1 {
			return
		}

		fromTs := now - windowSeconds

		stats, lastOverall, err := fetchTickerStats(ctx, pool, tickers, fromTs)
		if err != nil {
			log.Printf("failed to query ticker stats: %v", err)
			fmt.Fprintf(w, "crypto_client_metrics_query_ok 0\n")
			return
		}
		fmt.Fprintf(w, "crypto_client_metrics_query_ok 1\n")

		for _, t := range tickers {
			s, ok := stats[t]
			if !ok {
				s = stat{lastTs: 0, cnt: 0}
			}

			freshness := int64(0)
			if s.lastTs > 0 {
				freshness = now - s.lastTs
			}

			fmt.Fprintf(w, "crypto_client_last_tick_ts{ticker=%q} %d\n", t, s.lastTs)
			fmt.Fprintf(w, "crypto_client_ticks_freshness_seconds{ticker=%q} %d\n", t, freshness)
			fmt.Fprintf(
				w,
				"crypto_client_ticks_last_window_total{ticker=%q,window_seconds=%q} %d\n",
				t,
				fmt.Sprintf("%d", windowSeconds),
				s.cnt,
			)

			fmt.Fprintf(w, "crypto_client_ingestion_lag_seconds{ticker=%q} %d\n", t, freshness)

			ingestionOK := int64(0)
			if (s.lastTs > 0) && (freshness <= maxLagSeconds) && (s.cnt >= minTicksInWindow) {
				ingestionOK = 1
			}
			fmt.Fprintf(
				w,
				"crypto_client_ingestion_ok{ticker=%q,max_lag_seconds=%q,min_ticks_in_window=%q,window_seconds=%q} %d\n",
				t,
				fmt.Sprintf("%d", maxLagSeconds),
				fmt.Sprintf("%d", minTicksInWindow),
				fmt.Sprintf("%d", windowSeconds),
				ingestionOK,
			)
		}

		overallFreshness := int64(0)
		if lastOverall > 0 {
			overallFreshness = now - lastOverall
		}
		fmt.Fprintf(w, "crypto_client_last_tick_ts %d\n", lastOverall)
		fmt.Fprintf(w, "crypto_client_ticks_freshness_seconds %d\n", overallFreshness)
	})

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().Unix()
		fromTs := now - windowSeconds

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := pool.Ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"db_unavailable"}`))
			return
		}

		stats, _, err := fetchTickerStats(ctx, pool, tickers, fromTs)
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"stats_unavailable"}`))
			return
		}

		for _, t := range tickers {
			s := stats[t]

			if s.lastTs == 0 {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(fmt.Sprintf(
					`{"status":"no_data","ticker":"%s"}`,
					t,
				)))
				return
			}

			freshness := now - s.lastTs
			if freshness > maxLagSeconds || s.cnt < minTicksInWindow {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(fmt.Sprintf(
					`{"status":"ingestion_lagging","ticker":"%s","freshness_seconds":%d,"last_ts":%d,"cnt_window":%d,"max_lag_seconds":%d,"min_ticks_in_window":%d,"window_seconds":%d}`,
					t, freshness, s.lastTs, s.cnt, maxLagSeconds, minTicksInWindow, windowSeconds,
				)))
				return
			}
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	})

	addr := ":9100"

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("metrics server listening on %s", addr)
		errCh <- server.ListenAndServe()
	}()

	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case <-sigCtx.Done():
		log.Printf("shutdown signal received")
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
		if err := server.Close(); err != nil {
			log.Printf("server close failed: %v", err)
		}
	}

	log.Printf("server stopped")
}
