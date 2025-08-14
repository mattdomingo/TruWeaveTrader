package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/TruWeaveTrader/alpaca-tui/internal/cache"
	"github.com/TruWeaveTrader/alpaca-tui/internal/config"
	"github.com/TruWeaveTrader/alpaca-tui/internal/models"
	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// StreamClient manages websocket connections for real-time data
type StreamClient struct {
	cfg                   *config.Config
	cache                 *cache.Cache
	logger                *zap.Logger
	conn                  *websocket.Conn
	mu                    sync.RWMutex
	subscriptions         map[string]bool
	handlers              map[string]Handler
	reconnectDelay        time.Duration
	ctx                   context.Context
	cancel                context.CancelFunc
	isConnected           bool
	isAuthenticated       bool
	connectionAttempts    int
	maxConnectionAttempts int
}

// Handler is a callback for stream messages
type Handler func(message interface{})

// Message envelopes and concrete message types
// We decode per-type to avoid conflicting JSON keys (e.g., "c" is conditions for trades but close for bars)
type messageEnvelope struct {
	MessageType string    `json:"T"`
	Timestamp   time.Time `json:"t,omitempty"` // Explicitly include to avoid conflicts
}

type tradeMessage struct {
	T    string          `json:"T"`
	S    string          `json:"S"`
	X    string          `json:"x"`
	P    decimal.Decimal `json:"p"`
	Size int32           `json:"s"`
	C    []string        `json:"c"`
	I    int64           `json:"i"`
	Z    string          `json:"z"`
	Time time.Time       `json:"t"`
}

type quoteMessage struct {
	T        string          `json:"T"`
	S        string          `json:"S"`
	BidPrice decimal.Decimal `json:"bp"`
	BidSize  int32           `json:"bs"`
	AskPrice decimal.Decimal `json:"ap"`
	AskSize  int32           `json:"as"`
	Time     time.Time       `json:"t"`
	C        []string        `json:"c"`
	Z        string          `json:"z"`
}

type barMessage struct {
	T      string          `json:"T"`
	S      string          `json:"S"`
	Open   decimal.Decimal `json:"o"`
	High   decimal.Decimal `json:"h"`
	Low    decimal.Decimal `json:"l"`
	Close  decimal.Decimal `json:"c"`
	Volume int64           `json:"v"`
	Time   time.Time       `json:"t"`
	Count  int64           `json:"n"`
	VWAP   decimal.Decimal `json:"vw"`
}

type successMessage struct {
	T   string `json:"T"`
	Msg string `json:"msg"`
}

type errorMessage struct {
	T    string `json:"T"`
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	S    string `json:"S"`
}

// NewStreamClient creates a new streaming client
func NewStreamClient(cfg *config.Config, cache *cache.Cache, logger *zap.Logger) *StreamClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &StreamClient{
		cfg:                   cfg,
		cache:                 cache,
		logger:                logger,
		subscriptions:         make(map[string]bool),
		handlers:              make(map[string]Handler),
		reconnectDelay:        cfg.WebsocketReconnectDelay,
		ctx:                   ctx,
		cancel:                cancel,
		maxConnectionAttempts: 5,
	}
}

// Connect establishes websocket connection
func (c *StreamClient) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Close existing connection if any
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
		c.isConnected = false
		c.isAuthenticated = false
	}

	wsURL := "wss://stream.data.alpaca.markets/v2/iex"

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		c.connectionAttempts++
		return fmt.Errorf("websocket dial: %w", err)
	}

	c.conn = conn
	c.isConnected = true
	c.isAuthenticated = false
	c.connectionAttempts = 0

	// Authenticate immediately
	auth := struct {
		Action string `json:"action"`
		Key    string `json:"key"`
		Secret string `json:"secret"`
	}{
		Action: "auth",
		Key:    c.cfg.AlpacaKeyID,
		Secret: c.cfg.AlpacaSecretKey,
	}

	if err := c.conn.WriteJSON(auth); err != nil {
		c.conn.Close()
		c.conn = nil
		c.isConnected = false
		return fmt.Errorf("auth write: %w", err)
	}

	// Start message handler
	go c.handleMessages()

	c.logger.Info("Websocket connected", zap.String("url", wsURL))
	return nil
}

