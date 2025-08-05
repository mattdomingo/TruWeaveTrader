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

// MomentumStrategy implements a momentum-based trading strategy with stop losses
type MomentumStrategy struct {
	*BaseStrategy

	// Strategy parameters
	shortPeriod       int             // Short-term moving average period
	longPeriod        int             // Long-term moving average period
	rsiPeriod         int             // RSI period for momentum confirmation
	stopLossPercent   float64         // Stop loss percentage
	takeProfitPercent float64         // Take profit percentage
	minMomentum       float64         // Minimum momentum threshold
	maxPositionSize   decimal.Decimal // Maximum position size per symbol
	cooldownPeriod    time.Duration   // Minimum time between trades

	// Strategy state
	symbolData    map[string]*SymbolMomentumData // Data for each symbol
	signalManager func(*Signal)                  // Function to send signals
}

// SymbolMomentumData holds momentum data for each symbol
type SymbolMomentumData struct {
	Bars            []*models.Bar     `json:"bars"`
	ShortMA         decimal.Decimal   `json:"short_ma"`
	LongMA          decimal.Decimal   `json:"long_ma"`
	RSI             decimal.Decimal   `json:"rsi"`
	Position        decimal.Decimal   `json:"position"`
	EntryPrice      decimal.Decimal   `json:"entry_price"`
	StopLossPrice   decimal.Decimal   `json:"stop_loss_price"`
	TakeProfitPrice decimal.Decimal   `json:"take_profit_price"`
	LastSignalTime  time.Time         `json:"last_signal_time"`
	Trend           TrendDirection    `json:"trend"`
	PnL             decimal.Decimal   `json:"pnl"`
	RSIGains        []decimal.Decimal `json:"-"` // For RSI calculation
	RSILosses       []decimal.Decimal `json:"-"` // For RSI calculation
}

// TrendDirection represents the current trend direction
type TrendDirection string

const (
	TrendNone TrendDirection = "none"
	TrendUp   TrendDirection = "up"
	TrendDown TrendDirection = "down"
)

// MomentumConfig holds configuration for momentum strategy
type MomentumConfig struct {
	ShortPeriod       int     `json:"short_period"`        // Default: 10
	LongPeriod        int     `json:"long_period"`         // Default: 30
	RSIPeriod         int     `json:"rsi_period"`          // Default: 14
	StopLossPercent   float64 `json:"stop_loss_percent"`   // Default: 2.0
	TakeProfitPercent float64 `json:"take_profit_percent"` // Default: 4.0
	MinMomentum       float64 `json:"min_momentum"`        // Default: 1.0
	MaxPositionUSD    float64 `json:"max_position_usd"`    // Default: 10000
	CooldownMinutes   int     `json:"cooldown_minutes"`    // Default: 15
}

