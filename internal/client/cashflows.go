package client

import (
	"strings"
	"time"
)

type Dividend struct {
	ID            string  `json:"id"`
	Symbol        string  `json:"symbol"`
	Amount        float64 `json:"amount"`
	Rate          float64 `json:"rate"`
	Position      float64 `json:"position"`
	State         string  `json:"state"`
	PayableDate   string  `json:"payable_date"`
	PaidAt        string  `json:"paid_at"`
	WithheldTax   float64 `json:"withholding"`
}

type DividendsResult struct {
	Count        int        `json:"count"`
	TotalPaid    float64    `json:"total_paid"`
	TotalPending float64    `json:"total_pending"`
	Dividends    []Dividend `json:"dividends"`
}

type dividendRaw struct {
	ID            string `json:"id"`
	Amount        string `json:"amount"`
	Rate          string `json:"rate"`
	Position      string `json:"position"`
	Withholding   string `json:"withholding"`
	State         string `json:"state"`
	PayableDate   string `json:"payable_date"`
	PaidAt        string `json:"paid_at"`
	Instrument    string `json:"instrument"`
}

type dividendsResp struct {
	Results []dividendRaw `json:"results"`
	Next    string        `json:"next"`
}

func (c *Client) GetDividends(since string, limit int) (*DividendsResult, error) {
	if limit <= 0 {
		limit = 200
	}
	var sinceTime time.Time
	if since != "" {
		t, err := time.Parse("2006-01-02", since)
		if err != nil {
			return nil, errMsg("invalid --since")
		}
		sinceTime = t
	}

	raws, err := c.pageDividends(limit, sinceTime)
	if err != nil {
		return nil, err
	}

	// Resolve symbols.
	idMap := map[string]string{} // url → id
	for _, r := range raws {
		if r.Instrument != "" {
			idMap[r.Instrument] = lastPathSegment(strings.TrimSuffix(r.Instrument, "/"))
		}
	}
	symByURL := map[string]string{}
	if len(idMap) > 0 {
		ids := make([]string, 0, len(idMap))
		urlByID := map[string]string{}
		for u, id := range idMap {
			ids = append(ids, id)
			urlByID[id] = u
		}
		const batch = 75
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
				if u, ok := urlByID[inst.ID]; ok {
					symByURL[u] = inst.Symbol
				}
			}
		}
	}

	out := make([]Dividend, 0, len(raws))
	totalPaid := 0.0
	totalPending := 0.0
	for _, r := range raws {
		amt := parseFloat(r.Amount)
		d := Dividend{
			ID:          r.ID,
			Symbol:      symByURL[r.Instrument],
			Amount:      amt,
			Rate:        parseFloat(r.Rate),
			Position:    parseFloat(r.Position),
			State:       r.State,
			PayableDate: r.PayableDate,
			PaidAt:      r.PaidAt,
			WithheldTax: parseFloat(r.Withholding),
		}
		out = append(out, d)
		if r.State == "paid" {
			totalPaid += amt
		} else if r.State == "pending" {
			totalPending += amt
		}
	}
	return &DividendsResult{
		Count:        len(out),
		TotalPaid:    totalPaid,
		TotalPending: totalPending,
		Dividends:    out,
	}, nil
}

func (c *Client) pageDividends(maxRows int, since time.Time) ([]dividendRaw, error) {
	// Robinhood's /dividends/ silently defaults to the primary account.
	// Fan out across every account the user owns so IRA dividends are
	// included.
	accounts, err := c.ListAccountNumbers()
	if err != nil || len(accounts) == 0 {
		return c.pageDividendsSingle(maxRows, since, "")
	}
	var combined []dividendRaw
	for _, acct := range accounts {
		rows, err := c.pageDividendsSingle(maxRows, since, acct)
		if err != nil {
			return nil, err
		}
		combined = append(combined, rows...)
		if len(combined) >= maxRows {
			break
		}
	}
	return combined, nil
}

