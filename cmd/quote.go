package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/TruWeaveTrader/alpaca-tui/pkg/formatters"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(quoteCmd)
	rootCmd.AddCommand(qCmd)  // Alias
}

var qCmd = &cobra.Command{
	Use:   "q [symbol]",
	Short: "Quick quote (alias for quote)",
	Args:  cobra.ExactArgs(1),
	RunE:  runQuote,
}

var quoteCmd = &cobra.Command{
	Use:   "quote [symbol]",
	Short: "Get a quick market snapshot",
	Long:  `Fetches current quote, price, and daily statistics for a symbol.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runQuote,
}

func runQuote(cmd *cobra.Command, args []string) error {
	symbol := args[0]
	ctx := context.Background()
	
	// Start timer for performance tracking
	start := time.Now()
	
	// Check cache first
	if snapshot, found := dataCache.GetSnapshot(symbol); found {
		fmt.Println(formatters.FormatSnapshot(snapshot))
		fmt.Printf("\n⚡ Cached • %dms\n", time.Since(start).Milliseconds())
		return nil
	}
	
	// Fetch from API
	snapshot, err := client.GetSnapshot(ctx, symbol)
	if err != nil {
		return fmt.Errorf("failed to get quote: %w", err)
	}
	
	// Cache the result
	dataCache.SetSnapshot(symbol, snapshot)
	
	// Display formatted snapshot
	fmt.Println(formatters.FormatSnapshot(snapshot))
	
	// Show latency
	elapsed := time.Since(start)
	fmt.Printf("\n⏱  Fetched • %dms\n", elapsed.Milliseconds())
	
	// Performance warning if too slow
	if elapsed > 150*time.Millisecond {
		fmt.Println("⚠️  Performance warning: Quote took longer than 150ms")
	}
	
	return nil
} 