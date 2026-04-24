package client

import (
	"strings"
)

type Quote struct {
	Symbol             string   `json:"symbol"`
	CurrentPrice       *float64 `json:"current_price,omitempty"`
	LastPrice          *float64 `json:"last_price"`
	ExtendedHoursPrice *float64 `json:"extended_hours_price"`
	Bid                *float64 `json:"bid"`
	Ask                *float64 `json:"ask"`
	PreviousClose      *float64 `json:"previous_close,omitempty"`
	PreviousCloseDate  string   `json:"previous_close_date,omitempty"`
	DayChange          *float64 `json:"day_change,omitempty"`
	DayChangePct       *float64 `json:"day_change_pct,omitempty"`
	Is247Eligible      bool     `json:"is_24_7_eligible"`
	MarketCap          *float64 `json:"market_cap"`
	PERatio            *float64 `json:"pe_ratio"`
	DividendYield      *float64 `json:"dividend_yield"`
	// DividendYieldEstimate is populated when the upstream `dividend_yield`
	// field is null (Robinhood leaves it blank for many short-duration bond
	// ETFs) but we can infer it from `dividend_per_share` and
	// `distribution_frequency`. Always expressed as a percentage.
	DividendYieldEstimate *float64 `json:"dividend_yield_estimate,omitempty"`
	DividendPerShare      *float64 `json:"dividend_per_share,omitempty"`
	DistributionFrequency string   `json:"distribution_frequency,omitempty"`
	ExDividendDate        string   `json:"ex_dividend_date,omitempty"`
	Volume                *float64 `json:"volume"`
	AverageVolume         *float64 `json:"average_volume"`
	High                  *float64 `json:"high"`
	Low                   *float64 `json:"low"`
	Open                  *float64 `json:"open"`
	High52Weeks           *float64 `json:"high_52_weeks"`
	Low52Weeks            *float64 `json:"low_52_weeks"`
	UpdatedAt             string   `json:"updated_at"`
}

type instrumentLite struct {
	ID     string `json:"id"`
	Symbol string `json:"symbol"`
}

type instrumentsResp struct {
	Results []instrumentLite `json:"results"`
}

type fullQuote struct {
	Symbol                      string `json:"symbol"`
	LastTradePrice              string `json:"last_trade_price"`
	LastExtendedHoursTradePrice string `json:"last_extended_hours_trade_price"`
	BidPrice                    string `json:"bid_price"`
	AskPrice                    string `json:"ask_price"`
	PreviousClose               string `json:"previous_close"`
	PreviousCloseDate           string `json:"previous_close_date"`
	AdjustedPreviousClose       string `json:"adjusted_previous_close"`
	Is247Eligible               bool   `json:"is_24_7_eligible"`
	UpdatedAt                   string `json:"updated_at"`
}

type fundamentals struct {
	Results []struct {
		MarketCap             string `json:"market_cap"`
		PERatio               string `json:"pe_ratio"`
		DividendYield         string `json:"dividend_yield"`
		DividendPerShare      string `json:"dividend_per_share"`
		DistributionFrequency string `json:"distribution_frequency"`
		ExDividendDate        string `json:"ex_dividend_date"`
		Volume                string `json:"volume"`
		AverageVolume         string `json:"average_volume"`
		High                  string `json:"high"`
		Low                   string `json:"low"`
		Open                  string `json:"open"`
		High52Weeks           string `json:"high_52_weeks"`
		Low52Weeks            string `json:"low_52_weeks"`
	} `json:"results"`
}

type fundamentalsRow struct {
	MarketCap             string
	PERatio               string
	DividendYield         string
	DividendPerShare      string
	DistributionFrequency string
	ExDividendDate        string
	Volume                string
	AverageVolume         string
	High                  string
	Low                   string
	Open                  string
	High52Weeks           string
	Low52Weeks            string
}

func (c *Client) GetQuote(symbol string) (*Quote, error) {
	qs, err := c.GetQuotes([]string{symbol})
	if err != nil {
		return nil, err
	}
	if len(qs) == 0 {
		return nil, errMsg("symbol not found: " + strings.ToUpper(symbol))
	}
	return &qs[0], nil
}

