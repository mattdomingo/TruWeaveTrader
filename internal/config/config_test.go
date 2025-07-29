package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	// Set test environment variables
	testEnv := map[string]string{
		"ALPACA_KEY_ID":     "test_key",
		"ALPACA_SECRET_KEY": "test_secret",
		"LIVE_TRADING":      "false",
		"CACHE_TTL_MS":      "200",
	}

	// Set env vars
	for key, value := range testEnv {
		os.Setenv(key, value)
	}

	// Clean up after test
	defer func() {
		for key := range testEnv {
			os.Unsetenv(key)
		}
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Test required fields
	if cfg.AlpacaKeyID != "test_key" {
		t.Errorf("Expected AlpacaKeyID='test_key', got '%s'", cfg.AlpacaKeyID)
	}

	if cfg.AlpacaSecretKey != "test_secret" {
		t.Errorf("Expected AlpacaSecretKey='test_secret', got '%s'", cfg.AlpacaSecretKey)
	}

	if cfg.LiveTrading != false {
		t.Errorf("Expected LiveTrading=false, got %v", cfg.LiveTrading)
	}

	// Test parsed duration
	expectedTTL := 200 * time.Millisecond
	if cfg.CacheTTL != expectedTTL {
		t.Errorf("Expected CacheTTL=%v, got %v", expectedTTL, cfg.CacheTTL)
	}

	// Test defaults
	expectedURL := "https://paper-api.alpaca.markets"
	if cfg.AlpacaBaseURL != expectedURL {
		t.Errorf("Expected AlpacaBaseURL='%s', got '%s'", expectedURL, cfg.AlpacaBaseURL)
	}
}

func TestLoadMissingKeys(t *testing.T) {
	// Ensure no API keys are set
	os.Unsetenv("ALPACA_KEY_ID")
	os.Unsetenv("ALPACA_SECRET_KEY")

	_, err := Load()
	if err == nil {
		t.Error("Expected error when API keys are missing, got nil")
	}

	expectedError := "ALPACA_KEY_ID and ALPACA_SECRET_KEY must be set"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestIsPaperTrading(t *testing.T) {
	cfg := &Config{LiveTrading: false}
	if !cfg.IsPaperTrading() {
		t.Error("Expected IsPaperTrading()=true when LiveTrading=false")
	}

	cfg.LiveTrading = true
	if cfg.IsPaperTrading() {
		t.Error("Expected IsPaperTrading()=false when LiveTrading=true")
	}
}
