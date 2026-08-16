package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds runtime configuration for the cardboard music server.
type Config struct {
	Port              string
	StaticDir         string
	R2Endpoint        string
	R2Region          string
	R2AccessKeyID     string
	R2SecretAccessKey string
	R2Bucket          string
	R2PublicURLPrefix string
	R2PresignTTL      time.Duration
}

// LoadFromEnv reads configuration from environment variables and returns a validated Config.
func LoadFromEnv() (*Config, error) {
	cfg := &Config{
		Port:              getEnv("PORT", "8080"),
		StaticDir:         getEnv("STATIC_DIR", "./static"),
		R2Region:          getEnv("R2_REGION", "auto"),
		R2Endpoint:        os.Getenv("R2_ENDPOINT"),
		R2AccessKeyID:     os.Getenv("R2_ACCESS_KEY_ID"),
		R2SecretAccessKey: os.Getenv("R2_SECRET_ACCESS_KEY"),
		R2Bucket:          os.Getenv("R2_BUCKET"),
		R2PublicURLPrefix: os.Getenv("R2_PUBLIC_URL_PREFIX"),
		R2PresignTTL:      1 * time.Hour,
	}

	if v := os.Getenv("R2_PRESIGN_TTL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("invalid R2_PRESIGN_TTL: %w", err)
		}
		cfg.R2PresignTTL = d
	}

	if cfg.R2Endpoint == "" {
		return nil, fmt.Errorf("R2_ENDPOINT must be set")
	}
	if cfg.R2AccessKeyID == "" {
		return nil, fmt.Errorf("R2_ACCESS_KEY_ID must be set")
	}
	if cfg.R2SecretAccessKey == "" {
		return nil, fmt.Errorf("R2_SECRET_ACCESS_KEY must be set")
	}
	if cfg.R2Bucket == "" {
		return nil, fmt.Errorf("R2_BUCKET must be set")
	}
	if cfg.R2PublicURLPrefix == "" {
		return nil, fmt.Errorf("R2_PUBLIC_URL_PREFIX must be set")
	}

	// Ensure port is numeric so that ":" + Port is valid.
	if _, err := strconv.Atoi(cfg.Port); err != nil {
		return nil, fmt.Errorf("invalid PORT %q: %w", cfg.Port, err)
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
