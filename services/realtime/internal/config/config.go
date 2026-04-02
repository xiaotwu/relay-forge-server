package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Host           string
	Port           int
	LogLevel       string
	LogFormat      string
	JWTSecret      string
	MaxConnections int
	Valkey         ValkeyConfig
}

type ValkeyConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

func (c ValkeyConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func Load() (*Config, error) {
	return &Config{
		Host:           getEnv("REALTIME_HOST", "0.0.0.0"),
		Port:           getEnvInt("REALTIME_PORT", 8081),
		LogLevel:       getEnv("RELAY_LOG_LEVEL", "debug"),
		LogFormat:      getEnv("RELAY_LOG_FORMAT", "console"),
		JWTSecret:      getEnv("AUTH_JWT_SECRET", "change-me-in-production"),
		MaxConnections: getEnvInt("REALTIME_MAX_CONNECTIONS", 10000),
		Valkey: ValkeyConfig{
			Host:     getEnv("VALKEY_HOST", "localhost"),
			Port:     getEnvInt("VALKEY_PORT", 6379),
			Password: getEnv("VALKEY_PASSWORD", ""),
			DB:       getEnvInt("VALKEY_DB", 0),
		},
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}
