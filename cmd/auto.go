package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/TruWeaveTrader/alpaca-tui/internal/strategy"
	"github.com/TruWeaveTrader/alpaca-tui/pkg/formatters"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func init() {
	// Auto command with subcommands
	autoCmd.AddCommand(autoStartCmd)
	autoCmd.AddCommand(autoStopCmd)
	autoCmd.AddCommand(autoStatusCmd)
	autoCmd.AddCommand(autoListCmd)
	autoCmd.AddCommand(autoAddCmd)
	autoCmd.AddCommand(autoRemoveCmd)
	autoCmd.AddCommand(autoEnableCmd)
	autoCmd.AddCommand(autoDisableCmd)
	autoCmd.AddCommand(autoMetricsCmd)

	// Add flags
	autoAddCmd.Flags().String("type", "", "Strategy type (mean_reversion, momentum, pairs_trading)")
	autoAddCmd.Flags().StringSlice("symbols", []string{}, "Symbols to trade")
	autoAddCmd.Flags().String("config", "", "JSON configuration parameters")
	autoAddCmd.MarkFlagRequired("type")

	rootCmd.AddCommand(autoCmd)
}

var autoCmd = &cobra.Command{
	Use:   "auto",
	Short: "Automated trading strategy management",
	Long: `Manage automated trading strategies including mean reversion, momentum, and pairs trading.
Strategies can be started, stopped, monitored, and configured through this interface.`,
}

var autoStartCmd = &cobra.Command{
	Use:   "start [strategy-name]",
	Short: "Start automated trading (all strategies or specific strategy)",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runAutoStart,
}

var autoStopCmd = &cobra.Command{
	Use:   "stop [strategy-name]",
	Short: "Stop automated trading (all strategies or specific strategy)",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runAutoStop,
}

var autoStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show automation status",
	RunE:  runAutoStatus,
}

var autoListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured strategies",
	RunE:  runAutoList,
}

var autoAddCmd = &cobra.Command{
	Use:   "add [strategy-name]",
	Short: "Add a new strategy",
	Args:  cobra.ExactArgs(1),
	RunE:  runAutoAdd,
}

var autoRemoveCmd = &cobra.Command{
	Use:   "remove [strategy-name]",
	Short: "Remove a strategy",
	Args:  cobra.ExactArgs(1),
	RunE:  runAutoRemove,
}

var autoEnableCmd = &cobra.Command{
	Use:   "enable [strategy-name]",
	Short: "Enable a strategy",
	Args:  cobra.ExactArgs(1),
	RunE:  runAutoEnable,
}

var autoDisableCmd = &cobra.Command{
	Use:   "disable [strategy-name]",
	Short: "Disable a strategy",
	Args:  cobra.ExactArgs(1),
	RunE:  runAutoDisable,
}

var autoMetricsCmd = &cobra.Command{
	Use:   "metrics [strategy-name]",
	Short: "Show strategy performance metrics",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runAutoMetrics,
}

func runAutoStart(cmd *cobra.Command, args []string) error {
	if !cfg.StrategiesEnabled {
		fmt.Println("🚫 Automated trading is disabled in configuration")
		fmt.Println("   Set STRATEGIES_ENABLED=true to enable automation")
		return nil
	}

	if err := checkLiveMode(); err != nil {
		return err
	}

	// Initialize default strategies from config
	if err := initializeDefaultStrategies(); err != nil {
		return fmt.Errorf("failed to initialize strategies: %w", err)
	}

	if len(args) == 0 {
		// Start all strategies
		fmt.Println("🚀 Starting automated trading...")
		if err := strategyManager.StartAll(); err != nil {
			return fmt.Errorf("failed to start automation: %w", err)
		}
		fmt.Println("✅ Automated trading started successfully")

		// Show active strategies
		strategies := strategyManager.GetActiveStrategies()
		if len(strategies) > 0 {
			fmt.Printf("\n📊 Active strategies (%d):\n", len(strategies))
			for _, s := range strategies {
				fmt.Printf("  • %s (%s) - %v symbols\n",
					formatters.ColorGreen.Sprint(s.Name()),
					s.Type(),
					len(s.GetSymbols()))
			}
		}
	} else {
		// Start specific strategy
		strategyName := args[0]
		fmt.Printf("🚀 Starting strategy: %s\n", strategyName)
		if err := strategyManager.StartStrategy(strategyName); err != nil {
			return fmt.Errorf("failed to start strategy %s: %w", strategyName, err)
		}
		fmt.Printf("✅ Strategy %s started successfully\n", strategyName)
	}

	return nil
}

