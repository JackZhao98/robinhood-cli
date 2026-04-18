package client

import (
	"sort"
	"strings"
	"time"
)

type ActivityEntry struct {
	ID            string  `json:"id"`
	AssetClass    string  `json:"asset_class"` // "equity" or "option"
	AccountNumber string  `json:"account_number"`
	Symbol        string  `json:"symbol"`
	Side          string  `json:"side"` // buy / sell (equity) or buy_to_open / sell_to_close (option)
	Quantity      float64 `json:"quantity"`
	AveragePrice  float64 `json:"average_price"`
	TotalValue    float64 `json:"total_value"`
	OrderType     string  `json:"order_type"` // market / limit
	State         string  `json:"state"`      // filled / cancelled / partially_filled
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
	// Option-only:
	Strategy       string             `json:"strategy,omitempty"`
	Direction      string             `json:"direction,omitempty"` // debit/credit
	Legs           []ActivityOptionLeg `json:"legs,omitempty"`
}

type ActivityOptionLeg struct {
	Side           string  `json:"side"`            // buy / sell
	PositionEffect string  `json:"position_effect"` // open / close
	Strike         float64 `json:"strike"`
	ExpirationDate string  `json:"expiration_date"`
	OptionType     string  `json:"option_type"`     // call / put
	RatioQuantity  float64 `json:"ratio_quantity"`
}

type ActivityResult struct {
	Count   int             `json:"count"`
	Filters map[string]any  `json:"filters"`
	Orders  []ActivityEntry `json:"orders"`
}

// --- equity orders ---

type equityOrderRaw struct {
	ID                 string `json:"id"`
	AccountURL         string `json:"account"`
	Instrument         string `json:"instrument"`
	Side               string `json:"side"`
	Type               string `json:"type"`
	State              string `json:"state"`
	Quantity           string `json:"quantity"`
	CumulativeQuantity string `json:"cumulative_quantity"`
	Price              string `json:"price"`
	AveragePrice       string `json:"average_price"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
	LastTransactionAt  string `json:"last_transaction_at"`
}

type equityOrdersResp struct {
	Results []equityOrderRaw `json:"results"`
	Next    string           `json:"next"`
}

// --- option orders ---

type optionOrderRaw struct {
	ID                 string `json:"id"`
	AccountNumber      string `json:"account_number"`
	ChainSymbol        string `json:"chain_symbol"`
	Type               string `json:"type"`
	State              string `json:"state"`
	Direction          string `json:"direction"`
	Strategy           string `json:"opening_strategy"`
	ClosingStrategy    string `json:"closing_strategy"`
	ProcessedQuantity  string `json:"processed_quantity"`
	Quantity           string `json:"quantity"`
	Price              string `json:"price"`
	ProcessedPremium   string `json:"processed_premium"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
	Legs               []optionOrderLegRaw `json:"legs"`
}

type optionOrderLegRaw struct {
	Side           string `json:"side"`
	PositionEffect string `json:"position_effect"`
	RatioQuantity int    `json:"ratio_quantity"`
	Option        string `json:"option"`
	// Some payloads include expanded fields directly:
	StrikePrice    string `json:"strike_price"`
	ExpirationDate string `json:"expiration_date"`
	OptionType     string `json:"option_type"`
}

type optionOrdersResp struct {
	Results []optionOrderRaw `json:"results"`
	Next    string           `json:"next"`
}

type optionInstrumentRaw struct {
	ID             string `json:"id"`
	StrikePrice    string `json:"strike_price"`
	ExpirationDate string `json:"expiration_date"`
	Type           string `json:"type"`
}

