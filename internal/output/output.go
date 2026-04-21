// Package output renders command results in three formats:
//   - table: ASCII table for the largest slice in the response, falls back
//     to plain for non-tabular shapes
//   - plain: YAML-like indented key/value (best for AI consumption,
//     no JSON quoting noise, nested structures render as nested indents)
//   - json:  pretty-printed JSON for programmatic use
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"

	prettytable "github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

type Format string

const (
	FormatPlain Format = "plain"
	FormatJSON  Format = "json"
	FormatTable Format = "table"
)

// CurrentFormat is set by the root command's persistent pre-run. It defaults
// to plain here as a safe package fallback, but the CLI default is table.
var CurrentFormat Format = FormatPlain

func ParseFormat(s string) (Format, error) {
	switch strings.ToLower(s) {
	case "", "plain", "yaml":
		return FormatPlain, nil
	case "json":
		return FormatJSON, nil
	case "table":
		return FormatTable, nil
	default:
		return "", fmt.Errorf("unknown format %q (want plain | json | table)", s)
	}
}

// Emit writes v to stdout in the currently-selected format.
func Emit(v any) error {
	switch CurrentFormat {
	case FormatJSON:
		return emitJSON(os.Stdout, v)
	case FormatTable:
		return emitTable(os.Stdout, v)
	default:
		return emitPlain(os.Stdout, v)
	}
}

// Fail reports an error in a format consistent with the chosen output mode
// and exits non-zero. Always goes to stderr.
func Fail(err error) {
	switch CurrentFormat {
	case FormatJSON:
		_ = json.NewEncoder(os.Stderr).Encode(map[string]string{"error": err.Error()})
	default:
		fmt.Fprintf(os.Stderr, "error: %s\n", err.Error())
	}
	os.Exit(1)
}

// --- JSON ---

func emitJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// --- Plain (YAML) ---
//
// We round-trip through JSON so the user-facing field names match the JSON
// tags already declared on every result struct (no need for separate yaml
// tags). yaml.v3 then renders the generic structure with sane defaults.
func emitPlain(w io.Writer, v any) error {
	generic, err := toGeneric(v)
	if err != nil {
		return err
	}
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	defer enc.Close()
	return enc.Encode(generic)
}

func toGeneric(v any) (any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var g any
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, err
	}
	return g, nil
}

// --- Table ---
//
// Strategy: pick the largest []object field at the top of the response and
// render it as a boxed table. Top-level scalars are rendered as a separate
// key/value table. If the payload has no scalar keys and no slices, fall back
// to plain rendering.
func emitTable(w io.Writer, v any) error {
	generic, err := toGeneric(v)
	if err != nil {
		return err
	}
	root, ok := generic.(map[string]any)
	if !ok {
		// Top-level isn't an object — fall back to plain.
		return emitPlain(w, v)
	}

	scalars := map[string]any{}
	var slices []string
	for k, val := range root {
		if val == nil {
			scalars[k] = val
			continue
		}
		switch reflect.ValueOf(val).Kind() {
		case reflect.Slice:
			slices = append(slices, k)
		case reflect.Map, reflect.Struct:
			// Nested complex values are better shown in plain mode.
		default:
			scalars[k] = val
		}
	}

	if len(slices) == 0 && len(scalars) == 0 {
		return emitPlain(w, v)
	}

	if len(scalars) > 0 {
		title := "summary"
		if len(slices) == 0 {
			title = "details"
		}
		renderKeyValueTable(w, title, scalars)
		if len(slices) > 0 {
			fmt.Fprintln(w)
		}
	}
	if len(slices) == 0 {
		return nil
	}
	sort.Strings(slices)

	// Pick the slice with the most elements as the "primary" table.
	primary := slices[0]
	primaryLen := -1
	for _, name := range slices {
		if arr, ok := root[name].([]any); ok && len(arr) > primaryLen {
			primary = name
			primaryLen = len(arr)
		}
	}

	if err := renderSliceAsTable(w, primary, root[primary], root); err != nil {
		return err
	}

	// For any other slices, just note them — let the user --format plain
	// to see them all.
	for _, name := range slices {
		if name == primary {
			continue
		}
		if arr, ok := root[name].([]any); ok {
			fmt.Fprintf(w, "\n(%d item(s) in %s — use `--format plain` to see)\n", len(arr), name)
		}
	}
	return nil
}