// GetQuotes fetches real-time quotes + fundamentals for N symbols in 3 API
// calls total (instruments / marketdata quotes / fundamentals) instead of 3N.
// Preserves input order; drops symbols the upstream doesn't resolve.
func (c *Client) GetQuotes(symbols []string) ([]Quote, error) {
	if len(symbols) == 0 {
		return nil, errMsg("no symbols provided")
	}
	upper := make([]string, 0, len(symbols))
	seen := make(map[string]bool, len(symbols))
	for _, s := range symbols {
		u := strings.ToUpper(strings.TrimSpace(s))
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		upper = append(upper, u)
	}
	joined := strings.Join(upper, ",")

	var instr instrumentsResp
	if err := c.GetJSON(URL("/instruments/", map[string]string{"symbols": joined}), &instr); err != nil {
		return nil, err
	}
	if len(instr.Results) == 0 {
		return nil, errMsg("no symbols resolved: " + joined)
	}
	instrBySym := make(map[string]string, len(instr.Results))
	ids := make([]string, 0, len(instr.Results))
	resolved := make([]string, 0, len(instr.Results))
	for _, it := range instr.Results {
		sym := strings.ToUpper(it.Symbol)
		instrBySym[sym] = it.ID
		ids = append(ids, it.ID)
		resolved = append(resolved, sym)
	}

	var q struct {
		Results []fullQuote `json:"results"`
	}
	if err := c.GetJSON(URL("/marketdata/quotes/", map[string]string{
		"bounds": "24_5",
		"ids":    strings.Join(ids, ","),
	}), &q); err != nil {
		return nil, err
	}
	quoteBySym := make(map[string]fullQuote, len(q.Results))
	for _, r := range q.Results {
		quoteBySym[strings.ToUpper(r.Symbol)] = r
	}

	var f fundamentals
	if err := c.GetJSON(URL("/fundamentals/", map[string]string{"symbols": strings.Join(resolved, ",")}), &f); err != nil {
		return nil, err
	}
	fundBySym := mapFundamentalsByResolvedOrder(f, resolved)

	out := make([]Quote, 0, len(upper))
	for _, sym := range upper {
		if _, ok := instrBySym[sym]; !ok {
			continue
		}
		r, ok := quoteBySym[sym]
		if !ok {
			continue
		}
		fr, hasFund := fundBySym[sym]

		currentPrice := optFloat(r.LastTradePrice)
		if v := optFloat(r.LastExtendedHoursTradePrice); v != nil && *v > 0 {
			currentPrice = v
		}
		previousClose := optFloat(r.PreviousClose)
		if previousClose == nil {
			previousClose = optFloat(r.AdjustedPreviousClose)
		}

		q0 := Quote{
			Symbol:                sym,
			CurrentPrice:          currentPrice,
			LastPrice:             optFloat(r.LastTradePrice),
			ExtendedHoursPrice:    optFloat(r.LastExtendedHoursTradePrice),
			Bid:                   optFloat(r.BidPrice),
			Ask:                   optFloat(r.AskPrice),
			PreviousClose:         previousClose,
			PreviousCloseDate:     r.PreviousCloseDate,
			Is247Eligible:         r.Is247Eligible,
			MarketCap:             optFloat(fr.MarketCap),
			PERatio:               optFloat(fr.PERatio),
			DividendYield:         optFloat(fr.DividendYield),
			DividendPerShare:      optFloat(fr.DividendPerShare),
			DistributionFrequency: fr.DistributionFrequency,
			ExDividendDate:        fr.ExDividendDate,
			Volume:                optFloat(fr.Volume),
			AverageVolume:         optFloat(fr.AverageVolume),
			High:                  optFloat(fr.High),
			Low:                   optFloat(fr.Low),
			Open:                  optFloat(fr.Open),
			High52Weeks:           optFloat(fr.High52Weeks),
			Low52Weeks:            optFloat(fr.Low52Weeks),
			UpdatedAt:             r.UpdatedAt,
		}
		if !hasFund {
			q0.MarketCap = nil
			q0.PERatio = nil
			q0.DividendYield = nil
			q0.DividendPerShare = nil
			q0.DistributionFrequency = ""
			q0.ExDividendDate = ""
			q0.Volume = nil
			q0.AverageVolume = nil
			q0.High = nil
			q0.Low = nil
			q0.Open = nil
			q0.High52Weeks = nil
			q0.Low52Weeks = nil
		}
		if q0.CurrentPrice != nil && q0.PreviousClose != nil && *q0.PreviousClose != 0 {
			change := *q0.CurrentPrice - *q0.PreviousClose
			changePct := (change / *q0.PreviousClose) * 100
			q0.DayChange = &change
			q0.DayChangePct = &changePct
		}
		if q0.DividendYield == nil && q0.DividendPerShare != nil && q0.LastPrice != nil && *q0.LastPrice > 0 {
			if mult := frequencyMultiplier(fr.DistributionFrequency); mult > 0 {
				est := (*q0.DividendPerShare) * mult / (*q0.LastPrice) * 100
				q0.DividendYieldEstimate = &est
			}
		}
		out = append(out, q0)
	}
	return out, nil
}

func mapFundamentalsByResolvedOrder(f fundamentals, resolved []string) map[string]fundamentalsRow {
	out := make(map[string]fundamentalsRow, len(f.Results))
	for i, sym := range resolved {
		if i >= len(f.Results) {
			break
		}
		row := f.Results[i]
		out[sym] = fundamentalsRow{
			MarketCap:             row.MarketCap,
			PERatio:               row.PERatio,
			DividendYield:         row.DividendYield,
			DividendPerShare:      row.DividendPerShare,
			DistributionFrequency: row.DistributionFrequency,
			ExDividendDate:        row.ExDividendDate,
			Volume:                row.Volume,
			AverageVolume:         row.AverageVolume,
			High:                  row.High,
			Low:                   row.Low,
			Open:                  row.Open,
			High52Weeks:           row.High52Weeks,
			Low52Weeks:            row.Low52Weeks,
		}
	}
	return out
}

// frequencyMultiplier maps Robinhood's distribution_frequency strings to
// annualization factors. Returns 0 for unknown/blank frequencies so the
// caller can skip the estimate rather than guess.
func frequencyMultiplier(f string) float64 {
	switch strings.ToLower(strings.TrimSpace(f)) {
	case "monthly":
		return 12
	case "quarterly":
		return 4
	case "semi-annual", "semiannual", "semi_annual":
		return 2
	case "annual", "annually", "yearly":
		return 1
	}
	return 0
}

func optFloat(s string) *float64 {
	if s == "" {
		return nil
	}
	v := parseFloat(s)
	return &v
}