// NewMomentumStrategy creates a new momentum strategy
func NewMomentumStrategy(name string, symbols []string, config *StrategyConfig, signalManager func(*Signal)) *MomentumStrategy {
	base := NewBaseStrategy(name, "momentum", symbols, config)

	// Parse strategy-specific parameters
	params := config.Parameters
	momConfig := &MomentumConfig{
		ShortPeriod:       10,
		LongPeriod:        30,
		RSIPeriod:         14,
		StopLossPercent:   2.0,
		TakeProfitPercent: 4.0,
		MinMomentum:       1.0,
		MaxPositionUSD:    10000.0,
		CooldownMinutes:   15,
	}

	// Override with provided parameters
	if val, ok := params["short_period"].(float64); ok {
		momConfig.ShortPeriod = int(val)
	}
	if val, ok := params["long_period"].(float64); ok {
		momConfig.LongPeriod = int(val)
	}
	if val, ok := params["rsi_period"].(float64); ok {
		momConfig.RSIPeriod = int(val)
	}
	if val, ok := params["stop_loss_percent"].(float64); ok {
		momConfig.StopLossPercent = val
	}
	if val, ok := params["take_profit_percent"].(float64); ok {
		momConfig.TakeProfitPercent = val
	}
	if val, ok := params["min_momentum"].(float64); ok {
		momConfig.MinMomentum = val
	}
	if val, ok := params["max_position_usd"].(float64); ok {
		momConfig.MaxPositionUSD = val
	}
	if val, ok := params["cooldown_minutes"].(float64); ok {
		momConfig.CooldownMinutes = int(val)
	}

	strategy := &MomentumStrategy{
		BaseStrategy:      base,
		shortPeriod:       momConfig.ShortPeriod,
		longPeriod:        momConfig.LongPeriod,
		rsiPeriod:         momConfig.RSIPeriod,
		stopLossPercent:   momConfig.StopLossPercent,
		takeProfitPercent: momConfig.TakeProfitPercent,
		minMomentum:       momConfig.MinMomentum,
		maxPositionSize:   decimal.NewFromFloat(momConfig.MaxPositionUSD),
		cooldownPeriod:    time.Duration(momConfig.CooldownMinutes) * time.Minute,
		symbolData:        make(map[string]*SymbolMomentumData),
		signalManager:     signalManager,
	}

	// Initialize symbol data
	for _, symbol := range symbols {
		strategy.symbolData[symbol] = &SymbolMomentumData{
			Bars:            make([]*models.Bar, 0, momConfig.LongPeriod+10),
			ShortMA:         decimal.Zero,
			LongMA:          decimal.Zero,
			RSI:             decimal.NewFromInt(50), // Neutral RSI
			Position:        decimal.Zero,
			EntryPrice:      decimal.Zero,
			StopLossPrice:   decimal.Zero,
			TakeProfitPrice: decimal.Zero,
			Trend:           TrendNone,
			PnL:             decimal.Zero,
			RSIGains:        make([]decimal.Decimal, 0, momConfig.RSIPeriod),
			RSILosses:       make([]decimal.Decimal, 0, momConfig.RSIPeriod),
		}
	}

	return strategy
}

// Initialize sets up the strategy
func (s *MomentumStrategy) Initialize(ctx context.Context, client *alpaca.Client, risk *risk.Manager, logger *zap.Logger) error {
	if err := s.BaseStrategy.Initialize(ctx, client, risk, logger); err != nil {
		return err
	}

	s.logger.Info("momentum strategy initialized",
		zap.Strings("symbols", s.symbols),
		zap.Int("short_period", s.shortPeriod),
		zap.Int("long_period", s.longPeriod),
		zap.Int("rsi_period", s.rsiPeriod),
		zap.Float64("stop_loss_percent", s.stopLossPercent),
		zap.Float64("take_profit_percent", s.takeProfitPercent),
		zap.Float64("min_momentum", s.minMomentum),
		zap.String("max_position_usd", s.maxPositionSize.String()),
	)

	// Load historical data for each symbol
	for _, symbol := range s.symbols {
		if err := s.loadHistoricalData(ctx, symbol); err != nil {
			s.logger.Warn("failed to load historical data, strategy will initialize as data becomes available",
				zap.String("symbol", symbol),
				zap.Error(err))
		}
	}

	return nil
}

