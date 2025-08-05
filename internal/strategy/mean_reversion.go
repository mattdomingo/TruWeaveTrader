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

// MeanReversionStrategy implements a mean reversion trading strategy
type MeanReversionStrategy struct {
	*BaseStrategy

	// Strategy parameters
	lookbackPeriod   int             // Number of bars for moving average
	thresholdPercent float64         // Percentage deviation from MA to trigger signal
	maxPositionSize  decimal.Decimal // Maximum position size per symbol
	cooldownPeriod   time.Duration   // Minimum time between trades for same symbol

	// Strategy state
	symbolData    map[string]*SymbolMeanReversionData // Data for each symbol
	signalManager func(*Signal)                       // Function to send signals
}

// SymbolMeanReversionData holds data for each symbol
type SymbolMeanReversionData struct {
	Bars           []*models.Bar   `json:"bars"`
	MovingAverage  decimal.Decimal `json:"moving_average"`
	LastSignalTime time.Time       `json:"last_signal_time"`
	Position       decimal.Decimal `json:"position"` // Current position size
	EntryPrice     decimal.Decimal `json:"entry_price"`
	LastTrade      time.Time       `json:"last_trade"`
	PnL            decimal.Decimal `json:"pnl"`
}

// MeanReversionConfig holds configuration for mean reversion strategy
type MeanReversionConfig struct {
	LookbackPeriod   int     `json:"lookback_period"`   // Default: 20
	ThresholdPercent float64 `json:"threshold_percent"` // Default: 2.0 (2%)
	MaxPositionUSD   float64 `json:"max_position_usd"`  // Default: 5000
	CooldownMinutes  int     `json:"cooldown_minutes"`  // Default: 30
}

// NewMeanReversionStrategy creates a new mean reversion strategy
func NewMeanReversionStrategy(name string, symbols []string, config *StrategyConfig, signalManager func(*Signal)) *MeanReversionStrategy {
	base := NewBaseStrategy(name, "mean_reversion", symbols, config)

	// Parse strategy-specific parameters
	params := config.Parameters
	mrConfig := &MeanReversionConfig{
		LookbackPeriod:   20, // Default values
		ThresholdPercent: 2.0,
		MaxPositionUSD:   5000.0,
		CooldownMinutes:  30,
	}

	// Override with provided parameters
	if val, ok := params["lookback_period"].(float64); ok {
		mrConfig.LookbackPeriod = int(val)
	}
	if val, ok := params["threshold_percent"].(float64); ok {
		mrConfig.ThresholdPercent = val
	}
	if val, ok := params["max_position_usd"].(float64); ok {
		mrConfig.MaxPositionUSD = val
	}
	if val, ok := params["cooldown_minutes"].(float64); ok {
		mrConfig.CooldownMinutes = int(val)
	}

	strategy := &MeanReversionStrategy{
		BaseStrategy:     base,
		lookbackPeriod:   mrConfig.LookbackPeriod,
		thresholdPercent: mrConfig.ThresholdPercent,
		maxPositionSize:  decimal.NewFromFloat(mrConfig.MaxPositionUSD),
		cooldownPeriod:   time.Duration(mrConfig.CooldownMinutes) * time.Minute,
		symbolData:       make(map[string]*SymbolMeanReversionData),
		signalManager:    signalManager,
	}

	// Initialize symbol data
	for _, symbol := range symbols {
		strategy.symbolData[symbol] = &SymbolMeanReversionData{
			Bars:          make([]*models.Bar, 0, mrConfig.LookbackPeriod),
			MovingAverage: decimal.Zero,
			Position:      decimal.Zero,
			EntryPrice:    decimal.Zero,
			PnL:           decimal.Zero,
		}
	}

	return strategy
}

