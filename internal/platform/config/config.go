// Package config loads settings from environment variables.
package config

import (
	"errors"
	"os"
	"time"
)

type Config struct {
	Addr            string
	DatabaseURL     string
	JWTSecret       []byte
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	// Dev enables the log mailer (OTP codes printed to the log).
	Dev bool
}

func Load() (Config, error) {
	cfg := Config{
		Addr:            getenv("ADDR", ":8080"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		JWTSecret:       []byte(os.Getenv("JWT_SECRET")),
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 60 * 24 * time.Hour,
		Dev:             os.Getenv("APP_ENV") == "dev",
	}
	if cfg.DatabaseURL == "" {
		return cfg, errors.New("DATABASE_URL is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return cfg, errors.New("JWT_SECRET must be at least 32 bytes")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
