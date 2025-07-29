package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// OrderSide represents buy or sell
type OrderSide string

const (
	Buy  OrderSide = "buy"
	Sell OrderSide = "sell"
)

// OrderType represents the order type
type OrderType string

const (
	Market    OrderType = "market"
	Limit     OrderType = "limit"
	Stop      OrderType = "stop"
	StopLimit OrderType = "stop_limit"
)

// TimeInForce represents order duration
type TimeInForce string

const (
	Day TimeInForce = "day"
	GTC TimeInForce = "gtc"
	IOC TimeInForce = "ioc"
	FOK TimeInForce = "fok"
)

// OrderStatus represents the current status of an order
type OrderStatus string

const (
	OrderNew             OrderStatus = "new"
	OrderPartiallyFilled OrderStatus = "partially_filled"
	OrderFilled          OrderStatus = "filled"
	OrderDoneForDay      OrderStatus = "done_for_day"
	OrderCanceled        OrderStatus = "canceled"
	OrderExpired         OrderStatus = "expired"
	OrderReplaced        OrderStatus = "replaced"
	OrderPending         OrderStatus = "pending_new"
	OrderAccepted        OrderStatus = "accepted"
	OrderPendingCancel   OrderStatus = "pending_cancel"
	OrderStopped         OrderStatus = "stopped"
	OrderRejected        OrderStatus = "rejected"
	OrderSuspended       OrderStatus = "suspended"
)