// GetActivity returns combined equity + option order history sorted by
// most-recent first. assetFilter: "" (both) | "equity" | "option".
// since: optional ISO date "2026-01-01" — only orders newer than this.
// limit: maximum entries returned across both asset classes after filtering.
// account: optional account number filter. When empty the call fans out
// across every account the user owns (individual + IRA + …) because
// Robinhood's order endpoints silently default to the primary account.
func (c *Client) GetActivity(limit int, since, assetFilter, account string) (*ActivityResult, error) {
	if limit <= 0 {
		limit = 50
	}
	var sinceTime time.Time
	if since != "" {
		t, err := time.Parse("2006-01-02", since)
		if err != nil {
			return nil, errMsg("invalid --since, expected YYYY-MM-DD")
		}
		sinceTime = t
	}

	// Equity + option endpoints default to the primary individual account
	// when no account_numbers filter is passed. Fan out across all
	// accounts so IRA orders are included. Crypto uses a separate Nummus
	// API and is single-account by nature.
	accounts := []string{account}
	if account == "" && (assetFilter == "" || assetFilter == "equity" || assetFilter == "option") {
		nums, err := c.ListAccountNumbers()
		if err != nil {
			return nil, err
		}
		if len(nums) > 0 {
			accounts = nums
		}
	}

	combined := []ActivityEntry{}

	if assetFilter == "" || assetFilter == "equity" {
		for _, acct := range accounts {
			entries, err := c.fetchEquityOrders(limit, sinceTime, acct)
			if err != nil {
				return nil, err
			}
			combined = append(combined, entries...)
		}
	}
	if assetFilter == "" || assetFilter == "option" {
		for _, acct := range accounts {
			entries, err := c.fetchOptionOrders(limit, sinceTime, acct)
			if err != nil {
				return nil, err
			}
			combined = append(combined, entries...)
		}
	}
	if assetFilter == "crypto" {
		entries, err := c.fetchCryptoOrders(limit, sinceTime)
		if err != nil {
			return nil, err
		}
		combined = append(combined, entries...)
	}

	sort.Slice(combined, func(i, j int) bool {
		return combined[i].CreatedAt > combined[j].CreatedAt
	})
	if len(combined) > limit {
		combined = combined[:limit]
	}

	filters := map[string]any{
		"limit": limit,
	}
	if since != "" {
		filters["since"] = since
	}
	if assetFilter != "" {
		filters["asset_class"] = assetFilter
	}
	if account != "" {
		filters["account"] = account
	}

	return &ActivityResult{
		Count:   len(combined),
		Filters: filters,
		Orders:  combined,
	}, nil
}

func (c *Client) fetchEquityOrders(limit int, since time.Time, account string) ([]ActivityEntry, error) {
	raws, err := c.pageEquityOrders(limit*2, since, account) // fetch extra so date filter still hits limit
	if err != nil {
		return nil, err
	}
	// Resolve symbols in one batched call.
	idSet := map[string]string{} // instrument URL → id
	for _, r := range raws {
		if r.Instrument != "" && idSet[r.Instrument] == "" {
			idSet[r.Instrument] = lastPathSegment(r.Instrument)
		}
	}
	symbols := map[string]string{} // instrument URL → symbol
	ids := make([]string, 0, len(idSet))
	for url, id := range idSet {
		ids = append(ids, id)
		_ = url
	}
	if len(ids) > 0 {
		const batch = 75
		urlByID := map[string]string{}
		for url, id := range idSet {
			urlByID[id] = url
		}
		for start := 0; start < len(ids); start += batch {
			end := start + batch
			if end > len(ids) {
				end = len(ids)
			}
			var resp instrumentsExtResp
			if err := c.GetJSON(URL("/instruments/", map[string]string{
				"ids": strings.Join(ids[start:end], ","),
			}), &resp); err != nil {
				return nil, err
			}
			for _, inst := range resp.Results {
				if url, ok := urlByID[inst.ID]; ok {
					symbols[url] = inst.Symbol
				}
			}
		}
	}

	out := make([]ActivityEntry, 0, len(raws))
	for _, r := range raws {
		qty := parseFloat(r.CumulativeQuantity)
		if qty == 0 {
			qty = parseFloat(r.Quantity)
		}
		avg := parseFloat(r.AveragePrice)
		if avg == 0 {
			avg = parseFloat(r.Price)
		}
		out = append(out, ActivityEntry{
			ID:            r.ID,
			AssetClass:    "equity",
			AccountNumber: lastPathSegment(strings.TrimSuffix(r.AccountURL, "/")),
			Symbol:        symbols[r.Instrument],
			Side:          r.Side,
			Quantity:      qty,
			AveragePrice:  avg,
			TotalValue:    qty * avg,
			OrderType:     r.Type,
			State:         r.State,
			CreatedAt:     r.CreatedAt,
			UpdatedAt:     r.UpdatedAt,
		})
	}
	return out, nil
}

func (c *Client) pageEquityOrders(maxRows int, since time.Time, account string) ([]equityOrderRaw, error) {
	out := []equityOrderRaw{}
	q := map[string]string{}
	if account != "" {
		q["account_numbers"] = account
	}
	url := URL("/orders/", q)
	for url != "" && len(out) < maxRows {
		var page equityOrdersResp
		if err := c.GetJSON(url, &page); err != nil {
			return nil, err
		}
		stop := false
		for _, r := range page.Results {
			if !since.IsZero() {
				t, err := time.Parse(time.RFC3339Nano, r.CreatedAt)
				if err == nil && t.Before(since) {
					stop = true
					break
				}
			}
			out = append(out, r)
			if len(out) >= maxRows {
				break
			}
		}
		if stop {
			break
		}
		url = page.Next
	}
	return out, nil
}

