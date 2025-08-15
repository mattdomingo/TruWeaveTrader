package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/TruWeaveTrader/alpaca-tui/internal/alpaca"
	"github.com/TruWeaveTrader/alpaca-tui/internal/cache"
	"github.com/TruWeaveTrader/alpaca-tui/internal/config"
	"github.com/TruWeaveTrader/alpaca-tui/internal/risk"
	"github.com/TruWeaveTrader/alpaca-tui/internal/strategy"
	"github.com/TruWeaveTrader/alpaca-tui/internal/websocket"
)

var (
	// Global instances
	cfg             *config.Config
	client          *alpaca.Client
	dataCache       *cache.Cache
	riskManager     *risk.Manager
	streamClient    *websocket.StreamClient
	strategyManager *strategy.Manager
	logger          *zap.Logger
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "alpaca-tui",
	Short: "Blazing-fast terminal trading app for Alpaca",
	Long: `alpaca-tui is a high-performance terminal trading application
for the Alpaca API. Features include real-time quotes, one-keystroke
trading, options support, and comprehensive risk management.`,
	PersistentPreRunE: initializeApp,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().String("config", "", "config file (default is $HOME/.alpaca-tui.yaml)")
	rootCmd.PersistentFlags().Bool("verbose", false, "verbose output")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// Configure logger: default INFO, DEBUG if DEBUG env is truthy
	verbose := false
	if v := os.Getenv("DEBUG"); v == "true" || v == "1" || v == "yes" {
		verbose = true
	}

	zcfg := zap.NewProductionConfig()
	zcfg.EncoderConfig.TimeKey = "time"
	zcfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	if verbose {
		zcfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	} else {
		zcfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	var err error
	logger, err = zcfg.Build()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}
}

// initializeApp sets up all dependencies
func initializeApp(cmd *cobra.Command, args []string) error {
	// Load configuration
	var err error
	cfg, err = config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize components
	client = alpaca.NewClient(cfg)
	dataCache = cache.NewCache(cfg.CacheTTL)
	riskManager = risk.NewManager(cfg)
	streamClient = websocket.NewStreamClient(cfg, dataCache, logger)
	strategyManager = strategy.NewManager(client, riskManager, streamClient, dataCache, logger, cfg)

	// Display trading mode
	mode := "PAPER"
	if cfg.LiveTrading {
		mode = "LIVE"
	}
	fmt.Printf("🚀 Alpaca TUI - %s Trading Mode\n", mode)

	return nil
}

// Helper function to check if in live mode
func checkLiveMode() error {
	if cfg.LiveTrading {
		fmt.Println("⚠️  WARNING: You are in LIVE trading mode!")
		fmt.Print("Type 'confirm-live' to proceed: ")

		var confirm string
		fmt.Scanln(&confirm)

		if confirm != "confirm-live" {
			return fmt.Errorf("live trading not confirmed")
		}
	}
	return nil
}
