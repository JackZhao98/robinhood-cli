package client

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ─────────────────────────────────────────────────────────────────────────
//  Equity order placement (POST /orders/)
// ─────────────────────────────────────────────────────────────────────────

// DollarAmount is the RH envelope for fractional-share dollar-based market buys.
type DollarAmount struct {
	Amount       string `json:"amount"`
	CurrencyCode string `json:"currency_code"`
}

// EquityOrderRequest is the payload for POST /orders/.
// RH requires `price` even on market orders (it's used as a sanity bound
// during routing — the actual fill can differ).
type EquityOrderRequest struct {
	Account      string        `json:"account"`                  // full URL
	Instrument   string        `json:"instrument"`               // full URL
	Symbol       string        `json:"symbol"`
	Type         string        `json:"type"`                     // market | limit
	TimeInForce  string        `json:"time_in_force"`            // gfd | gtc | ioc | fok
	Trigger      string        `json:"trigger"`                  // immediate | stop
	Quantity     string        `json:"quantity"`                 // decimal-as-string
	Price        string        `json:"price"`                    // decimal-as-string
	Side         string        `json:"side"`                     // buy | sell
	RefID        string        `json:"ref_id,omitempty"`
	DollarBased  *DollarAmount `json:"dollar_based_amount,omitempty"`
}

// PlacedOrder is the minimal response we care about.
type PlacedOrder struct {
	ID           string `json:"id"`
	State        string `json:"state"`
	Symbol       string `json:"symbol"`
	Quantity     string `json:"quantity"`
	AveragePrice string `json:"average_price"`
	CreatedAt    string `json:"created_at"`
	// Raw response kept for surfacing validation errors to the user.
	Raw map[string]any `json:"-"`
}

// PlaceEquityOrder POSTs to /orders/. Returns placed order on 201 (or 200),
// or an error containing the server's validation message on 4xx/5xx.
func (c *Client) PlaceEquityOrder(req EquityOrderRequest) (*PlacedOrder, error) {
	var raw map[string]any
	status, err := c.PostJSON(URL("/orders/", nil), req, &raw)
	if err != nil {
		return nil, fmt.Errorf("place order: %w", err)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("order rejected (http %d): %s", status, formatRHError(raw))
	}

	// Decode the fields we care about by re-marshaling the raw map.
	body, _ := json.Marshal(raw)
	out := &PlacedOrder{Raw: raw}
	_ = json.Unmarshal(body, out)
	return out, nil
}

// formatRHError turns Robinhood's nested validation JSON into a one-line
// human message suitable for CLI output.
func formatRHError(raw map[string]any) string {
	if raw == nil {
		return "(no body)"
	}
	// Common shapes:
	//   {"detail": "..."}
	//   {"non_field_errors": ["...", "..."]}
	//   {"<field>": ["...", "..."]}
	if d, ok := raw["detail"].(string); ok {
		return d
	}
	parts := []string{}
	for k, v := range raw {
		switch vv := v.(type) {
		case string:
			parts = append(parts, fmt.Sprintf("%s: %s", k, vv))
		case []any:
			msgs := []string{}
			for _, it := range vv {
				if s, ok := it.(string); ok {
					msgs = append(msgs, s)
				}
			}
			if len(msgs) > 0 {
				parts = append(parts, fmt.Sprintf("%s: %s", k, strings.Join(msgs, "; ")))
			}
		}
	}
	if len(parts) == 0 {
		b, _ := json.Marshal(raw)
		return string(b)
	}
	return strings.Join(parts, " | ")
}

// InstrumentBySymbol returns {id, url, symbol} for a ticker.
// Used by the trade command to build the instrument URL in the order payload.
type InstrumentInfo struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	Symbol string `json:"symbol"`
}

type instrumentsLookupResp struct {
	Results []InstrumentInfo `json:"results"`
}

func (c *Client) InstrumentBySymbol(symbol string) (*InstrumentInfo, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	var resp instrumentsLookupResp
	if err := c.GetJSON(URL("/instruments/", map[string]string{"symbols": symbol}), &resp); err != nil {
		return nil, fmt.Errorf("lookup instrument %s: %w", symbol, err)
	}
	if len(resp.Results) == 0 {
		return nil, fmt.Errorf("symbol not found: %s", symbol)
	}
	return &resp.Results[0], nil
}

// AccountURL builds the full /accounts/<num>/ URL used in order payloads.
func AccountURL(accountNumber string) string {
	return URL("/accounts/"+accountNumber+"/", nil)
}

// QuoteSnapshot is a thin wrapper returning bid/ask/last so the trade command
// can compute fractional share counts without loading the full quote machinery.
type QuoteSnapshot struct {
	BidPrice       string `json:"bid_price"`
	AskPrice       string `json:"ask_price"`
	LastTradePrice string `json:"last_trade_price"`
	Symbol         string `json:"symbol"`
}

type quoteSnapshotResp struct {
	Results []QuoteSnapshot `json:"results"`
}

func (c *Client) QuoteSnapshot(instrumentID string) (*QuoteSnapshot, error) {
	var resp quoteSnapshotResp
	err := c.GetJSON(URL("/marketdata/quotes/", map[string]string{
		"bounds": "24_5",
		"ids":    instrumentID,
	}), &resp)
	if err != nil {
		return nil, fmt.Errorf("quote snapshot: %w", err)
	}
	if len(resp.Results) == 0 {
		return nil, fmt.Errorf("no quote returned")
	}
	return &resp.Results[0], nil
}
