// Package config loads runtime configuration from environment variables.
package config

import "os"

// Config holds all environment-derived settings for the server.
type Config struct {
	Port          string
	DatabaseURL   string
	AllowedOrigin string
}

// Load reads configuration from the environment, applying defaults for local dev.
func Load() Config {
	return Config{
		Port:          getEnv("PORT", "8080"),
		DatabaseURL:   getEnv("DATABASE_URL", ""),
		AllowedOrigin: getEnv("ALLOWED_ORIGIN", "*"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
