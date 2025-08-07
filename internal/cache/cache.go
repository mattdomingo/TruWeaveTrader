package cache

import (
	"time"

	"github.com/TruWeaveTrader/alpaca-tui/internal/models"
	gocache "github.com/patrickmn/go-cache"
)

// Cache provides fast in-memory caching for market data
type Cache struct {
	snapshots *gocache.Cache
	quotes    *gocache.Cache
	bars      *gocache.Cache
	ttl       time.Duration
}

// NewCache creates a new cache instance
func NewCache(ttl time.Duration) *Cache {
	// Use go-cache with default expiration and cleanup interval
	return &Cache{
		snapshots: gocache.New(ttl, ttl*2),
		quotes:    gocache.New(ttl, ttl*2),
		bars:      gocache.New(5*time.Minute, 10*time.Minute), // Bars cached longer
		ttl:       ttl,
	}
}

// GetSnapshot retrieves a cached snapshot
func (c *Cache) GetSnapshot(symbol string) (*models.Snapshot, bool) {
	if val, found := c.snapshots.Get(symbol); found {
		if snapshot, ok := val.(*models.Snapshot); ok {
			return snapshot, true
		}
	}
	return nil, false
}

// SetSnapshot caches a snapshot
func (c *Cache) SetSnapshot(symbol string, snapshot *models.Snapshot) {
	c.snapshots.Set(symbol, snapshot, c.ttl)
}

// GetQuote retrieves a cached quote
func (c *Cache) GetQuote(symbol string) (*models.Quote, bool) {
	if val, found := c.quotes.Get(symbol); found {
		if quote, ok := val.(*models.Quote); ok {
			return quote, true
		}
	}
	return nil, false
}

// SetQuote caches a quote
func (c *Cache) SetQuote(symbol string, quote *models.Quote) {
	c.quotes.Set(symbol, quote, c.ttl)
}

// UpdateQuoteFromStream updates a quote from streaming data
func (c *Cache) UpdateQuoteFromStream(quote *models.Quote) {
	c.quotes.Set(quote.Symbol, quote, c.ttl)

	// Always update the snapshot, creating one if needed
	snapshot, found := c.GetSnapshot(quote.Symbol)
	if !found || snapshot == nil {
		snapshot = &models.Snapshot{Symbol: quote.Symbol}
	}
	snapshot.LatestQuote = quote
	c.SetSnapshot(quote.Symbol, snapshot)
}

// UpdateTradeFromStream updates trade data from streaming
func (c *Cache) UpdateTradeFromStream(trade *models.Trade) {
	// Always update the snapshot, creating one if needed
	snapshot, found := c.GetSnapshot(trade.Symbol)
	if !found || snapshot == nil {
		snapshot = &models.Snapshot{Symbol: trade.Symbol}
	}
	snapshot.LatestTrade = trade
	c.SetSnapshot(trade.Symbol, snapshot)
}

// GetBar retrieves cached bar data
func (c *Cache) GetBar(key string) (*models.Bar, bool) {
	if val, found := c.bars.Get(key); found {
		if bar, ok := val.(*models.Bar); ok {
			return bar, true
		}
	}
	return nil, false
}

// SetBar caches bar data
func (c *Cache) SetBar(key string, bar *models.Bar) {
	c.bars.Set(key, bar, 5*time.Minute)
}

// Clear removes all cached data
func (c *Cache) Clear() {
	c.snapshots.Flush()
	c.quotes.Flush()
	c.bars.Flush()
}

// Stats returns cache statistics
type Stats struct {
	SnapshotCount int
	QuoteCount    int
	BarCount      int
}

// GetStats returns current cache statistics
func (c *Cache) GetStats() Stats {
	return Stats{
		SnapshotCount: c.snapshots.ItemCount(),
		QuoteCount:    c.quotes.ItemCount(),
		BarCount:      c.bars.ItemCount(),
	}
}
