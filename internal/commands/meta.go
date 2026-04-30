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
//
// Version, Commit, and BuiltAt are populated via SetBuildInfo from main.go,
// where they're injected at build time by the Makefile using -ldflags.
// When the binary is built without ldflags (e.g. `go run`), they default
// to "dev" / "unknown" so the output is still meaningful.
type versionPayload struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	BuiltAt string `json:"built_at"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
	Go      string `json:"go"`
}

// Build info populated by main.go (which receives values from -ldflags).
// Lives in this package so the version command can read them without
// leaking the main package's internal state.
var (
	buildVersion = "dev"
	buildCommit  = "unknown"
	buildBuiltAt = "unknown"
)

// SetBuildInfo is called from main.main with the ldflags-injected values.
// It's a function (not exported package vars set directly) so the API
// stays stable if we later swap to debug.ReadBuildInfo or another source.
func SetBuildInfo(version, commit, builtAt string) {
	if version != "" {
		buildVersion = version
	}
	if commit != "" {
		buildCommit = commit
	}
	if builtAt != "" {
		buildBuiltAt = builtAt
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print rh version and build info",
		RunE: func(cmd *cobra.Command, args []string) error {
			v := cmd.Root().Version
			if v == "" {
				v = buildVersion
			}
			return output.Emit(versionPayload{
				Version: v,
				Commit:  buildCommit,
				BuiltAt: buildBuiltAt,
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
		Use:   "order UUID",
		Short: "Single equity order by UUID (use `rh activity` to list order IDs)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOrderID(args[0]); err != nil {
				return err
			}
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
