package config

import (
	"os"
	"strconv"
)

type Config struct {
	AppPort          string
	HMACSecret       string
	SigMaxAgeSeconds int64
	SQLiteDSN        string
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getInt64(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

func Load() Config {
	return Config{
		AppPort:          getenv("APP_PORT", "8080"),
		HMACSecret:       getenv("HMAC_SECRET", "supersecret-dev"),
		SigMaxAgeSeconds: getInt64("SIG_MAX_AGE_SECONDS", 300),
		SQLiteDSN:        getenv("SQLITE_DSN", "./app.db"),
	}
}
