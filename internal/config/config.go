// Package config は、環境変数からアプリケーション設定を読み込み・検証します。
package config

import (
	"time"
)

const (
	// DefaultPort は、PORT環境変数が未設定の場合に使用されるデフォルトのリッスンポートです。
	DefaultPort = "8080"
	// DefaultGeminiModel は、作詞・作曲フェーズで使用するデフォルトのGeminiモデルです。
	DefaultGeminiModel = "gemini-3-flash-preview"
	// DefaultLyriaModel は、音声生成フェーズで使用するデフォルトのLyriaモデルです。
	DefaultLyriaModel = "lyria-3-pro-preview"
	// SignedURLExpiration は、生成物の署名付きURLの有効期限です。
	SignedURLExpiration = 30 * time.Minute
	// DefaultRateIntervalSec は、音声生成APIの呼び出し間隔（秒）のデフォルト値です。
	DefaultRateIntervalSec = 10
)

// Config はアプリ設定です。
type Config struct {
	ServiceURL          string
	Port                string
	ProjectID           string
	LocationID          string
	QueueID             string
	TaskAudienceURL     string
	ServiceAccountEmail string
	GCSBucket           string
	SlackWebhookURL     string
	GeminiAPIKey        string
	GeminiModel         string
	LyriaModel          string
	RateInterval        time.Duration

	// OAuth & Session Settings
	GoogleClientID     string
	GoogleClientSecret string
	// SessionSecret はセッションデータのHMAC署名用シークレットキーです。
	SessionSecret string
	// SessionEncryptKey はセッションデータのAES暗号化用シークレットキーです。 16, 24, 32 バイトのいずれかである必要があります。
	SessionEncryptKey string

	// Authz Settings
	AllowedEmails  []string
	AllowedDomains []string
}

// LoadConfig は環境変数から設定を読み込みます。
func LoadConfig() *Config {
	serviceURL := getEnv("SERVICE_URL", "http://localhost:8080")
	allowedEmails := getEnv("ALLOWED_EMAILS", "")
	allowedDomains := getEnv("ALLOWED_DOMAINS", "")
	intervalSec := getEnvAsInt("RATE_INTERVAL_SEC", DefaultRateIntervalSec)

	cfg := Config{
		ServiceURL:          serviceURL,
		Port:                getEnv("PORT", DefaultPort),
		ProjectID:           getEnv("GCP_PROJECT_ID", ""),
		LocationID:          getEnv("GCP_LOCATION_ID", ""),
		QueueID:             getEnv("CLOUD_TASKS_QUEUE_ID", ""),
		TaskAudienceURL:     getEnv("TASK_AUDIENCE_URL", serviceURL),
		ServiceAccountEmail: getEnv("SERVICE_ACCOUNT_EMAIL", ""),
		GCSBucket:           getEnv("GCS_MUSIC_BUCKET", ""),
		SlackWebhookURL:     getEnv("SLACK_WEBHOOK_URL", ""),
		GeminiAPIKey:        getEnv("GEMINI_API_KEY", ""),
		GeminiModel:         getEnv("GEMINI_MODEL", DefaultGeminiModel),
		LyriaModel:          getEnv("LYRIA_MODEL", DefaultLyriaModel),

		// OAuth & Session
		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		SessionSecret:      getEnv("SESSION_SECRET", ""),
		SessionEncryptKey:  getEnv("SESSION_ENCRYPT_KEY", ""),

		AllowedEmails:  parseCommaSeparatedList(allowedEmails),
		AllowedDomains: parseCommaSeparatedList(allowedDomains),

		// Generation Settings
		RateInterval: time.Duration(intervalSec) * time.Second,
	}

	return &cfg
}
