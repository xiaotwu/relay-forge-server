package config

import (
	"os"
	"strconv"
)

type Config struct {
	LogLevel  string
	LogFormat string
	Database  DatabaseConfig
	Valkey    ValkeyConfig
}

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
}

func (c DatabaseConfig) DSN() string {
	return "postgres://" + c.User + ":" + c.Password + "@" + c.Host + ":" + strconv.Itoa(c.Port) + "/" + c.Name + "?sslmode=" + c.SSLMode
}

type ValkeyConfig struct {
	Host     string
	Port     int
	Password string
}

func Load() (*Config, error) {
	return &Config{
		LogLevel:  getEnv("RELAY_LOG_LEVEL", "debug"),
		LogFormat: getEnv("RELAY_LOG_FORMAT", "console"),
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "relayforge"),
			Password: getEnv("DB_PASSWORD", "relayforge_dev"),
			Name:     getEnv("DB_NAME", "relayforge"),
			SSLMode:  getEnv("DB_SSL_MODE", "disable"),
		},
		Valkey: ValkeyConfig{
			Host:     getEnv("VALKEY_HOST", "localhost"),
			Port:     getEnvInt("VALKEY_PORT", 6379),
			Password: getEnv("VALKEY_PASSWORD", ""),
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