// Order represents a trading order
type Order struct {
	ID             string           `json:"id"`
	ClientOrderID  string           `json:"client_order_id"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
	SubmittedAt    time.Time        `json:"submitted_at"`
	FilledAt       *time.Time       `json:"filled_at"`
	ExpiredAt      *time.Time       `json:"expired_at"`
	CanceledAt     *time.Time       `json:"canceled_at"`
	FailedAt       *time.Time       `json:"failed_at"`
	ReplacedAt     *time.Time       `json:"replaced_at"`
	ReplacedBy     *string          `json:"replaced_by"`
	Replaces       *string          `json:"replaces"`
	AssetID        string           `json:"asset_id"`
	Symbol         string           `json:"symbol"`
	AssetClass     string           `json:"asset_class"`
	Notional       *decimal.Decimal `json:"notional"`
	Qty            decimal.Decimal  `json:"qty"`
	FilledQty      decimal.Decimal  `json:"filled_qty"`
	FilledAvgPrice *decimal.Decimal `json:"filled_avg_price"`
	OrderClass     string           `json:"order_class"`
	OrderType      OrderType        `json:"order_type"`
	Type           OrderType        `json:"type"`
	Side           OrderSide        `json:"side"`
	TimeInForce    TimeInForce      `json:"time_in_force"`
	LimitPrice     *decimal.Decimal `json:"limit_price"`
	StopPrice      *decimal.Decimal `json:"stop_price"`
	Status         OrderStatus      `json:"status"`
	ExtendedHours  bool             `json:"extended_hours"`
}

// OrderRequest represents a request to create a new order
type OrderRequest struct {
	Symbol        string           `json:"symbol"`
	Qty           *decimal.Decimal `json:"qty,omitempty"`
	Notional      *decimal.Decimal `json:"notional,omitempty"`
	Side          OrderSide        `json:"side"`
	Type          OrderType        `json:"type"`
	TimeInForce   TimeInForce      `json:"time_in_force"`
	LimitPrice    *decimal.Decimal `json:"limit_price,omitempty"`
	StopPrice     *decimal.Decimal `json:"stop_price,omitempty"`
	ExtendedHours bool             `json:"extended_hours,omitempty"`
	ClientOrderID string           `json:"client_order_id,omitempty"`
}

// Position represents a current position
type Position struct {
	AssetID                string          `json:"asset_id"`
	Symbol                 string          `json:"symbol"`
	Exchange               string          `json:"exchange"`
	AssetClass             string          `json:"asset_class"`
	Qty                    decimal.Decimal `json:"qty"`
	AvgEntryPrice          decimal.Decimal `json:"avg_entry_price"`
	Side                   string          `json:"side"`
	MarketValue            decimal.Decimal `json:"market_value"`
	CostBasis              decimal.Decimal `json:"cost_basis"`
	UnrealizedPL           decimal.Decimal `json:"unrealized_pl"`
	UnrealizedPLPC         decimal.Decimal `json:"unrealized_plpc"`
	UnrealizedIntradayPL   decimal.Decimal `json:"unrealized_intraday_pl"`
	UnrealizedIntradayPLPC decimal.Decimal `json:"unrealized_intraday_plpc"`
	CurrentPrice           decimal.Decimal `json:"current_price"`
	LastdayPrice           decimal.Decimal `json:"lastday_price"`
	ChangeToday            decimal.Decimal `json:"change_today"`
}

// Account represents account information
type Account struct {
	ID                    string          `json:"id"`
	AccountNumber         string          `json:"account_number"`
	Status                string          `json:"status"`
	Currency              string          `json:"currency"`
	BuyingPower           decimal.Decimal `json:"buying_power"`
	RegtBuyingPower       decimal.Decimal `json:"regt_buying_power"`
	DaytradingBuyingPower decimal.Decimal `json:"daytrading_buying_power"`
	Cash                  decimal.Decimal `json:"cash"`
	CashWithdrawable      decimal.Decimal `json:"cash_withdrawable"`
	CashTransferable      decimal.Decimal `json:"cash_transferable"`
	PendingTransferOut    decimal.Decimal `json:"pending_transfer_out"`
	PortfolioValue        decimal.Decimal `json:"portfolio_value"`
	PatternDayTrader      bool            `json:"pattern_day_trader"`
	TradingBlocked        bool            `json:"trading_blocked"`
	TransfersBlocked      bool            `json:"transfers_blocked"`
	AccountBlocked        bool            `json:"account_blocked"`
	CreatedAt             time.Time       `json:"created_at"`
	TradeSuspendedByUser  bool            `json:"trade_suspended_by_user"`
	Multiplier            decimal.Decimal `json:"multiplier"`
	ShortsideValue        decimal.Decimal `json:"shortside_value"`
	Equity                decimal.Decimal `json:"equity"`
	LastEquity            decimal.Decimal `json:"last_equity"`
	LongMarketValue       decimal.Decimal `json:"long_market_value"`
	ShortMarketValue      decimal.Decimal `json:"short_market_value"`
	InitialMargin         decimal.Decimal `json:"initial_margin"`
	MaintenanceMargin     decimal.Decimal `json:"maintenance_margin"`
	LastMaintenanceMargin decimal.Decimal `json:"last_maintenance_margin"`
	SMA                   decimal.Decimal `json:"sma"`
	DaytradeCount         int64           `json:"daytrade_count"`
}

// Quote represents a market quote
type Quote struct {
	Symbol     string          `json:"symbol"`
	BidPrice   decimal.Decimal `json:"bp"`
	BidSize    int32           `json:"bs"`
	AskPrice   decimal.Decimal `json:"ap"`
	AskSize    int32           `json:"as"`
	Timestamp  time.Time       `json:"t"`
	Conditions []string        `json:"c"`
	Tape       string          `json:"z"`
}

// Trade represents a market trade
type Trade struct {
	Symbol     string          `json:"symbol"`
	Price      decimal.Decimal `json:"p"`
	Size       int32           `json:"s"`
	Timestamp  time.Time       `json:"t"`
	Conditions []string        `json:"c"`
	ID         int64           `json:"i"`
	Tape       string          `json:"z"`
}

// Bar represents an OHLCV bar
type Bar struct {
	Symbol     string          `json:"symbol"`
	Open       decimal.Decimal `json:"o"`
	High       decimal.Decimal `json:"h"`
	Low        decimal.Decimal `json:"l"`
	Close      decimal.Decimal `json:"c"`
	Volume     int64           `json:"v"`
	Timestamp  time.Time       `json:"t"`
	TradeCount int64           `json:"n"`
	VWAP       decimal.Decimal `json:"vw"`
}

// Snapshot represents a market snapshot
type Snapshot struct {
	Symbol       string `json:"symbol"`
	LatestTrade  *Trade `json:"latestTrade"`
	LatestQuote  *Quote `json:"latestQuote"`
	MinuteBar    *Bar   `json:"minuteBar"`
	DailyBar     *Bar   `json:"dailyBar"`
	PrevDailyBar *Bar   `json:"prevDailyBar"`
}

// OptionContract represents an options contract
type OptionContract struct {
	Symbol       string          `json:"symbol"`
	Underlying   string          `json:"underlying"`
	Strike       decimal.Decimal `json:"strike"`
	Expiry       time.Time       `json:"expiry"`
	Type         string          `json:"type"` // "call" or "put"
	OpenInterest int64           `json:"open_interest"`
	Volume       int64           `json:"volume"`
	IV           decimal.Decimal `json:"iv"`
	Delta        decimal.Decimal `json:"delta"`
	Gamma        decimal.Decimal `json:"gamma"`
	Theta        decimal.Decimal `json:"theta"`
	Vega         decimal.Decimal `json:"vega"`
	Rho          decimal.Decimal `json:"rho"`
}
