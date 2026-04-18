// Package output renders command results in three formats:
//   - plain: YAML-like indented key/value (default — best for AI consumption,
//     no JSON quoting noise, nested structures render as nested indents)
//   - json:  pretty-printed JSON for programmatic use
//   - table: ASCII table for the largest slice in the response, falls back
//     to plain for non-tabular shapes
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

type Format string

const (
	FormatPlain Format = "plain"
	FormatJSON  Format = "json"
	FormatTable Format = "table"
)

// CurrentFormat is set by the root command's persistent pre-run.
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
// Strategy: pick the first []object field at the top of the response, render
// each object as a row with one column per scalar field. Top-level scalars
// are printed above the table as a small header. If no slice exists, fall
// back to plain rendering.
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

	// Print scalar/string headers first.
	scalars := map[string]any{}
	var slices []string
	for k, val := range root {
		switch reflect.ValueOf(val).Kind() {
		case reflect.Slice:
			slices = append(slices, k)
		default:
			scalars[k] = val
		}
	}
	if len(scalars) > 0 {
		printScalarHeader(w, scalars)
		fmt.Fprintln(w)
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

	if err := renderSliceAsTable(w, primary, root[primary]); err != nil {
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

func printScalarHeader(w io.Writer, scalars map[string]any) {
	keys := make([]string, 0, len(scalars))
	for k := range scalars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, k := range keys {
		fmt.Fprintf(tw, "%s:\t%s\n", k, formatScalar(scalars[k]))
	}
	tw.Flush()
}

func renderSliceAsTable(w io.Writer, name string, slice any) error {
	arr, ok := slice.([]any)
	if !ok || len(arr) == 0 {
		fmt.Fprintf(w, "%s: (empty)\n", name)
		return nil
	}

	// Collect column names from union of scalar keys across rows.
	colSet := map[string]bool{}
	for _, row := range arr {
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
			if isScalar(v) {
				colSet[k] = true
			}
		}
	}
	cols := make([]string, 0, len(colSet))
	for c := range colSet {
		cols = append(cols, c)
	}
	sort.Strings(cols)

	fmt.Fprintf(w, "%s (%d row%s):\n", name, len(arr), pluralS(len(arr)))
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	// Header
	fmt.Fprintln(tw, strings.Join(cols, "\t"))
	// Underlines (dashes per column header length)
	dashRow := make([]string, len(cols))
	for i, c := range cols {
		dashRow[i] = strings.Repeat("-", len(c))
	}
	fmt.Fprintln(tw, strings.Join(dashRow, "\t"))
	// Data
	for _, row := range arr {
		obj, _ := row.(map[string]any)
		vals := make([]string, len(cols))
		for i, c := range cols {
			vals[i] = formatScalar(obj[c])
		}
		fmt.Fprintln(tw, strings.Join(vals, "\t"))
	}
	return tw.Flush()
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
