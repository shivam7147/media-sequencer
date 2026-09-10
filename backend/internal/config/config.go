// Package config loads runtime configuration from environment variables.
package config

import "os"

// Config holds all environment-derived settings for the server.
type Config struct {
	Port        string
	DatabaseURL string
	// AllowedOrigins is a comma-separated list of exact origins the CORS
	// middleware will echo back — see internal/middleware.CORS. Never "*":
	// the fallback below is a concrete origin, not a wildcard, so an
	// unconfigured deployment fails closed (blocks cross-origin requests)
	// rather than silently allowing everyone.
	AllowedOrigins string
}

// Load reads configuration from the environment, applying defaults for local dev.
func Load() Config {
	return Config{
		Port:           getEnv("PORT", "8080"),
		DatabaseURL:    getEnv("DATABASE_URL", ""),
		AllowedOrigins: getEnv("ALLOWED_ORIGIN", "http://localhost:5173"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
