package commands

import (
	"github.com/spf13/cobra"

	"github.com/jackzhao/robinhood-cli/internal/client"
	"github.com/jackzhao/robinhood-cli/internal/output"
)

func newCryptoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "crypto",
		Short: "Crypto holdings + real-time prices (Robinhood Crypto)",
	}
	cmd.AddCommand(newCryptoHoldingsCmd(), newCryptoQuoteCmd())
	return cmd
}

func newCryptoHoldingsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "holdings",
		Short: "All crypto positions with mark price, market value, return",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.GetCryptoHoldings()
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
}

func newCryptoQuoteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "quote SYMBOL",
		Short: "Real-time quote for a crypto symbol (BTC, ETH, BTCUSD, etc.)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.GetCryptoQuote(args[0])
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
}
