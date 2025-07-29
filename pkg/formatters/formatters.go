package formatters

import (
	"fmt"
	"strings"
	"time"

	"github.com/TruWeaveTrader/alpaca-tui/internal/models"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/shopspring/decimal"
)

// Colors for different values
var (
	ColorGreen  = text.FgGreen
	ColorRed    = text.FgRed
	ColorYellow = text.FgYellow
	ColorBlue   = text.FgCyan
	ColorWhite  = text.FgWhite
	ColorGray   = text.FgHiBlack
)

// FormatPrice formats a price with color based on change
func FormatPrice(price decimal.Decimal, change decimal.Decimal) string {
	priceStr := fmt.Sprintf("$%.2f", price.InexactFloat64())
	
	if change.IsPositive() {
		return ColorGreen.Sprint(priceStr)
	} else if change.IsNegative() {
		return ColorRed.Sprint(priceStr)
	}
	return priceStr
}

// FormatPercent formats a percentage with color
func FormatPercent(percent decimal.Decimal) string {
	sign := ""
	if percent.IsPositive() {
		sign = "+"
	}
	
	percentStr := fmt.Sprintf("%s%.2f%%", sign, percent.InexactFloat64())
	
	if percent.IsPositive() {
		return ColorGreen.Sprint(percentStr)
	} else if percent.IsNegative() {
		return ColorRed.Sprint(percentStr)
	}
	return percentStr
}

// FormatDollarAmount formats a dollar amount with appropriate color
func FormatDollarAmount(amount decimal.Decimal) string {
	amountStr := fmt.Sprintf("$%.2f", amount.Abs().InexactFloat64())
	
	if amount.IsNegative() {
		return ColorRed.Sprint("-" + amountStr)
	}
	return ColorGreen.Sprint(amountStr)
}

// FormatSnapshot creates a pretty snapshot display
func FormatSnapshot(snapshot *models.Snapshot) string {
	if snapshot == nil {
		return "No data available"
	}

	var parts []string
	
	// Header
	parts = append(parts, fmt.Sprintf("\n%s %s", 
		text.Bold.Sprint(snapshot.Symbol),
		ColorGray.Sprint(time.Now().Format("15:04:05"))))
	
	// Latest trade
	if snapshot.LatestTrade != nil {
		change := decimal.Zero
		changePercent := decimal.Zero
		
		if snapshot.PrevDailyBar != nil {
			change = snapshot.LatestTrade.Price.Sub(snapshot.PrevDailyBar.Close)
			if !snapshot.PrevDailyBar.Close.IsZero() {
				changePercent = change.Div(snapshot.PrevDailyBar.Close).Mul(decimal.NewFromInt(100))
			}
		}
		
		parts = append(parts, fmt.Sprintf("Last: %s %s (%s)",
			FormatPrice(snapshot.LatestTrade.Price, change),
			FormatDollarAmount(change),
			FormatPercent(changePercent)))
	}
	
	// Quote
	if snapshot.LatestQuote != nil {
		spread := snapshot.LatestQuote.AskPrice.Sub(snapshot.LatestQuote.BidPrice)
		midPrice := snapshot.LatestQuote.BidPrice.Add(snapshot.LatestQuote.AskPrice).Div(decimal.NewFromInt(2))
		spreadPercent := decimal.Zero
		if !midPrice.IsZero() {
			spreadPercent = spread.Div(midPrice).Mul(decimal.NewFromInt(100))
		}
		
		parts = append(parts, fmt.Sprintf("Bid: %s x %d | Ask: %s x %d | Spread: %.3f%%",
			ColorGreen.Sprintf("$%.2f", snapshot.LatestQuote.BidPrice.InexactFloat64()),
			snapshot.LatestQuote.BidSize,
			ColorRed.Sprintf("$%.2f", snapshot.LatestQuote.AskPrice.InexactFloat64()),
			snapshot.LatestQuote.AskSize,
			spreadPercent.InexactFloat64()))
	}
	
	// Daily bar
	if snapshot.DailyBar != nil {
		parts = append(parts, fmt.Sprintf("Day Range: %s - %s | Volume: %s",
			ColorRed.Sprintf("$%.2f", snapshot.DailyBar.Low.InexactFloat64()),
			ColorGreen.Sprintf("$%.2f", snapshot.DailyBar.High.InexactFloat64()),
			FormatVolume(snapshot.DailyBar.Volume)))
	}
	
	return strings.Join(parts, "\n")
}

// FormatVolume formats large numbers with K/M/B suffixes
func FormatVolume(volume int64) string {
	if volume >= 1_000_000_000 {
		return fmt.Sprintf("%.1fB", float64(volume)/1_000_000_000)
	} else if volume >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(volume)/1_000_000)
	} else if volume >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(volume)/1_000)
	}
	return fmt.Sprintf("%d", volume)
}

