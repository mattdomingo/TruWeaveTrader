package websocket

import (
	"context"
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
	cfg            *config.Config
	cache          *cache.Cache
	logger         *zap.Logger
	conn           *websocket.Conn
	mu             sync.RWMutex
	subscriptions  map[string]bool
	handlers       map[string]Handler
	reconnectDelay time.Duration
	ctx            context.Context
	cancel         context.CancelFunc
	isConnected    bool
}

// Handler is a callback for stream messages
type Handler func(message interface{})

// Message types
type authMessage struct {
	Action string `json:"action"`
	Key    string `json:"key"`
	Secret string `json:"secret"`
}

type subscribeMessage struct {
	Action string   `json:"action"`
	Trades []string `json:"trades,omitempty"`
	Quotes []string `json:"quotes,omitempty"`
	Bars   []string `json:"bars,omitempty"`
}

type streamMessage struct {
	T  string          `json:"T"`  // message type
	S  string          `json:"S"`  // symbol
	X  string          `json:"x"`  // exchange
	P  decimal.Decimal `json:"p"`  // price
	S2 int32           `json:"s"`  // size
	C  []string        `json:"c"`  // conditions
	I  int64           `json:"i"`  // ID
	Z  string          `json:"z"`  // tape
	T2 time.Time       `json:"t"`  // timestamp
	BP decimal.Decimal `json:"bp"` // bid price
	BS int32           `json:"bs"` // bid size
	AP decimal.Decimal `json:"ap"` // ask price
	AS int32           `json:"as"` // ask size

	// Error message fields
	Code int    `json:"code"` // error code
	Msg  string `json:"msg"`  // error message
}

// NewStreamClient creates a new streaming client
func NewStreamClient(cfg *config.Config, cache *cache.Cache, logger *zap.Logger) *StreamClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &StreamClient{
		cfg:            cfg,
		cache:          cache,
		logger:         logger,
		subscriptions:  make(map[string]bool),
		handlers:       make(map[string]Handler),
		reconnectDelay: cfg.WebsocketReconnectDelay,
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Connect establishes websocket connection
func (c *StreamClient) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	wsURL := "wss://stream.data.alpaca.markets/v2/iex"

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("websocket dial: %w", err)
	}

	c.conn = conn
	c.isConnected = true

	// Authenticate
	auth := authMessage{
		Action: "auth",
		Key:    c.cfg.AlpacaKeyID,
		Secret: c.cfg.AlpacaSecretKey,
	}

	if err := c.conn.WriteJSON(auth); err != nil {
		return fmt.Errorf("auth write: %w", err)
	}

	// Start message handler
	go c.handleMessages()

	// Resubscribe to existing symbols
	if len(c.subscriptions) > 0 {
		symbols := make([]string, 0, len(c.subscriptions))
		for symbol := range c.subscriptions {
			symbols = append(symbols, symbol)
		}
		c.subscribeSymbols(symbols)
	}

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

	if c.isConnected {
		return c.subscribeSymbols(symbols)
	}

	return nil
}

// subscribeSymbols sends subscription message
func (c *StreamClient) subscribeSymbols(symbols []string) error {
	msg := subscribeMessage{
		Action: "subscribe",
		Trades: symbols,
		Quotes: symbols,
		Bars:   symbols,
	}

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

	if c.isConnected {
		msg := subscribeMessage{
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
		c.mu.Unlock()
		c.reconnect()
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			var msg []streamMessage

			// Set read deadline
			c.conn.SetReadDeadline(time.Now().Add(30 * time.Second))

			err := c.conn.ReadJSON(&msg)
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					c.logger.Error("Websocket read error", zap.Error(err))
				}
				return
			}

			// Process messages
			for _, m := range msg {
				c.processMessage(m)
			}
		}
	}
}

// processMessage handles individual stream messages
func (c *StreamClient) processMessage(msg streamMessage) {
	switch msg.T {
	case "t": // Trade
		trade := &models.Trade{
			Symbol:     msg.S,
			Price:      msg.P,
			Size:       msg.S2,
			Timestamp:  msg.T2,
			Conditions: msg.C,
			ID:         msg.I,
			Tape:       msg.Z,
		}
		c.cache.UpdateTradeFromStream(trade)
		if handler, ok := c.handlers["trade"]; ok {
			handler(trade)
		}

	case "q": // Quote
		quote := &models.Quote{
			Symbol:     msg.S,
			BidPrice:   msg.BP,
			BidSize:    msg.BS,
			AskPrice:   msg.AP,
			AskSize:    msg.AS,
			Timestamp:  msg.T2,
			Conditions: msg.C,
			Tape:       msg.Z,
		}
		c.cache.UpdateQuoteFromStream(quote)
		if handler, ok := c.handlers["quote"]; ok {
			handler(quote)
		}

	case "b": // Bar
		// Handle bar messages if needed
		if handler, ok := c.handlers["bar"]; ok {
			handler(msg)
		}

	case "success":
		c.logger.Info("Stream message", zap.String("type", msg.T))

	case "error":
		c.logger.Error("Stream error",
			zap.String("type", msg.T),
			zap.Int("code", msg.Code),
			zap.String("message", msg.Msg),
			zap.String("symbol", msg.S))
		if handler, ok := c.handlers["error"]; ok {
			handler(msg)
		}
	}
}

// reconnect attempts to reconnect with exponential backoff
func (c *StreamClient) reconnect() {
	backoff := c.reconnectDelay
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-time.After(backoff):
			c.logger.Info("Attempting to reconnect", zap.Duration("backoff", backoff))

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
		return c.conn.Close()
	}

	return nil
}

// IsConnected returns connection status
func (c *StreamClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isConnected
}
