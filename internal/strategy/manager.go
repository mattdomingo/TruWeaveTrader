package strategy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/TruWeaveTrader/alpaca-tui/internal/alpaca"
	"github.com/TruWeaveTrader/alpaca-tui/internal/cache"
	"github.com/TruWeaveTrader/alpaca-tui/internal/config"
	"github.com/TruWeaveTrader/alpaca-tui/internal/models"
	"github.com/TruWeaveTrader/alpaca-tui/internal/risk"
	"github.com/TruWeaveTrader/alpaca-tui/internal/websocket"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// Manager orchestrates multiple trading strategies
type Manager struct {
	strategies map[string]Strategy
	client     *alpaca.Client
	risk       *risk.Manager
	stream     *websocket.StreamClient
	cache      *cache.Cache
	logger     *zap.Logger
	cfg        *config.Config

	// State management
	active bool
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc

	// Channel for receiving signals
	signalChan chan *Signal

	// Active symbols being monitored
	activeSymbols map[string]bool
	symbolMu      sync.RWMutex

	// Bar aggregation from trades
	barMu       sync.Mutex
	currentBars map[string]*models.Bar
}

// NewManager creates a new strategy manager
func NewManager(client *alpaca.Client, risk *risk.Manager, stream *websocket.StreamClient, cache *cache.Cache, logger *zap.Logger, cfg *config.Config) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		strategies:    make(map[string]Strategy),
		client:        client,
		risk:          risk,
		stream:        stream,
		cache:         cache,
		logger:        logger.With(zap.String("component", "strategy_manager")),
		cfg:           cfg,
		ctx:           ctx,
		cancel:        cancel,
		signalChan:    make(chan *Signal, 100),
		activeSymbols: make(map[string]bool),
		currentBars:   make(map[string]*models.Bar),
	}
}

// AddStrategy adds a new strategy to the manager
func (m *Manager) AddStrategy(strategy Strategy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := strategy.Name()
	if _, exists := m.strategies[name]; exists {
		return fmt.Errorf("strategy %s already exists", name)
	}

	// Initialize the strategy
	if err := strategy.Initialize(m.ctx, m.client, m.risk, m.logger); err != nil {
		return fmt.Errorf("failed to initialize strategy %s: %w", name, err)
	}

	m.strategies[name] = strategy
	m.logger.Info("strategy added", zap.String("name", name), zap.String("type", strategy.Type()))

	// If manager is active, refresh subscriptions to include new strategy symbols
	if m.active {
		m.updateSymbolStream()
	}

	m.persistStateLocked()
	return nil
}

// RemoveStrategy removes a strategy from the manager
func (m *Manager) RemoveStrategy(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	strategy, exists := m.strategies[name]
	if !exists {
		return fmt.Errorf("strategy %s not found", name)
	}

	// Stop the strategy
	if strategy.IsActive() {
		if err := strategy.Stop(m.ctx); err != nil {
			m.logger.Error("failed to stop strategy", zap.String("name", name), zap.Error(err))
		}
	}

	// Shutdown the strategy
	if err := strategy.Shutdown(m.ctx); err != nil {
		m.logger.Error("failed to shutdown strategy", zap.String("name", name), zap.Error(err))
	}

	delete(m.strategies, name)
	m.logger.Info("strategy removed", zap.String("name", name))

	// Update symbol monitoring
	if m.active {
		m.updateSymbolStream()
	}

	m.persistStateLocked()
	return nil
}

// GetStrategy returns a strategy by name
func (m *Manager) GetStrategy(name string) (Strategy, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	strategy, exists := m.strategies[name]
	return strategy, exists
}

// ListStrategies returns all strategy names
func (m *Manager) ListStrategies() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.strategies))
	for name := range m.strategies {
		names = append(names, name)
	}
	return names
}

// GetActiveStrategies returns all active strategies
func (m *Manager) GetActiveStrategies() []Strategy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	active := make([]Strategy, 0)
	for _, strategy := range m.strategies {
		if strategy.IsActive() {
			active = append(active, strategy)
		}
	}
	return active
}

