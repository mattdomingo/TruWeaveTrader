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

	// Connect to websocket with retry logic
	maxRetries := 3
	var connectErr error
	for i := 0; i < maxRetries; i++ {
		connectErr = streamClient.Connect()
		if connectErr == nil {
			break
		}

		fmt.Printf("❌ Connection attempt %d/%d failed: %v\n", i+1, maxRetries, connectErr)
		if i < maxRetries-1 {
			fmt.Printf("⏳ Retrying in 2 seconds...\n")
			time.Sleep(2 * time.Second)
		}
	}

	if connectErr != nil {
		return fmt.Errorf("failed to connect after %d attempts: %w", maxRetries, connectErr)
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

	// Enhanced error handler with more detailed feedback
	streamClient.RegisterHandler("error", func(msg interface{}) {
		fmt.Printf("⚠️  Stream error received - check logs for details\n")

		// Check connection status and provide guidance
		connected, authenticated, attempts := streamClient.GetConnectionStatus()
		if !connected {
			fmt.Printf("🔌 Connection lost - attempting to reconnect (attempt %d/5)\n", attempts)
		} else if !authenticated {
			fmt.Printf("🔐 Authentication failed - please check your API credentials\n")
		}
	})

	// Wait for authentication before subscribing
	fmt.Printf("🔐 Authenticating...\n")
	authTimeout := time.After(10 * time.Second)
	authTicker := time.NewTicker(500 * time.Millisecond)
	defer authTicker.Stop()

	authenticated := false
	for !authenticated {
		select {
		case <-authTimeout:
			return fmt.Errorf("authentication timeout - check your API credentials")
		case <-authTicker.C:
			if streamClient.IsConnected() {
				authenticated = true
			}
		}
	}

	fmt.Printf("✅ Authenticated successfully\n")

	// Subscribe to symbols
	if err := streamClient.Subscribe(symbols); err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	fmt.Printf("📊 Streaming %d symbols: %s\n", len(symbols), strings.Join(symbols, ", "))
	fmt.Println("Press Ctrl+C to stop...")

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Create a context for the stream
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Status ticker with enhanced information
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Track last activity time
	lastActivityTime := time.Now()

	// Update activity time on data reception
	streamClient.RegisterHandler("trade", func(msg interface{}) {
		lastActivityTime = time.Now()
		if trade, ok := msg.(*models.Trade); ok {
			fmt.Printf("[%s] Trade: %s @ $%.2f x %d\n",
				formatters.FormatTimestamp(trade.Timestamp),
				formatters.ColorBlue.Sprint(trade.Symbol),
				trade.Price.InexactFloat64(),
				trade.Size)
		}
	})

	streamClient.RegisterHandler("quote", func(msg interface{}) {
		lastActivityTime = time.Now()
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

	for {
		select {
		case <-sigChan:
			fmt.Println("\n📴 Stopping stream...")
			return nil
		case <-ticker.C:
			connected, authenticated, attempts := streamClient.GetConnectionStatus()

			if connected && authenticated {
				stats := dataCache.GetStats()
				timeSinceActivity := time.Since(lastActivityTime)
				fmt.Printf("📊 Connected | Cached: %d quotes, %d snapshots | Last activity: %v ago\n",
					stats.QuoteCount, stats.SnapshotCount, timeSinceActivity.Truncate(time.Second))
			} else if connected && !authenticated {
				fmt.Printf("🔐 Connected but not authenticated (attempt %d/5)\n", attempts)
			} else {
				fmt.Printf("🔌 Disconnected - reconnecting (attempt %d/5)\n", attempts)
			}
		case <-ctx.Done():
			return nil
		}
	}
}
