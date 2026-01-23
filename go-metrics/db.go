package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type stat struct {
	lastTs int64
	cnt    int64
}

const fetchTickerStatsQuery = `
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

func fetchTickerStats(
	ctx context.Context,
	pool *pgxpool.Pool,
	tickers []string,
	fromTs int64,
) (map[string]stat, int64, error) {
	rows, err := pool.Query(ctx, fetchTickerStatsQuery, tickers, fromTs)
	if err != nil {
		return nil, 0, fmt.Errorf("query ticker stats: %w", err)
	}
	defer rows.Close()

	stats := make(map[string]stat, len(tickers))
	var lastOverall int64 = 0

	for rows.Next() {
		var t string
		var lastTs int64
		var cnt int64

		if err := rows.Scan(&t, &lastTs, &cnt); err != nil {
			return nil, 0, fmt.Errorf("scan ticker stats row: %w", err)
		}

		stats[t] = stat{lastTs: lastTs, cnt: cnt}
		if lastTs > lastOverall {
			lastOverall = lastTs
		}
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("ticker stats rows error: %w", err)
	}

	return stats, lastOverall, nil
}

func waitForDB(
	ctx context.Context,
	pool *pgxpool.Pool,
	interval time.Duration,
	pingTimeout time.Duration,
) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
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
