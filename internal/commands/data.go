package commands

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/jackzhao/robinhood-cli/internal/client"
	"github.com/jackzhao/robinhood-cli/internal/output"
)

// ---------- account namespace ----------

func newAccountCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account",
		Short: "Account overviews and per-account holdings",
	}
	cmd.AddCommand(newAccountListCmd(), newAccountShowCmd(), newAccountSnapshotCmd(), newAccountHistoryCmd())
	return cmd
}

func newAccountListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List every account with totals/cash (no holdings — use `account show` to drill in)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.GetAccountsSummary()
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
}

func newAccountShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show ACCOUNT",
		Short: "Holdings for one account, sorted by equity",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.GetHoldings(args[0])
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
}

func newAccountSnapshotCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "snapshot",
		Short: "Full one-shot view: totals + every account's holdings",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.GetProfile()
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
}

func newAccountHistoryCmd() *cobra.Command {
	var account, span, interval string
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Portfolio equity over time for one account",
		RunE: func(cmd *cobra.Command, args []string) error {
			if account == "" {
				return errors.New("--account is required")
			}
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.GetPortfolioHistory(account, span, interval)
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "account number (from `rh account list`)")
	cmd.Flags().StringVar(&span, "span", "year", "day | week | month | 3month | year | 5year | all")
	cmd.Flags().StringVar(&interval, "interval", "", "5minute | 10minute | hour | day | week (auto-picked from span)")
	return cmd
}

// ---------- market data ----------

func newQuoteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "quote SYMBOL",
		Short: "Real-time quote and fundamentals for a symbol",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.GetQuote(args[0])
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
}

func newBarsCmd() *cobra.Command {
	var begin, end, interval string
	cmd := &cobra.Command{
		Use:   "bars SYMBOL",
		Short: "Historical OHLCV bars for a symbol",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if begin == "" || end == "" {
				return errors.New("--from and --to are required (YYYY-MM-DD)")
			}
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.GetHistorical(args[0], begin, end, interval)
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
	cmd.Flags().StringVar(&begin, "from", "", "begin date YYYY-MM-DD")
	cmd.Flags().StringVar(&end, "to", "", "end date YYYY-MM-DD")
	cmd.Flags().StringVar(&interval, "interval", "day", "5minute|10minute|hour|day|week")
	return cmd
}

// ---------- option namespace ----------

func newOptionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "option",
		Short: "Options data: expirations, full chain w/ greeks, current positions",
	}
	cmd.AddCommand(newOptionExpirationsCmd(), newOptionChainCmd(), newOptionPositionsCmd(), newOptionHistoryCmd())
	return cmd
}

func newOptionExpirationsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "expirations SYMBOL",
		Short: "List available expiration dates for a symbol's options",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.GetOptionChain(args[0])
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
}

func newOptionChainCmd() *cobra.Command {
	var exp, optType, side string
	cmd := &cobra.Command{
		Use:   "chain SYMBOL",
		Short: "Full option chain (greeks, IV, bid/ask) at one expiration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if exp == "" {
				return errors.New("--exp is required (use `rh option expirations SYMBOL` to list dates)")
			}
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.GetOptionGreeks(args[0], exp, optType, side)
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
	cmd.Flags().StringVar(&exp, "exp", "", "expiration date YYYY-MM-DD")
	cmd.Flags().StringVar(&optType, "type", "call", "call|put")
	cmd.Flags().StringVar(&side, "side", "buy", "buy|sell")
	return cmd
}

func newOptionPositionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "positions",
		Short: "Current option positions in the account",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.GetOptionPositions()
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
}