// FormatPositionsTable creates a pretty positions table
func FormatPositionsTable(positions []*models.Position) string {
	t := table.NewWriter()
	t.SetStyle(table.StyleLight)
	
	t.AppendHeader(table.Row{
		"Symbol", "Qty", "Avg Cost", "Current", "P&L", "P&L %", "Value"})
	
	totalPL := decimal.Zero
	totalValue := decimal.Zero
	
	for _, pos := range positions {
		plColor := ColorGreen
		if pos.UnrealizedPL.IsNegative() {
			plColor = ColorRed
		}
		
		t.AppendRow(table.Row{
			pos.Symbol,
			pos.Qty.String(),
			fmt.Sprintf("$%.2f", pos.AvgEntryPrice.InexactFloat64()),
			fmt.Sprintf("$%.2f", pos.CurrentPrice.InexactFloat64()),
			plColor.Sprintf("$%.2f", pos.UnrealizedPL.InexactFloat64()),
			FormatPercent(pos.UnrealizedPLPC.Mul(decimal.NewFromInt(100))),
			fmt.Sprintf("$%.2f", pos.MarketValue.InexactFloat64()),
		})
		
		totalPL = totalPL.Add(pos.UnrealizedPL)
		totalValue = totalValue.Add(pos.MarketValue)
	}
	
	// Footer
	t.AppendSeparator()
	t.AppendRow(table.Row{
		"TOTAL", "", "", "",
		FormatDollarAmount(totalPL),
		"",
		fmt.Sprintf("$%.2f", totalValue.InexactFloat64()),
	})
	
	return t.Render()
}

// FormatOrdersTable creates a pretty orders table
func FormatOrdersTable(orders []*models.Order) string {
	t := table.NewWriter()
	t.SetStyle(table.StyleLight)
	
	t.AppendHeader(table.Row{
		"Time", "Symbol", "Side", "Type", "Qty", "Price", "Status"})
	
	for _, order := range orders {
		sideColor := ColorGreen
		if order.Side == models.Sell {
			sideColor = ColorRed
		}
		
		price := "Market"
		if order.LimitPrice != nil {
			price = fmt.Sprintf("$%.2f", order.LimitPrice.InexactFloat64())
		}
		
		statusColor := ColorWhite
		switch order.Status {
		case models.OrderFilled:
			statusColor = ColorGreen
		case models.OrderCanceled, models.OrderRejected:
			statusColor = ColorRed
		case models.OrderPending, models.OrderAccepted:
			statusColor = ColorYellow
		}
		
		t.AppendRow(table.Row{
			order.CreatedAt.Format("15:04:05"),
			order.Symbol,
			sideColor.Sprint(strings.ToUpper(string(order.Side))),
			order.Type,
			order.Qty.String(),
			price,
			statusColor.Sprint(order.Status),
		})
	}
	
	if len(orders) == 0 {
		t.AppendRow(table.Row{"No orders", "", "", "", "", "", ""})
	}
	
	return t.Render()
}

// FormatAccount creates a pretty account summary
func FormatAccount(account *models.Account) string {
	t := table.NewWriter()
	t.SetStyle(table.StyleLight)
	
	// Calculate daily P&L
	dailyPL := account.Equity.Sub(account.LastEquity)
	dailyPLPercent := decimal.Zero
	if !account.LastEquity.IsZero() {
		dailyPLPercent = dailyPL.Div(account.LastEquity).Mul(decimal.NewFromInt(100))
	}
	
	t.AppendRow(table.Row{"Account Number", account.AccountNumber})
	t.AppendRow(table.Row{"Status", ColorGreen.Sprint(account.Status)})
	t.AppendSeparator()
	t.AppendRow(table.Row{"Portfolio Value", fmt.Sprintf("$%.2f", account.PortfolioValue.InexactFloat64())})
	t.AppendRow(table.Row{"Cash", fmt.Sprintf("$%.2f", account.Cash.InexactFloat64())})
	t.AppendRow(table.Row{"Buying Power", ColorGreen.Sprintf("$%.2f", account.BuyingPower.InexactFloat64())})
	t.AppendSeparator()
	t.AppendRow(table.Row{"Daily P&L", FormatDollarAmount(dailyPL)})
	t.AppendRow(table.Row{"Daily P&L %", FormatPercent(dailyPLPercent)})
	
	if account.PatternDayTrader {
		t.AppendSeparator()
		t.AppendRow(table.Row{"Day Trades", fmt.Sprintf("%d/3", account.DaytradeCount)})
		t.AppendRow(table.Row{"DT Buying Power", fmt.Sprintf("$%.2f", account.DaytradingBuyingPower.InexactFloat64())})
	}
	
	return t.Render()
}

// FormatTimestamp formats a timestamp for display
func FormatTimestamp(t time.Time) string {
	return t.Format("15:04:05")
}

// TruncateString truncates a string to specified length
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
} 