// StartStrategy starts a specific strategy
func (m *Manager) StartStrategy(name string) error {
	m.mu.Lock()
	strategy, exists := m.strategies[name]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("strategy %s not found", name)
	}
	if strategy.IsActive() {
		m.mu.Unlock()
		return fmt.Errorf("strategy %s is already active", name)
	}

	// Ensure manager is active (single-strategy start should bring up processing and streaming)
	wasInactive := !m.active
	if wasInactive {
		m.active = true
		go m.processSignals()
	}
	m.mu.Unlock()

	if err := strategy.Start(m.ctx); err != nil {
		return fmt.Errorf("failed to start strategy %s: %w", name, err)
	}

	// Refresh subscriptions
	m.updateSymbolStream()

	m.logger.Info("strategy started", zap.String("name", name))
	m.persistState()
	return nil
}

// StopStrategy stops a specific strategy
func (m *Manager) StopStrategy(name string) error {
	m.mu.RLock()
	strategy, exists := m.strategies[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("strategy %s not found", name)
	}

	if !strategy.IsActive() {
		return fmt.Errorf("strategy %s is not active", name)
	}

	if err := strategy.Stop(m.ctx); err != nil {
		return fmt.Errorf("failed to stop strategy %s: %w", name, err)
	}

	// Update symbol monitoring
	if m.active {
		m.updateSymbolStream()
	}

	m.logger.Info("strategy stopped", zap.String("name", name))
	m.persistState()
	return nil
}

// StartAll starts the strategy manager and all configured strategies
func (m *Manager) StartAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.active {
		return fmt.Errorf("strategy manager is already active")
	}

	m.active = true

	// Start signal processing goroutine
	go m.processSignals()

	// Start all strategies
	for name, strategy := range m.strategies {
		if err := strategy.Start(m.ctx); err != nil {
			m.logger.Error("failed to start strategy", zap.String("name", name), zap.Error(err))
			continue
		}
	}

	// Setup symbol streaming
	m.updateSymbolStream()

	m.logger.Info("strategy manager started", zap.Int("strategies", len(m.strategies)))

	m.persistStateLocked()
	return nil
}

// StopAll stops all strategies and the manager
func (m *Manager) StopAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.active {
		return nil
	}

	m.active = false

	// Stop all strategies
	for name, strategy := range m.strategies {
		if strategy.IsActive() {
			if err := strategy.Stop(m.ctx); err != nil {
				m.logger.Error("failed to stop strategy", zap.String("name", name), zap.Error(err))
			}
		}
	}

	// Cancel context to stop goroutines
	m.cancel()

	// Close signal channel
	close(m.signalChan)

	// Remove automation state file to indicate automation is stopped
	if err := RemoveAutomationState(); err != nil {
		m.logger.Warn("failed to remove automation state", zap.Error(err))
	}

	m.logger.Info("strategy manager stopped")
	return nil
}

// Shutdown gracefully shuts down all strategies
func (m *Manager) Shutdown() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop all strategies first
	for name, strategy := range m.strategies {
		if err := strategy.Shutdown(m.ctx); err != nil {
			m.logger.Error("failed to shutdown strategy", zap.String("name", name), zap.Error(err))
		}
	}

	// Cancel context
	m.cancel()

	m.logger.Info("strategy manager shutdown complete")
	return nil
}

// OnTick processes real-time market data
func (m *Manager) OnTick(symbol string, data *models.Snapshot) {
	if !m.active {
		return
	}

	m.mu.RLock()
	strategies := make([]Strategy, 0, len(m.strategies))
	for _, strategy := range m.strategies {
		if strategy.IsActive() {
			// Check if strategy trades this symbol
			for _, s := range strategy.GetSymbols() {
				if s == symbol {
					strategies = append(strategies, strategy)
					break
				}
			}
		}
	}
	m.mu.RUnlock()

	// Process tick for relevant strategies
	for _, strategy := range strategies {
		go func(s Strategy) {
			if err := s.OnTick(m.ctx, symbol, data); err != nil {
				m.logger.Error("strategy tick processing failed",
					zap.String("strategy", s.Name()),
					zap.String("symbol", symbol),
					zap.Error(err))
			}
		}(strategy)
	}
}

// OnBar processes new bar data
func (m *Manager) OnBar(symbol string, bar *models.Bar) {
	if !m.active {
		return
	}

	m.mu.RLock()
	strategies := make([]Strategy, 0, len(m.strategies))
	for _, strategy := range m.strategies {
		if strategy.IsActive() {
			// Check if strategy trades this symbol
			for _, s := range strategy.GetSymbols() {
				if s == symbol {
					strategies = append(strategies, strategy)
					break
				}
			}
		}
	}
	m.mu.RUnlock()

	// Process bar for relevant strategies
	for _, strategy := range strategies {
		go func(s Strategy) {
			if err := s.OnBar(m.ctx, symbol, bar); err != nil {
				m.logger.Error("strategy bar processing failed",
					zap.String("strategy", s.Name()),
					zap.String("symbol", symbol),
					zap.Error(err))
			}
		}(strategy)
	}
}

