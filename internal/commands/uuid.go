package commands

import (
	"fmt"
	"regexp"
)

// uuidRE matches canonical 8-4-4-4-12 hex UUIDs (case-insensitive).
// Robinhood order IDs are always UUIDs; account numbers are 9-digit
// integers and get confused for order IDs surprisingly often.
var uuidRE = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// validateOrderID returns a friendly error when the input is not UUID-shaped.
// Saves a round trip to the RH 404 HTML page when users paste an
// account_number or a symbol by mistake.
func validateOrderID(id string) error {
	if uuidRE.MatchString(id) {
		return nil
	}
	return fmt.Errorf("order ID must be a UUID (got %q) — did you paste an account_number? run `rh activity` and copy the `id` field", id)
}
