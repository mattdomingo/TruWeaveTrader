package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/TruWeaveTrader/alpaca-tui/internal/models"
	"github.com/TruWeaveTrader/alpaca-tui/pkg/formatters"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(streamCmd)
}

var streamCmd = &cobra.Command{
	Use:   "stream [symbols...]",
	Short: "Stream live market data",
	Long:  `Streams real-time quotes and trades for specified symbols.`,
	Args:  cobra.MinimumNArgs(1),
	RunE:  runStream,
}

func runStream(cmd *cobra.Command, args []string) error {
	// Parse symbols
	symbols := make([]string, len(args))
	for i, s := range args {
		symbols[i] = strings.ToUpper(s)
	}

	fmt.Printf("📡 Connecting to market data stream...\n")

	// Connect to websocket
	if err := streamClient.Connect(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer streamClient.Close()

	// Register handlers
	streamClient.RegisterHandler("trade", func(msg interface{}) {
		if trade, ok := msg.(*models.Trade); ok {
			fmt.Printf("[%s] Trade: %s @ $%.2f x %d\n",
				formatters.FormatTimestamp(trade.Timestamp),
				formatters.ColorBlue.Sprint(trade.Symbol),
				trade.Price.InexactFloat64(),
				trade.Size)
		}
	})

	streamClient.RegisterHandler("quote", func(msg interface{}) {
		if quote, ok := msg.(*models.Quote); ok {
			spread := quote.AskPrice.Sub(quote.BidPrice)
			fmt.Printf("[%s] Quote: %s Bid: %s x %d | Ask: %s x %d | Spread: $%.3f\n",
				formatters.FormatTimestamp(quote.Timestamp),
				formatters.ColorBlue.Sprint(quote.Symbol),
				formatters.ColorGreen.Sprintf("$%.2f", quote.BidPrice.InexactFloat64()),
				quote.BidSize,
				formatters.ColorRed.Sprintf("$%.2f", quote.AskPrice.InexactFloat64()),
				quote.AskSize,
				spread.InexactFloat64())
		}
	})

	// Register error handler to show stream errors in the UI
	streamClient.RegisterHandler("error", func(msg interface{}) {
		// This will be called when stream errors occur
		fmt.Printf("⚠️  Stream error received - check logs for details\n")
	})

	// Subscribe to symbols
	if err := streamClient.Subscribe(symbols); err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	fmt.Printf("✅ Streaming %d symbols: %s\n", len(symbols), strings.Join(symbols, ", "))
	fmt.Println("Press Ctrl+C to stop...")

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Create a context for the stream
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Status ticker
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sigChan:
			fmt.Println("\n📴 Stopping stream...")
			return nil
		case <-ticker.C:
			if streamClient.IsConnected() {
				stats := dataCache.GetStats()
				fmt.Printf("📊 Connected | Cached: %d quotes, %d snapshots\n",
					stats.QuoteCount, stats.SnapshotCount)
			} else {
				fmt.Println("⚠️  Reconnecting...")
			}
		case <-ctx.Done():
			return nil
		}
	}
}
