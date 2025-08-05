package strategy

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/TruWeaveTrader/alpaca-tui/internal/alpaca"
	"github.com/TruWeaveTrader/alpaca-tui/internal/models"
	"github.com/TruWeaveTrader/alpaca-tui/internal/risk"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// PairsTradingStrategy implements a pairs trading strategy
type PairsTradingStrategy struct {
	*BaseStrategy

	// Strategy parameters
	lookbackPeriod    int             // Period for calculating spread statistics
	entryThreshold    float64         // Z-score threshold for entry
	exitThreshold     float64         // Z-score threshold for exit
	stopLossThreshold float64         // Z-score threshold for stop loss
	maxPositionSize   decimal.Decimal // Maximum position size per pair
	cooldownPeriod    time.Duration   // Minimum time between trades

	// Trading pairs configuration
	pairs         []*TradingPair            // List of trading pairs
	pairData      map[string]*PairData      // Data for each pair
	symbolPairs   map[string][]*TradingPair // Map from symbol to pairs it belongs to
	signalManager func(*Signal)             // Function to send signals
}

// TradingPair represents a pair of symbols to trade
type TradingPair struct {
	Symbol1 string  `json:"symbol1"` // First symbol (typically the one to buy)
	Symbol2 string  `json:"symbol2"` // Second symbol (typically the one to sell)
	Ratio   float64 `json:"ratio"`   // Hedge ratio (Symbol1/Symbol2)
	Name    string  `json:"name"`    // Pair name for identification
}

// PairData holds data and state for a trading pair
type PairData struct {
	Symbol1Prices  []decimal.Decimal `json:"symbol1_prices"`
	Symbol2Prices  []decimal.Decimal `json:"symbol2_prices"`
	Spreads        []decimal.Decimal `json:"spreads"`
	SpreadMean     decimal.Decimal   `json:"spread_mean"`
	SpreadStdDev   decimal.Decimal   `json:"spread_std_dev"`
	CurrentSpread  decimal.Decimal   `json:"current_spread"`
	CurrentZScore  decimal.Decimal   `json:"current_z_score"`
	Position       PairPosition      `json:"position"`
	EntrySpread    decimal.Decimal   `json:"entry_spread"`
	EntryTime      time.Time         `json:"entry_time"`
	LastSignalTime time.Time         `json:"last_signal_time"`
	PnL            decimal.Decimal   `json:"pnl"`
	TotalTrades    int64             `json:"total_trades"`
}

// PairPosition represents the current position in a pair
type PairPosition struct {
	Symbol1Qty   decimal.Decimal `json:"symbol1_qty"`   // Quantity of symbol1
	Symbol2Qty   decimal.Decimal `json:"symbol2_qty"`   // Quantity of symbol2
	Symbol1Entry decimal.Decimal `json:"symbol1_entry"` // Entry price of symbol1
	Symbol2Entry decimal.Decimal `json:"symbol2_entry"` // Entry price of symbol2
	Direction    PairDirection   `json:"direction"`     // Long/Short spread
	IsOpen       bool            `json:"is_open"`       // Whether position is open
}

// PairDirection represents the direction of the pair trade
type PairDirection string

const (
	PairNone        PairDirection = "none"
	PairLongSpread  PairDirection = "long_spread"  // Buy symbol1, sell symbol2
	PairShortSpread PairDirection = "short_spread" // Sell symbol1, buy symbol2
)

// PairsTradingConfig holds configuration for pairs trading
type PairsTradingConfig struct {
	LookbackPeriod    int            `json:"lookback_period"`     // Default: 60
	EntryThreshold    float64        `json:"entry_threshold"`     // Default: 2.0
	ExitThreshold     float64        `json:"exit_threshold"`      // Default: 0.5
	StopLossThreshold float64        `json:"stop_loss_threshold"` // Default: 3.0
	MaxPositionUSD    float64        `json:"max_position_usd"`    // Default: 10000
	CooldownMinutes   int            `json:"cooldown_minutes"`    // Default: 30
	Pairs             []*TradingPair `json:"pairs"`               // List of pairs to trade
}

