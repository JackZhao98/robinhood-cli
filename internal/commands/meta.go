package commands

import (
	"runtime"

	"github.com/spf13/cobra"

	"github.com/jackzhao/robinhood-cli/internal/client"
	"github.com/jackzhao/robinhood-cli/internal/output"
)

// versionPayload is what `rh version` emits. Mirrors `rh --version` but
// goes through the standard output formatter so JSON / table callers get
// structured fields instead of a raw string.
type versionPayload struct {
	Version string `json:"version"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
	Go      string `json:"go"`
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print rh version and build info",
		RunE: func(cmd *cobra.Command, args []string) error {
			v := cmd.Root().Version
			if v == "" {
				v = "dev"
			}
			return output.Emit(versionPayload{
				Version: v,
				OS:      runtime.GOOS,
				Arch:    runtime.GOARCH,
				Go:      runtime.Version(),
			})
		},
	}
}

func newDocumentsCmd() *cobra.Command {
	var taxYear int
	cmd := &cobra.Command{
		Use:   "documents",
		Short: "Account documents (1099, statements, confirmations) with download URLs",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.GetDocuments(taxYear)
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
	cmd.Flags().IntVar(&taxYear, "tax-year", 0, "filter to a specific tax year (e.g. 2025)")
	return cmd
}

func newGoldCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "gold",
		Short: "Robinhood Gold subscription status + tier",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.GetGold()
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
}

func newMarginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "margin",
		Short: "Margin balances and overnight/day-trade buying power per account",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.GetMargin()
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
}

func newPDTCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pdt",
		Short: "Pattern day-trader status + day-trade count per account",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.GetPDT()
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
}

func newNotificationsCmd() *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "notifications",
		Short: "Recent notifications (best-effort; endpoint may vary)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.GetNotifications(limit)
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "max rows")
	return cmd
}

func newOrderCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "order ID",
		Short: "Single equity order with per-execution detail",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.GetOrder(args[0])
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
}