func runAutoStop(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		// Stop all strategies
		fmt.Println("🛑 Stopping automated trading...")
		if err := strategyManager.StopAll(); err != nil {
			return fmt.Errorf("failed to stop automation: %w", err)
		}
		fmt.Println("✅ Automated trading stopped successfully")
	} else {
		// Stop specific strategy
		strategyName := args[0]
		fmt.Printf("🛑 Stopping strategy: %s\n", strategyName)
		if err := strategyManager.StopStrategy(strategyName); err != nil {
			return fmt.Errorf("failed to stop strategy %s: %w", strategyName, err)
		}
		fmt.Printf("✅ Strategy %s stopped successfully\n", strategyName)
	}

	return nil
}

func runAutoStatus(cmd *cobra.Command, args []string) error {
	fmt.Println("📊 Automation Status")
	fmt.Println(strings.Repeat("=", 50))

	// General status
	if cfg.StrategiesEnabled {
		fmt.Printf("Status: %s\n", formatters.ColorGreen.Sprint("ENABLED"))
	} else {
		fmt.Printf("Status: %s\n", formatters.ColorRed.Sprint("DISABLED"))
	}

	mode := "PAPER"
	if cfg.LiveTrading {
		mode = "LIVE"
	}
	fmt.Printf("Trading Mode: %s\n", mode)

	// Strategy status
	strategies := strategyManager.ListStrategies()
	activeStrategies := strategyManager.GetActiveStrategies()

	fmt.Printf("\nStrategies: %d total, %d active\n", len(strategies), len(activeStrategies))

	if len(strategies) > 0 {
		fmt.Println("\nStrategy Details:")
		for _, name := range strategies {
			s, exists := strategyManager.GetStrategy(name)
			if !exists {
				continue
			}

			status := "STOPPED"
			color := formatters.ColorRed
			if s.IsActive() {
				status = "RUNNING"
				color = formatters.ColorGreen
			}

			fmt.Printf("  • %s (%s) - %s\n",
				s.Name(),
				s.Type(),
				color.Sprint(status))
			fmt.Printf("    Symbols: %v\n", s.GetSymbols())
		}
	}

	return nil
}

func runAutoList(cmd *cobra.Command, args []string) error {
	strategies := strategyManager.ListStrategies()

	if len(strategies) == 0 {
		fmt.Println("No strategies configured")
		return nil
	}

	fmt.Printf("📋 Configured Strategies (%d)\n", len(strategies))
	fmt.Println(strings.Repeat("=", 50))

	for _, name := range strategies {
		s, exists := strategyManager.GetStrategy(name)
		if !exists {
			continue
		}

		status := "stopped"
		if s.IsActive() {
			status = formatters.ColorGreen.Sprint("running")
		} else {
			status = formatters.ColorRed.Sprint("stopped")
		}

		fmt.Printf("\n%s (%s)\n", formatters.ColorBlue.Sprint(s.Name()), s.Type())
		fmt.Printf("  Status: %s\n", status)
		fmt.Printf("  Symbols: %v\n", s.GetSymbols())

		metrics := s.GetMetrics()
		if metrics.TotalTrades > 0 {
			fmt.Printf("  Trades: %d (%.1f%% win rate)\n",
				metrics.TotalTrades,
				metrics.WinRate*100)
			fmt.Printf("  P&L: %s\n", formatColorizedPnL(metrics.TotalPnL))
		}
	}

	return nil
}

