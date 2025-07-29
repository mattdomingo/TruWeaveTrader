package alpaca

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/TruWeaveTrader/alpaca-tui/internal/config"
	"github.com/TruWeaveTrader/alpaca-tui/internal/models"
	"github.com/shopspring/decimal"
)

// Client is a thin wrapper around Alpaca REST API
type Client struct {
	cfg        *config.Config
	httpClient *http.Client
	baseURL    string
	dataURL    string
}

// NewClient creates a new Alpaca client
func NewClient(cfg *config.Config) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: cfg.HTTPTimeout,
		},
		baseURL: cfg.AlpacaBaseURL,
		dataURL: cfg.AlpacaDataURL,
	}
}

// doRequest performs an HTTP request with auth headers
func (c *Client) doRequest(ctx context.Context, method, url string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("APCA-API-KEY-ID", c.cfg.AlpacaKeyID)
	req.Header.Set("APCA-API-SECRET-KEY", c.cfg.AlpacaSecretKey)
	req.Header.Set("Content-Type", "application/json")

	return c.httpClient.Do(req)
}

// parseResponse reads and unmarshals the response
func parseResponse(resp *http.Response, target interface{}) error {
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

// GetAccount retrieves account information
func (c *Client) GetAccount(ctx context.Context) (*models.Account, error) {
	resp, err := c.doRequest(ctx, "GET", c.baseURL+"/v2/account", nil)
	if err != nil {
		return nil, err
	}

	var account models.Account
	if err := parseResponse(resp, &account); err != nil {
		return nil, err
	}

	return &account, nil
}

// GetSnapshot retrieves a market snapshot for a symbol
func (c *Client) GetSnapshot(ctx context.Context, symbol string) (*models.Snapshot, error) {
	url := fmt.Sprintf("%s/v2/stocks/%s/snapshot", c.dataURL, url.PathEscape(symbol))
	resp, err := c.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	// First, let's capture the raw response for debugging
	defer resp.Body.Close()
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("failed to read response body: %w", readErr)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	// The correct JSON structure for Alpaca API v2 snapshot
	var result struct {
		Symbol       string        `json:"symbol"`
		LatestTrade  *models.Trade `json:"latestTrade"`
		LatestQuote  *models.Quote `json:"latestQuote"`
		MinuteBar    *models.Bar   `json:"minuteBar"`
		DailyBar     *models.Bar   `json:"dailyBar"`
		PrevDailyBar *models.Bar   `json:"prevDailyBar"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &models.Snapshot{
		Symbol:       result.Symbol,
		LatestTrade:  result.LatestTrade,
		LatestQuote:  result.LatestQuote,
		MinuteBar:    result.MinuteBar,
		DailyBar:     result.DailyBar,
		PrevDailyBar: result.PrevDailyBar,
	}, nil
}

// PlaceOrder submits a new order
func (c *Client) PlaceOrder(ctx context.Context, order *models.OrderRequest) (*models.Order, error) {
	resp, err := c.doRequest(ctx, "POST", c.baseURL+"/v2/orders", order)
	if err != nil {
		return nil, err
	}

	var newOrder models.Order
	if err := parseResponse(resp, &newOrder); err != nil {
		return nil, err
	}

	return &newOrder, nil
}

// GetOrders retrieves a list of orders
func (c *Client) GetOrders(ctx context.Context, status string) ([]*models.Order, error) {
	url := c.baseURL + "/v2/orders"
	if status != "" {
		url += "?status=" + status
	}

	resp, err := c.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	var orders []*models.Order
	if err := parseResponse(resp, &orders); err != nil {
		return nil, err
	}

	return orders, nil
}

// GetOrder retrieves a specific order by ID
func (c *Client) GetOrder(ctx context.Context, orderID string) (*models.Order, error) {
	url := fmt.Sprintf("%s/v2/orders/%s", c.baseURL, orderID)
	resp, err := c.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	var order models.Order
	if err := parseResponse(resp, &order); err != nil {
		return nil, err
	}

	return &order, nil
}

// CancelOrder cancels an existing order
func (c *Client) CancelOrder(ctx context.Context, orderID string) error {
	url := fmt.Sprintf("%s/v2/orders/%s", c.baseURL, orderID)
	resp, err := c.doRequest(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cancel order failed %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetPositions retrieves all positions
func (c *Client) GetPositions(ctx context.Context) ([]*models.Position, error) {
	resp, err := c.doRequest(ctx, "GET", c.baseURL+"/v2/positions", nil)
	if err != nil {
		return nil, err
	}

	var positions []*models.Position
	if err := parseResponse(resp, &positions); err != nil {
		return nil, err
	}

	return positions, nil
}

// GetPosition retrieves a specific position
func (c *Client) GetPosition(ctx context.Context, symbol string) (*models.Position, error) {
	url := fmt.Sprintf("%s/v2/positions/%s", c.baseURL, url.PathEscape(symbol))
	resp, err := c.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	var position models.Position
	if err := parseResponse(resp, &position); err != nil {
		return nil, err
	}

	return &position, nil
}

// ClosePosition closes a position
func (c *Client) ClosePosition(ctx context.Context, symbol string) (*models.Order, error) {
	url := fmt.Sprintf("%s/v2/positions/%s", c.baseURL, url.PathEscape(symbol))
	resp, err := c.doRequest(ctx, "DELETE", url, nil)
	if err != nil {
		return nil, err
	}

	var order models.Order
	if err := parseResponse(resp, &order); err != nil {
		return nil, err
	}

	return &order, nil
}

// GetBars retrieves historical bars
func (c *Client) GetBars(ctx context.Context, symbol string, timeframe string, start, end time.Time, limit int) ([]*models.Bar, error) {
	params := url.Values{}
	params.Set("symbols", symbol)
	params.Set("timeframe", timeframe)
	if !start.IsZero() {
		params.Set("start", start.Format(time.RFC3339))
	}
	if !end.IsZero() {
		params.Set("end", end.Format(time.RFC3339))
	}
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}

	url := fmt.Sprintf("%s/v2/stocks/bars?%s", c.dataURL, params.Encode())
	resp, err := c.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Bars map[string][]*models.Bar `json:"bars"`
	}
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return result.Bars[symbol], nil
}

// GetOptionChain retrieves option chain data (placeholder - Alpaca doesn't have direct option chain endpoint)
func (c *Client) GetOptionChain(ctx context.Context, underlying string, expiry time.Time) ([]*models.OptionContract, error) {
	// Note: This is a placeholder. Alpaca's options API may require different endpoints
	// You would need to implement this based on their actual options API
	return nil, fmt.Errorf("option chain not yet implemented")
}

// Helper to create decimal pointer
func DecimalPtr(v float64) *decimal.Decimal {
	d := decimal.NewFromFloat(v)
	return &d
}
