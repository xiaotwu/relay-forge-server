package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Env       string
	Host      string
	Port      int
	BaseURL   string
	LogLevel  string
	LogFormat string
	CORS      CORSConfig
	Database  DatabaseConfig
	Valkey    ValkeyConfig
	Auth      AuthConfig
	SMTP      SMTPConfig
	S3        S3Config
	LiveKit   LiveKitConfig
	Antivirus AntivirusConfig
	Upload    UploadConfig
	RateLimit RateLimitConfig
	Metrics   MetricsConfig
	OTel      OTelConfig
}

type CORSConfig struct {
	Origins []string
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

type ValkeyConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
	PoolSize int
}

func (c ValkeyConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

type AuthConfig struct {
	JWTSecret         string
	AccessTTL         time.Duration
	RefreshTTL        time.Duration
	PasswordMinLen    int
	MaxDevicesPerUser int
	TOTPIssuer        string
}

type SMTPConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
	TLS      bool
	Enabled  bool
}

type S3Config struct {
	Endpoint      string
	Region        string
	AccessKey     string
	SecretKey     string
	BucketUploads string
	BucketAvatars string
	BucketEmoji   string
	UsePathStyle  bool
	PresignExpiry time.Duration
}

type LiveKitConfig struct {
	Host      string
	Port      int
	APIKey    string
	APISecret string
	URL       string
}

type AntivirusConfig struct {
	Enabled bool
	Host    string
	Port    int
}

type UploadConfig struct {
	MaxFileSize int64
	AllowedMIME string
	ChunkSize   int64
}

type RateLimitConfig struct {
	Enabled           bool
	RequestsPerSecond int
	Burst             int
}

type MetricsConfig struct {
	Enabled bool
	Port    int
}

type OTelConfig struct {
	Enabled  bool
	Endpoint string
	Service  string
}

func Load() (*Config, error) {
	cfg := &Config{
		Env:       getEnv("RELAY_ENV", "development"),
		Host:      getEnv("API_HOST", "0.0.0.0"),
		Port:      getEnvInt("API_PORT", 8080),
		BaseURL:   getEnv("API_BASE_URL", "http://localhost:8080"),
		LogLevel:  getEnv("RELAY_LOG_LEVEL", "debug"),
		LogFormat: getEnv("RELAY_LOG_FORMAT", "console"),
		CORS: CORSConfig{
			Origins: splitCSV(getEnv("API_CORS_ORIGINS", "http://localhost:3000,http://localhost:5174")),
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
		Valkey: ValkeyConfig{
			Host:     getEnv("VALKEY_HOST", "localhost"),
			Port:     getEnvInt("VALKEY_PORT", 6379),
			Password: getEnv("VALKEY_PASSWORD", ""),
			DB:       getEnvInt("VALKEY_DB", 0),
			PoolSize: getEnvInt("VALKEY_POOL_SIZE", 10),
		},
		Auth: AuthConfig{
			JWTSecret:         getEnv("AUTH_JWT_SECRET", "change-me-in-production"),
			AccessTTL:         getEnvDuration("AUTH_JWT_ACCESS_TTL", 15*time.Minute),
			RefreshTTL:        getEnvDuration("AUTH_JWT_REFRESH_TTL", 7*24*time.Hour),
			PasswordMinLen:    getEnvInt("AUTH_PASSWORD_MIN_LENGTH", 8),
			MaxDevicesPerUser: getEnvInt("AUTH_MAX_DEVICES_PER_USER", 10),
			TOTPIssuer:        getEnv("AUTH_TOTP_ISSUER", "RelayForge"),
		},
		SMTP: SMTPConfig{
			Host:     getEnv("SMTP_HOST", "localhost"),
			Port:     getEnvInt("SMTP_PORT", 1025),
			User:     getEnv("SMTP_USER", ""),
			Password: getEnv("SMTP_PASSWORD", ""),
			From:     getEnv("SMTP_FROM", "noreply@relayforge.local"),
			TLS:      getEnvBool("SMTP_TLS", false),
			Enabled:  getEnvBool("EMAIL_VERIFICATION_ENABLED", false),
		},
		S3: S3Config{
			Endpoint:      getEnv("S3_ENDPOINT", "http://localhost:9000"),
			Region:        getEnv("S3_REGION", "us-east-1"),
			AccessKey:     getEnv("S3_ACCESS_KEY", "minioadmin"),
			SecretKey:     getEnv("S3_SECRET_KEY", "minioadmin"),
			BucketUploads: getEnv("S3_BUCKET_UPLOADS", "relay-uploads"),
			BucketAvatars: getEnv("S3_BUCKET_AVATARS", "relay-avatars"),
			BucketEmoji:   getEnv("S3_BUCKET_EMOJI", "relay-emoji"),
			UsePathStyle:  getEnvBool("S3_USE_PATH_STYLE", true),
			PresignExpiry: getEnvDuration("S3_PRESIGN_EXPIRY", 3600*time.Second),
		},
		LiveKit: LiveKitConfig{
			Host:      getEnv("LIVEKIT_HOST", "localhost"),
			Port:      getEnvInt("LIVEKIT_PORT", 7880),
			APIKey:    getEnv("LIVEKIT_API_KEY", "devkey"),
			APISecret: getEnv("LIVEKIT_API_SECRET", "devsecret"),
			URL:       getEnv("LIVEKIT_URL", "ws://localhost:7880"),
		},
		Antivirus: AntivirusConfig{
			Enabled: getEnvBool("ANTIVIRUS_ENABLED", false),
			Host:    getEnv("ANTIVIRUS_HOST", "localhost"),
			Port:    getEnvInt("ANTIVIRUS_PORT", 3310),
		},
		Upload: UploadConfig{
			MaxFileSize: getEnvInt64("UPLOAD_MAX_FILE_SIZE", 52428800),
			AllowedMIME: getEnv("UPLOAD_ALLOWED_MIME_TYPES", "image/png,image/jpeg,image/gif,image/webp"),
			ChunkSize:   getEnvInt64("UPLOAD_CHUNK_SIZE", 5242880),
		},
		RateLimit: RateLimitConfig{
			Enabled:           getEnvBool("RATE_LIMIT_ENABLED", true),
			RequestsPerSecond: getEnvInt("RATE_LIMIT_REQUESTS_PER_SECOND", 10),
			Burst:             getEnvInt("RATE_LIMIT_BURST", 20),
		},
		Metrics: MetricsConfig{
			Enabled: getEnvBool("METRICS_ENABLED", true),
			Port:    getEnvInt("METRICS_PORT", 9090),
		},
		OTel: OTelConfig{
			Enabled:  getEnvBool("OTEL_ENABLED", false),
			Endpoint: getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4317"),
			Service:  getEnv("OTEL_SERVICE_NAME", "relayforge-api"),
		},
	}

	if cfg.Auth.JWTSecret == "change-me-in-production" && cfg.Env == "production" {
		return nil, fmt.Errorf("AUTH_JWT_SECRET must be set in production")
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

func getEnvInt64(key string, fallback int64) int64 {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
		if secs, err := strconv.Atoi(v); err == nil {
			return time.Duration(secs) * time.Second
		}
	}
	return fallback
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	for _, part := range splitString(s, ',') {
		trimmed := trimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func splitString(s string, sep byte) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