// OnTick processes real-time market data
func (s *MomentumStrategy) OnTick(ctx context.Context, symbol string, data *models.Snapshot) error {
	if !s.IsActive() || !s.CheckSchedule() {
		return nil
	}

	symbolData, exists := s.symbolData[symbol]
	if !exists {
		return fmt.Errorf("no data for symbol %s", symbol)
	}

	// Need enough bars for calculation
	if len(symbolData.Bars) < s.longPeriod {
		return nil
	}

	// Get current price
	currentPrice := decimal.Zero
	if data.LatestTrade != nil && !data.LatestTrade.Price.IsZero() {
		currentPrice = data.LatestTrade.Price
	} else if data.LatestQuote != nil && !data.LatestQuote.BidPrice.IsZero() && !data.LatestQuote.AskPrice.IsZero() {
		currentPrice = data.LatestQuote.BidPrice.Add(data.LatestQuote.AskPrice).Div(decimal.NewFromInt(2))
	} else {
		return nil
	}

	// Check stop loss and take profit first
	if err := s.checkStopLossAndTakeProfit(symbol, currentPrice); err != nil {
		return err
	}

	// Check cooldown period
	if time.Since(symbolData.LastSignalTime) < s.cooldownPeriod {
		return nil
	}

	// Calculate momentum signals
	if symbolData.ShortMA.IsZero() || symbolData.LongMA.IsZero() {
		return nil // Not enough data for moving averages
	}

	// Determine trend direction
	previousTrend := symbolData.Trend
	if symbolData.ShortMA.GreaterThan(symbolData.LongMA) {
		symbolData.Trend = TrendUp
	} else if symbolData.ShortMA.LessThan(symbolData.LongMA) {
		symbolData.Trend = TrendDown
	}

	// Calculate momentum strength
	momentum := symbolData.ShortMA.Sub(symbolData.LongMA).Div(symbolData.LongMA).Mul(decimal.NewFromInt(100))
	momentumStrength := momentum.Abs().InexactFloat64()

	// Generate signals
	if symbolData.Trend == TrendUp && previousTrend != TrendUp &&
		momentumStrength >= s.minMomentum &&
		symbolData.RSI.LessThan(decimal.NewFromInt(70)) && // Not overbought
		symbolData.Position.LessThanOrEqual(decimal.Zero) {
		return s.generateBuySignal(symbol, currentPrice, momentum, data)
	}

	if symbolData.Trend == TrendDown && previousTrend != TrendDown &&
		momentumStrength >= s.minMomentum &&
		symbolData.RSI.GreaterThan(decimal.NewFromInt(30)) && // Not oversold
		symbolData.Position.GreaterThanOrEqual(decimal.Zero) {
		// Only short if we have a long position to close
		if symbolData.Position.GreaterThan(decimal.Zero) {
			return s.generateSellSignal(symbol, currentPrice, momentum, data)
		}
	}

	return nil
}

// OnBar processes new bar data
func (s *MomentumStrategy) OnBar(ctx context.Context, symbol string, bar *models.Bar) error {
	if !s.IsActive() {
		return nil
	}

	symbolData, exists := s.symbolData[symbol]
	if !exists {
		return fmt.Errorf("no data for symbol %s", symbol)
	}

	// Add new bar
	symbolData.Bars = append(symbolData.Bars, bar)

	// Keep only necessary bars (long period + buffer)
	maxBars := s.longPeriod + 20
	if len(symbolData.Bars) > maxBars {
		symbolData.Bars = symbolData.Bars[len(symbolData.Bars)-maxBars:]
	}

	// Calculate indicators
	s.calculateMovingAverages(symbolData)
	s.calculateRSI(symbolData)

	s.logger.Debug("bar processed",
		zap.String("symbol", symbol),
		zap.Time("timestamp", bar.Timestamp),
		zap.String("close", bar.Close.String()),
		zap.String("short_ma", symbolData.ShortMA.String()),
		zap.String("long_ma", symbolData.LongMA.String()),
		zap.String("rsi", symbolData.RSI.String()),
		zap.String("trend", string(symbolData.Trend)),
	)

	return nil
}

// OnOrderUpdate handles order status updates
func (s *MomentumStrategy) OnOrderUpdate(ctx context.Context, order *models.Order) error {
	if order.Status != models.OrderFilled {
		return nil
	}

	symbolData, exists := s.symbolData[order.Symbol]
	if !exists {
		return nil
	}

	if order.Side == models.Buy {
		symbolData.Position = symbolData.Position.Add(order.FilledQty)
		if symbolData.Position.GreaterThan(decimal.Zero) && order.FilledAvgPrice != nil {
			symbolData.EntryPrice = *order.FilledAvgPrice
			// Set stop loss and take profit
			s.setStopLossAndTakeProfit(symbolData, symbolData.EntryPrice, models.Buy)
		}
	} else {
		oldPosition := symbolData.Position
		symbolData.Position = symbolData.Position.Sub(order.FilledQty)

		// Calculate P&L
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

		// Clear stop levels if position closed
		if symbolData.Position.IsZero() {
			symbolData.StopLossPrice = decimal.Zero
			symbolData.TakeProfitPrice = decimal.Zero
		}
	}

	return nil
}