func renderKeyValueTable(w io.Writer, title string, scalars map[string]any) {
	display, keys := reshapeKeyValueTable(title, scalars)
	tw := prettytable.NewWriter()
	tw.SetStyle(prettyTableStyle())
	if strings.TrimSpace(title) != "" && title != "details" {
		tw.SetTitle(title)
	}
	tw.AppendHeader(prettytable.Row{"field", "value"})
	for _, k := range keys {
		tw.AppendRow(prettytable.Row{
			stylizeHeader(k, writerSupportsColor(w)),
			stylizeValue(k, display[k], display, writerSupportsColor(w)),
		})
	}
	io.WriteString(w, tw.Render())
}

func reshapeKeyValueTable(title string, scalars map[string]any) (map[string]any, []string) {
	if title == "details" {
		if display, keys, ok := reshapeQuoteDetails(scalars); ok {
			return display, keys
		}
	}
	keys := make([]string, 0, len(scalars))
	for k := range scalars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return scalars, keys
}

func reshapeQuoteDetails(scalars map[string]any) (map[string]any, []string, bool) {
	if _, ok := scalars["symbol"]; !ok {
		return nil, nil, false
	}
	currentPrice, ok := firstFloat(scalars["current_price"], scalars["last_price"])
	if !ok {
		return nil, nil, false
	}

	display := map[string]any{
		"symbol": scalars["symbol"],
		"price":  currentPrice,
	}
	order := []string{"symbol", "price"}

	dayChange, dayChangeOK := firstFloat(scalars["day_change"])
	dayChangePct, dayChangePctOK := firstFloat(scalars["day_change_pct"])
	if !dayChangeOK || !dayChangePctOK {
		if previousClose, ok := firstFloat(scalars["previous_close"]); ok && previousClose != 0 {
			dayChange = currentPrice - previousClose
			dayChangePct = (dayChange / previousClose) * 100
			dayChangeOK = true
			dayChangePctOK = true
		}
	}
	if dayChangeOK && dayChangePctOK {
		display["today_change"] = fmt.Sprintf("%+.4f (%+.2f%%)", dayChange, dayChangePct)
		display["table_day_change_raw"] = dayChange
		order = append(order, "today_change")
	}

	for _, key := range []string{
		"previous_close",
		"previous_close_date",
		"open",
		"bid",
		"ask",
		"high",
		"low",
		"extended_hours_price",
		"volume",
		"average_volume",
		"high_52_weeks",
		"low_52_weeks",
		"market_cap",
		"pe_ratio",
		"dividend_yield",
		"dividend_yield_estimate",
		"updated_at",
	} {
		if v, ok := scalars[key]; ok && hasDisplayValue(v) {
			display[key] = v
			order = append(order, key)
		}
	}

	return display, order, true
}

func hasDisplayValue(v any) bool {
	if v == nil {
		return false
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s) != ""
	}
	return true
}

func firstFloat(values ...any) (float64, bool) {
	for _, v := range values {
		if n, ok := asFloat(v); ok {
			return n, true
		}
	}
	return 0, false
}