// SendSignal sends a trading signal to be processed
func (m *Manager) SendSignal(signal *Signal) {
	if !m.active {
		m.logger.Warn("manager not active, dropping signal",
			zap.String("strategy", signal.Strategy),
			zap.String("symbol", signal.Symbol))
		return
	}

	m.logger.Info("received trading signal",
		zap.String("strategy", signal.Strategy),
		zap.String("symbol", signal.Symbol),
		zap.String("action", string(signal.Action)),
		zap.String("quantity", signal.Quantity.String()),
		zap.String("reason", signal.Reason))

	select {
	case m.signalChan <- signal:
		m.logger.Debug("signal queued for processing",
			zap.String("strategy", signal.Strategy),
			zap.String("symbol", signal.Symbol))
	case <-m.ctx.Done():
		m.logger.Debug("context cancelled, dropping signal",
			zap.String("strategy", signal.Strategy),
			zap.String("symbol", signal.Symbol))
		return
	default:
		m.logger.Warn("signal channel full, dropping signal",
			zap.String("strategy", signal.Strategy),
			zap.String("symbol", signal.Symbol))
	}
}

// GetMetrics returns aggregated metrics for all strategies
func (m *Manager) GetMetrics() map[string]*StrategyMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := make(map[string]*StrategyMetrics)
	for name, strategy := range m.strategies {
		metrics[name] = strategy.GetMetrics()
	}
	return metrics
}

// processSignals processes trading signals in a separate goroutine
func (m *Manager) processSignals() {
	for {
		select {
		case signal := <-m.signalChan:
			if signal == nil {
				return // Channel closed
			}
			m.executeSignal(signal)
		case <-m.ctx.Done():
			return
		}
	}
}

// executeSignal executes a trading signal
func (m *Manager) executeSignal(signal *Signal) {
	logger := m.logger.With(
		zap.String("strategy", signal.Strategy),
		zap.String("symbol", signal.Symbol),
		zap.String("action", string(signal.Action)),
	)

	logger.Info("executing signal", zap.String("reason", signal.Reason))

	// Get current market data for validation (fallback to cache if API snapshot fails)
	snapshot, err := m.client.GetSnapshot(m.ctx, signal.Symbol)
	if err != nil {
		logger.Warn("failed to get market snapshot from API, falling back to cache", zap.Error(err))
		if m.cache != nil {
			if cached, found := m.cache.GetSnapshot(signal.Symbol); found {
				snapshot = cached
			} else {
				snapshot = &models.Snapshot{Symbol: signal.Symbol}
			}
		} else {
			snapshot = &models.Snapshot{Symbol: signal.Symbol}
		}
	}

	// Get account information
	account, err := m.client.GetAccount(m.ctx)
	if err != nil {
		logger.Error("failed to get account", zap.Error(err))
		return
	}

	// Create order request
	order := &models.OrderRequest{
		Symbol:      signal.Symbol,
		Qty:         &signal.Quantity,
		Side:        signal.Action,
		Type:        signal.OrderType,
		TimeInForce: models.Day,
	}

	// Set price for limit orders
	if signal.OrderType == models.Limit {
		order.LimitPrice = &signal.Price
	}

	// Validate order with risk management
	riskResult := m.risk.ValidateOrder(order, account, signal.Price)
	if !riskResult.Passed {
		logger.Warn("signal rejected by risk management", zap.String("reason", riskResult.Reason))
		return
	}

	// Log any warnings
	for _, warning := range riskResult.Warnings {
		logger.Warn("risk warning", zap.String("warning", warning))
	}

	// Validate spread if we have quote data
	if snapshot != nil && snapshot.LatestQuote != nil {
		spreadResult := m.risk.CheckSpread(snapshot.LatestQuote)
		if !spreadResult.Passed {
			logger.Warn("signal rejected due to spread", zap.String("reason", spreadResult.Reason))
			return
		}
	}

	// Execute the order
	placedOrder, err := m.client.PlaceOrder(m.ctx, order)
	if err != nil {
		logger.Error("failed to place order", zap.Error(err))
		return
	}

	logger.Info("order placed successfully",
		zap.String("order_id", placedOrder.ID),
		zap.String("status", string(placedOrder.Status)))
}

