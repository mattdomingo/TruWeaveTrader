package strategy

import (
	"context"
	"fmt"
	"sync"

	"github.com/TruWeaveTrader/alpaca-tui/internal/alpaca"
	"github.com/TruWeaveTrader/alpaca-tui/internal/cache"
	"github.com/TruWeaveTrader/alpaca-tui/internal/config"
	"github.com/TruWeaveTrader/alpaca-tui/internal/models"
	"github.com/TruWeaveTrader/alpaca-tui/internal/risk"
	"github.com/TruWeaveTrader/alpaca-tui/internal/websocket"
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

	// Add symbols to monitoring list if manager is active
	if m.active {
		m.addSymbolsToStream(strategy.GetSymbols())
	}

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
	m.mu.RLock()
	strategy, exists := m.strategies[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("strategy %s not found", name)
	}

	if strategy.IsActive() {
		return fmt.Errorf("strategy %s is already active", name)
	}

	if err := strategy.Start(m.ctx); err != nil {
		return fmt.Errorf("failed to start strategy %s: %w", name, err)
	}

	// Add symbols to stream if manager is active
	if m.active {
		m.addSymbolsToStream(strategy.GetSymbols())
	}

	m.logger.Info("strategy started", zap.String("name", name))
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
		return
	}

	select {
	case m.signalChan <- signal:
	case <-m.ctx.Done():
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

	// Get current market data for validation
	snapshot, err := m.client.GetSnapshot(m.ctx, signal.Symbol)
	if err != nil {
		logger.Error("failed to get market snapshot", zap.Error(err))
		return
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
	if snapshot.LatestQuote != nil {
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
		// Connect to websocket if not already connected
		if !m.stream.IsConnected() {
			if err := m.stream.Connect(); err != nil {
				m.logger.Error("failed to connect to websocket", zap.Error(err))
				return
			}
		}

		// Subscribe to symbols
		if err := m.stream.Subscribe(symbols); err != nil {
			m.logger.Error("failed to subscribe to symbols", zap.Error(err))
			return
		}

		// Register handlers for market data
		m.stream.RegisterHandler("trade", func(msg interface{}) {
			if trade, ok := msg.(*models.Trade); ok {
				m.logger.Debug("received trade", zap.String("symbol", trade.Symbol), zap.String("price", trade.Price.String()))

				// Build snapshot and forward to strategies
				snapshot, _ := m.cache.GetSnapshot(trade.Symbol)
				if snapshot == nil {
					snapshot = &models.Snapshot{Symbol: trade.Symbol}
				}
				snapshot.LatestTrade = trade
				m.cache.SetSnapshot(trade.Symbol, snapshot)
				m.OnTick(trade.Symbol, snapshot)
			}
		})

		m.stream.RegisterHandler("quote", func(msg interface{}) {
			if quote, ok := msg.(*models.Quote); ok {
				m.logger.Debug("received quote", zap.String("symbol", quote.Symbol), zap.String("bid", quote.BidPrice.String()), zap.String("ask", quote.AskPrice.String()))

				// Build snapshot and forward to strategies
				snapshot, _ := m.cache.GetSnapshot(quote.Symbol)
				if snapshot == nil {
					snapshot = &models.Snapshot{Symbol: quote.Symbol}
				}
				snapshot.LatestQuote = quote
				m.cache.SetSnapshot(quote.Symbol, snapshot)
				m.OnTick(quote.Symbol, snapshot)
			}
		})

		m.stream.RegisterHandler("bar", func(msg interface{}) {
			if bar, ok := msg.(*models.Bar); ok {
				m.logger.Debug("received bar", zap.String("symbol", bar.Symbol), zap.String("close", bar.Close.String()))
				m.OnBar(bar.Symbol, bar)
			}
		})
	}
}
