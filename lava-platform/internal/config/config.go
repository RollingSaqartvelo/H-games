package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Log      LogConfig
	Operator OperatorConfig
	Telegram TelegramConfig
	Gemini   GeminiConfig
}

type ServerConfig struct {
	Port            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
	RateLimit       int
	RateBurst       int
	AllowedOrigins  string
}

type DatabaseConfig struct {
	DSN             string
	MaxOpenConns    int
	MinOpenConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	HealthCheckPeriod time.Duration
}

type RedisConfig struct {
	URL          string
	PoolSize     int
	MinIdleConns int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type LogConfig struct {
	Level  string
	Pretty bool
}

type OperatorConfig struct {
	SystemAPIKey     string
	SessionTTL       time.Duration
	OperatorCacheTTL time.Duration
	CallbackTimeout  time.Duration
	CallbackRetries  int
}

type TelegramConfig struct {
	BotToken        string // TELEGRAM_BOT_TOKEN
	TMAOperatorID   int64  // TELEGRAM_TMA_OPERATOR_ID — pre-seeded operator row ID
	AuthMaxAge      time.Duration // how old initData auth_date can be
	AppURL          string // TELEGRAM_APP_URL — public HTTPS URL of the Mini App
}

type GeminiConfig struct {
	APIKey string // GEMINI_API_KEY
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port:            getEnv("PORT", "8080"),
			ReadTimeout:     getDuration("SERVER_READ_TIMEOUT", 10*time.Second),
			WriteTimeout:    getDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
			ShutdownTimeout: getDuration("SERVER_SHUTDOWN_TIMEOUT", 15*time.Second),
			RateLimit:       getInt("RATE_LIMIT_RPS", 100),
			RateBurst:       getInt("RATE_LIMIT_BURST", 200),
			AllowedOrigins:  getEnv("ALLOWED_ORIGINS", "*"),
		},
		Database: DatabaseConfig{
			DSN:               getEnv("DATABASE_URL", "postgres://lava:lava@localhost:5432/lava?sslmode=disable"),
			MaxOpenConns:      getInt("DB_MAX_OPEN_CONNS", 25),
			MinOpenConns:      getInt("DB_MIN_OPEN_CONNS", 5),
			ConnMaxLifetime:   getDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
			ConnMaxIdleTime:   getDuration("DB_CONN_MAX_IDLE_TIME", 1*time.Minute),
			HealthCheckPeriod: getDuration("DB_HEALTH_CHECK_PERIOD", 30*time.Second),
		},
		Redis: RedisConfig{
			URL:          getEnv("REDIS_URL", "redis://localhost:6379/0"),
			PoolSize:     getInt("REDIS_POOL_SIZE", 20),
			MinIdleConns: getInt("REDIS_MIN_IDLE_CONNS", 5),
			DialTimeout:  getDuration("REDIS_DIAL_TIMEOUT", 5*time.Second),
			ReadTimeout:  getDuration("REDIS_READ_TIMEOUT", 3*time.Second),
			WriteTimeout: getDuration("REDIS_WRITE_TIMEOUT", 3*time.Second),
		},
		Log: LogConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Pretty: getBool("LOG_PRETTY", false),
		},
		Operator: OperatorConfig{
			SystemAPIKey:     getEnv("SYSTEM_API_KEY", ""),
			SessionTTL:       getDuration("SESSION_TTL", 24*time.Hour),
			OperatorCacheTTL: getDuration("OPERATOR_CACHE_TTL", 5*time.Minute),
			CallbackTimeout:  getDuration("CALLBACK_TIMEOUT", 10*time.Second),
			CallbackRetries:  getInt("CALLBACK_RETRIES", 3),
		},
		Telegram: TelegramConfig{
			BotToken:      getEnv("TELEGRAM_BOT_TOKEN", ""),
			TMAOperatorID: int64(getInt("TELEGRAM_TMA_OPERATOR_ID", 1)),
			AuthMaxAge:    getDuration("TELEGRAM_AUTH_MAX_AGE", 24*time.Hour),
			AppURL:        getEnv("TELEGRAM_APP_URL", ""),
		},
		Gemini: GeminiConfig{
			APIKey: getEnv("GEMINI_API_KEY", ""),
		},
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