// Subscribe adds symbols to stream
func (c *StreamClient) Subscribe(symbols []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Update subscriptions map
	for _, symbol := range symbols {
		c.subscriptions[symbol] = true
	}

	if c.isConnected && c.isAuthenticated {
		c.logger.Info("Sending subscription", zap.Strings("symbols", symbols))
		return c.subscribeSymbols(symbols)
	}

	// Not connected or not authenticated yet; stage the subscriptions
	c.logger.Info("Staged subscriptions (will subscribe after authentication)", zap.Strings("symbols", symbols))
	return nil
}

// subscribeSymbols sends subscription message
func (c *StreamClient) subscribeSymbols(symbols []string) error {
	msg := struct {
		Action string   `json:"action"`
		Trades []string `json:"trades,omitempty"`
		Quotes []string `json:"quotes,omitempty"`
		Bars   []string `json:"bars,omitempty"`
	}{
		Action: "subscribe",
		Trades: symbols,
		Quotes: symbols,
		Bars:   symbols,
	}

	c.logger.Info("Sending subscription", zap.Strings("symbols", symbols))
	return c.conn.WriteJSON(msg)
}

// Unsubscribe removes symbols from stream
func (c *StreamClient) Unsubscribe(symbols []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Update subscriptions map
	for _, symbol := range symbols {
		delete(c.subscriptions, symbol)
	}

	if c.isConnected && c.isAuthenticated {
		msg := struct {
			Action string   `json:"action"`
			Trades []string `json:"trades,omitempty"`
			Quotes []string `json:"quotes,omitempty"`
			Bars   []string `json:"bars,omitempty"`
		}{
			Action: "unsubscribe",
			Trades: symbols,
			Quotes: symbols,
			Bars:   symbols,
		}
		return c.conn.WriteJSON(msg)
	}

	return nil
}

// RegisterHandler registers a callback for a specific message type
func (c *StreamClient) RegisterHandler(msgType string, handler Handler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers[msgType] = handler
}

// handleMessages processes incoming websocket messages
func (c *StreamClient) handleMessages() {
	defer func() {
		c.mu.Lock()
		c.isConnected = false
		c.isAuthenticated = false
		c.mu.Unlock()

		// Only attempt reconnect if we haven't exceeded max attempts
		if c.connectionAttempts < c.maxConnectionAttempts {
			c.reconnect()
		} else {
			c.logger.Error("Max connection attempts reached, stopping reconnection")
		}
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			var rawMsgs []json.RawMessage

			// Set read deadline
			c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

			if err := c.conn.ReadJSON(&rawMsgs); err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					c.logger.Error("Websocket read error", zap.Error(err))
				}
				return
			}

			// Process messages
			for _, raw := range rawMsgs {
				c.processMessage(raw)
			}
		}
	}
}

