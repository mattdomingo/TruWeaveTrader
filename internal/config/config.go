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

	// Strategy Configuration
	StrategiesEnabled bool                     `json:"strategies_enabled"`
	StrategyConfigs   []map[string]interface{} `json:"strategy_configs"`
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
		RiskMaxPositionUSD:   getEnvFloat("RISK_MAX_POSITION_USD", 20000.0),
		RiskMaxDailyLossUSD:  getEnvFloat("RISK_MAX_DAILY_LOSS_USD", 5000.0),
		RiskMaxSpreadPercent: getEnvFloat("RISK_MAX_SPREAD_PERCENT", 1.0),

		// Performance
		CacheTTL:                getEnvDuration("CACHE_TTL_MS", 100) * time.Millisecond,
		WebsocketReconnectDelay: getEnvDuration("WEBSOCKET_RECONNECT_DELAY_MS", 1000) * time.Millisecond,
		HTTPTimeout:             getEnvDuration("HTTP_TIMEOUT_MS", 3000) * time.Millisecond,

		// Strategy Configuration
		StrategiesEnabled: getEnvBool("STRATEGIES_ENABLED", false),
		StrategyConfigs:   loadStrategyConfigs(),
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

// loadStrategyConfigs loads strategy configurations from environment or returns default configs
func loadStrategyConfigs() []map[string]interface{} {
	// Default strategy configurations (loosened for demo)
	return []map[string]interface{}{
		{
			"name":    "meanrev_etfs",
			"type":    "mean_reversion",
			"enabled": true,
			"symbols": []interface{}{"SPY", "QQQ", "IWM", "DIA", "XLF", "XLE", "XLV", "XLY"},
			"parameters": map[string]interface{}{
				"lookback_period":   10,
				"threshold_percent": 0.15,
				"max_position_usd":  12000.0,
				"cooldown_minutes":  1,
			},
			"schedule": map[string]interface{}{
				"start_time": "09:30",
				"end_time":   "16:00",
				"days":       []int{1, 2, 3, 4, 5},
				"timezone":   "America/New_York",
			},
		},
		{
			"name":    "momentum_liquid",
			"type":    "momentum",
			"enabled": true,
			"symbols": []interface{}{"AAPL", "MSFT", "AMZN", "NVDA", "GOOGL", "META", "TSLA", "AMD", "NFLX", "CRM", "UBER", "SHOP", "INTC", "ADBE", "CSCO", "ORCL", "BA", "DIS", "JPM", "XOM", "AVGO", "V"},
			"parameters": map[string]interface{}{
				"short_period":        3,
				"long_period":         10,
				"rsi_period":          7,
				"stop_loss_percent":   2.0,
				"take_profit_percent": 3.0,
				"min_momentum":        0.1,
				"max_position_usd":    15000.0,
				"cooldown_minutes":    1,
			},
			"schedule": map[string]interface{}{
				"start_time": "09:30",
				"end_time":   "16:00",
				"days":       []int{1, 2, 3, 4, 5},
				"timezone":   "America/New_York",
			},
		},
		{
			"name":    "pairs_tech_correlation",
			"type":    "pairs_trading",
			"enabled": true,
			"symbols": []interface{}{},
			"parameters": map[string]interface{}{
				"lookback_period":     20,
				"entry_threshold":     0.75,
				"exit_threshold":      0.2,
				"stop_loss_threshold": 1.5,
				"max_position_usd":    15000.0,
				"cooldown_minutes":    1,
				"pairs": []map[string]interface{}{
					{"symbol1": "NVDA", "symbol2": "AMD", "ratio": 1.0},
					{"symbol1": "META", "symbol2": "GOOGL", "ratio": 1.0},
				},
			},
			"schedule": map[string]interface{}{
				"start_time": "09:30",
				"end_time":   "16:00",
				"days":       []int{1, 2, 3, 4, 5},
				"timezone":   "America/New_York",
			},
		},
	}
}