// NewPairsTradingStrategy creates a new pairs trading strategy
func NewPairsTradingStrategy(name string, config *StrategyConfig, signalManager func(*Signal)) *PairsTradingStrategy {
	// Extract symbols from pairs
	symbols := make([]string, 0)
	symbolSet := make(map[string]bool)

	// Parse strategy-specific parameters
	params := config.Parameters
	pairsConfig := &PairsTradingConfig{
		LookbackPeriod:    60,
		EntryThreshold:    2.0,
		ExitThreshold:     0.5,
		StopLossThreshold: 3.0,
		MaxPositionUSD:    10000.0,
		CooldownMinutes:   30,
		Pairs:             make([]*TradingPair, 0),
	}

	// Override with provided parameters
	if val, ok := params["lookback_period"].(float64); ok {
		pairsConfig.LookbackPeriod = int(val)
	}
	if val, ok := params["entry_threshold"].(float64); ok {
		pairsConfig.EntryThreshold = val
	}
	if val, ok := params["exit_threshold"].(float64); ok {
		pairsConfig.ExitThreshold = val
	}
	if val, ok := params["stop_loss_threshold"].(float64); ok {
		pairsConfig.StopLossThreshold = val
	}
	if val, ok := params["max_position_usd"].(float64); ok {
		pairsConfig.MaxPositionUSD = val
	}
	if val, ok := params["cooldown_minutes"].(float64); ok {
		pairsConfig.CooldownMinutes = int(val)
	}

	// Parse pairs from configuration
	if pairsRaw, ok := params["pairs"].([]interface{}); ok {
		for _, pairRaw := range pairsRaw {
			if pairMap, ok := pairRaw.(map[string]interface{}); ok {
				symbol1, _ := pairMap["symbol1"].(string)
				symbol2, _ := pairMap["symbol2"].(string)
				ratio, _ := pairMap["ratio"].(float64)

				if symbol1 != "" && symbol2 != "" && ratio > 0 {
					pair := &TradingPair{
						Symbol1: symbol1,
						Symbol2: symbol2,
						Ratio:   ratio,
						Name:    fmt.Sprintf("%s/%s", symbol1, symbol2),
					}
					pairsConfig.Pairs = append(pairsConfig.Pairs, pair)

					// Add symbols to set
					if !symbolSet[symbol1] {
						symbols = append(symbols, symbol1)
						symbolSet[symbol1] = true
					}
					if !symbolSet[symbol2] {
						symbols = append(symbols, symbol2)
						symbolSet[symbol2] = true
					}
				}
			}
		}
	}

	// Default pairs if none provided (examples with common correlations)
	if len(pairsConfig.Pairs) == 0 {
		defaultPairs := []*TradingPair{
			{Symbol1: "NVDA", Symbol2: "AMD", Ratio: 1.0, Name: "NVDA/AMD"},
			{Symbol1: "META", Symbol2: "GOOGL", Ratio: 1.0, Name: "META/GOOGL"},
		}

		for _, pair := range defaultPairs {
			pairsConfig.Pairs = append(pairsConfig.Pairs, pair)
			if !symbolSet[pair.Symbol1] {
				symbols = append(symbols, pair.Symbol1)
				symbolSet[pair.Symbol1] = true
			}
			if !symbolSet[pair.Symbol2] {
				symbols = append(symbols, pair.Symbol2)
				symbolSet[pair.Symbol2] = true
			}
		}
	}

	base := NewBaseStrategy(name, "pairs_trading", symbols, config)

	strategy := &PairsTradingStrategy{
		BaseStrategy:      base,
		lookbackPeriod:    pairsConfig.LookbackPeriod,
		entryThreshold:    pairsConfig.EntryThreshold,
		exitThreshold:     pairsConfig.ExitThreshold,
		stopLossThreshold: pairsConfig.StopLossThreshold,
		maxPositionSize:   decimal.NewFromFloat(pairsConfig.MaxPositionUSD),
		cooldownPeriod:    time.Duration(pairsConfig.CooldownMinutes) * time.Minute,
		pairs:             pairsConfig.Pairs,
		pairData:          make(map[string]*PairData),
		symbolPairs:       make(map[string][]*TradingPair),
		signalManager:     signalManager,
	}

	// Initialize pair data and symbol mapping
	for _, pair := range strategy.pairs {
		strategy.pairData[pair.Name] = &PairData{
			Symbol1Prices: make([]decimal.Decimal, 0, pairsConfig.LookbackPeriod),
			Symbol2Prices: make([]decimal.Decimal, 0, pairsConfig.LookbackPeriod),
			Spreads:       make([]decimal.Decimal, 0, pairsConfig.LookbackPeriod),
			SpreadMean:    decimal.Zero,
			SpreadStdDev:  decimal.Zero,
			CurrentSpread: decimal.Zero,
			CurrentZScore: decimal.Zero,
			Position: PairPosition{
				Direction: PairNone,
				IsOpen:    false,
			},
			PnL: decimal.Zero,
		}

		// Map symbols to pairs
		strategy.symbolPairs[pair.Symbol1] = append(strategy.symbolPairs[pair.Symbol1], pair)
		strategy.symbolPairs[pair.Symbol2] = append(strategy.symbolPairs[pair.Symbol2], pair)
	}

	return strategy
}

