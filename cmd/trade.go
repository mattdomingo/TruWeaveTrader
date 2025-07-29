package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/TruWeaveTrader/alpaca-tui/internal/models"
	"github.com/TruWeaveTrader/alpaca-tui/pkg/formatters"
	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
)

func init() {
	// Trade command
	tradeCmd.AddCommand(buyCmd)
	tradeCmd.AddCommand(sellCmd)
	tradeCmd.AddCommand(cancelCmd)

	// Add flags to buy/sell commands
	for _, cmd := range []*cobra.Command{buyCmd, sellCmd} {
		cmd.Flags().String("type", "market", "Order type: market, limit, stop, stop_limit")
		cmd.Flags().Float64("limit", 0, "Limit price (required for limit/stop_limit orders)")
		cmd.Flags().Float64("stop", 0, "Stop price (required for stop/stop_limit orders)")
		cmd.Flags().String("tif", "day", "Time in force: day, gtc, ioc, fok")
		cmd.Flags().Bool("extended", false, "Allow extended hours trading")
	}

	rootCmd.AddCommand(tradeCmd)

	// Direct buy/sell shortcuts
	rootCmd.AddCommand(buyCmd)
	rootCmd.AddCommand(sellCmd)
}

var tradeCmd = &cobra.Command{
	Use:   "trade",
	Short: "Execute trades",
	Long:  `Parent command for trading operations (buy, sell, cancel).`,
}

var buyCmd = &cobra.Command{
	Use:   "buy [symbol] [quantity]",
	Short: "Buy shares",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return executeTrade(cmd, args, models.Buy)
	},
}

var sellCmd = &cobra.Command{
	Use:   "sell [symbol] [quantity]",
	Short: "Sell shares",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return executeTrade(cmd, args, models.Sell)
	},
}

var cancelCmd = &cobra.Command{
	Use:   "cancel [order-id]",
	Short: "Cancel an order",
	Args:  cobra.ExactArgs(1),
	RunE:  runCancel,
}

func executeTrade(cmd *cobra.Command, args []string, side models.OrderSide) error {
	symbol := strings.ToUpper(args[0])
	qty, err := decimal.NewFromString(args[1])
	if err != nil {
		return fmt.Errorf("invalid quantity: %w", err)
	}

	if err := checkLiveMode(); err != nil {
		return err
	}

	ctx := context.Background()
	snapshot, account, err := getMarketData(ctx, symbol)
	if err != nil {
		return err
	}

	order, currentPrice, err := buildOrder(cmd, symbol, qty, side, snapshot)
	if err != nil {
		return err
	}

	if err := validateRisks(order, account, currentPrice, snapshot); err != nil {
		return err
	}

	showOrderPreview(order, currentPrice, qty, snapshot)

	if !confirmOrder() {
		fmt.Println("Order cancelled")
		return nil
	}

	return submitOrder(ctx, order)
}

func runCancel(cmd *cobra.Command, args []string) error {
	orderID := args[0]
	ctx := context.Background()

	// Check live mode
	if err := checkLiveMode(); err != nil {
		return err
	}

	// Get order details first
	order, err := client.GetOrder(ctx, orderID)
	if err != nil {
		return fmt.Errorf("failed to get order: %w", err)
	}

	// Display order to be cancelled
	fmt.Printf("🚫 Cancelling order:\n")
	fmt.Printf("  Symbol: %s\n", order.Symbol)
	fmt.Printf("  Side: %s\n", order.Side)
	fmt.Printf("  Quantity: %s\n", order.Qty)
	fmt.Printf("  Status: %s\n", order.Status)

	// Confirm cancellation
	fmt.Print("\nConfirm cancellation? (y/N): ")
	var confirm string
	fmt.Scanln(&confirm)

	if strings.ToLower(confirm) != "y" {
		fmt.Println("Cancellation aborted")
		return nil
	}

	// Cancel order
	if err := client.CancelOrder(ctx, orderID); err != nil {
		return fmt.Errorf("failed to cancel order: %w", err)
	}

	fmt.Println("✅ Order cancelled successfully")
	return nil
}

func getMarketData(ctx context.Context, symbol string) (*models.Snapshot, *models.Account, error) {
	snapshot, err := client.GetSnapshot(ctx, symbol)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get current price: %w", err)
	}

	account, err := client.GetAccount(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get account: %w", err)
	}

	return snapshot, account, nil
}