// processMessage handles individual stream messages
func (c *StreamClient) processMessage(raw json.RawMessage) {
	var env messageEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		c.logger.Error("failed to parse message envelope", zap.Error(err))
		return
	}

	// Debug: uncomment to see raw messages
	// fmt.Printf("🔍 DEBUG: Raw message: %s\n", string(raw))
	// fmt.Printf("🔍 DEBUG: Parsed type: %s\n", env.MessageType)

	switch env.MessageType {
	case "t": // Trade
		var tm tradeMessage
		if err := json.Unmarshal(raw, &tm); err != nil {
			c.logger.Error("failed to parse trade message", zap.Error(err))
			return
		}
		trade := &models.Trade{
			Symbol:     tm.S,
			Price:      tm.P,
			Size:       tm.Size,
			Timestamp:  tm.Time,
			Conditions: tm.C,
			ID:         tm.I,
			Tape:       tm.Z,
		}
		c.cache.UpdateTradeFromStream(trade)
		if handler, ok := c.handlers["trade"]; ok {
			handler(trade)
		}

	case "q": // Quote
		var qm quoteMessage
		if err := json.Unmarshal(raw, &qm); err != nil {
			c.logger.Error("failed to parse quote message", zap.Error(err))
			return
		}
		quote := &models.Quote{
			Symbol:     qm.S,
			BidPrice:   qm.BidPrice,
			BidSize:    qm.BidSize,
			AskPrice:   qm.AskPrice,
			AskSize:    qm.AskSize,
			Timestamp:  qm.Time,
			Conditions: qm.C,
			Tape:       qm.Z,
		}
		c.cache.UpdateQuoteFromStream(quote)
		if handler, ok := c.handlers["quote"]; ok {
			handler(quote)
		}

	case "b": // Bar
		var bm barMessage
		if err := json.Unmarshal(raw, &bm); err != nil {
			c.logger.Error("failed to parse bar message", zap.Error(err))
			return
		}
		bar := &models.Bar{
			Symbol:     bm.S,
			Open:       bm.Open,
			High:       bm.High,
			Low:        bm.Low,
			Close:      bm.Close,
			Volume:     bm.Volume,
			Timestamp:  bm.Time,
			TradeCount: bm.Count,
			VWAP:       bm.VWAP,
		}
		if handler, ok := c.handlers["bar"]; ok {
			handler(bar)
		}

	case "success":
		var sm successMessage
		if err := json.Unmarshal(raw, &sm); err != nil {
			c.logger.Error("failed to parse success message", zap.Error(err))
			return
		}
		c.logger.Info("Stream message", zap.String("type", env.MessageType), zap.String("msg", sm.Msg))

		// Mark as authenticated only on "authenticated" message, not "connected"
		if sm.Msg == "authenticated" {
			c.mu.Lock()
			c.isAuthenticated = true
			c.mu.Unlock()
		}

		// Resubscribe to existing symbols after authentication
		if sm.Msg == "authenticated" {
			c.mu.RLock()
			haveSubs := len(c.subscriptions) > 0
			c.mu.RUnlock()
			if haveSubs {
				symbols := make([]string, 0)
				c.mu.RLock()
				for symbol := range c.subscriptions {
					symbols = append(symbols, symbol)
				}
				c.mu.RUnlock()
				c.logger.Info("Resubscribing after authentication", zap.Strings("symbols", symbols))
				if err := c.subscribeSymbols(symbols); err != nil {
					c.logger.Error("Failed to resubscribe after authentication", zap.Error(err))
				}
			} else {
				c.logger.Info("No staged subscriptions to resubscribe after authentication")
			}
		}

	case "error":
		var em errorMessage
		if err := json.Unmarshal(raw, &em); err != nil {
			c.logger.Error("failed to parse error message", zap.Error(err))
			return
		}
		c.logger.Error("Stream error",
			zap.String("type", env.MessageType),
			zap.Int("code", em.Code),
			zap.String("message", em.Msg),
			zap.String("symbol", em.S))

		// Handle specific error codes
		if em.Code == 406 { // Connection limit exceeded
			c.logger.Error("Connection limit exceeded - waiting longer before retry")
			// Increase retry delay for connection limit errors
			c.reconnectDelay = 30 * time.Second
		} else if em.Code == 401 { // Authentication error
			c.logger.Error("Authentication failed - check API credentials")
			c.mu.Lock()
			c.isAuthenticated = false
			c.mu.Unlock()
		}

		if handler, ok := c.handlers["error"]; ok {
			handler(em)
		}
	}
}

// reconnect attempts to reconnect with exponential backoff
func (c *StreamClient) reconnect() {
	backoff := c.reconnectDelay
	maxBackoff := 60 * time.Second

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-time.After(backoff):
			if c.connectionAttempts >= c.maxConnectionAttempts {
				c.logger.Error("Max connection attempts reached, stopping reconnection",
					zap.Int("attempts", c.connectionAttempts))
				return
			}

			c.logger.Info("Attempting to reconnect",
				zap.Duration("backoff", backoff),
				zap.Int("attempt", c.connectionAttempts+1))

			if err := c.Connect(); err != nil {
				c.logger.Error("Reconnect failed", zap.Error(err))
				// Exponential backoff
				backoff = backoff * 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			} else {
				c.logger.Info("Reconnected successfully")
				return
			}
		}
	}
}

// Close gracefully shuts down the stream client
func (c *StreamClient) Close() error {
	c.cancel()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		// Send close message
		err := c.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			c.logger.Error("Error sending close message", zap.Error(err))
		}

		// Close connection
		closeErr := c.conn.Close()
		c.conn = nil
		c.isConnected = false
		c.isAuthenticated = false
		return closeErr
	}

	return nil
}

// IsConnected returns connection status
func (c *StreamClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isConnected && c.isAuthenticated
}

// GetConnectionStatus returns detailed connection status
func (c *StreamClient) GetConnectionStatus() (connected, authenticated bool, attempts int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isConnected, c.isAuthenticated, c.connectionAttempts
}
