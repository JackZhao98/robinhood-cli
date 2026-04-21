package commands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jackzhao/robinhood-cli/internal/client"
	"github.com/jackzhao/robinhood-cli/internal/config"
	"github.com/jackzhao/robinhood-cli/internal/output"
)

// indexListItem is what --list emits (sorted, deterministic).
type indexListItem struct {
	Symbol string `json:"symbol"`
	UUID   string `json:"uuid"`
}

func newIndexCmd() *cobra.Command {
	var (
		doList      bool
		registerIn  []string
		valuesOnly  bool
	)

	cmd := &cobra.Command{
		Use:   "index [SYMBOL ...]",
		Short: "Market indexes (VIX, etc.) — values + registry",
		Long: strings.TrimSpace(`
Query Robinhood's internal index-values endpoint (/marketdata/indexes/values/v1).

Robinhood has no public symbol→UUID lookup for indexes. Sniff a UUID from
the RH web/app once, then register it:

    rh index --register VIX=3b912aa2-88f9-4682-8ae3-e39520bdf4db
    rh index VIX
    rh index VIX SPX NDX           # batch
    rh index --list                # show known mappings

Registry is stored at ~/.config/rh/indexes.json.
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, err := config.LoadIndexRegistry()
			if err != nil {
				return err
			}

			// --register SYMBOL UUID (repeatable: --register VIX=<uuid>)
			if len(registerIn) > 0 {
				for _, pair := range registerIn {
					sym, uuid, ok := splitRegisterPair(pair)
					if !ok {
						return fmt.Errorf("invalid --register %q (expected SYMBOL=UUID)", pair)
					}
					if err := reg.Register(sym, uuid); err != nil {
						return err
					}
				}
				// If there are no positional symbols, just confirm the write.
				if len(args) == 0 && !doList {
					return output.Emit(map[string]any{
						"registered": registerIn,
						"registry":   flattenRegistry(reg),
					})
				}
			}

			if doList {
				return output.Emit(map[string]any{
					"count":   len(reg.Indexes),
					"indexes": flattenRegistry(reg),
				})
			}

			if len(args) == 0 {
				return fmt.Errorf("pass at least one SYMBOL, or use --list / --register")
			}

			// Resolve each symbol → UUID via the registry.
			uuids := make([]string, 0, len(args))
			missing := make([]string, 0)
			for _, s := range args {
				if id, ok := reg.Lookup(s); ok {
					uuids = append(uuids, id)
				} else {
					missing = append(missing, strings.ToUpper(s))
				}
			}
			if len(missing) > 0 {
				return fmt.Errorf("unregistered symbol(s): %s — sniff UUID from RH web then `rh index --register SYMBOL=UUID`",
					strings.Join(missing, ", "))
			}

			c, err := client.New()
			if err != nil {
				return err
			}
			values, err := c.GetIndexValues(uuids, !valuesOnly)
			if err != nil {
				return err
			}
			return output.Emit(values)
		},
	}

	cmd.Flags().BoolVar(&doList, "list", false, "list known SYMBOL→UUID mappings")
	cmd.Flags().StringArrayVar(&registerIn, "register", nil,
		"register a SYMBOL=UUID pair (repeatable; e.g. --register VIX=3b91...-...)")
	cmd.Flags().BoolVar(&valuesOnly, "values-only", false,
		"skip fundamentals call (open/high/low/52w/pe) — faster, less data")

	return cmd
}

// splitRegisterPair parses "VIX=<uuid>" or legacy space-separated "VIX <uuid>"
// if the user passes them as one arg. We only accept the SYMBOL=UUID form for
// clarity; any space in the pair is rejected.
func splitRegisterPair(s string) (string, string, bool) {
	s = strings.TrimSpace(s)
	eq := strings.IndexByte(s, '=')
	if eq <= 0 || eq == len(s)-1 {
		return "", "", false
	}
	sym := strings.TrimSpace(s[:eq])
	uuid := strings.TrimSpace(s[eq+1:])
	if sym == "" || uuid == "" {
		return "", "", false
	}
	return sym, uuid, true
}

func flattenRegistry(r *config.IndexRegistry) []indexListItem {
	items := make([]indexListItem, 0, len(r.Indexes))
	for k, v := range r.Indexes {
		items = append(items, indexListItem{Symbol: k, UUID: v})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Symbol < items[j].Symbol })
	return items
}