// Initialize sets up the strategy
func (s *MeanReversionStrategy) Initialize(ctx context.Context, client *alpaca.Client, risk *risk.Manager, logger *zap.Logger) error {
	if err := s.BaseStrategy.Initialize(ctx, client, risk, logger); err != nil {
		return err
	}

	s.logger.Info("mean reversion strategy initialized",
		zap.Strings("symbols", s.symbols),
		zap.Int("lookback_period", s.lookbackPeriod),
		zap.Float64("threshold_percent", s.thresholdPercent),
		zap.String("max_position_usd", s.maxPositionSize.String()),
		zap.Duration("cooldown_period", s.cooldownPeriod),
	)

	// Load historical data for each symbol
	for _, symbol := range s.symbols {
		if err := s.loadHistoricalData(ctx, symbol); err != nil {
			s.logger.Warn("failed to load historical data, strategy will initialize as data becomes available",
				zap.String("symbol", symbol),
				zap.Error(err))
			// Continue with other symbols - strategy will work once live data starts flowing
		}
	}

	return nil
}

// OnTick processes real-time market data
func (s *MeanReversionStrategy) OnTick(ctx context.Context, symbol string, data *models.Snapshot) error {
	if !s.IsActive() || !s.CheckSchedule() {
		return nil
	}

	symbolData, exists := s.symbolData[symbol]
	if !exists {
		return fmt.Errorf("no data for symbol %s", symbol)
	}

	// Check if we have enough bars for calculation
	if len(symbolData.Bars) < s.lookbackPeriod {
		return nil // Need more data
	}

	// Use latest trade price
	currentPrice := decimal.Zero
	if data.LatestTrade != nil && !data.LatestTrade.Price.IsZero() {
		currentPrice = data.LatestTrade.Price
	} else if data.LatestQuote != nil && !data.LatestQuote.BidPrice.IsZero() && !data.LatestQuote.AskPrice.IsZero() {
		// Use mid price if no trade
		currentPrice = data.LatestQuote.BidPrice.Add(data.LatestQuote.AskPrice).Div(decimal.NewFromInt(2))
	} else {
		return nil // No valid price data
	}

	// Calculate deviation from moving average
	if symbolData.MovingAverage.IsZero() {
		return nil // Moving average not calculated yet
	}

	deviation := currentPrice.Sub(symbolData.MovingAverage).Div(symbolData.MovingAverage).Mul(decimal.NewFromInt(100))

	// Check cooldown period
	if time.Since(symbolData.LastSignalTime) < s.cooldownPeriod {
		return nil
	}

	// Generate signals based on mean reversion logic
	threshold := decimal.NewFromFloat(s.thresholdPercent)

	// Price significantly below moving average - potential BUY signal
	if deviation.LessThan(threshold.Neg()) && symbolData.Position.LessThanOrEqual(decimal.Zero) {
		return s.generateBuySignal(symbol, currentPrice, deviation, data)
	}

	// Price significantly above moving average - potential SELL signal (if we have position)
	if deviation.GreaterThan(threshold) && symbolData.Position.GreaterThan(decimal.Zero) {
		return s.generateSellSignal(symbol, currentPrice, deviation, data)
	}

	// Price back near moving average - close position
	if deviation.Abs().LessThan(decimal.NewFromFloat(0.5)) && !symbolData.Position.IsZero() {
		return s.generateCloseSignal(symbol, currentPrice, data)
	}

	return nil
}

// OnBar processes new bar data
func (s *MeanReversionStrategy) OnBar(ctx context.Context, symbol string, bar *models.Bar) error {
	if !s.IsActive() {
		return nil
	}

	symbolData, exists := s.symbolData[symbol]
	if !exists {
		return fmt.Errorf("no data for symbol %s", symbol)
	}

	// Add new bar to symbol data
	symbolData.Bars = append(symbolData.Bars, bar)

	// Keep only the required number of bars
	if len(symbolData.Bars) > s.lookbackPeriod {
		symbolData.Bars = symbolData.Bars[1:]
	}

	// Calculate moving average
	s.calculateMovingAverage(symbolData)

	s.logger.Debug("bar processed",
		zap.String("symbol", symbol),
		zap.Time("timestamp", bar.Timestamp),
		zap.String("close", bar.Close.String()),
		zap.String("moving_average", symbolData.MovingAverage.String()),
		zap.Int("bars_count", len(symbolData.Bars)),
	)

	return nil
}

