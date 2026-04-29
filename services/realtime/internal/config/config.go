package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env            string
	Host           string
	Port           int
	LogLevel       string
	LogFormat      string
	JWTSecret      string
	MaxConnections int
	AllowedOrigins []string
	Valkey         ValkeyConfig
	Database       DatabaseConfig
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

type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Name            string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

func (c DatabaseConfig) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.Name, c.SSLMode)
}

func Load() (*Config, error) {
	cfg := &Config{
		Env:            getEnv("RELAY_ENV", "development"),
		Host:           getEnv("REALTIME_HOST", "0.0.0.0"),
		Port:           getEnvInt("REALTIME_PORT", 8081),
		LogLevel:       getEnv("RELAY_LOG_LEVEL", "debug"),
		LogFormat:      getEnv("RELAY_LOG_FORMAT", "console"),
		JWTSecret:      getEnv("AUTH_JWT_SECRET", "change-me-in-production"),
		MaxConnections: getEnvInt("REALTIME_MAX_CONNECTIONS", 10000),
		AllowedOrigins: splitCSV(getEnv("REALTIME_ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:5174")),
		Valkey: ValkeyConfig{
			Host:     getEnv("VALKEY_HOST", "localhost"),
			Port:     getEnvInt("VALKEY_PORT", 6379),
			Password: getEnv("VALKEY_PASSWORD", ""),
			DB:       getEnvInt("VALKEY_DB", 0),
		},
		Database: DatabaseConfig{
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            getEnvInt("DB_PORT", 5432),
			User:            getEnv("DB_USER", "relayforge"),
			Password:        getEnv("DB_PASSWORD", "relayforge_dev"),
			Name:            getEnv("DB_NAME", "relayforge"),
			SSLMode:         getEnv("DB_SSL_MODE", "disable"),
			MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", 300*time.Second),
		},
	}

	if cfg.JWTSecret == "change-me-in-production" && cfg.Env == "production" {
		return nil, fmt.Errorf("AUTH_JWT_SECRET must be set in production")
	}
	if cfg.Env == "production" {
		if len(cfg.AllowedOrigins) == 0 {
			return nil, fmt.Errorf("REALTIME_ALLOWED_ORIGINS must be set in production")
		}
		for _, origin := range cfg.AllowedOrigins {
			if origin == "*" {
				return nil, fmt.Errorf("REALTIME_ALLOWED_ORIGINS cannot contain * in production")
			}
		}
	}

	return cfg, nil
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

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
		if seconds, err := strconv.Atoi(v); err == nil {
			return time.Duration(seconds) * time.Second
		}
	}
	return fallback
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}

	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
