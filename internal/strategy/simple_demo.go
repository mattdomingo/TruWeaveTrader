package strategy

import (
	"context"
	"fmt"
	"time"

	"github.com/TruWeaveTrader/alpaca-tui/internal/alpaca"
	"github.com/TruWeaveTrader/alpaca-tui/internal/models"
	"github.com/TruWeaveTrader/alpaca-tui/internal/risk"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// SimpleDemoStrategy is a very simple strategy for testing trade execution
type SimpleDemoStrategy struct {
	*BaseStrategy
	maxPositionSize decimal.Decimal
	signalManager   func(*Signal)
	lastTradeTime   time.Time
	tickCount       int
	priceThreshold  decimal.Decimal
	hasPosition     bool
}

// NewSimpleDemoStrategy creates a new demo strategy
func NewSimpleDemoStrategy(name string, symbols []string, config *StrategyConfig, signalManager func(*Signal)) *SimpleDemoStrategy {
	maxPosition := decimal.NewFromFloat(1000.0)
	if val, ok := config.Parameters["max_position_usd"].(float64); ok {
		maxPosition = decimal.NewFromFloat(val)
	}

	return &SimpleDemoStrategy{
		BaseStrategy:    NewBaseStrategy(name, "simple_demo", symbols, config),
		maxPositionSize: maxPosition,
		signalManager:   signalManager,
		priceThreshold:  decimal.NewFromFloat(0.01), // 1% price change threshold
	}
}

// Initialize sets up the strategy
func (s *SimpleDemoStrategy) Initialize(ctx context.Context, client *alpaca.Client, risk *risk.Manager, logger *zap.Logger) error {
	if err := s.BaseStrategy.Initialize(ctx, client, risk, logger); err != nil {
		return err
	}

	s.logger.Info("simple demo strategy initialized",
		zap.String("name", s.name),
		zap.Strings("symbols", s.symbols),
		zap.String("max_position", s.maxPositionSize.String()))

	return nil
}

// OnTick processes real-time market data
func (s *SimpleDemoStrategy) OnTick(ctx context.Context, symbol string, data *models.Snapshot) error {
	if !s.IsActive() {
		return nil
	}

	s.tickCount++

	// Get current price
	currentPrice := decimal.Zero
	if data.LatestTrade != nil && !data.LatestTrade.Price.IsZero() {
		currentPrice = data.LatestTrade.Price
	} else if data.LatestQuote != nil && !data.LatestQuote.BidPrice.IsZero() && !data.LatestQuote.AskPrice.IsZero() {
		currentPrice = data.LatestQuote.BidPrice.Add(data.LatestQuote.AskPrice).Div(decimal.NewFromInt(2))
	} else {
		return nil
	}

	// Log every 10th tick to show we're receiving data
	if s.tickCount%10 == 0 {
		s.logger.Info("demo strategy received tick",
			zap.String("symbol", symbol),
			zap.String("price", currentPrice.String()),
			zap.Int("tick_count", s.tickCount),
			zap.Bool("has_position", s.hasPosition))
	}

	// Simple logic: buy after 20 ticks if no position, sell after 40 ticks if has position
	if s.tickCount >= 20 && !s.hasPosition && time.Since(s.lastTradeTime) > time.Minute {
		// Generate buy signal
		quantity := s.maxPositionSize.Div(currentPrice).Floor()
		if quantity.GreaterThan(decimal.Zero) {
			signal := &Signal{
				Strategy:  s.name,
				Symbol:    symbol,
				Action:    models.Buy,
				Quantity:  quantity,
				Price:     currentPrice,
				OrderType: models.Market,
				Reason:    fmt.Sprintf("Demo buy after %d ticks at %s", s.tickCount, currentPrice),
				Timestamp: time.Now(),
			}
			s.signalManager(signal)
			s.hasPosition = true
			s.lastTradeTime = time.Now()
			s.tickCount = 0
			s.logger.Info("demo strategy generated buy signal",
				zap.String("symbol", symbol),
				zap.String("quantity", quantity.String()),
				zap.String("price", currentPrice.String()))
		}
	} else if s.tickCount >= 40 && s.hasPosition && time.Since(s.lastTradeTime) > time.Minute {
		// Generate sell signal
		quantity := decimal.NewFromFloat(1.0) // Sell 1 share
		signal := &Signal{
			Strategy:  s.name,
			Symbol:    symbol,
			Action:    models.Sell,
			Quantity:  quantity,
			Price:     currentPrice,
			OrderType: models.Market,
			Reason:    fmt.Sprintf("Demo sell after %d ticks at %s", s.tickCount, currentPrice),
			Timestamp: time.Now(),
		}
		s.signalManager(signal)
		s.hasPosition = false
		s.lastTradeTime = time.Now()
		s.tickCount = 0
		s.logger.Info("demo strategy generated sell signal",
			zap.String("symbol", symbol),
			zap.String("quantity", quantity.String()),
			zap.String("price", currentPrice.String()))
	}

	return nil
}

// OnBar processes new bar data
func (s *SimpleDemoStrategy) OnBar(ctx context.Context, symbol string, bar *models.Bar) error {
	if !s.IsActive() {
		return nil
	}

	s.logger.Debug("demo strategy received bar",
		zap.String("symbol", symbol),
		zap.Time("timestamp", bar.Timestamp),
		zap.String("close", bar.Close.String()))

	return nil
}

// OnOrderUpdate handles order status updates
func (s *SimpleDemoStrategy) OnOrderUpdate(ctx context.Context, order *models.Order) error {
	s.logger.Info("demo strategy order update",
		zap.String("order_id", order.ID),
		zap.String("symbol", order.Symbol),
		zap.String("status", string(order.Status)))

	if order.Status == models.OrderFilled {
		// Update metrics with a dummy P&L value
		pnl := decimal.NewFromFloat(10.0)
		if order.Side == models.Sell {
			pnl = pnl.Neg()
		}
		s.UpdateMetrics(pnl)
	}

	return nil
}

// OnPositionUpdate handles position updates
func (s *SimpleDemoStrategy) OnPositionUpdate(ctx context.Context, position *models.Position) error {
	s.logger.Info("demo strategy position update",
		zap.String("symbol", position.Symbol),
		zap.String("qty", position.Qty.String()))

	// Update has position flag based on position quantity
	s.hasPosition = !position.Qty.IsZero()

	return nil
}

// GetState returns the current strategy state
func (s *SimpleDemoStrategy) GetState() interface{} {
	return map[string]interface{}{
		"tick_count":      s.tickCount,
		"has_position":    s.hasPosition,
		"last_trade_time": s.lastTradeTime,
	}
}

// Shutdown gracefully shuts down the strategy
func (s *SimpleDemoStrategy) Shutdown(ctx context.Context) error {
	return s.Stop(ctx)
}