func (c *Client) fetchOptionOrders(limit int, since time.Time, account string) ([]ActivityEntry, error) {
	raws, err := c.pageOptionOrders(limit*2, since, account)
	if err != nil {
		return nil, err
	}

	// Collect leg option instrument IDs that need enrichment.
	missingIDs := map[string]struct{}{}
	for _, r := range raws {
		for _, l := range r.Legs {
			if l.StrikePrice == "" && l.Option != "" {
				missingIDs[lastPathSegment(strings.TrimSuffix(l.Option, "/"))] = struct{}{}
			}
		}
	}
	enriched := map[string]optionInstrumentRaw{}
	if len(missingIDs) > 0 {
		idList := make([]string, 0, len(missingIDs))
		for id := range missingIDs {
			idList = append(idList, id)
		}
		const batch = 50
		for start := 0; start < len(idList); start += batch {
			end := start + batch
			if end > len(idList) {
				end = len(idList)
			}
			var resp struct {
				Results []optionInstrumentRaw `json:"results"`
			}
			if err := c.GetJSON(URL("/options/instruments/", map[string]string{
				"ids": strings.Join(idList[start:end], ","),
			}), &resp); err != nil {
				return nil, err
			}
			for _, r := range resp.Results {
				enriched[r.ID] = r
			}
		}
	}

	out := make([]ActivityEntry, 0, len(raws))
	for _, r := range raws {
		qty := parseFloat(r.ProcessedQuantity)
		if qty == 0 {
			qty = parseFloat(r.Quantity)
		}
		price := parseFloat(r.Price)
		// processed_premium is total premium (price * qty * 100).
		premium := parseFloat(r.ProcessedPremium)
		total := premium
		if total == 0 {
			total = qty * price * 100
		}

		legs := make([]ActivityOptionLeg, 0, len(r.Legs))
		side := ""
		for _, l := range r.Legs {
			strike := parseFloat(l.StrikePrice)
			exp := l.ExpirationDate
			optType := l.OptionType
			if (strike == 0 || exp == "" || optType == "") && l.Option != "" {
				if e, ok := enriched[lastPathSegment(strings.TrimSuffix(l.Option, "/"))]; ok {
					if strike == 0 {
						strike = parseFloat(e.StrikePrice)
					}
					if exp == "" {
						exp = e.ExpirationDate
					}
					if optType == "" {
						optType = e.Type
					}
				}
			}
			legs = append(legs, ActivityOptionLeg{
				Side:           l.Side,
				PositionEffect: l.PositionEffect,
				Strike:         strike,
				ExpirationDate: exp,
				OptionType:     optType,
				RatioQuantity:  float64(l.RatioQuantity),
			})
			if side == "" {
				side = l.Side
			}
		}

		strategy := r.Strategy
		if strategy == "" {
			strategy = r.ClosingStrategy
		}

		out = append(out, ActivityEntry{
			ID:            r.ID,
			AssetClass:    "option",
			AccountNumber: r.AccountNumber,
			Symbol:        r.ChainSymbol,
			Side:          side,
			Quantity:      qty,
			AveragePrice:  price,
			TotalValue:    total,
			OrderType:     r.Type,
			State:         r.State,
			CreatedAt:     r.CreatedAt,
			UpdatedAt:     r.UpdatedAt,
			Strategy:      strategy,
			Direction:     r.Direction,
			Legs:          legs,
		})
	}
	return out, nil
}

func (c *Client) pageOptionOrders(maxRows int, since time.Time, account string) ([]optionOrderRaw, error) {
	out := []optionOrderRaw{}
	q := map[string]string{}
	if account != "" {
		q["account_numbers"] = account
	}
	url := URL("/options/orders/", q)
	for url != "" && len(out) < maxRows {
		var page optionOrdersResp
		if err := c.GetJSON(url, &page); err != nil {
			return nil, err
		}
		stop := false
		for _, r := range page.Results {
			if !since.IsZero() {
				t, err := time.Parse(time.RFC3339Nano, r.CreatedAt)
				if err == nil && t.Before(since) {
					stop = true
					break
				}
			}
			out = append(out, r)
			if len(out) >= maxRows {
				break
			}
		}
		if stop {
			break
		}
		url = page.Next
	}
	return out, nil
}

func lastPathSegment(s string) string {
	s = strings.TrimSuffix(s, "/")
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}