// OnOrderUpdate handles order status updates
func (s *MeanReversionStrategy) OnOrderUpdate(ctx context.Context, order *models.Order) error {
	if order.Status != models.OrderFilled {
		return nil
	}

	symbolData, exists := s.symbolData[order.Symbol]
	if !exists {
		return nil
	}

	// Update position tracking
	if order.Side == models.Buy {
		symbolData.Position = symbolData.Position.Add(order.FilledQty)
		if symbolData.Position.GreaterThan(decimal.Zero) {
			symbolData.EntryPrice = *order.FilledAvgPrice
		}
	} else {
		oldPosition := symbolData.Position
		symbolData.Position = symbolData.Position.Sub(order.FilledQty)

		// Calculate P&L if closing position
		if oldPosition.GreaterThan(decimal.Zero) && order.FilledAvgPrice != nil {
			pnl := order.FilledAvgPrice.Sub(symbolData.EntryPrice).Mul(order.FilledQty)
			symbolData.PnL = symbolData.PnL.Add(pnl)
			s.UpdateMetrics(pnl)

			s.logger.Info("position closed with P&L",
				zap.String("symbol", order.Symbol),
				zap.String("pnl", pnl.String()),
				zap.String("entry_price", symbolData.EntryPrice.String()),
				zap.String("exit_price", order.FilledAvgPrice.String()),
			)
		}
	}

	symbolData.LastTrade = time.Now()
	s.logger.Info("position updated",
		zap.String("symbol", order.Symbol),
		zap.String("side", string(order.Side)),
		zap.String("filled_qty", order.FilledQty.String()),
		zap.String("current_position", symbolData.Position.String()),
	)

	return nil
}

// OnPositionUpdate handles position updates from the broker
func (s *MeanReversionStrategy) OnPositionUpdate(ctx context.Context, position *models.Position) error {
	symbolData, exists := s.symbolData[position.Symbol]
	if !exists {
		return nil
	}

	// Sync our position tracking with broker
	symbolData.Position = position.Qty
	if !position.AvgEntryPrice.IsZero() {
		symbolData.EntryPrice = position.AvgEntryPrice
	}

	return nil
}

// Shutdown cleans up the strategy
func (s *MeanReversionStrategy) Shutdown(ctx context.Context) error {
	s.logger.Info("mean reversion strategy shutting down")

	// Log final metrics for each symbol
	for symbol, data := range s.symbolData {
		s.logger.Info("final symbol metrics",
			zap.String("symbol", symbol),
			zap.String("total_pnl", data.PnL.String()),
			zap.String("position", data.Position.String()),
		)
	}

	return nil
}

// generateBuySignal creates a buy signal
func (s *MeanReversionStrategy) generateBuySignal(symbol string, currentPrice, deviation decimal.Decimal, data *models.Snapshot) error {
	symbolData := s.symbolData[symbol]

	// Calculate position size (conservative approach)
	positionValue := decimal.Min(s.maxPositionSize, s.maxPositionSize.Div(decimal.NewFromInt(int64(len(s.symbols)))))
	quantity := positionValue.Div(currentPrice).Floor()

	if quantity.LessThan(decimal.NewFromInt(1)) {
		return nil // Position too small
	}

	signal := &Signal{
		Strategy:   s.Name(),
		Symbol:     symbol,
		Action:     models.Buy,
		Quantity:   quantity,
		Price:      currentPrice,
		OrderType:  models.Market,
		Reason:     fmt.Sprintf("Mean reversion buy: %.2f%% below MA", deviation.Abs().InexactFloat64()),
		Confidence: s.calculateConfidence(deviation),
		Timestamp:  time.Now(),
	}

	s.LogSignal(signal)
	s.signalManager(signal)
	symbolData.LastSignalTime = time.Now()

	return nil
}

// generateSellSignal creates a sell signal
func (s *MeanReversionStrategy) generateSellSignal(symbol string, currentPrice, deviation decimal.Decimal, data *models.Snapshot) error {
	symbolData := s.symbolData[symbol]

	// Sell half of current position
	quantity := symbolData.Position.Div(decimal.NewFromInt(2)).Floor()

	if quantity.LessThan(decimal.NewFromInt(1)) {
		return nil // Position too small
	}

	signal := &Signal{
		Strategy:   s.Name(),
		Symbol:     symbol,
		Action:     models.Sell,
		Quantity:   quantity,
		Price:      currentPrice,
		OrderType:  models.Market,
		Reason:     fmt.Sprintf("Mean reversion sell: %.2f%% above MA", deviation.InexactFloat64()),
		Confidence: s.calculateConfidence(deviation),
		Timestamp:  time.Now(),
	}

	s.LogSignal(signal)
	s.signalManager(signal)
	symbolData.LastSignalTime = time.Now()

	return nil
}

