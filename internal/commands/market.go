package commands

import (
	"github.com/spf13/cobra"

	"github.com/jackzhao/robinhood-cli/internal/client"
	"github.com/jackzhao/robinhood-cli/internal/output"
)

func newMarketCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "market",
		Short: "Market open/closed status + today's hours",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.GetMarketStatus()
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
}

func newMoversCmd() *cobra.Command {
	var direction string
	cmd := &cobra.Command{
		Use:   "movers",
		Short: "Top S&P 500 gainers/losers (--direction up|down)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.GetMovers(direction)
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
	cmd.Flags().StringVar(&direction, "direction", "up", "up | down")
	return cmd
}

func newWatchlistCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watchlist",
		Short: "List your watchlists or show one by name",
	}
	cmd.AddCommand(newWatchlistListCmd(), newWatchlistShowCmd())
	return cmd
}

func newWatchlistListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all watchlist names",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.ListWatchlists()
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
}

func newWatchlistShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show NAME",
		Short: "Show items in a named watchlist (default: 'Default')",
		Args:  cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := "Default"
			if len(args) > 0 {
				name = joinArgs(args)
			}
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.ShowWatchlist(name)
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
}
