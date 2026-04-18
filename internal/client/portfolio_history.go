package client

type PortfolioPoint struct {
	BeginsAt          string  `json:"begins_at"`
	OpenEquity        float64 `json:"open_equity"`
	CloseEquity       float64 `json:"close_equity"`
	OpenMarketValue   float64 `json:"open_market_value"`
	CloseMarketValue  float64 `json:"close_market_value"`
	AdjustedOpen      float64 `json:"adjusted_open_equity"`
	AdjustedClose     float64 `json:"adjusted_close_equity"`
	NetReturn         float64 `json:"net_return"`
	Session           string  `json:"session"`
}

type PortfolioHistory struct {
	AccountNumber  string           `json:"account_number"`
	Span           string           `json:"span"`
	Interval       string           `json:"interval"`
	StartEquity    float64          `json:"start_equity"`
	EndEquity      float64          `json:"end_equity"`
	NetChange      float64          `json:"net_change"`
	PercentChange  float64          `json:"percent_change"`
	Count          int              `json:"count"`
	Points         []PortfolioPoint `json:"points"`
}

type portfolioHistoricalRaw struct {
	Span                          string `json:"span"`
	Interval                      string `json:"interval"`
	OpenEquity                    string `json:"open_equity"`
	OpenTime                      string `json:"open_time"`
	AdjustedOpenEquity            string `json:"adjusted_open_equity"`
	TotalReturn                   string `json:"total_return"`
	EquityHistoricals []struct {
		BeginsAt              string `json:"begins_at"`
		OpenEquity            string `json:"open_equity"`
		CloseEquity           string `json:"close_equity"`
		OpenMarketValue       string `json:"open_market_value"`
		CloseMarketValue      string `json:"close_market_value"`
		AdjustedOpenEquity    string `json:"adjusted_open_equity"`
		AdjustedCloseEquity   string `json:"adjusted_close_equity"`
		NetReturn             string `json:"net_return"`
		Session               string `json:"session"`
	} `json:"equity_historicals"`
}

// GetPortfolioHistory returns the equity-over-time series for one account.
// span: day | week | month | 3month | year | 5year | all
// interval: 5minute | 10minute | hour | day | week
func (c *Client) GetPortfolioHistory(accountNumber, span, interval string) (*PortfolioHistory, error) {
	if accountNumber == "" {
		return nil, errMsg("account_number is required")
	}
	if span == "" {
		span = "year"
	}
	if interval == "" {
		// pick a sensible default per span
		switch span {
		case "day":
			interval = "5minute"
		case "week":
			interval = "10minute"
		case "month", "3month":
			interval = "hour"
		default:
			interval = "day"
		}
	}
	var raw portfolioHistoricalRaw
	err := c.GetJSON(URL("/portfolios/historicals/"+accountNumber+"/", map[string]string{
		"span":     span,
		"interval": interval,
		"bounds":   "regular",
	}), &raw)
	if err != nil {
		// Robinhood migrated this to a rosetta-backed endpoint that no
		// longer accepts span/interval via the public API and refuses
		// per-account calls on retirement accounts. Give the caller a
		// clear message instead of a naked 404 so they know it's not a
		// local bug.
		return nil, errMsg("account history endpoint has been removed by Robinhood " +
			"(was /portfolios/historicals/<account>/). Workaround: use " +
			"`rh account snapshot` for current equity and reconstruct " +
			"the curve from `rh transfers` + `rh activity` + `rh dividends`.")
	}

	points := make([]PortfolioPoint, 0, len(raw.EquityHistoricals))
	for _, h := range raw.EquityHistoricals {
		points = append(points, PortfolioPoint{
			BeginsAt:         h.BeginsAt,
			OpenEquity:       parseFloat(h.OpenEquity),
			CloseEquity:      parseFloat(h.CloseEquity),
			OpenMarketValue:  parseFloat(h.OpenMarketValue),
			CloseMarketValue: parseFloat(h.CloseMarketValue),
			AdjustedOpen:     parseFloat(h.AdjustedOpenEquity),
			AdjustedClose:    parseFloat(h.AdjustedCloseEquity),
			NetReturn:        parseFloat(h.NetReturn),
			Session:          h.Session,
		})
	}

	startEq := parseFloat(raw.AdjustedOpenEquity)
	if startEq == 0 {
		startEq = parseFloat(raw.OpenEquity)
	}
	endEq := 0.0
	if n := len(points); n > 0 {
		endEq = points[n-1].AdjustedClose
		if endEq == 0 {
			endEq = points[n-1].CloseEquity
		}
	}
	netChange := endEq - startEq
	pct := 0.0
	if startEq > 0 {
		pct = (endEq - startEq) / startEq * 100
	}

	return &PortfolioHistory{
		AccountNumber: accountNumber,
		Span:          span,
		Interval:      interval,
		StartEquity:   startEq,
		EndEquity:     endEq,
		NetChange:     netChange,
		PercentChange: pct,
		Count:         len(points),
		Points:        points,
	}, nil
}
