package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func registerRoutes(mux *http.ServeMux, pool *pgxpool.Pool, cfg Config) {
	mux.HandleFunc("/health", healthHandler(pool, cfg))
	mux.HandleFunc("/metrics", metricsHandler(pool, cfg))
	mux.HandleFunc("/ready", readyHandler(pool, cfg))
}

func healthHandler(pool *pgxpool.Pool, cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), cfg.DBPingTimeout)
		defer cancel()

		if err := pool.Ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"db_unavailable"}`))
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
}

func metricsHandler(pool *pgxpool.Pool, cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		now := time.Now().Unix()

		ctx, cancel := context.WithTimeout(r.Context(), cfg.DBPingTimeout)
		defer cancel()

		dbUp := 0
		if err := pool.Ping(ctx); err == nil {
			dbUp = 1
		}
		fmt.Fprintf(w, "crypto_client_db_up %d\n", dbUp)

		if dbUp != 1 {
			return
		}

		windowSeconds := int64(cfg.Window / time.Second)
		if windowSeconds <= 0 {
			windowSeconds = 300
		}
		fromTs := now - windowSeconds

		stats, lastOverall, err := fetchTickerStats(ctx, pool, cfg.Tickers, fromTs)
		if err != nil {
			fmt.Fprintf(w, "crypto_client_metrics_query_ok 0\n")
			fmt.Fprintf(w, "crypto_client_metrics_error 1\n")
			return
		}
		fmt.Fprintf(w, "crypto_client_metrics_query_ok 1\n")

		maxLagSeconds := int64(cfg.MaxLag / time.Second)
		if maxLagSeconds <= 0 {
			maxLagSeconds = 120
		}
		minTicks := cfg.MinTicksInWindow
		if minTicks <= 0 {
			minTicks = 4
		}

		for _, t := range cfg.Tickers {
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
			if (s.lastTs > 0) && (freshness <= maxLagSeconds) && (s.cnt >= minTicks) {
				ingestionOK = 1
			}
			fmt.Fprintf(
				w,
				"crypto_client_ingestion_ok{ticker=%q,max_lag_seconds=%q,min_ticks_in_window=%q,window_seconds=%q} %d\n",
				t,
				fmt.Sprintf("%d", maxLagSeconds),
				fmt.Sprintf("%d", minTicks),
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
	}
}

func readyHandler(pool *pgxpool.Pool, cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().Unix()

		windowSeconds := int64(cfg.Window / time.Second)
		if windowSeconds <= 0 {
			windowSeconds = 300
		}
		fromTs := now - windowSeconds

		maxLagSeconds := int64(cfg.MaxLag / time.Second)
		if maxLagSeconds <= 0 {
			maxLagSeconds = 120
		}
		minTicks := cfg.MinTicksInWindow
		if minTicks <= 0 {
			minTicks = 4
		}

		ctx, cancel := context.WithTimeout(r.Context(), cfg.DBPingTimeout)
		defer cancel()

		if err := pool.Ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"db_unavailable"}`))
			return
		}

		stats, _, err := fetchTickerStats(ctx, pool, cfg.Tickers, fromTs)
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"stats_unavailable"}`))
			return
		}

		for _, t := range cfg.Tickers {
			s, ok := stats[t]
			if !ok {
				s = stat{lastTs: 0, cnt: 0}
			}

			if s.lastTs == 0 {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(fmt.Sprintf(`{"status":"no_data","ticker":"%s"}`, t)))
				return
			}

			freshness := now - s.lastTs
			if freshness > maxLagSeconds || s.cnt < minTicks {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(fmt.Sprintf(
					`{"status":"ingestion_lagging","ticker":"%s","freshness_seconds":%d,"last_ts":%d,"cnt_window":%d,"max_lag_seconds":%d,"min_ticks_in_window":%d,"window_seconds":%d}`,
					t, freshness, s.lastTs, s.cnt, maxLagSeconds, minTicks, windowSeconds,
				)))
				return
			}
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	}
}