func runAutoAdd(cmd *cobra.Command, args []string) error {
	strategyName := args[0]
	strategyType, _ := cmd.Flags().GetString("type")
	symbols, _ := cmd.Flags().GetStringSlice("symbols")
	configJSON, _ := cmd.Flags().GetString("config")

	// Parse configuration
	parameters := make(map[string]interface{})
	if configJSON != "" {
		if err := json.Unmarshal([]byte(configJSON), &parameters); err != nil {
			return fmt.Errorf("invalid config JSON: %w", err)
		}
	}

	// Create strategy config
	config := &strategy.StrategyConfig{
		Name:       strategyName,
		Type:       strategyType,
		Enabled:    true,
		Symbols:    symbols,
		Parameters: parameters,
		Schedule: &strategy.ScheduleConfig{
			StartTime: "09:30",
			EndTime:   "16:00",
			Days:      []int{1, 2, 3, 4, 5}, // Mon-Fri
			Timezone:  "America/New_York",
		},
	}

	// Create strategy instance
	var newStrategy strategy.Strategy
	switch strategyType {
	case "mean_reversion":
		newStrategy = strategy.NewMeanReversionStrategy(strategyName, symbols, config, strategyManager.SendSignal)
	case "momentum":
		newStrategy = strategy.NewMomentumStrategy(strategyName, symbols, config, strategyManager.SendSignal)
	case "pairs_trading":
		newStrategy = strategy.NewPairsTradingStrategy(strategyName, config, strategyManager.SendSignal)
	default:
		return fmt.Errorf("unknown strategy type: %s", strategyType)
	}

	// Add to strategy manager
	if err := strategyManager.AddStrategy(newStrategy); err != nil {
		return fmt.Errorf("failed to add strategy: %w", err)
	}

	fmt.Printf("✅ Strategy '%s' added successfully\n", strategyName)
	fmt.Printf("   Type: %s\n", strategyType)
	fmt.Printf("   Symbols: %v\n", symbols)

	return nil
}

func runAutoRemove(cmd *cobra.Command, args []string) error {
	strategyName := args[0]

	if err := strategyManager.RemoveStrategy(strategyName); err != nil {
		return fmt.Errorf("failed to remove strategy: %w", err)
	}

	fmt.Printf("✅ Strategy '%s' removed successfully\n", strategyName)
	return nil
}

func runAutoEnable(cmd *cobra.Command, args []string) error {
	strategyName := args[0]

	if err := strategyManager.StartStrategy(strategyName); err != nil {
		return fmt.Errorf("failed to enable strategy: %w", err)
	}

	fmt.Printf("✅ Strategy '%s' enabled\n", strategyName)
	return nil
}

func runAutoDisable(cmd *cobra.Command, args []string) error {
	strategyName := args[0]

	if err := strategyManager.StopStrategy(strategyName); err != nil {
		return fmt.Errorf("failed to disable strategy: %w", err)
	}

	fmt.Printf("✅ Strategy '%s' disabled\n", strategyName)
	return nil
}

func runAutoMetrics(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		// Show metrics for all strategies
		metrics := strategyManager.GetMetrics()
		if len(metrics) == 0 {
			fmt.Println("No strategy metrics available")
			return nil
		}

		fmt.Println("📈 Strategy Performance Metrics")
		fmt.Println(strings.Repeat("=", 60))

		for name, m := range metrics {
			showStrategyMetrics(name, m)
		}
	} else {
		// Show metrics for specific strategy
		strategyName := args[0]
		s, exists := strategyManager.GetStrategy(strategyName)
		if !exists {
			return fmt.Errorf("strategy '%s' not found", strategyName)
		}

		fmt.Printf("📈 Metrics for %s\n", strategyName)
		fmt.Println(strings.Repeat("=", 40))
		showStrategyMetrics(strategyName, s.GetMetrics())
	}

	return nil
}

