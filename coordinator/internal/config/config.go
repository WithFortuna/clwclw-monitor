package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port      int
	AuthToken string
	DatabaseURL string
	EventRetentionDays      int
	RetentionIntervalHours  int
}

func Load() Config {
	cfg := Config{
		Port:                 8080,
		AuthToken:            os.Getenv("COORDINATOR_AUTH_TOKEN"),
		DatabaseURL:          os.Getenv("COORDINATOR_DATABASE_URL"),
		EventRetentionDays:   30,
		RetentionIntervalHours: 24,
	}

	if cfg.DatabaseURL == "" {
		cfg.DatabaseURL = os.Getenv("DATABASE_URL")
	}

	if v := os.Getenv("COORDINATOR_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 && p < 65536 {
			cfg.Port = p
		}
	}

	if v := os.Getenv("COORDINATOR_EVENT_RETENTION_DAYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			cfg.EventRetentionDays = n
		}
	}

	if v := os.Getenv("COORDINATOR_RETENTION_INTERVAL_HOURS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.RetentionIntervalHours = n
		}
	}

	return cfg
}

func (c Config) ListenAddr() string {
	return ":" + strconv.Itoa(c.Port)
}