// Initialize sets up the strategy
func (s *PairsTradingStrategy) Initialize(ctx context.Context, client *alpaca.Client, risk *risk.Manager, logger *zap.Logger) error {
	if err := s.BaseStrategy.Initialize(ctx, client, risk, logger); err != nil {
		return err
	}

	s.logger.Info("pairs trading strategy initialized",
		zap.Int("num_pairs", len(s.pairs)),
		zap.Int("lookback_period", s.lookbackPeriod),
		zap.Float64("entry_threshold", s.entryThreshold),
		zap.Float64("exit_threshold", s.exitThreshold),
		zap.Float64("stop_loss_threshold", s.stopLossThreshold),
		zap.String("max_position_usd", s.maxPositionSize.String()),
	)

	// Log pairs
	for _, pair := range s.pairs {
		s.logger.Info("trading pair configured",
			zap.String("pair", pair.Name),
			zap.String("symbol1", pair.Symbol1),
			zap.String("symbol2", pair.Symbol2),
			zap.Float64("ratio", pair.Ratio),
		)
	}

	// Load historical data for each symbol
	for _, symbol := range s.symbols {
		if err := s.loadHistoricalData(ctx, symbol); err != nil {
			s.logger.Error("failed to load historical data",
				zap.String("symbol", symbol),
				zap.Error(err))
		}
	}

	return nil
}