// generateCloseSignal creates a position close signal
func (s *MeanReversionStrategy) generateCloseSignal(symbol string, currentPrice decimal.Decimal, data *models.Snapshot) error {
	symbolData := s.symbolData[symbol]

	if symbolData.Position.IsZero() {
		return nil
	}

	var action models.OrderSide
	quantity := symbolData.Position.Abs()

	if symbolData.Position.IsPositive() {
		action = models.Sell
	} else {
		action = models.Buy
	}

	signal := &Signal{
		Strategy:   s.Name(),
		Symbol:     symbol,
		Action:     action,
		Quantity:   quantity,
		Price:      currentPrice,
		OrderType:  models.Market,
		Reason:     "Price returned to moving average - close position",
		Confidence: 0.8,
		Timestamp:  time.Now(),
	}

	s.LogSignal(signal)
	s.signalManager(signal)
	symbolData.LastSignalTime = time.Now()

	return nil
}

// calculateConfidence calculates signal confidence based on deviation
func (s *MeanReversionStrategy) calculateConfidence(deviation decimal.Decimal) float64 {
	// Higher deviation = higher confidence (up to a limit)
	absDeviation := deviation.Abs().InexactFloat64()
	threshold := s.thresholdPercent

	if absDeviation < threshold {
		return 0.3
	}

	// Scale confidence from 0.5 to 0.95 based on how far beyond threshold
	confidence := 0.5 + (absDeviation-threshold)/(threshold*2)*0.45
	if confidence > 0.95 {
		confidence = 0.95
	}

	return confidence
}

// calculateMovingAverage calculates the simple moving average
func (s *MeanReversionStrategy) calculateMovingAverage(symbolData *SymbolMeanReversionData) {
	if len(symbolData.Bars) < s.lookbackPeriod {
		return
	}

	sum := decimal.Zero
	for _, bar := range symbolData.Bars {
		sum = sum.Add(bar.Close)
	}

	symbolData.MovingAverage = sum.Div(decimal.NewFromInt(int64(len(symbolData.Bars))))
}

// loadHistoricalData loads historical bar data for initialization
func (s *MeanReversionStrategy) loadHistoricalData(ctx context.Context, symbol string) error {
	if s.client == nil {
		return fmt.Errorf("client not initialized")
	}

	// Get historical bars (last 50 bars to have enough for initial calculation)
	end := time.Now()
	start := end.Add(-time.Hour * 24 * 30) // Last 30 days

	bars, err := s.client.GetBars(ctx, symbol, "1Min", start, end, s.lookbackPeriod*2)
	if err != nil {
		return fmt.Errorf("failed to load historical data for %s: %w", symbol, err)
	}

	if len(bars) == 0 {
		s.logger.Warn("no historical data available", zap.String("symbol", symbol))
		return nil
	}

	// Take the most recent bars up to lookback period
	symbolData := s.symbolData[symbol]
	if len(bars) > s.lookbackPeriod {
		symbolData.Bars = bars[len(bars)-s.lookbackPeriod:]
	} else {
		symbolData.Bars = bars
	}

	// Calculate initial moving average
	s.calculateMovingAverage(symbolData)

	s.logger.Info("historical data loaded",
		zap.String("symbol", symbol),
		zap.Int("bars_loaded", len(symbolData.Bars)),
		zap.String("moving_average", symbolData.MovingAverage.String()),
	)

	return nil
}

// GetSymbolData returns the current data for a symbol (for testing/monitoring)
func (s *MeanReversionStrategy) GetSymbolData(symbol string) (*SymbolMeanReversionData, bool) {
	data, exists := s.symbolData[symbol]
	return data, exists
}
