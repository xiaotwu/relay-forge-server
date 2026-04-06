package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Host      string
	Port      int
	Env       string
	LogLevel  string
	LogFormat string
	CORS      CORSConfig
	Auth      AuthConfig
	S3        S3Config
	LiveKit   LiveKitConfig
	Upload    UploadConfig
	Antivirus AntivirusConfig
}

type CORSConfig struct {
	Origins []string
}

type AuthConfig struct {
	JWTSecret string
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
	APIKey    string
	APISecret string
	URL       string
}

type UploadConfig struct {
	MaxFileSize int64
	AllowedMIME string
	ChunkSize   int64
}

type AntivirusConfig struct {
	Enabled bool
	Host    string
	Port    int
}

func Load() (*Config, error) {
	return &Config{
		Host:      getEnv("MEDIA_HOST", "0.0.0.0"),
		Port:      getEnvInt("MEDIA_PORT", 8082),
		Env:       getEnv("RELAY_ENV", "development"),
		LogLevel:  getEnv("RELAY_LOG_LEVEL", "debug"),
		LogFormat: getEnv("RELAY_LOG_FORMAT", "console"),
		CORS: CORSConfig{
			Origins: splitCSV(getEnv("MEDIA_CORS_ORIGINS", "http://localhost:3000,http://localhost:5174")),
		},
		Auth: AuthConfig{
			JWTSecret: getEnv("AUTH_JWT_SECRET", "change-me-in-production"),
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
			APIKey:    getEnv("LIVEKIT_API_KEY", "devkey"),
			APISecret: getEnv("LIVEKIT_API_SECRET", "devsecret"),
			URL:       getEnv("LIVEKIT_URL", "ws://localhost:7880"),
		},
		Upload: UploadConfig{
			MaxFileSize: getEnvInt64("UPLOAD_MAX_FILE_SIZE", 52428800),
			AllowedMIME: getEnv("UPLOAD_ALLOWED_MIME_TYPES", "image/png,image/jpeg,image/gif,image/webp,image/avif,application/pdf,text/plain,application/zip,application/x-zip-compressed"),
			ChunkSize:   getEnvInt64("UPLOAD_CHUNK_SIZE", 5242880),
		},
		Antivirus: AntivirusConfig{
			Enabled: getEnvBool("ANTIVIRUS_ENABLED", false),
			Host:    getEnv("ANTIVIRUS_HOST", "localhost"),
			Port:    getEnvInt("ANTIVIRUS_PORT", 3310),
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
	}
	return fallback
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	var values []string
	start := 0
	for index := 0; index <= len(s); index++ {
		if index == len(s) || s[index] == ',' {
			part := trimSpace(s[start:index])
			if part != "" {
				values = append(values, part)
			}
			start = index + 1
		}
	}
	return values
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
