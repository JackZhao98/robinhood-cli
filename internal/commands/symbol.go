package commands

import (
	"github.com/spf13/cobra"

	"github.com/jackzhao/robinhood-cli/internal/client"
	"github.com/jackzhao/robinhood-cli/internal/output"
)

func newSymbolCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "symbol",
		Short: "Symbol research: search, news, earnings, ratings, similar, tags, splits",
	}
	cmd.AddCommand(
		newSymbolSearchCmd(),
		newSymbolNewsCmd(),
		newSymbolEarningsCmd(),
		newSymbolRatingsCmd(),
		newSymbolSimilarCmd(),
		newSymbolTagsCmd(),
		newSymbolSplitsCmd(),
	)
	return cmd
}

func newSymbolSearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search QUERY",
		Short: "Search instruments by company name or ticker fragment",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := joinArgs(args)
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.SymbolSearch(query)
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
}

func newSymbolNewsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "news SYMBOL",
		Short: "Recent news headlines for a symbol",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.GetNews(args[0])
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
}

func newSymbolEarningsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "earnings SYMBOL",
		Short: "Past + upcoming earnings reports (EPS actual/estimate, report date)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.GetEarnings(args[0])
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
}

func newSymbolRatingsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ratings SYMBOL",
		Short: "Analyst buy/hold/sell breakdown",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.GetRatings(args[0])
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
}

func newSymbolSimilarCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "similar SYMBOL",
		Short: "Robinhood-suggested similar stocks",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.GetSimilar(args[0])
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
}

func newSymbolTagsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tags SYMBOL",
		Short: "Sector/theme tags (collections) the symbol belongs to",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.GetTags(args[0])
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
}

func newSymbolSplitsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "splits SYMBOL",
		Short: "Historical stock splits",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New()
			if err != nil {
				return err
			}
			res, err := c.GetSplits(args[0])
			if err != nil {
				return err
			}
			return output.Emit(res)
		},
	}
}

func joinArgs(args []string) string {
	out := ""
	for i, a := range args {
		if i > 0 {
			out += " "
		}
		out += a
	}
	return out
}
