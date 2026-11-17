package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppName string
	AppEnv  string

	DiscordToken string

	AIProvider string
	AIBaseURL  string
	AIAPIKey   string
	AIModel    string

	AITemperature float64
	AIMaxTokens   int
	AITimeout     time.Duration

	DatabaseURL string
	DBMaxConns  int32
	DBMinConns  int32
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		AppName: getEnv("APP_NAME", "mcqueen-ai"),
		AppEnv:  getEnv("APP_ENV", "development"),

		DiscordToken: os.Getenv("DISCORD_TOKEN"),

		AIProvider: getEnv("AI_PROVIDER", "9router"),
		AIBaseURL:  os.Getenv("AI_BASE_URL"),
		AIAPIKey:   os.Getenv("AI_API_KEY"),
		AIModel:    os.Getenv("AI_MODEL"),

		AITimeout: 60 * time.Second,

		DatabaseURL: os.Getenv("DATABASE_URL"),
		DBMaxConns:  10,
		DBMinConns:  2,
	}

	if cfg.DiscordToken == "" {
		return nil, fmt.Errorf("DISCORD_TOKEN is required")
	}

	if cfg.AIProvider != "9router" {
		return nil, fmt.Errorf(
			"unsupported AI provider: %s",
			cfg.AIProvider,
		)
	}

	if cfg.AIBaseURL == "" ||
		cfg.AIAPIKey == "" ||
		cfg.AIModel == "" {
		return nil, fmt.Errorf(
			"AI_BASE_URL, AI_API_KEY, and AI_MODEL are required",
		)
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	if value := os.Getenv("AI_TEMPERATURE"); value != "" {
		temperature, err := strconv.ParseFloat(value, 64)
		if err != nil || temperature < 0 || temperature > 2 {
			return nil, fmt.Errorf("invalid AI_TEMPERATURE")
		}

		cfg.AITemperature = temperature
	} else {
		cfg.AITemperature = 0.8
	}

	cfg.AIMaxTokens = 1000

	if value := os.Getenv("AI_MAX_TOKENS"); value != "" {
		maxTokens, err := strconv.Atoi(value)
		if err != nil || maxTokens <= 0 {
			return nil, fmt.Errorf("invalid AI_MAX_TOKENS")
		}

		cfg.AIMaxTokens = maxTokens
	}

	if value := os.Getenv("AI_TIMEOUT"); value != "" {
		timeout, err := time.ParseDuration(value)
		if err != nil || timeout <= 0 {
			return nil, fmt.Errorf("invalid AI_TIMEOUT")
		}

		cfg.AITimeout = timeout
	}

	if value := os.Getenv("DB_MAX_CONNS"); value != "" {
		maxConns, err := strconv.ParseInt(value, 10, 32)
		if err != nil || maxConns <= 0 {
			return nil, fmt.Errorf("invalid DB_MAX_CONNS")
		}

		cfg.DBMaxConns = int32(maxConns)
	}

	if value := os.Getenv("DB_MIN_CONNS"); value != "" {
		minConns, err := strconv.ParseInt(value, 10, 32)
		if err != nil || minConns < 0 {
			return nil, fmt.Errorf("invalid DB_MIN_CONNS")
		}

		cfg.DBMinConns = int32(minConns)
	}

	if cfg.DBMinConns > cfg.DBMaxConns {
		return nil, fmt.Errorf(
			"DB_MIN_CONNS cannot exceed DB_MAX_CONNS",
		)
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)

	if value == "" {
		return fallback
	}

	return value
}
