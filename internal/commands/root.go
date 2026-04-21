package commands

import (
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/jackzhao/robinhood-cli/internal/audit"
	"github.com/jackzhao/robinhood-cli/internal/output"
)

// cmdStart captures the start time of the root command so PersistentPostRun
// can record command duration in audit.jsonl. Single-process CLI → safe.
var cmdStart time.Time

// skipAuditFirstArg lists argv[1] values that should NOT produce an
// audit.jsonl line (help/completion scaffolding is pure noise).
func skipAuditFirstArg(arg string) bool {
	switch arg {
	case "completion", "help", "--help", "-h":
		return true
	}
	return false
}

func NewRoot() *cobra.Command {
	var formatStr string

	root := &cobra.Command{
		Use:           "rh",
		Short:         "Robinhood CLI — pure-HTTP client for AI skill use",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cmdStart = time.Now()
			f, err := output.ParseFormat(formatStr)
			if err != nil {
				return err
			}
			output.CurrentFormat = f
			return nil
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			// cobra PostRun only fires on success (exit 0).
			if len(os.Args) <= 1 || skipAuditFirstArg(os.Args[1]) {
				return
			}
			_ = audit.Command(audit.CommandRecord{
				Argv:     os.Args,
				ExitCode: 0,
				DurMs:    time.Since(cmdStart).Milliseconds(),
			})
		},
	}

	root.PersistentFlags().StringVarP(&formatStr, "format", "f", "table",
		"output format: table (human default) | plain (AI-friendly) | json")

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
		newIndexCmd(),

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
