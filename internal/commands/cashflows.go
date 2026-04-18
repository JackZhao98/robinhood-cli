package commands

import (
	"github.com/spf13/cobra"

	"github.com/jackzhao/robinhood-cli/internal/client"
	"github.com/jackzhao/robinhood-cli/internal/output"
)

func newDividendsCmd() *cobra.Command {
	var since string
	var limit int
	cmd := &cobra.Command{
		Use:   "dividends",
		Short: "Dividends received (paid + pending), with totals",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.GetDividends(since, limit)
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "only events on/after YYYY-MM-DD")
	cmd.Flags().IntVar(&limit, "limit", 200, "max rows")
	return cmd
}

func newTransfersCmd() *cobra.Command {
	var since string
	var limit int
	cmd := &cobra.Command{
		Use:   "transfers",
		Short: "ACH money in/out (deposits, withdrawals)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.GetTransfers(since, limit)
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "only transfers on/after YYYY-MM-DD")
	cmd.Flags().IntVar(&limit, "limit", 200, "max rows")
	return cmd
}

func newRecurringCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "recurring",
		Short: "Recurring investment / DCA configurations",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.GetRecurring()
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
}
