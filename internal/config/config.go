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
		RiskMaxPositionUSD:   getEnvFloat("RISK_MAX_POSITION_USD", 10000.0), //knobs to turn with Owen's Model
		RiskMaxDailyLossUSD:  getEnvFloat("RISK_MAX_DAILY_LOSS_USD", 1000.0),
		RiskMaxSpreadPercent: getEnvFloat("RISK_MAX_SPREAD_PERCENT", 0.5),

		// Performance
		CacheTTL:                getEnvDuration("CACHE_TTL_MS", 100) * time.Millisecond,
		WebsocketReconnectDelay: getEnvDuration("WEBSOCKET_RECONNECT_DELAY_MS", 1000) * time.Millisecond,
		HTTPTimeout:             getEnvDuration("HTTP_TIMEOUT_MS", 2000) * time.Millisecond,

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
	// Default strategy configurations
	return []map[string]interface{}{
		{
			"name":    "mean_reversion_tech",
			"type":    "mean_reversion",
			"enabled": true,
			"symbols": []interface{}{"FIG", "NEGG", "NVDA", "META"},
			"parameters": map[string]interface{}{
				"lookback_period": 5,
				/*
					Shorter Lookback (10 periods):
					✅ More responsive to recent price changes
					✅ Faster signals when trends change
					❌ More noise - can trigger on temporary spikes
					❌ More false signals in choppy markets
					Longer Lookback (20 periods):
					✅ Smoother signals - less noise
					✅ More reliable in choppy markets
					❌ Slower to react to trend changes
					❌ Misses opportunities in fast-moving markets
				*/
				"threshold_percent": 0.5,     //lower threshold
				"max_position_usd":  10000.0, //larger position
				"cooldown_minutes":  1,       //faster trading
			},
			"schedule": map[string]interface{}{
				"start_time": "09:30",
				"end_time":   "16:00",
				"days":       []int{1, 2, 3, 4, 5}, // Mon-Fri
				"timezone":   "America/New_York",
			},
		},
		{
			"name":    "momentum_trending",
			"type":    "momentum",
			"enabled": true,
			"symbols": []interface{}{"AAPL", "MSFT", "GOOGL", "TSLA"},
			"parameters": map[string]interface{}{
				"short_period":        3,       // Very short MA (was 10)
				"long_period":         7,       // Short long MA (was 30)
				"rsi_period":          7,       // Shorter RSI (was 14)
				"stop_loss_percent":   1.0,     // Tighter stop loss (was 2.0)
				"take_profit_percent": 2.0,     // Lower take profit (was 4.0)
				"min_momentum":        0.1,     // Very low momentum requirement (was 1.0)
				"max_position_usd":    15000.0, // Larger position (was 8000)
				"cooldown_minutes":    1,       // Minimal cooldown (was 15)
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
			"symbols": []interface{}{}, // Will be derived from pairs
			"parameters": map[string]interface{}{
				"lookback_period":     20,      // Shorter lookback (was 60)
				"entry_threshold":     1.0,     // Lower entry threshold (was 2.0)
				"exit_threshold":      0.1,     // Lower exit threshold (was 0.5)
				"stop_loss_threshold": 2.0,     // Lower stop loss (was 3.0)
				"max_position_usd":    20000.0, // Larger position (was 10000)
				"cooldown_minutes":    1,       // Minimal cooldown (was 30)
				"pairs": []map[string]interface{}{
					{
						"symbol1": "NVDA",
						"symbol2": "AMD",
						"ratio":   1.0,
					},
					{
						"symbol1": "META",
						"symbol2": "GOOGL",
						"ratio":   1.0,
					},
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
