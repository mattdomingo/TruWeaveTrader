package cache

import (
	"testing"
	"time"

	"github.com/TruWeaveTrader/alpaca-tui/internal/models"
	"github.com/shopspring/decimal"
)

func TestNewCache(t *testing.T) {
	ttl := 100 * time.Millisecond
	cache := NewCache(ttl)

	if cache == nil {
		t.Fatal("NewCache() returned nil")
	}

	if cache.ttl != ttl {
		t.Errorf("Expected TTL=%v, got %v", ttl, cache.ttl)
	}
}

func TestSnapshotCaching(t *testing.T) {
	cache := NewCache(1 * time.Second)
	symbol := "AAPL"

	// Test cache miss
	snapshot, found := cache.GetSnapshot(symbol)
	if found {
		t.Error("Expected cache miss, but found snapshot")
	}
	if snapshot != nil {
		t.Error("Expected nil snapshot on cache miss")
	}

	// Test cache set and hit
	testSnapshot := &models.Snapshot{
		Symbol: symbol,
		LatestTrade: &models.Trade{
			Symbol: symbol,
			Price:  decimal.NewFromFloat(150.50),
			Size:   100,
		},
	}

	cache.SetSnapshot(symbol, testSnapshot)

	// Test cache hit
	cachedSnapshot, found := cache.GetSnapshot(symbol)
	if !found {
		t.Error("Expected cache hit, but got miss")
	}
	if cachedSnapshot == nil {
		t.Fatal("Expected snapshot, got nil")
	}
	if cachedSnapshot.Symbol != symbol {
		t.Errorf("Expected symbol=%s, got %s", symbol, cachedSnapshot.Symbol)
	}
}

func TestQuoteCaching(t *testing.T) {
	cache := NewCache(1 * time.Second)
	symbol := "AAPL"

	// Test cache miss
	quote, found := cache.GetQuote(symbol)
	if found {
		t.Error("Expected cache miss, but found quote")
	}
	if quote != nil {
		t.Error("Expected nil quote on cache miss")
	}

	// Test cache set and hit
	testQuote := &models.Quote{
		Symbol:   symbol,
		BidPrice: decimal.NewFromFloat(150.00),
		AskPrice: decimal.NewFromFloat(150.10),
		BidSize:  100,
		AskSize:  200,
	}

	cache.SetQuote(symbol, testQuote)

	// Test cache hit
	cachedQuote, found := cache.GetQuote(symbol)
	if !found {
		t.Error("Expected cache hit, but got miss")
	}
	if cachedQuote == nil {
		t.Fatal("Expected quote, got nil")
	}
	if cachedQuote.Symbol != symbol {
		t.Errorf("Expected symbol=%s, got %s", symbol, cachedQuote.Symbol)
	}
}

func TestUpdateFromStream(t *testing.T) {
	cache := NewCache(1 * time.Second)
	symbol := "AAPL"

	// Set initial snapshot
	testSnapshot := &models.Snapshot{
		Symbol:      symbol,
		LatestTrade: &models.Trade{Symbol: symbol, Price: decimal.NewFromFloat(150.00)},
		LatestQuote: &models.Quote{Symbol: symbol, BidPrice: decimal.NewFromFloat(149.95)},
	}
	cache.SetSnapshot(symbol, testSnapshot)

	// Update quote from stream
	newQuote := &models.Quote{
		Symbol:   symbol,
		BidPrice: decimal.NewFromFloat(150.50),
		AskPrice: decimal.NewFromFloat(150.60),
	}
	cache.UpdateQuoteFromStream(newQuote)

	// Verify quote was updated
	cachedQuote, found := cache.GetQuote(symbol)
	if !found {
		t.Fatal("Quote should be cached")
	}
	if !cachedQuote.BidPrice.Equal(decimal.NewFromFloat(150.50)) {
		t.Errorf("Expected bid price=150.50, got %s", cachedQuote.BidPrice.String())
	}

	// Verify snapshot quote was also updated
	cachedSnapshot, found := cache.GetSnapshot(symbol)
	if !found {
		t.Fatal("Snapshot should be cached")
	}
	if !cachedSnapshot.LatestQuote.BidPrice.Equal(decimal.NewFromFloat(150.50)) {
		t.Errorf("Expected snapshot quote bid price=150.50, got %s", cachedSnapshot.LatestQuote.BidPrice.String())
	}
}

func TestClear(t *testing.T) {
	cache := NewCache(1 * time.Second)

	// Add some data
	cache.SetSnapshot("AAPL", &models.Snapshot{Symbol: "AAPL"})
	cache.SetQuote("MSFT", &models.Quote{Symbol: "MSFT"})

	// Verify data is there
	_, found1 := cache.GetSnapshot("AAPL")
	_, found2 := cache.GetQuote("MSFT")
	if !found1 || !found2 {
		t.Fatal("Data should be cached before clear")
	}

	// Clear cache
	cache.Clear()

	// Verify data is gone
	_, found1 = cache.GetSnapshot("AAPL")
	_, found2 = cache.GetQuote("MSFT")
	if found1 || found2 {
		t.Error("Data should be cleared after Clear()")
	}
}

func TestStats(t *testing.T) {
	cache := NewCache(1 * time.Second)

	// Initially empty
	stats := cache.GetStats()
	if stats.SnapshotCount != 0 || stats.QuoteCount != 0 {
		t.Error("Expected empty cache stats")
	}

	// Add some data
	cache.SetSnapshot("AAPL", &models.Snapshot{Symbol: "AAPL"})
	cache.SetSnapshot("MSFT", &models.Snapshot{Symbol: "MSFT"})
	cache.SetQuote("GOOGL", &models.Quote{Symbol: "GOOGL"})

	// Check stats
	stats = cache.GetStats()
	if stats.SnapshotCount != 2 {
		t.Errorf("Expected 2 snapshots, got %d", stats.SnapshotCount)
	}
	if stats.QuoteCount != 1 {
		t.Errorf("Expected 1 quote, got %d", stats.QuoteCount)
	}
}