func renderSliceAsTable(w io.Writer, name string, slice any, root map[string]any) error {
	arr, ok := slice.([]any)
	if !ok || len(arr) == 0 {
		fmt.Fprintf(w, "%s: (empty)\n", name)
		return nil
	}

	displayRows := arr
	preferredCols := []string{}
	if name == "options" {
		displayRows, preferredCols = reshapeOptionChainRows(arr, root)
	}

	// Collect column names from union of scalar keys across rows.
	colSet := map[string]bool{}
	for _, row := range displayRows {
		obj, ok := row.(map[string]any)
		if !ok {
			// Non-object slice — print as bullet list.
			fmt.Fprintf(w, "%s:\n", name)
			for _, x := range arr {
				fmt.Fprintf(w, "  - %s\n", formatScalar(x))
			}
			return nil
		}
		for k, v := range obj {
			if strings.HasSuffix(k, "_raw") || strings.HasPrefix(k, "table_") {
				continue
			}
			if isScalar(v) {
				colSet[k] = true
			}
		}
	}
	cols := make([]string, 0, len(colSet))
	if len(preferredCols) > 0 {
		for _, c := range preferredCols {
			if colSet[c] {
				cols = append(cols, c)
				delete(colSet, c)
			}
		}
	}
	for c := range colSet {
		cols = append(cols, c)
	}
	if len(preferredCols) == 0 {
		sort.Strings(cols)
	} else {
		extras := make([]string, 0, len(colSet))
		for _, c := range cols[len(cols)-len(colSet):] {
			extras = append(extras, c)
		}
		sort.Strings(extras)
		cols = append(cols[:len(cols)-len(colSet)], extras...)
	}

	useColor := writerSupportsColor(w)
	tw := prettytable.NewWriter()
	tw.SetStyle(prettyTableStyle())
	tw.SetTitle(fmt.Sprintf("%s (%d row%s)", name, len(arr), pluralS(len(arr))))
	header := make(prettytable.Row, len(cols))
	for i, c := range cols {
		header[i] = stylizeHeader(c, useColor)
	}
	tw.AppendHeader(header)
	for _, row := range displayRows {
		obj, _ := row.(map[string]any)
		vals := make(prettytable.Row, len(cols))
		for i, c := range cols {
			vals[i] = stylizeValue(c, obj[c], obj, useColor)
		}
		tw.AppendRow(vals)
	}
	io.WriteString(w, tw.Render())
	return nil
}

func isScalar(v any) bool {
	if v == nil {
		return true
	}
	switch reflect.ValueOf(v).Kind() {
	case reflect.Map, reflect.Slice, reflect.Struct:
		return false
	default:
		return true
	}
}

func formatScalar(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		// Integer floats render without trailing .0
		if x == float64(int64(x)) && x < 1e15 {
			return fmt.Sprintf("%d", int64(x))
		}
		return fmt.Sprintf("%.4f", x)
	}
	// Fallback: JSON-encode complex types.
	b, _ := json.Marshal(v)
	return string(b)
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func prettyTableStyle() prettytable.Style {
	style := prettytable.StyleRounded
	style.Options.SeparateRows = false
	style.Options.DrawBorder = true
	return style
}

