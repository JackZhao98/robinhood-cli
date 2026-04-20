package commands

import (
	"github.com/spf13/cobra"

	"github.com/jackzhao/robinhood-cli/internal/output"
)

func NewRoot() *cobra.Command {
	var formatStr string

	root := &cobra.Command{
		Use:           "rh",
		Short:         "Robinhood CLI — pure-HTTP client for AI skill use",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			f, err := output.ParseFormat(formatStr)
			if err != nil {
				return err
			}
			output.CurrentFormat = f
			return nil
		},
	}

	root.PersistentFlags().StringVarP(&formatStr, "format", "f", "plain",
		"output format: plain (YAML-like, AI-friendly) | json | table")

	root.AddCommand(
		// meta
		newVersionCmd(),

		// auth
		newLoginCmd(),
		newLogoutCmd(),

		// account / portfolio
		newAccountCmd(),

		// market data — equities
		newQuoteCmd(),
		newBarsCmd(),

		// options
		newOptionCmd(),

		// crypto
		newCryptoCmd(),

		// symbol research
		newSymbolCmd(),

		// cash flows
		newDividendsCmd(),
		newTransfersCmd(),

		// market state
		newMarketCmd(),
		newMoversCmd(),
		newWatchlistCmd(),

		// account meta
		newDocumentsCmd(),
		newGoldCmd(),
		newMarginCmd(),
		newPDTCmd(),
		newNotificationsCmd(),

		// orders
		newActivityCmd(),
		newOrderCmd(),
		newTradeCmd(),

		// hidden legacy aliases
		newLegacyProfileCmd(),
		newLegacyAccountsCmd(),
		newLegacyHoldingsCmd(),
		newLegacyHistoricalCmd(),
		newLegacyOptionsCmd(),
	)
	return root
}