func newOptionHistoryCmd() *cobra.Command {
	var span, interval string
	cmd := &cobra.Command{
		Use:   "history INSTRUMENT_ID",
		Short: "Price history for one option contract",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.GetOptionHistory(args[0], span, interval)
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
	cmd.Flags().StringVar(&span, "span", "week", "day | week | month | year")
	cmd.Flags().StringVar(&interval, "interval", "", "5minute | 10minute | hour | day (auto-picked from span)")
	return cmd
}

// ---------- activity ----------

func newActivityCmd() *cobra.Command {
	var limit int
	var since, asset, account string
	cmd := &cobra.Command{
		Use:   "activity",
		Short: "Recent buy/sell activity (equity + option orders)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if asset != "" && asset != "equity" && asset != "option" && asset != "crypto" {
				return errors.New("--asset must be equity, option, or crypto (or omitted for both equity+option)")
			}
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.GetActivity(limit, since, asset, account)
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "max orders to return (across both asset classes)")
	cmd.Flags().StringVar(&since, "since", "", "only orders created on/after YYYY-MM-DD")
	cmd.Flags().StringVar(&asset, "asset", "", "filter: equity | option | crypto (default: equity+option)")
	cmd.Flags().StringVar(&account, "account", "", "filter to a single account number (default: all accounts incl. IRA)")
	return cmd
}

// ---------- hidden legacy aliases ----------
// These keep muscle memory and any cached AI prompts working but don't show
// in `rh --help`. Each one delegates to the underlying client function with
// the original behavior so flipping `chain`/`greeks` semantics doesn't
// silently break old usage.

func newLegacyProfileCmd() *cobra.Command {
	c := newAccountSnapshotCmd()
	c.Use = "profile"
	c.Hidden = true
	c.Short = "[deprecated] alias for `account snapshot`"
	return c
}

func newLegacyAccountsCmd() *cobra.Command {
	c := newAccountListCmd()
	c.Use = "accounts"
	c.Hidden = true
	c.Short = "[deprecated] alias for `account list`"
	return c
}

func newLegacyHoldingsCmd() *cobra.Command {
	var account string
	cmd := &cobra.Command{
		Use:    "holdings",
		Hidden: true,
		Short:  "[deprecated] alias for `account show ACCOUNT`",
		RunE: func(cmd *cobra.Command, args []string) error {
			if account == "" {
				return errors.New("--account is required (or use `rh account show ACCOUNT`)")
			}
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.GetHoldings(account)
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "account number")
	return cmd
}

func newLegacyHistoricalCmd() *cobra.Command {
	c := newBarsCmd()
	c.Use = "historical SYMBOL"
	c.Hidden = true
	c.Short = "[deprecated] alias for `bars`"
	return c
}

func newLegacyOptionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "options",
		Hidden: true,
		Short:  "[deprecated] alias for `option`",
	}
	// Old `options chain` listed expirations — preserve that.
	chain := newOptionExpirationsCmd()
	chain.Use = "chain SYMBOL"
	chain.Short = "[deprecated] alias for `option expirations`"
	// Old `options greeks` returned the chain w/ greeks.
	greeks := newOptionChainCmd()
	greeks.Use = "greeks SYMBOL"
	greeks.Short = "[deprecated] alias for `option chain`"
	// Old `options instruments` (rare) — keep working but undocumented.
	insts := newLegacyOptionsInstrumentsCmd()
	// Old `options positions` → unchanged behavior.
	pos := newOptionPositionsCmd()

	cmd.AddCommand(chain, greeks, insts, pos)
	return cmd
}

func newLegacyOptionsInstrumentsCmd() *cobra.Command {
	var exp, optType string
	var strike float64
	cmd := &cobra.Command{
		Use:    "instruments SYMBOL",
		Hidden: true,
		Short:  "[deprecated] list option contracts (subset of `option chain`)",
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if exp == "" {
				return errors.New("--exp is required")
			}
			c, err := client.New()
			if err != nil {
				return err
			}
			var strikePtr *float64
			if cmd.Flags().Changed("strike") {
				strikePtr = &strike
			}
			res, err := c.GetOptionInstruments(args[0], exp, optType, strikePtr)
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
	cmd.Flags().StringVar(&exp, "exp", "", "expiration date YYYY-MM-DD")
	cmd.Flags().StringVar(&optType, "type", "", "call|put")
	cmd.Flags().Float64Var(&strike, "strike", 0, "filter to a specific strike")
	return cmd
}