func writerSupportsColor(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

func stylizeHeader(s string, useColor bool) string {
	if !useColor {
		return s
	}
	return text.Colors{text.FgHiWhite, text.Bold}.Sprint(s)
}

func stylizeValue(key string, value any, row map[string]any, useColor bool) string {
	raw := formatScalar(value)
	if !useColor {
		return raw
	}
	if raw == "" {
		return raw
	}
	lk := strings.ToLower(key)
	if lk == "ask" {
		return stylizeAskValue(raw, row)
	}
	if lk == "price" || lk == "current_price" {
		return stylizeQuotePriceValue(raw, row)
	}
	if !isSignedMetric(lk) {
		return raw
	}
	if n, ok := value.(float64); ok {
		switch {
		case n > 0:
			return text.Colors{text.FgGreen}.Sprint(raw)
		case n < 0:
			return text.Colors{text.FgRed}.Sprint(raw)
		default:
			return raw
		}
	}
	if strings.HasPrefix(raw, "+") {
		return text.Colors{text.FgGreen}.Sprint(raw)
	}
	if strings.HasPrefix(raw, "-") {
		return text.Colors{text.FgRed}.Sprint(raw)
	}
	return raw
}

func isSignedMetric(key string) bool {
	for _, needle := range []string{
		"change", "return", "gain", "loss", "pnl", "move_pct",
	} {
		if strings.Contains(key, needle) {
			return true
		}
	}
	return false
}

func reshapeOptionChainRows(arr []any, root map[string]any) ([]any, []string) {
	optType, _ := root["option_type"].(string)
	side, _ := root["side"].(string)
	costKey := optionCostKey(optType, side)
	cols := []string{
		"strike",
		"bid",
		"ask",
		costKey,
		"break_even",
		"ask_move_pct",
		"chance_of_profit_long",
		"chance_of_profit_short",
		"delta",
		"gamma",
		"iv",
		"volume",
		"open_interest",
	}

	out := make([]any, 0, len(arr))
	for _, row := range arr {
		obj, ok := row.(map[string]any)
		if !ok {
			out = append(out, row)
			continue
		}
		cloned := map[string]any{}
		for k, v := range obj {
			if k == "instrument_id" {
				continue
			}
			cloned[k] = v
		}
		cloned["table_side"] = side

		premium := optionPremium(obj, side)
		if premium != nil {
			cloned[costKey] = *premium
			if strike, ok := asFloat(obj["strike"]); ok {
				if strings.EqualFold(optType, "put") {
					cloned["break_even"] = strike - *premium
				} else {
					cloned["break_even"] = strike + *premium
				}
			}
		}
		if ask, ok := asFloat(obj["ask"]); ok {
			if ref, ok := optionReferencePrice(obj); ok && ref != 0 {
				movePct := ((ask - ref) / ref) * 100
				cloned["ask_move_pct_raw"] = movePct
				cloned["ask_move_pct"] = fmt.Sprintf("%+.2f%%", movePct)
			} else {
				cloned["ask_move_pct_raw"] = float64(0)
				cloned["ask_move_pct"] = "0.00%"
			}
		}

		out = append(out, cloned)
	}
	return out, cols
}

func optionCostKey(optType, side string) string {
	switch {
	case strings.EqualFold(optType, "put") && strings.EqualFold(side, "sell"):
		return "put_credit"
	case strings.EqualFold(optType, "put"):
		return "put_price"
	case strings.EqualFold(side, "sell"):
		return "call_credit"
	default:
		return "call_price"
	}
}

func optionPremium(row map[string]any, side string) *float64 {
	key := "ask"
	if strings.EqualFold(side, "sell") {
		key = "bid"
	}
	if v, ok := asFloat(row[key]); ok {
		return &v
	}
	return nil
}

func optionReferencePrice(row map[string]any) (float64, bool) {
	if v, ok := asFloat(row["last"]); ok && v != 0 {
		return v, true
	}
	if v, ok := asFloat(row["mark"]); ok && v != 0 {
		return v, true
	}
	if bid, ok := asFloat(row["bid"]); ok {
		if ask, ok := asFloat(row["ask"]); ok && bid != 0 && ask != 0 {
			return (bid + ask) / 2, true
		}
	}
	return 0, false
}

func asFloat(v any) (float64, bool) {
	n, ok := v.(float64)
	return n, ok
}

func stylizeAskValue(raw string, row map[string]any) string {
	move, ok := asFloat(row["ask_move_pct_raw"])
	if !ok {
		return raw
	}
	side, _ := row["table_side"].(string)
	if side == "" {
		side = "buy"
	}
	switch {
	case strings.EqualFold(side, "sell") && move > 0:
		return text.Colors{text.FgGreen}.Sprint(raw)
	case strings.EqualFold(side, "sell") && move < 0:
		return text.Colors{text.FgRed}.Sprint(raw)
	case move > 0:
		return text.Colors{text.FgRed}.Sprint(raw)
	case move < 0:
		return text.Colors{text.FgGreen}.Sprint(raw)
	default:
		return raw
	}
}

func stylizeQuotePriceValue(raw string, row map[string]any) string {
	move, ok := asFloat(row["table_day_change_raw"])
	if !ok {
		return raw
	}
	switch {
	case move > 0:
		return text.Colors{text.FgGreen}.Sprint(raw)
	case move < 0:
		return text.Colors{text.FgRed}.Sprint(raw)
	default:
		return raw
	}
}
