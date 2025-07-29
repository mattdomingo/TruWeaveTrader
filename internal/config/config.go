package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all application configuration
type Config struct {
	// Alpaca API
	AlpacaKeyID     string
	AlpacaSecretKey string
	AlpacaBaseURL   string
	AlpacaDataURL   string

	// Trading
	LiveTrading      bool
	DefaultTimeframe string

	// Risk Management
	RiskMaxPositionUSD   float64
	RiskMaxDailyLossUSD  float64
	RiskMaxSpreadPercent float64

	// Performance
	CacheTTL                time.Duration
	WebsocketReconnectDelay time.Duration
	HTTPTimeout             time.Duration
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Try to load .env file (ignore error if not found)
	_ = godotenv.Load()

	cfg := &Config{
		// Alpaca API
		AlpacaKeyID:     getEnv("ALPACA_KEY_ID", ""),
		AlpacaSecretKey: getEnv("ALPACA_SECRET_KEY", ""),
		AlpacaBaseURL:   getEnv("ALPACA_BASE_URL", "https://paper-api.alpaca.markets"),
		AlpacaDataURL:   getEnv("ALPACA_DATA_URL", "https://data.alpaca.markets"),

		// Trading
		LiveTrading:      getEnvBool("LIVE_TRADING", false),
		DefaultTimeframe: getEnv("DEFAULT_TIMEFRAME", "1Min"),

		// Risk Management
		RiskMaxPositionUSD:   getEnvFloat("RISK_MAX_POSITION_USD", 10000.0), //knobs to turn with Owen's Model
		RiskMaxDailyLossUSD:  getEnvFloat("RISK_MAX_DAILY_LOSS_USD", 1000.0),
		RiskMaxSpreadPercent: getEnvFloat("RISK_MAX_SPREAD_PERCENT", 0.5),

		// Performance
		CacheTTL:                getEnvDuration("CACHE_TTL_MS", 100) * time.Millisecond,
		WebsocketReconnectDelay: getEnvDuration("WEBSOCKET_RECONNECT_DELAY_MS", 1000) * time.Millisecond,
		HTTPTimeout:             getEnvDuration("HTTP_TIMEOUT_MS", 2000) * time.Millisecond,
	}

	// Validate required fields
	if cfg.AlpacaKeyID == "" || cfg.AlpacaSecretKey == "" {
		return nil, fmt.Errorf("ALPACA_KEY_ID and ALPACA_SECRET_KEY must be set")
	}

	return cfg, nil
}

// IsPaperTrading returns true if using paper trading API
func (c *Config) IsPaperTrading() bool {
	return !c.LiveTrading
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue int64) time.Duration {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
			return time.Duration(parsed)
		}
	}
	return time.Duration(defaultValue)
}
