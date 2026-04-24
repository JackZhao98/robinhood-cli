package commands

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/jackzhao/robinhood-cli/internal/client"
	"github.com/jackzhao/robinhood-cli/internal/output"
)

func newScanCmd() *cobra.Command {
	var (
		minChange float64
		maxChange float64
		sortDir   string
	)

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Bonfire screener — scan US equities by 1d % change",
		Long: strings.TrimSpace(`
Hits Robinhood's internal Bonfire screener
(https://bonfire.robinhood.com/screeners/scan/) and flattens the SDUI
response into a clean rows list.

Currently supports the 1-day price change % filter (the endpoint supports
dozens more indicators — add them as they're needed).

Examples:
    rh scan --min-change 10           # up 10%+ today (default)
    rh scan --min-change 20           # up 20%+ today — rare events
    rh scan --max-change -10          # down 10%+ today
    rh scan --min-change 5 --max-change 15  # bounded range
    rh scan --min-change 5 --sort-dir asc   # smallest first
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New()
			if err != nil {
				return err
			}
			var minP, maxP *float64
			if cmd.Flags().Changed("min-change") {
				v := minChange
				minP = &v
			}
			if cmd.Flags().Changed("max-change") {
				v := maxChange
				maxP = &v
			}
			// If user gave nothing, default to min=10 (common "what moved" scan).
			if minP == nil && maxP == nil {
				v := 10.0
				minP = &v
			}
			req := client.BuildScan1dChangeRequest(minP, maxP, sortDir)
			res, err := c.Scan(req)
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}

	cmd.Flags().Float64Var(&minChange, "min-change", 10, "minimum 1d percent change (e.g. 10 = up >=10%, -10 = down >=10%)")
	cmd.Flags().Float64Var(&maxChange, "max-change", 0, "maximum 1d percent change (unset = no upper bound)")
	cmd.Flags().StringVar(&sortDir, "sort-dir", "DESC", "sort direction: DESC (biggest first) or ASC")

	return cmd
}
