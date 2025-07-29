package cmd

import (
	"context"
	"fmt"

	"github.com/TruWeaveTrader/alpaca-tui/pkg/formatters"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(positionsCmd)
	rootCmd.AddCommand(posCmd)  // Alias
}

var posCmd = &cobra.Command{
	Use:   "pos",
	Short: "Show positions (alias)",
	RunE:  runPositions,
}

var positionsCmd = &cobra.Command{
	Use:   "positions",
	Short: "Display all open positions",
	Long:  `Shows current positions with P&L, cost basis, and market value.`,
	RunE:  runPositions,
}

func runPositions(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	
	// Fetch positions
	positions, err := client.GetPositions(ctx)
	if err != nil {
		return fmt.Errorf("failed to get positions: %w", err)
	}
	
	if len(positions) == 0 {
		fmt.Println("No open positions")
		return nil
	}
	
	// Display formatted positions table
	fmt.Println(formatters.FormatPositionsTable(positions))
	
	return nil
} 