// OnPositionUpdate handles position updates from the broker
func (s *MomentumStrategy) OnPositionUpdate(ctx context.Context, position *models.Position) error {
	symbolData, exists := s.symbolData[position.Symbol]
	if !exists {
		return nil
	}

	// Sync position tracking
	symbolData.Position = position.Qty
	if !position.AvgEntryPrice.IsZero() {
		symbolData.EntryPrice = position.AvgEntryPrice
		// Recalculate stop levels if we have a position but no stops set
		if !symbolData.Position.IsZero() && symbolData.StopLossPrice.IsZero() {
			side := models.Buy
			if symbolData.Position.IsNegative() {
				side = models.Sell
			}
			s.setStopLossAndTakeProfit(symbolData, symbolData.EntryPrice, side)
		}
	}

	return nil
}

// Shutdown cleans up the strategy
func (s *MomentumStrategy) Shutdown(ctx context.Context) error {
	s.logger.Info("momentum strategy shutting down")

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
func (s *MomentumStrategy) generateBuySignal(symbol string, currentPrice, momentum decimal.Decimal, data *models.Snapshot) error {
	symbolData := s.symbolData[symbol]

	// Calculate position size
	positionValue := decimal.Min(s.maxPositionSize, s.maxPositionSize.Div(decimal.NewFromInt(int64(len(s.symbols)))))
	quantity := positionValue.Div(currentPrice).Floor()

	if quantity.LessThan(decimal.NewFromInt(1)) {
		return nil
	}

	// Calculate confidence based on momentum strength and RSI
	confidence := s.calculateConfidence(momentum, symbolData.RSI, true)

	signal := &Signal{
		Strategy:   s.Name(),
		Symbol:     symbol,
		Action:     models.Buy,
		Quantity:   quantity,
		Price:      currentPrice,
		OrderType:  models.Market,
		Reason:     fmt.Sprintf("Momentum buy: %.2f%% momentum, RSI %.1f", momentum.InexactFloat64(), symbolData.RSI.InexactFloat64()),
		Confidence: confidence,
		Timestamp:  time.Now(),
	}

	s.LogSignal(signal)
	s.signalManager(signal)
	symbolData.LastSignalTime = time.Now()

	return nil
}

// generateSellSignal creates a sell signal
func (s *MomentumStrategy) generateSellSignal(symbol string, currentPrice, momentum decimal.Decimal, data *models.Snapshot) error {
	symbolData := s.symbolData[symbol]

	// Sell entire position
	quantity := symbolData.Position
	if quantity.LessThan(decimal.NewFromInt(1)) {
		return nil
	}

	confidence := s.calculateConfidence(momentum, symbolData.RSI, false)

	signal := &Signal{
		Strategy:   s.Name(),
		Symbol:     symbol,
		Action:     models.Sell,
		Quantity:   quantity,
		Price:      currentPrice,
		OrderType:  models.Market,
		Reason:     fmt.Sprintf("Momentum sell: %.2f%% negative momentum, RSI %.1f", momentum.InexactFloat64(), symbolData.RSI.InexactFloat64()),
		Confidence: confidence,
		Timestamp:  time.Now(),
	}

	s.LogSignal(signal)
	s.signalManager(signal)
	symbolData.LastSignalTime = time.Now()

	return nil
}

// checkStopLossAndTakeProfit checks and executes stop loss or take profit
func (s *MomentumStrategy) checkStopLossAndTakeProfit(symbol string, currentPrice decimal.Decimal) error {
	symbolData := s.symbolData[symbol]

	if symbolData.Position.IsZero() {
		return nil
	}

	var shouldClose bool
	var reason string

	// Check stop loss
	if symbolData.Position.IsPositive() && !symbolData.StopLossPrice.IsZero() && currentPrice.LessThanOrEqual(symbolData.StopLossPrice) {
		shouldClose = true
		reason = fmt.Sprintf("Stop loss triggered at $%.2f", currentPrice.InexactFloat64())
	}

	// Check take profit
	if symbolData.Position.IsPositive() && !symbolData.TakeProfitPrice.IsZero() && currentPrice.GreaterThanOrEqual(symbolData.TakeProfitPrice) {
		shouldClose = true
		reason = fmt.Sprintf("Take profit triggered at $%.2f", currentPrice.InexactFloat64())
	}

	if shouldClose {
		signal := &Signal{
			Strategy:   s.Name(),
			Symbol:     symbol,
			Action:     models.Sell,
			Quantity:   symbolData.Position.Abs(),
			Price:      currentPrice,
			OrderType:  models.Market,
			Reason:     reason,
			Confidence: 0.9,
			Timestamp:  time.Now(),
		}

		s.LogSignal(signal)
		s.signalManager(signal)
	}

	return nil
}

// setStopLossAndTakeProfit sets stop loss and take profit levels
func (s *MomentumStrategy) setStopLossAndTakeProfit(symbolData *SymbolMomentumData, entryPrice decimal.Decimal, side models.OrderSide) {
	stopLossMultiplier := decimal.NewFromFloat(1.0 - s.stopLossPercent/100.0)
	takeProfitMultiplier := decimal.NewFromFloat(1.0 + s.takeProfitPercent/100.0)

	if side == models.Buy {
		symbolData.StopLossPrice = entryPrice.Mul(stopLossMultiplier)
		symbolData.TakeProfitPrice = entryPrice.Mul(takeProfitMultiplier)
	} else {
		// For short positions (if implemented)
		symbolData.StopLossPrice = entryPrice.Mul(takeProfitMultiplier)
		symbolData.TakeProfitPrice = entryPrice.Mul(stopLossMultiplier)
	}
}

// calculateConfidence calculates signal confidence
func (s *MomentumStrategy) calculateConfidence(momentum, rsi decimal.Decimal, isBuy bool) float64 {
	momentumStrength := momentum.Abs().InexactFloat64()
	rsiValue := rsi.InexactFloat64()

	// Base confidence on momentum strength
	confidence := 0.4 + (momentumStrength/5.0)*0.4 // Scale 0.4 to 0.8 based on momentum

	// Adjust based on RSI
	if isBuy {
		// Better confidence when RSI is not overbought
		if rsiValue < 50 {
			confidence += 0.1
		} else if rsiValue > 70 {
			confidence -= 0.2
		}
	} else {
		// Better confidence when RSI is not oversold
		if rsiValue > 50 {
			confidence += 0.1
		} else if rsiValue < 30 {
			confidence -= 0.2
		}
	}

	// Cap confidence
	if confidence > 0.95 {
		confidence = 0.95
	}
	if confidence < 0.1 {
		confidence = 0.1
	}

	return confidence
}

// calculateMovingAverages calculates short and long moving averages
func (s *MomentumStrategy) calculateMovingAverages(symbolData *SymbolMomentumData) {
	if len(symbolData.Bars) < s.longPeriod {
		return
	}

	// Short MA
	if len(symbolData.Bars) >= s.shortPeriod {
		shortSum := decimal.Zero
		for i := len(symbolData.Bars) - s.shortPeriod; i < len(symbolData.Bars); i++ {
			shortSum = shortSum.Add(symbolData.Bars[i].Close)
		}
		symbolData.ShortMA = shortSum.Div(decimal.NewFromInt(int64(s.shortPeriod)))
	}

	// Long MA
	longSum := decimal.Zero
	for i := len(symbolData.Bars) - s.longPeriod; i < len(symbolData.Bars); i++ {
		longSum = longSum.Add(symbolData.Bars[i].Close)
	}
	symbolData.LongMA = longSum.Div(decimal.NewFromInt(int64(s.longPeriod)))
}

// calculateRSI calculates the Relative Strength Index
func (s *MomentumStrategy) calculateRSI(symbolData *SymbolMomentumData) {
	if len(symbolData.Bars) < 2 {
		return
	}

	// Calculate price change
	current := symbolData.Bars[len(symbolData.Bars)-1].Close
	previous := symbolData.Bars[len(symbolData.Bars)-2].Close
	change := current.Sub(previous)

	// Add to gains/losses
	if change.IsPositive() {
		symbolData.RSIGains = append(symbolData.RSIGains, change)
		symbolData.RSILosses = append(symbolData.RSILosses, decimal.Zero)
	} else {
		symbolData.RSIGains = append(symbolData.RSIGains, decimal.Zero)
		symbolData.RSILosses = append(symbolData.RSILosses, change.Abs())
	}

	// Keep only RSI period
	if len(symbolData.RSIGains) > s.rsiPeriod {
		symbolData.RSIGains = symbolData.RSIGains[1:]
		symbolData.RSILosses = symbolData.RSILosses[1:]
	}

	// Calculate RSI if we have enough data
	if len(symbolData.RSIGains) >= s.rsiPeriod {
		avgGain := decimal.Zero
		avgLoss := decimal.Zero

		for i := 0; i < s.rsiPeriod; i++ {
			avgGain = avgGain.Add(symbolData.RSIGains[i])
			avgLoss = avgLoss.Add(symbolData.RSILosses[i])
		}

		avgGain = avgGain.Div(decimal.NewFromInt(int64(s.rsiPeriod)))
		avgLoss = avgLoss.Div(decimal.NewFromInt(int64(s.rsiPeriod)))

		if avgLoss.IsZero() {
			symbolData.RSI = decimal.NewFromInt(100)
		} else {
			rs := avgGain.Div(avgLoss)
			symbolData.RSI = decimal.NewFromInt(100).Sub(decimal.NewFromInt(100).Div(decimal.NewFromInt(1).Add(rs)))
		}
	}
}

// loadHistoricalData loads historical data for strategy initialization
func (s *MomentumStrategy) loadHistoricalData(ctx context.Context, symbol string) error {
	if s.client == nil {
		return fmt.Errorf("client not initialized")
	}

	end := time.Now()
	start := end.Add(-time.Hour * 24 * 60) // Last 60 days

	bars, err := s.client.GetBars(ctx, symbol, "1Min", start, end, (s.longPeriod+s.rsiPeriod)*2)
	if err != nil {
		return fmt.Errorf("failed to load historical data for %s: %w", symbol, err)
	}

	if len(bars) == 0 {
		s.logger.Warn("no historical data available", zap.String("symbol", symbol))
		return nil
	}

	symbolData := s.symbolData[symbol]
	maxBars := s.longPeriod + 20
	if len(bars) > maxBars {
		symbolData.Bars = bars[len(bars)-maxBars:]
	} else {
		symbolData.Bars = bars
	}

	// Calculate initial indicators
	s.calculateMovingAverages(symbolData)
	s.calculateRSI(symbolData)

	s.logger.Info("historical data loaded",
		zap.String("symbol", symbol),
		zap.Int("bars_loaded", len(symbolData.Bars)),
		zap.String("short_ma", symbolData.ShortMA.String()),
		zap.String("long_ma", symbolData.LongMA.String()),
		zap.String("rsi", symbolData.RSI.String()),
	)

	return nil
}

// GetSymbolData returns the current data for a symbol
func (s *MomentumStrategy) GetSymbolData(symbol string) (*SymbolMomentumData, bool) {
	data, exists := s.symbolData[symbol]
	return data, exists
}
