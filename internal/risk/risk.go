package risk

import (
	"fmt"

	"github.com/TruWeaveTrader/alpaca-tui/internal/config"
	"github.com/TruWeaveTrader/alpaca-tui/internal/models"
	"github.com/shopspring/decimal"
)

// Manager handles risk checks and position sizing
type Manager struct {
	cfg *config.Config
}

// NewManager creates a new risk manager
func NewManager(cfg *config.Config) *Manager {
	return &Manager{cfg: cfg}
}

// CheckResult contains the result of a risk check
type CheckResult struct {
	Passed   bool
	Reason   string
	Warnings []string
}

// ValidateOrder performs pre-trade risk checks
func (m *Manager) ValidateOrder(order *models.OrderRequest, account *models.Account, currentPrice decimal.Decimal) CheckResult {
	result := CheckResult{Passed: true, Warnings: []string{}}

	// Check if trading is blocked
	if account.TradingBlocked || account.AccountBlocked {
		return CheckResult{
			Passed: false,
			Reason: "Trading is blocked on this account",
		}
	}

	// Calculate order value
	orderValue := m.calculateOrderValue(order, currentPrice)

	// Check against max position size
	maxPosition := decimal.NewFromFloat(m.cfg.RiskMaxPositionUSD)
	if orderValue.GreaterThan(maxPosition) {
		return CheckResult{
			Passed: false,
			Reason: fmt.Sprintf("Order value $%.2f exceeds max position size $%.2f",
				orderValue.InexactFloat64(), maxPosition.InexactFloat64()),
		}
	}

	// Check buying power
	if order.Side == models.Buy {
		if orderValue.GreaterThan(account.BuyingPower) {
			return CheckResult{
				Passed: false,
				Reason: fmt.Sprintf("Insufficient buying power. Need $%.2f, have $%.2f",
					orderValue.InexactFloat64(), account.BuyingPower.InexactFloat64()),
			}
		}
	}

	// Check day trading buying power if applicable
	if account.PatternDayTrader && orderValue.GreaterThan(account.DaytradingBuyingPower) {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Order uses %.1f%% of day trading buying power",
				orderValue.Div(account.DaytradingBuyingPower).Mul(decimal.NewFromInt(100)).InexactFloat64()))
	}

	// Warn if using significant portion of buying power
	buyingPowerUsage := orderValue.Div(account.BuyingPower).Mul(decimal.NewFromInt(100))
	if buyingPowerUsage.GreaterThan(decimal.NewFromInt(50)) {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Order uses %.1f%% of available buying power", buyingPowerUsage.InexactFloat64()))
	}

	return result
}

// ValidateOptionOrder validates options orders
func (m *Manager) ValidateOptionOrder(order *models.OrderRequest, account *models.Account, optionPrice decimal.Decimal) CheckResult {
	// Options are traded in contracts of 100 shares
	contractMultiplier := decimal.NewFromInt(100)

	// Calculate total premium
	totalPremium := optionPrice.Mul(*order.Qty).Mul(contractMultiplier)

	// For buying options, check buying power
	if order.Side == models.Buy {
		if totalPremium.GreaterThan(account.BuyingPower) {
			return CheckResult{
				Passed: false,
				Reason: fmt.Sprintf("Insufficient buying power for options. Need $%.2f, have $%.2f",
					totalPremium.InexactFloat64(), account.BuyingPower.InexactFloat64()),
			}
		}
	}

	// Check max position
	maxPosition := decimal.NewFromFloat(m.cfg.RiskMaxPositionUSD)
	if totalPremium.GreaterThan(maxPosition) {
		return CheckResult{
			Passed: false,
			Reason: fmt.Sprintf("Option order value $%.2f exceeds max position size $%.2f",
				totalPremium.InexactFloat64(), maxPosition.InexactFloat64()),
		}
	}

	return CheckResult{Passed: true}
}

// CheckSpread validates bid-ask spread isn't too wide
func (m *Manager) CheckSpread(quote *models.Quote) CheckResult {
	if quote.BidPrice.IsZero() || quote.AskPrice.IsZero() {
		return CheckResult{
			Passed: false,
			Reason: "Invalid quote: missing bid or ask",
		}
	}

	spread := quote.AskPrice.Sub(quote.BidPrice)
	midPrice := quote.BidPrice.Add(quote.AskPrice).Div(decimal.NewFromInt(2))

	if midPrice.IsZero() {
		return CheckResult{
			Passed: false,
			Reason: "Invalid quote: mid price is zero",
		}
	}

	spreadPercent := spread.Div(midPrice).Mul(decimal.NewFromInt(100))
	maxSpread := decimal.NewFromFloat(m.cfg.RiskMaxSpreadPercent)

	if spreadPercent.GreaterThan(maxSpread) {
		return CheckResult{
			Passed: false,
			Reason: fmt.Sprintf("Spread %.2f%% exceeds maximum %.2f%%",
				spreadPercent.InexactFloat64(), maxSpread.InexactFloat64()),
		}
	}

	// Add warning for wide spreads
	result := CheckResult{Passed: true}
	if spreadPercent.GreaterThan(decimal.NewFromFloat(0.25)) {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Wide spread: %.2f%%", spreadPercent.InexactFloat64()))
	}

	return result
}

// CalculatePositionSize determines appropriate position size based on risk
func (m *Manager) CalculatePositionSize(account *models.Account, price decimal.Decimal) decimal.Decimal {
	// Simple position sizing: use configured max or 10% of buying power, whichever is less
	maxPosition := decimal.NewFromFloat(m.cfg.RiskMaxPositionUSD)
	tenPercentBuyingPower := account.BuyingPower.Mul(decimal.NewFromFloat(0.1))

	positionValue := decimal.Min(maxPosition, tenPercentBuyingPower)

	// Calculate number of shares
	shares := positionValue.Div(price).Floor()

	// Ensure at least 1 share
	if shares.LessThan(decimal.NewFromInt(1)) {
		shares = decimal.NewFromInt(1)
	}

	return shares
}

// CheckDailyLoss validates against max daily loss limit
func (m *Manager) CheckDailyLoss(account *models.Account) CheckResult {
	// Calculate today's P&L
	todayPL := account.Equity.Sub(account.LastEquity)
	maxLoss := decimal.NewFromFloat(m.cfg.RiskMaxDailyLossUSD).Neg()

	if todayPL.LessThan(maxLoss) {
		return CheckResult{
			Passed: false,
			Reason: fmt.Sprintf("Daily loss $%.2f exceeds limit $%.2f",
				todayPL.InexactFloat64(), m.cfg.RiskMaxDailyLossUSD),
		}
	}

	// Add warning if approaching limit
	result := CheckResult{Passed: true}
	if todayPL.IsNegative() {
		lossPercent := todayPL.Div(maxLoss).Mul(decimal.NewFromInt(100))
		if lossPercent.GreaterThan(decimal.NewFromInt(75)) {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Approaching daily loss limit: %.1f%% used", lossPercent.InexactFloat64()))
		}
	}

	return result
}

// calculateOrderValue calculates the total value of an order
func (m *Manager) calculateOrderValue(order *models.OrderRequest, currentPrice decimal.Decimal) decimal.Decimal {
	// Handle notional orders
	if order.Notional != nil {
		return *order.Notional
	}

	// Handle quantity orders
	if order.Qty != nil {
		// Use limit price if available, otherwise current price
		price := currentPrice
		if order.LimitPrice != nil {
			price = *order.LimitPrice
		}
		return price.Mul(*order.Qty)
	}

	return decimal.Zero
}
