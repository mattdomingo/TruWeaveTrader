package cmd

import (
	"context"
	"fmt"

	"github.com/TruWeaveTrader/alpaca-tui/pkg/formatters"
	"github.com/spf13/cobra"
)

func init() {
	ordersCmd.Flags().Bool("all", false, "Show all orders (default: open only)")
	ordersCmd.Flags().Bool("open", true, "Show open orders only")

	rootCmd.AddCommand(ordersCmd)
}

var ordersCmd = &cobra.Command{
	Use:   "orders",
	Short: "Display orders",
	Long:  `Shows open orders by default. Use --all to see all orders.`,
	RunE:  runOrders,
}

func runOrders(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	showAll, _ := cmd.Flags().GetBool("all")

	// Determine status filter
	status := "open"
	if showAll {
		status = ""
	}

	// Fetch orders
	orders, err := client.GetOrders(ctx, status)
	if err != nil {
		return fmt.Errorf("failed to get orders: %w", err)
	}

	if len(orders) == 0 {
		if showAll {
			fmt.Println("No orders found")
		} else {
			fmt.Println("No open orders")
		}
		return nil
	}

	// Display formatted orders table
	fmt.Println(formatters.FormatOrdersTable(orders))

	return nil
}