// addSymbolsToStream adds symbols to the streaming service
func (m *Manager) addSymbolsToStream(symbols []string) {
	m.symbolMu.Lock()
	defer m.symbolMu.Unlock()

	for _, symbol := range symbols {
		if !m.activeSymbols[symbol] {
			m.activeSymbols[symbol] = true
			// Note: You would integrate with your websocket streaming here
			// m.stream.Subscribe(symbol)
		}
	}
}

// updateSymbolStream updates the symbols being streamed based on active strategies
func (m *Manager) updateSymbolStream() {
	m.symbolMu.Lock()
	defer m.symbolMu.Unlock()

	// Clear current symbols
	m.activeSymbols = make(map[string]bool)

	// Collect symbols from all active strategies
	for _, strategy := range m.strategies {
		if strategy.IsActive() {
			for _, symbol := range strategy.GetSymbols() {
				m.activeSymbols[symbol] = true
			}
		}
	}

	// Update streaming service
	symbols := make([]string, 0, len(m.activeSymbols))
	for symbol := range m.activeSymbols {
		symbols = append(symbols, symbol)
	}

	m.logger.Info("updated symbol stream", zap.Strings("symbols", symbols))

	// Connect to websocket and subscribe to symbols
	if m.stream != nil && len(symbols) > 0 {
		// Stage subscriptions BEFORE connecting to avoid race with auth
		if err := m.stream.Subscribe(symbols); err != nil {
			m.logger.Error("failed to stage/subscribe symbols", zap.Error(err))
		} else {
			m.logger.Info("staged symbol subscriptions", zap.Strings("symbols", symbols))
		}

		// Connect to websocket if not already connected
		if !m.stream.IsConnected() {
			m.logger.Info("connecting to websocket for market data streaming")
			if err := m.stream.Connect(); err != nil {
				m.logger.Error("failed to connect to websocket", zap.Error(err))
				// Continue anyway - strategies might work with polling or other data sources
			} else {
				m.logger.Info("websocket connected successfully")
			}
		}

		// Register handlers for market data
		m.stream.RegisterHandler("trade", func(msg interface{}) {
			if trade, ok := msg.(*models.Trade); ok {
				// Update cache and build snapshot
				var snapshot *models.Snapshot
				if m.cache != nil {
					m.cache.UpdateTradeFromStream(trade)
					if cached, found := m.cache.GetSnapshot(trade.Symbol); found {
						snapshot = cached
					}
				}
				if snapshot == nil {
					snapshot = &models.Snapshot{Symbol: trade.Symbol}
				}
				snapshot.LatestTrade = trade
				if m.cache != nil {
					m.cache.SetSnapshot(trade.Symbol, snapshot)
				}
				// Process trade data
				m.logger.Debug("received trade", zap.String("symbol", trade.Symbol), zap.String("price", trade.Price.String()))

				// Emit tick to strategies
				m.OnTick(trade.Symbol, snapshot)

				// Update bar aggregator from trade
				m.updateAggregatedBarFromTrade(trade)
			}
		})

		m.stream.RegisterHandler("quote", func(msg interface{}) {
			if quote, ok := msg.(*models.Quote); ok {
				// Update cache and build snapshot
				var snapshot *models.Snapshot
				if m.cache != nil {
					m.cache.UpdateQuoteFromStream(quote)
					if cached, found := m.cache.GetSnapshot(quote.Symbol); found {
						snapshot = cached
					}
				}
				if snapshot == nil {
					snapshot = &models.Snapshot{Symbol: quote.Symbol}
				}
				snapshot.LatestQuote = quote
				if m.cache != nil {
					m.cache.SetSnapshot(quote.Symbol, snapshot)
				}
				// Process quote data
				m.logger.Debug("received quote", zap.String("symbol", quote.Symbol), zap.String("bid", quote.BidPrice.String()), zap.String("ask", quote.AskPrice.String()))
				m.OnTick(quote.Symbol, snapshot)

				// Update bar aggregator from quote mid price (in case trade stream is limited)
				m.updateAggregatedBarFromQuote(quote)
			}
		})

		m.stream.RegisterHandler("bar", func(msg interface{}) {
			if bar, ok := msg.(*models.Bar); ok {
				// Process bar data
				m.logger.Debug("received bar", zap.String("symbol", bar.Symbol), zap.String("close", bar.Close.String()))
				m.OnBar(bar.Symbol, bar)
			}
		})
	}
}

