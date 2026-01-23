package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func parseTickers(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	// 1) JSON формат: ["btc_usd","eth_usd"]
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
		// если JSON битый — упадём ниже в CSV как fallback
	}

	// 2) CSV формат: btc_usd,eth_usd
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

func parseWindowSeconds(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 300
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return 300
	}
	return n
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

	windowSeconds := parseWindowSeconds(os.Getenv("WINDOW_SECONDS"))

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("failed to create pg pool: %v", err)
	}
	defer pool.Close()

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
		var lastOverall int64 = 0

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := pool.Ping(ctx); err == nil {
			dbUp = 1

			row := pool.QueryRow(ctx, "SELECT COALESCE(MAX(ts), 0) FROM price_ticks;")
			if err := row.Scan(&lastOverall); err != nil {
				log.Printf("failed to query overall last ts: %v", err)
			}
		}

		overallFreshness := int64(0)
		if lastOverall > 0 {
			overallFreshness = now - lastOverall
		}

		fmt.Fprintf(w, "crypto_client_db_up %d\n", dbUp)
		fmt.Fprintf(w, "crypto_client_last_tick_ts %d\n", lastOverall)
		fmt.Fprintf(w, "crypto_client_ticks_freshness_seconds %d\n", overallFreshness)

		if dbUp == 1 {
			fromTs := now - windowSeconds

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
				log.Printf("failed to query ticker stats: %v", err)
			} else {
				defer rows.Close()

				type stat struct {
					lastTs int64
					cnt    int64
				}
				stats := make(map[string]stat, len(tickers))

				for rows.Next() {
					var t string
					var lastTs int64
					var cnt int64

					if err := rows.Scan(&t, &lastTs, &cnt); err != nil {
						log.Printf("failed to scan ticker stats row: %v", err)
						continue
					}
					stats[t] = stat{lastTs: lastTs, cnt: cnt}
				}

				if err := rows.Err(); err != nil {
					log.Printf("rows error: %v", err)
				}

				var lastOverall int64 = 0
				for _, t := range tickers {
					s, ok := stats[t]
					if !ok {
						s = stat{lastTs: 0, cnt: 0}
					}

					if s.lastTs > lastOverall {
						lastOverall = s.lastTs
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
				}

				overallFreshness := int64(0)
				if lastOverall > 0 {
					overallFreshness = now - lastOverall
				}
				fmt.Fprintf(w, "crypto_client_last_tick_ts %d\n", lastOverall)
				fmt.Fprintf(w, "crypto_client_ticks_freshness_seconds %d\n", overallFreshness)
			}

		}
	})

	addr := ":9100"
	log.Printf("metrics server listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
