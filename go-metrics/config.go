package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr             string
	DatabaseURL      string
	Tickers          []string
	Window           time.Duration
	MaxLag           time.Duration
	MinTicksInWindow int64

	DBPingTimeout     time.Duration
	StartupTimeout    time.Duration
	StartupInterval   time.Duration
	ShutdownTimeout   time.Duration
	ReadHeaderTimeout time.Duration
}

func LoadConfig() Config {
	cfg := Config{
		Addr:             getEnvString("ADDR", ":9100"),
		DatabaseURL:      mustEnv("DATABASE_URL"),
		Tickers:          parseTickers(mustEnv("TICKERS")),
		Window:           time.Duration(getEnvInt64("WINDOW_SECONDS", 300)) * time.Second,
		MaxLag:           time.Duration(getEnvInt64("MAX_LAG_SECONDS", 120)) * time.Second,
		MinTicksInWindow: getEnvInt64("MIN_TICKS_IN_WINDOW", 4),

		DBPingTimeout:     2 * time.Second,
		StartupTimeout:    30 * time.Second,
		StartupInterval:   500 * time.Millisecond,
		ShutdownTimeout:   10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}

	if len(cfg.Tickers) == 0 {
		log.Fatal("TICKERS is required, example: TICKERS=btc_usd,eth_usd")
	}

	return cfg
}

func mustEnv(key string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		log.Fatalf("%s is required", key)
	}
	return v
}

func getEnvString(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return v
}

func getEnvInt64(key string, def int64) int64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		log.Printf("invalid %s=%q, using default %d", key, raw, def)
		return def
	}
	return n
}

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

func (c Config) String() string {
	return fmt.Sprintf("addr=%s tickers=%v window=%s maxLag=%s minTicks=%d",
		c.Addr, c.Tickers, c.Window, c.MaxLag, c.MinTicksInWindow)
}

func (c Config) WindowSeconds() int64 {
	return int64(c.Window / time.Second)
}

func (c Config) MaxLagSeconds() int64 {
	return int64(c.MaxLag / time.Second)
}