// updateAggregatedBarFromTrade updates or finalizes a 1-minute bar from trade ticks
func (m *Manager) updateAggregatedBarFromTrade(trade *models.Trade) {
	minute := trade.Timestamp.Truncate(time.Minute)

	m.barMu.Lock()
	defer m.barMu.Unlock()

	bar, exists := m.currentBars[trade.Symbol]
	if !exists || bar == nil || bar.Timestamp.Before(minute) {
		// Finalize previous bar if it exists and emit
		if exists && bar != nil {
			prev := *bar
			m.logger.Debug("finalizing bar", zap.String("symbol", prev.Symbol), zap.Time("timestamp", prev.Timestamp), zap.String("close", prev.Close.String()))
			go m.OnBar(prev.Symbol, &prev)
		}
		// Start new bar
		m.currentBars[trade.Symbol] = &models.Bar{
			Symbol:     trade.Symbol,
			Open:       trade.Price,
			High:       trade.Price,
			Low:        trade.Price,
			Close:      trade.Price,
			Volume:     int64(trade.Size),
			Timestamp:  minute,
			TradeCount: 1,
			VWAP:       decimal.Zero,
		}
		return
	}

	// Update existing bar
	if trade.Price.GreaterThan(bar.High) {
		bar.High = trade.Price
	}
	if trade.Price.LessThan(bar.Low) {
		bar.Low = trade.Price
	}
	bar.Close = trade.Price
	bar.Volume += int64(trade.Size)
	bar.TradeCount++
}

// updateAggregatedBarFromQuote updates bar data from quotes using mid-price
func (m *Manager) updateAggregatedBarFromQuote(quote *models.Quote) {
	price := decimal.Zero
	if !quote.BidPrice.IsZero() && !quote.AskPrice.IsZero() {
		price = quote.BidPrice.Add(quote.AskPrice).Div(decimal.NewFromInt(2))
	} else if !quote.BidPrice.IsZero() {
		price = quote.BidPrice
	} else if !quote.AskPrice.IsZero() {
		price = quote.AskPrice
	}
	if price.IsZero() {
		return
	}

	minute := quote.Timestamp.Truncate(time.Minute)

	m.barMu.Lock()
	defer m.barMu.Unlock()

	bar, exists := m.currentBars[quote.Symbol]
	if !exists || bar == nil || bar.Timestamp.Before(minute) {
		// Finalize previous bar if it exists and emit
		if exists && bar != nil {
			prev := *bar
			m.logger.Debug("finalizing bar (quote)", zap.String("symbol", prev.Symbol), zap.Time("timestamp", prev.Timestamp), zap.String("close", prev.Close.String()))
			go m.OnBar(prev.Symbol, &prev)
		}
		// Start new bar (volume/tradecount unknown from quotes)
		m.currentBars[quote.Symbol] = &models.Bar{
			Symbol:     quote.Symbol,
			Open:       price,
			High:       price,
			Low:        price,
			Close:      price,
			Volume:     0,
			Timestamp:  minute,
			TradeCount: 0,
			VWAP:       decimal.Zero,
		}
		return
	}

	// Update existing bar
	if price.GreaterThan(bar.High) {
		bar.High = price
	}
	if price.LessThan(bar.Low) {
		bar.Low = price
	}
	bar.Close = price
}

// persistState and persistStateLocked persist current automation state to disk
func (m *Manager) persistState() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	m.persistStateLocked()
}

func (m *Manager) persistStateLocked() {
	strategies := make([]StrategyStatus, 0, len(m.strategies))
	hasActiveStrategy := false
	for _, s := range m.strategies {
		isActive := s.IsActive()
		if isActive {
			hasActiveStrategy = true
		}
		strategies = append(strategies, StrategyStatus{
			Name:    s.Name(),
			Type:    s.Type(),
			Symbols: s.GetSymbols(),
			Active:  isActive,
		})
	}

	// Only persist state if there are active strategies
	if hasActiveStrategy {
		state := &AutomationState{
			PID:        os.Getpid(),
			StartedAt:  time.Now(),
			Active:     m.active && hasActiveStrategy,
			Strategies: strategies,
		}
		if err := WriteAutomationState(state); err != nil {
			m.logger.Warn("failed to write automation state", zap.Error(err))
		}
	} else {
		// Remove state file if no active strategies
		if err := RemoveAutomationState(); err != nil && !errors.Is(err, os.ErrNotExist) {
			m.logger.Warn("failed to remove automation state", zap.Error(err))
		}
	}
}