func showStrategyMetrics(name string, metrics *strategy.StrategyMetrics) {
	fmt.Printf("\n%s:\n", formatters.ColorBlue.Sprint(name))
	fmt.Printf("  Total Trades: %d\n", metrics.TotalTrades)

	if metrics.TotalTrades > 0 {
		fmt.Printf("  Win Rate: %.1f%% (%d wins, %d losses)\n",
			metrics.WinRate*100,
			metrics.WinningTrades,
			metrics.LosingTrades)
		fmt.Printf("  Total P&L: %s\n", formatColorizedPnL(metrics.TotalPnL))
		fmt.Printf("  Max Drawdown: %s\n", formatColorizedPnL(metrics.MaxDrawdown))

		if metrics.WinningTrades > 0 {
			fmt.Printf("  Average Win: %s\n", formatColorizedPnL(metrics.AverageWin))
		}
		if metrics.LosingTrades > 0 {
			fmt.Printf("  Average Loss: %s\n", formatColorizedPnL(metrics.AverageLoss))
		}

		if !metrics.LastTradeTime.IsZero() {
			fmt.Printf("  Last Trade: %s\n", metrics.LastTradeTime.Format("2006-01-02 15:04:05"))
		}
	}

	fmt.Printf("  Active Positions: %d\n", metrics.ActivePositions)
}

func formatColorizedPnL(pnl interface{}) string {
	var value float64

	switch v := pnl.(type) {
	case float64:
		value = v
	default:
		// Handle decimal.Decimal
		if d, ok := pnl.(interface{ InexactFloat64() float64 }); ok {
			value = d.InexactFloat64()
		}
	}

	formatted := fmt.Sprintf("$%.2f", value)
	if value >= 0 {
		return formatters.ColorGreen.Sprint(formatted)
	}
	return formatters.ColorRed.Sprint(formatted)
}

func initializeDefaultStrategies() error {
	// Initialize strategies from configuration
	for _, configData := range cfg.StrategyConfigs {
		name, _ := configData["name"].(string)
		strategyType, _ := configData["type"].(string)
		enabled, _ := configData["enabled"].(bool)
		symbols := make([]string, 0)

		if symbolsRaw, ok := configData["symbols"].([]interface{}); ok {
			for _, s := range symbolsRaw {
				if symbol, ok := s.(string); ok {
					symbols = append(symbols, symbol)
				}
			}
		}

		parameters, _ := configData["parameters"].(map[string]interface{})
		scheduleData, _ := configData["schedule"].(map[string]interface{})

		// Convert schedule
		var schedule *strategy.ScheduleConfig
		if scheduleData != nil {
			schedule = &strategy.ScheduleConfig{
				StartTime: getStringFromMap(scheduleData, "start_time", "09:30"),
				EndTime:   getStringFromMap(scheduleData, "end_time", "16:00"),
				Timezone:  getStringFromMap(scheduleData, "timezone", "America/New_York"),
			}

			if daysRaw, ok := scheduleData["days"].([]interface{}); ok {
				days := make([]int, 0)
				for _, d := range daysRaw {
					if day, ok := d.(float64); ok {
						days = append(days, int(day))
					}
				}
				schedule.Days = days
			}
		}

		config := &strategy.StrategyConfig{
			Name:       name,
			Type:       strategyType,
			Enabled:    enabled,
			Symbols:    symbols,
			Parameters: parameters,
			Schedule:   schedule,
		}

		// Skip if strategy already exists
		if _, exists := strategyManager.GetStrategy(name); exists {
			continue
		}

		// Create strategy instance
		var newStrategy strategy.Strategy
		switch strategyType {
		case "mean_reversion":
			newStrategy = strategy.NewMeanReversionStrategy(name, symbols, config, strategyManager.SendSignal)
		case "momentum":
			newStrategy = strategy.NewMomentumStrategy(name, symbols, config, strategyManager.SendSignal)
		case "pairs_trading":
			newStrategy = strategy.NewPairsTradingStrategy(name, config, strategyManager.SendSignal)
		default:
			logger.Warn("unknown strategy type",
				zap.String("name", name),
				zap.String("type", strategyType))
			continue
		}

		// Add to strategy manager
		if err := strategyManager.AddStrategy(newStrategy); err != nil {
			logger.Error("failed to add strategy",
				zap.String("name", name),
				zap.Error(err))
			continue
		}

		logger.Info("strategy initialized from config",
			zap.String("name", name),
			zap.String("type", strategyType),
			zap.Bool("enabled", enabled))
	}

	return nil
}

func getStringFromMap(m map[string]interface{}, key, defaultValue string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return defaultValue
}
