package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port                   int
	AuthToken              string
	JWTSecret              string
	DatabaseURL            string
	EventRetentionDays     int
	RetentionIntervalHours int
	LogLevel               string
	LogFile                string
	LogFormat              string
}

func Load() Config {
	cfg := Config{
		Port:                   8080,
		AuthToken:              os.Getenv("COORDINATOR_AUTH_TOKEN"),
		JWTSecret:              os.Getenv("COORDINATOR_JWT_SECRET"),
		DatabaseURL:            os.Getenv("COORDINATOR_DATABASE_URL"),
		EventRetentionDays:     30,
		RetentionIntervalHours: 24,
		LogLevel:               "info",
		LogFile:                os.Getenv("COORDINATOR_LOG_FILE"),
		LogFormat:              "text",
	}

	if v := os.Getenv("COORDINATOR_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("COORDINATOR_LOG_FORMAT"); v != "" {
		cfg.LogFormat = v
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
