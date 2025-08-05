package cmd

import (
	"context"
	"fmt"

	"github.com/TruWeaveTrader/alpaca-tui/pkg/formatters"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(accountCmd)
	rootCmd.AddCommand(acctCmd) // Alias
}

var acctCmd = &cobra.Command{
	Use:   "acct",
	Short: "Account summary (alias)",
	RunE:  runAccount,
}

var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "Display account information",
	Long:  `Shows account status, buying power, portfolio value, and daily P&L.`,
	RunE:  runAccount,
}

func runAccount(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Fetch account info
	account, err := client.GetAccount(ctx)
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}

	// Display formatted account
	fmt.Println(formatters.FormatAccount(account))

	// Check daily loss limit
	result := riskManager.CheckDailyLoss(account)
	if !result.Passed {
		fmt.Printf("\n❌ %s\n", result.Reason)
	} else if len(result.Warnings) > 0 {
		for _, warning := range result.Warnings {
			fmt.Printf("\n⚠️  %s\n", warning)
		}
	}

	return nil
}