// OnTick processes real-time market data
func (s *PairsTradingStrategy) OnTick(ctx context.Context, symbol string, data *models.Snapshot) error {
	if !s.IsActive() || !s.CheckSchedule() {
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

	// Update price data and check pairs involving this symbol
	pairs := s.symbolPairs[symbol]
	for _, pair := range pairs {
		pairData := s.pairData[pair.Name]

		// Update prices and calculate spread
		if err := s.updatePairPrices(pair, pairData, symbol, currentPrice); err != nil {
			continue
		}

		// Check if we have enough data
		if len(pairData.Spreads) < s.lookbackPeriod {
			continue
		}

		// Calculate current statistics
		s.calculateSpreadStatistics(pairData)

		// Generate trading signals
		if err := s.evaluatePairSignals(pair, pairData); err != nil {
			s.logger.Error("failed to evaluate pair signals",
				zap.String("pair", pair.Name),
				zap.Error(err))
		}
	}

	return nil
}

// OnBar processes new bar data
func (s *PairsTradingStrategy) OnBar(ctx context.Context, symbol string, bar *models.Bar) error {
	// For pairs trading, we primarily use tick data, but we can use bars for historical analysis
	return s.OnTick(ctx, symbol, &models.Snapshot{
		Symbol: symbol,
		LatestTrade: &models.Trade{
			Symbol: symbol,
			Price:  bar.Close,
		},
	})
}

// OnOrderUpdate handles order status updates
func (s *PairsTradingStrategy) OnOrderUpdate(ctx context.Context, order *models.Order) error {
	if order.Status != models.OrderFilled {
		return nil
	}

	// Find which pair this order belongs to
	for _, pair := range s.pairs {
		pairData := s.pairData[pair.Name]

		if order.Symbol == pair.Symbol1 || order.Symbol == pair.Symbol2 {
			s.updatePairPosition(pair, pairData, order)
			break
		}
	}

	return nil
}

// OnPositionUpdate handles position updates from the broker
func (s *PairsTradingStrategy) OnPositionUpdate(ctx context.Context, position *models.Position) error {
	// Sync pair positions with broker positions
	for _, pair := range s.pairs {
		pairData := s.pairData[pair.Name]

		if position.Symbol == pair.Symbol1 {
			pairData.Position.Symbol1Qty = position.Qty
			if !position.AvgEntryPrice.IsZero() {
				pairData.Position.Symbol1Entry = position.AvgEntryPrice
			}
		} else if position.Symbol == pair.Symbol2 {
			pairData.Position.Symbol2Qty = position.Qty
			if !position.AvgEntryPrice.IsZero() {
				pairData.Position.Symbol2Entry = position.AvgEntryPrice
			}
		}
	}

	return nil
}

// Shutdown cleans up the strategy
func (s *PairsTradingStrategy) Shutdown(ctx context.Context) error {
	s.logger.Info("pairs trading strategy shutting down")

	for _, pair := range s.pairs {
		pairData := s.pairData[pair.Name]
		s.logger.Info("final pair metrics",
			zap.String("pair", pair.Name),
			zap.String("total_pnl", pairData.PnL.String()),
			zap.Int64("total_trades", pairData.TotalTrades),
			zap.Bool("position_open", pairData.Position.IsOpen),
		)
	}

	return nil
}

// updatePairPrices updates price data for a pair
func (s *PairsTradingStrategy) updatePairPrices(pair *TradingPair, pairData *PairData, symbol string, price decimal.Decimal) error {
	// We need both prices to calculate spread
	var symbol1Price, symbol2Price decimal.Decimal

	if symbol == pair.Symbol1 {
		symbol1Price = price
		// Get latest price for symbol2
		if len(pairData.Symbol2Prices) > 0 {
			symbol2Price = pairData.Symbol2Prices[len(pairData.Symbol2Prices)-1]
		}

		// Add to symbol1 prices
		pairData.Symbol1Prices = append(pairData.Symbol1Prices, price)
		if len(pairData.Symbol1Prices) > s.lookbackPeriod {
			pairData.Symbol1Prices = pairData.Symbol1Prices[1:]
		}
	} else if symbol == pair.Symbol2 {
		symbol2Price = price
		// Get latest price for symbol1
		if len(pairData.Symbol1Prices) > 0 {
			symbol1Price = pairData.Symbol1Prices[len(pairData.Symbol1Prices)-1]
		}

		// Add to symbol2 prices
		pairData.Symbol2Prices = append(pairData.Symbol2Prices, price)
		if len(pairData.Symbol2Prices) > s.lookbackPeriod {
			pairData.Symbol2Prices = pairData.Symbol2Prices[1:]
		}
	}

	// Calculate spread if we have both prices
	if !symbol1Price.IsZero() && !symbol2Price.IsZero() {
		// Spread = Symbol1 - (Ratio * Symbol2)
		spread := symbol1Price.Sub(decimal.NewFromFloat(pair.Ratio).Mul(symbol2Price))
		pairData.CurrentSpread = spread

		// Add to spreads history
		pairData.Spreads = append(pairData.Spreads, spread)
		if len(pairData.Spreads) > s.lookbackPeriod {
			pairData.Spreads = pairData.Spreads[1:]
		}
	}

	return nil
}

// calculateSpreadStatistics calculates mean and standard deviation of spread
func (s *PairsTradingStrategy) calculateSpreadStatistics(pairData *PairData) {
	if len(pairData.Spreads) < s.lookbackPeriod {
		return
	}

	// Calculate mean
	sum := decimal.Zero
	for _, spread := range pairData.Spreads {
		sum = sum.Add(spread)
	}
	pairData.SpreadMean = sum.Div(decimal.NewFromInt(int64(len(pairData.Spreads))))

	// Calculate standard deviation
	squaredDiffSum := 0.0
	for _, spread := range pairData.Spreads {
		diff := spread.Sub(pairData.SpreadMean).InexactFloat64()
		squaredDiffSum += diff * diff
	}

	variance := squaredDiffSum / float64(len(pairData.Spreads))
	pairData.SpreadStdDev = decimal.NewFromFloat(math.Sqrt(variance))

	// Calculate current Z-score
	if !pairData.SpreadStdDev.IsZero() {
		pairData.CurrentZScore = pairData.CurrentSpread.Sub(pairData.SpreadMean).Div(pairData.SpreadStdDev)
	}
}

// evaluatePairSignals evaluates and generates trading signals for a pair
func (s *PairsTradingStrategy) evaluatePairSignals(pair *TradingPair, pairData *PairData) error {
	// Check cooldown period
	if time.Since(pairData.LastSignalTime) < s.cooldownPeriod {
		return nil
	}

	zScore := pairData.CurrentZScore.InexactFloat64()

	// Check for position exit signals first
	if pairData.Position.IsOpen {
		return s.checkExitSignals(pair, pairData, zScore)
	}

	// Check for entry signals
	return s.checkEntrySignals(pair, pairData, zScore)
}

// checkEntrySignals checks for pair entry signals
func (s *PairsTradingStrategy) checkEntrySignals(pair *TradingPair, pairData *PairData, zScore float64) error {
	// Long spread signal (buy symbol1, sell symbol2)
	if zScore <= -s.entryThreshold {
		return s.generatePairEntrySignal(pair, pairData, PairLongSpread, fmt.Sprintf("Z-score %.2f <= %.2f (long spread)", zScore, -s.entryThreshold))
	}

	// Short spread signal (sell symbol1, buy symbol2)
	if zScore >= s.entryThreshold {
		return s.generatePairEntrySignal(pair, pairData, PairShortSpread, fmt.Sprintf("Z-score %.2f >= %.2f (short spread)", zScore, s.entryThreshold))
	}

	return nil
}

// checkExitSignals checks for pair exit signals
func (s *PairsTradingStrategy) checkExitSignals(pair *TradingPair, pairData *PairData, zScore float64) error {
	direction := pairData.Position.Direction

	// Stop loss conditions
	if (direction == PairLongSpread && zScore <= -s.stopLossThreshold) ||
		(direction == PairShortSpread && zScore >= s.stopLossThreshold) {
		return s.generatePairExitSignal(pair, pairData, fmt.Sprintf("Stop loss: Z-score %.2f", zScore))
	}

	// Normal exit conditions (mean reversion)
	if (direction == PairLongSpread && zScore >= s.exitThreshold) ||
		(direction == PairShortSpread && zScore <= -s.exitThreshold) {
		return s.generatePairExitSignal(pair, pairData, fmt.Sprintf("Mean reversion: Z-score %.2f", zScore))
	}

	return nil
}

// generatePairEntrySignal generates entry signals for a pair
func (s *PairsTradingStrategy) generatePairEntrySignal(pair *TradingPair, pairData *PairData, direction PairDirection, reason string) error {
	// Calculate position sizes
	positionValue := s.maxPositionSize.Div(decimal.NewFromInt(int64(len(s.pairs))))

	// Get current prices
	symbol1Price := decimal.Zero
	symbol2Price := decimal.Zero

	if len(pairData.Symbol1Prices) > 0 {
		symbol1Price = pairData.Symbol1Prices[len(pairData.Symbol1Prices)-1]
	}
	if len(pairData.Symbol2Prices) > 0 {
		symbol2Price = pairData.Symbol2Prices[len(pairData.Symbol2Prices)-1]
	}

	if symbol1Price.IsZero() || symbol2Price.IsZero() {
		return fmt.Errorf("missing prices for pair %s", pair.Name)
	}

	// Calculate quantities
	symbol1Qty := positionValue.Div(symbol1Price).Floor()
	symbol2Qty := positionValue.Div(symbol2Price).Floor()

	if symbol1Qty.LessThan(decimal.NewFromInt(1)) || symbol2Qty.LessThan(decimal.NewFromInt(1)) {
		return nil // Position too small
	}

	// Generate signals based on direction
	var signal1Action models.OrderSide

	if direction == PairLongSpread {
		signal1Action = models.Buy // Buy symbol1
		// signal2Action = models.Sell // Sell symbol2 (would need to short, but we'll skip this for now)
	} else {
		signal1Action = models.Sell // Sell symbol1
		// signal2Action = models.Buy  // Buy symbol2
	}

	// For now, only trade symbol1 (can extend to include shorting symbol2)
	signal := &Signal{
		Strategy:   s.Name(),
		Symbol:     pair.Symbol1,
		Action:     signal1Action,
		Quantity:   symbol1Qty,
		Price:      symbol1Price,
		OrderType:  models.Market,
		Reason:     fmt.Sprintf("Pairs entry: %s - %s", pair.Name, reason),
		Confidence: s.calculatePairConfidence(pairData.CurrentZScore.Abs().InexactFloat64()),
		Timestamp:  time.Now(),
	}

	s.LogSignal(signal)
	s.signalManager(signal)

	// Update pair state
	pairData.Position.Direction = direction
	pairData.Position.IsOpen = true
	pairData.EntrySpread = pairData.CurrentSpread
	pairData.EntryTime = time.Now()
	pairData.LastSignalTime = time.Now()

	return nil
}

// generatePairExitSignal generates exit signals for a pair
func (s *PairsTradingStrategy) generatePairExitSignal(pair *TradingPair, pairData *PairData, reason string) error {
	if !pairData.Position.IsOpen {
		return nil
	}

	// Get current price
	symbol1Price := decimal.Zero
	if len(pairData.Symbol1Prices) > 0 {
		symbol1Price = pairData.Symbol1Prices[len(pairData.Symbol1Prices)-1]
	}

	if symbol1Price.IsZero() {
		return fmt.Errorf("missing price for symbol %s", pair.Symbol1)
	}

	// Determine exit action (opposite of entry)
	var exitAction models.OrderSide
	if pairData.Position.Direction == PairLongSpread {
		exitAction = models.Sell // Close long position
	} else {
		exitAction = models.Buy // Close short position
	}

	// Use current position quantity (would need to track this properly)
	quantity := pairData.Position.Symbol1Qty.Abs()
	if quantity.IsZero() {
		quantity = decimal.NewFromInt(10) // Default for demo
	}

	signal := &Signal{
		Strategy:   s.Name(),
		Symbol:     pair.Symbol1,
		Action:     exitAction,
		Quantity:   quantity,
		Price:      symbol1Price,
		OrderType:  models.Market,
		Reason:     fmt.Sprintf("Pairs exit: %s - %s", pair.Name, reason),
		Confidence: 0.8,
		Timestamp:  time.Now(),
	}

	s.LogSignal(signal)
	s.signalManager(signal)

	// Calculate P&L
	pnl := s.calculatePairPnL(pairData, symbol1Price)
	pairData.PnL = pairData.PnL.Add(pnl)
	s.UpdateMetrics(pnl)
	pairData.TotalTrades++

	// Reset position
	pairData.Position.Direction = PairNone
	pairData.Position.IsOpen = false
	pairData.LastSignalTime = time.Now()

	s.logger.Info("pair position closed",
		zap.String("pair", pair.Name),
		zap.String("pnl", pnl.String()),
		zap.String("reason", reason),
	)

	return nil
}

// updatePairPosition updates the position tracking for a pair
func (s *PairsTradingStrategy) updatePairPosition(pair *TradingPair, pairData *PairData, order *models.Order) {
	if order.Symbol == pair.Symbol1 {
		if order.Side == models.Buy {
			pairData.Position.Symbol1Qty = pairData.Position.Symbol1Qty.Add(order.FilledQty)
		} else {
			pairData.Position.Symbol1Qty = pairData.Position.Symbol1Qty.Sub(order.FilledQty)
		}

		if order.FilledAvgPrice != nil {
			pairData.Position.Symbol1Entry = *order.FilledAvgPrice
		}
	} else if order.Symbol == pair.Symbol2 {
		if order.Side == models.Buy {
			pairData.Position.Symbol2Qty = pairData.Position.Symbol2Qty.Add(order.FilledQty)
		} else {
			pairData.Position.Symbol2Qty = pairData.Position.Symbol2Qty.Sub(order.FilledQty)
		}

		if order.FilledAvgPrice != nil {
			pairData.Position.Symbol2Entry = *order.FilledAvgPrice
		}
	}
}

// calculatePairPnL calculates P&L for a pair position
func (s *PairsTradingStrategy) calculatePairPnL(pairData *PairData, currentPrice decimal.Decimal) decimal.Decimal {
	if !pairData.Position.IsOpen || pairData.Position.Symbol1Entry.IsZero() {
		return decimal.Zero
	}

	// Simple P&L calculation based on symbol1 position
	priceDiff := currentPrice.Sub(pairData.Position.Symbol1Entry)

	if pairData.Position.Direction == PairLongSpread {
		return priceDiff.Mul(pairData.Position.Symbol1Qty.Abs())
	} else {
		return priceDiff.Neg().Mul(pairData.Position.Symbol1Qty.Abs())
	}
}

// calculatePairConfidence calculates signal confidence based on Z-score
func (s *PairsTradingStrategy) calculatePairConfidence(absZScore float64) float64 {
	// Higher Z-score = higher confidence
	confidence := 0.5 + (absZScore-s.entryThreshold)/(s.stopLossThreshold-s.entryThreshold)*0.4

	if confidence > 0.95 {
		confidence = 0.95
	}
	if confidence < 0.1 {
		confidence = 0.1
	}

	return confidence
}

// loadHistoricalData loads historical data for initialization
func (s *PairsTradingStrategy) loadHistoricalData(ctx context.Context, symbol string) error {
	if s.client == nil {
		return fmt.Errorf("client not initialized")
	}

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

	// Update pair data with historical prices
	for _, pair := range s.pairs {
		pairData := s.pairData[pair.Name]

		if symbol == pair.Symbol1 {
			for _, bar := range bars {
				pairData.Symbol1Prices = append(pairData.Symbol1Prices, bar.Close)
			}
			// Keep only lookback period
			if len(pairData.Symbol1Prices) > s.lookbackPeriod {
				pairData.Symbol1Prices = pairData.Symbol1Prices[len(pairData.Symbol1Prices)-s.lookbackPeriod:]
			}
		} else if symbol == pair.Symbol2 {
			for _, bar := range bars {
				pairData.Symbol2Prices = append(pairData.Symbol2Prices, bar.Close)
			}
			// Keep only lookback period
			if len(pairData.Symbol2Prices) > s.lookbackPeriod {
				pairData.Symbol2Prices = pairData.Symbol2Prices[len(pairData.Symbol2Prices)-s.lookbackPeriod:]
			}
		}

		// Calculate spreads if we have both price series
		if len(pairData.Symbol1Prices) > 0 && len(pairData.Symbol2Prices) > 0 {
			minLen := len(pairData.Symbol1Prices)
			if len(pairData.Symbol2Prices) < minLen {
				minLen = len(pairData.Symbol2Prices)
			}

			pairData.Spreads = make([]decimal.Decimal, 0, minLen)
			for i := 0; i < minLen; i++ {
				spread := pairData.Symbol1Prices[i].Sub(decimal.NewFromFloat(pair.Ratio).Mul(pairData.Symbol2Prices[i]))
				pairData.Spreads = append(pairData.Spreads, spread)
			}
		}
	}

	s.logger.Info("historical data loaded for pairs trading",
		zap.String("symbol", symbol),
		zap.Int("bars_loaded", len(bars)),
	)

	return nil
}

// GetPairData returns the current data for a pair
func (s *PairsTradingStrategy) GetPairData(pairName string) (*PairData, bool) {
	data, exists := s.pairData[pairName]
	return data, exists
}

// GetPairs returns all trading pairs
func (s *PairsTradingStrategy) GetPairs() []*TradingPair {
	return s.pairs
}