func (c *Client) pageDividendsSingle(maxRows int, since time.Time, account string) ([]dividendRaw, error) {
	out := []dividendRaw{}
	q := map[string]string{}
	if account != "" {
		q["account_numbers"] = account
	}
	u := URL("/dividends/", q)
	for u != "" && len(out) < maxRows {
		var page dividendsResp
		if err := c.GetJSON(u, &page); err != nil {
			return nil, err
		}
		stop := false
		for _, r := range page.Results {
			if !since.IsZero() {
				ref := r.PaidAt
				if ref == "" {
					ref = r.PayableDate
				}
				if t, err := time.Parse(time.RFC3339Nano, ref); err == nil {
					if t.Before(since) {
						stop = true
						break
					}
				} else if t, err := time.Parse("2006-01-02", ref); err == nil && t.Before(since) {
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
		u = page.Next
	}
	return out, nil
}

// --- ACH transfers ---

type Transfer struct {
	ID            string  `json:"id"`
	Direction     string  `json:"direction"` // deposit | withdraw
	Amount        float64 `json:"amount"`
	Fee           float64 `json:"fee"`
	State         string  `json:"state"`
	Scheduled     bool    `json:"scheduled"`
	ExpectedDate  string  `json:"expected_landing_date"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

type TransfersResult struct {
	Count          int        `json:"count"`
	NetDeposited   float64    `json:"net_deposited"`
	TotalDeposit   float64    `json:"total_deposit"`
	TotalWithdraw  float64    `json:"total_withdraw"`
	Transfers      []Transfer `json:"transfers"`
}

type transferRaw struct {
	ID                  string `json:"id"`
	Direction           string `json:"direction"`
	Amount              string `json:"amount"`
	Fees                string `json:"fees"`
	State               string `json:"state"`
	Scheduled           bool   `json:"scheduled"`
	ExpectedLandingDate string `json:"expected_landing_date"`
	CreatedAt           string `json:"created_at"`
	UpdatedAt           string `json:"updated_at"`
}

type transfersResp struct {
	Results []transferRaw `json:"results"`
	Next    string        `json:"next"`
}

func (c *Client) GetTransfers(since string, limit int) (*TransfersResult, error) {
	if limit <= 0 {
		limit = 200
	}
	var sinceTime time.Time
	if since != "" {
		t, err := time.Parse("2006-01-02", since)
		if err != nil {
			return nil, errMsg("invalid --since")
		}
		sinceTime = t
	}

	out := []Transfer{}
	totalDeposit := 0.0
	totalWithdraw := 0.0
	u := URL("/ach/transfers/", nil)
	for u != "" && len(out) < limit {
		var page transfersResp
		if err := c.GetJSON(u, &page); err != nil {
			return nil, err
		}
		stop := false
		for _, r := range page.Results {
			if !sinceTime.IsZero() {
				if t, err := time.Parse(time.RFC3339Nano, r.CreatedAt); err == nil && t.Before(sinceTime) {
					stop = true
					break
				}
			}
			amt := parseFloat(r.Amount)
			tr := Transfer{
				ID:           r.ID,
				Direction:    r.Direction,
				Amount:       amt,
				Fee:          parseFloat(r.Fees),
				State:        r.State,
				Scheduled:    r.Scheduled,
				ExpectedDate: r.ExpectedLandingDate,
				CreatedAt:    r.CreatedAt,
				UpdatedAt:    r.UpdatedAt,
			}
			if r.State == "completed" {
				if r.Direction == "deposit" {
					totalDeposit += amt
				} else if r.Direction == "withdraw" {
					totalWithdraw += amt
				}
			}
			out = append(out, tr)
			if len(out) >= limit {
				break
			}
		}
		if stop {
			break
		}
		u = page.Next
	}
	return &TransfersResult{
		Count:         len(out),
		NetDeposited:  totalDeposit - totalWithdraw,
		TotalDeposit:  totalDeposit,
		TotalWithdraw: totalWithdraw,
		Transfers:     out,
	}, nil
}

// --- recurring investments ---

type RecurringConfig struct {
	ID             string  `json:"id"`
	Symbol         string  `json:"symbol,omitempty"`
	Asset          string  `json:"asset_type,omitempty"`
	State          string  `json:"state"`
	Frequency      string  `json:"frequency"`
	Amount         float64 `json:"amount"`
	NextRunDate    string  `json:"next_run_date"`
	SourceOfFunds  string  `json:"source_of_funds,omitempty"`
}

type RecurringResult struct {
	Count   int               `json:"count"`
	Configs []RecurringConfig `json:"configurations"`
}

type recurringRaw struct {
	Results []struct {
		ID         string `json:"id"`
		AssetType  string `json:"asset_type"`
		State      string `json:"state"`
		Frequency  string `json:"frequency"`
		NextRunDate string `json:"next_run_date"`
		SourceOfFunds struct {
			ID string `json:"source_of_funds_id"`
			Type string `json:"source_of_funds_type"`
		} `json:"source_of_funds"`
		AmountInfo struct {
			Amount string `json:"amount"`
			Currency string `json:"currency_code"`
		} `json:"amount"`
		InvestmentSchedule struct {
			Symbol string `json:"symbol"`
		} `json:"investment_schedule"`
		InvestmentTarget struct {
			Symbol string `json:"symbol"`
		} `json:"investment_target"`
	} `json:"results"`
	Next string `json:"next"`
}

func (c *Client) GetRecurring() (*RecurringResult, error) {
	out := []RecurringConfig{}
	u := URL("/recurring/configurations/", nil)
	for u != "" {
		var page recurringRaw
		if err := c.GetJSON(u, &page); err != nil {
			return nil, errMsg("recurring investments endpoint has been removed by " +
				"Robinhood (was /recurring/configurations/). DCA configs are " +
				"now visible in-app only; no public replacement has been " +
				"discovered. Check the Robinhood app under Account → " +
				"Recurring Investments.")
		}
		for _, r := range page.Results {
			sym := r.InvestmentTarget.Symbol
			if sym == "" {
				sym = r.InvestmentSchedule.Symbol
			}
			out = append(out, RecurringConfig{
				ID:            r.ID,
				Symbol:        sym,
				Asset:         r.AssetType,
				State:         r.State,
				Frequency:     r.Frequency,
				Amount:        parseFloat(r.AmountInfo.Amount),
				NextRunDate:   r.NextRunDate,
				SourceOfFunds: r.SourceOfFunds.Type,
			})
		}
		u = page.Next
	}
	return &RecurringResult{Count: len(out), Configs: out}, nil
}