func buildOrder(cmd *cobra.Command, symbol string, qty decimal.Decimal, side models.OrderSide, snapshot *models.Snapshot) (*models.OrderRequest, decimal.Decimal, error) {
	orderType, _ := cmd.Flags().GetString("type")
	limitPrice, _ := cmd.Flags().GetFloat64("limit")
	stopPrice, _ := cmd.Flags().GetFloat64("stop")
	tif, _ := cmd.Flags().GetString("tif")
	extendedHours, _ := cmd.Flags().GetBool("extended")

	order := &models.OrderRequest{
		Symbol:        symbol,
		Qty:           &qty,
		Side:          side,
		Type:          models.OrderType(orderType),
		TimeInForce:   models.TimeInForce(tif),
		ExtendedHours: extendedHours,
	}

	if orderType == "limit" || orderType == "stop_limit" {
		if limitPrice == 0 {
			return nil, decimal.Zero, fmt.Errorf("limit price required for %s orders", orderType)
		}
		lp := decimal.NewFromFloat(limitPrice)
		order.LimitPrice = &lp
	}

	if orderType == "stop" || orderType == "stop_limit" {
		if stopPrice == 0 {
			return nil, decimal.Zero, fmt.Errorf("stop price required for %s orders", orderType)
		}
		sp := decimal.NewFromFloat(stopPrice)
		order.StopPrice = &sp
	}

	currentPrice := decimal.NewFromInt(150)
	if snapshot.LatestTrade != nil && !snapshot.LatestTrade.Price.IsZero() {
		currentPrice = snapshot.LatestTrade.Price
	} else if order.LimitPrice != nil {
		currentPrice = *order.LimitPrice
	}

	return order, currentPrice, nil
}

func validateRisks(order *models.OrderRequest, account *models.Account, currentPrice decimal.Decimal, snapshot *models.Snapshot) error {
	fmt.Println("🔍 Running pre-trade checks...")

	riskResult := riskManager.ValidateOrder(order, account, currentPrice)
	if !riskResult.Passed {
		return fmt.Errorf("risk check failed: %s", riskResult.Reason)
	}

	if snapshot.LatestQuote != nil && !snapshot.LatestQuote.BidPrice.IsZero() && !snapshot.LatestQuote.AskPrice.IsZero() {
		spreadResult := riskManager.CheckSpread(snapshot.LatestQuote)
		if !spreadResult.Passed {
			return fmt.Errorf("spread check failed: %s", spreadResult.Reason)
		}
		for _, warning := range spreadResult.Warnings {
			fmt.Printf("⚠️  %s\n", warning)
		}
	} else {
		fmt.Printf("⚠️  No live quote available (market may be closed) - spread check skipped\n")
	}

	for _, warning := range riskResult.Warnings {
		fmt.Printf("⚠️  %s\n", warning)
	}

	return nil
}

func showOrderPreview(order *models.OrderRequest, currentPrice decimal.Decimal, qty decimal.Decimal, snapshot *models.Snapshot) {
	fmt.Println("\n📋 Order Preview:")
	fmt.Printf("  Symbol: %s\n", order.Symbol)
	fmt.Printf("  Side: %s\n", formatters.ColorGreen.Sprint(strings.ToUpper(string(order.Side))))
	fmt.Printf("  Quantity: %s\n", qty)
	fmt.Printf("  Type: %s\n", order.Type)
	if order.LimitPrice != nil {
		fmt.Printf("  Limit Price: $%.2f\n", order.LimitPrice.InexactFloat64())
	}
	if order.StopPrice != nil {
		fmt.Printf("  Stop Price: $%.2f\n", order.StopPrice.InexactFloat64())
	}
	fmt.Printf("  Time in Force: %s\n", order.TimeInForce)

	estimatedValue := currentPrice.Mul(qty)
	fmt.Printf("  Estimated Value: $%.2f", estimatedValue.InexactFloat64())

	if snapshot.LatestTrade == nil || snapshot.LatestTrade.Price.IsZero() {
		fmt.Printf(" (estimated - no current price available)")
	}
	fmt.Printf("\n")
}

func confirmOrder() bool {
	fmt.Print("\nProceed with order? (y/N): ")
	var confirm string
	fmt.Scanln(&confirm)
	return strings.ToLower(confirm) == "y"
}

func submitOrder(ctx context.Context, order *models.OrderRequest) error {
	start := time.Now()
	newOrder, err := client.PlaceOrder(ctx, order)
	if err != nil {
		return fmt.Errorf("failed to place order: %w", err)
	}

	elapsed := time.Since(start)

	fmt.Printf("\n✅ Order submitted successfully!\n")
	fmt.Printf("  Order ID: %s\n", newOrder.ID)
	fmt.Printf("  Status: %s\n", newOrder.Status)
	fmt.Printf("  Latency: %dms\n", elapsed.Milliseconds())

	if elapsed > 100*time.Millisecond {
		fmt.Println("⚠️  Performance warning: Order submission took longer than 100ms")
	}

	return nil
}
