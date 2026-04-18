package client

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

type Holding struct {
	Symbol       string  `json:"symbol"`
	Shares       float64 `json:"shares"`
	CurrentPrice float64 `json:"current_price"`
	AvgCost      float64 `json:"avg_cost"`
	TotalReturn  float64 `json:"total_return"`
	TotalEquity  float64 `json:"total_equity"`
}

type AccountSummary struct {
	AccountNumber       string    `json:"account_number"`
	BrokerageAccountType string   `json:"brokerage_account_type,omitempty"`
	AccountType         string    `json:"account_type,omitempty"`
	Nickname            string    `json:"nickname,omitempty"`
	PortfolioValue      float64   `json:"portfolio_value"`
	Cash                float64   `json:"cash"`
	BuyingPower         float64   `json:"buying_power"`
	Holdings            []Holding `json:"holdings"`
}

type Profile struct {
	TotalPortfolio float64          `json:"total_portfolio"`
	TotalCash      float64          `json:"total_cash"`
	Accounts       []AccountSummary `json:"accounts"`
	Timestamp      string           `json:"timestamp"`
}

type HoldingsResult struct {
	AccountNumber string    `json:"account_number"`
	Count         int       `json:"count"`
	TotalEquity   float64   `json:"total_equity"`
	TotalReturn   float64   `json:"total_return"`
	Holdings      []Holding `json:"holdings"`
	Timestamp     string    `json:"timestamp"`
}

type accountRecord struct {
	AccountNumber        string `json:"account_number"`
	Type                 string `json:"type"` // "cash" / "margin"
	BrokerageAccountType string `json:"brokerage_account_type"`
	Nickname             string `json:"nickname"`
	PortfolioCash        string `json:"portfolio_cash"`
	Cash                 string `json:"cash"`
	BuyingPower          string `json:"buying_power"`
}

type accountsResp struct {
	Results []accountRecord `json:"results"`
	Next    string          `json:"next"`
}

type portfolioResp struct {
	Equity              string `json:"equity"`
	ExtendedHoursEquity string `json:"extended_hours_equity"`
}

type positionsResp struct {
	Results []positionRecord `json:"results"`
	Next    string           `json:"next"`
}

type positionRecord struct {
	AccountURL          string `json:"account"`
	Symbol              string `json:"symbol"`
	Quantity            string `json:"quantity"`
	AverageBuyPrice     string `json:"average_buy_price"`
	ClearingAverageCost string `json:"clearing_average_cost"`
	Instrument          string `json:"instrument"`
}

type quoteRecord struct {
	Symbol                      string `json:"symbol"`
	LastTradePrice              string `json:"last_trade_price"`
	LastExtendedHoursTradePrice string `json:"last_extended_hours_trade_price"`
	BidPrice                    string `json:"bid_price"`
	AskPrice                    string `json:"ask_price"`
	UpdatedAt                   string `json:"updated_at"`
}

type quotesResp struct {
	Results []*quoteRecord `json:"results"`
}

func (c *Client) GetProfile() (*Profile, error) {
	accounts, err := c.listAllAccounts()
	if err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, errMsg("no accounts returned")
	}

	// /positions/ defaults to the primary account only — must query per
	// account with `account_number=` (singular) to scope correctly.
	summaries := make([]AccountSummary, 0, len(accounts))
	allSymbols := map[string]struct{}{}
	posByAccount := map[string][]positionRecord{}

	for _, a := range accounts {
		positions, err := c.listAccountPositions(a.AccountNumber)
		if err != nil {
			return nil, err
		}
		for i, p := range positions {
			if p.Symbol == "" {
				if sym, err := c.symbolFromInstrument(p.Instrument); err == nil {
					positions[i].Symbol = sym
				}
			}
			if positions[i].Symbol != "" {
				allSymbols[positions[i].Symbol] = struct{}{}
			}
		}
		posByAccount[a.AccountNumber] = positions
	}

	// Single batched quotes call across every symbol in every account.
	quotesBySymbol, err := c.batchQuotes(symbolKeys(allSymbols))
	if err != nil {
		return nil, err
	}

	totalPortfolio := 0.0
	totalCash := 0.0

	for _, a := range accounts {
		var port portfolioResp
		if err := c.GetJSON(URL("/portfolios/"+a.AccountNumber+"/", nil), &port); err != nil {
			return nil, err
		}
		equity := parseFloat(port.Equity)
		if v := parseFloat(port.ExtendedHoursEquity); v > 0 {
			equity = v
		}

		holdings := make([]Holding, 0, len(posByAccount[a.AccountNumber]))
		for _, p := range posByAccount[a.AccountNumber] {
			qty := parseFloat(p.Quantity)
			if qty == 0 {
				continue
			}
			avg := parseFloat(p.AverageBuyPrice)
			if v := parseFloat(p.ClearingAverageCost); v > 0 {
				avg = v
			}
			current := 0.0
			if q := quotesBySymbol[p.Symbol]; q != nil {
				current = parseFloat(q.LastTradePrice)
				if v := parseFloat(q.LastExtendedHoursTradePrice); v > 0 {
					current = v
				}
			}
			holdings = append(holdings, Holding{
				Symbol:       p.Symbol,
				Shares:       qty,
				CurrentPrice: current,
				AvgCost:      avg,
				TotalReturn:  (current - avg) * qty,
				TotalEquity:  qty * current,
			})
		}

		cash := parseFloat(a.PortfolioCash)
		if cash == 0 {
			cash = parseFloat(a.Cash)
		}

		summaries = append(summaries, AccountSummary{
			AccountNumber:        a.AccountNumber,
			BrokerageAccountType: a.BrokerageAccountType,
			AccountType:          a.Type,
			Nickname:             a.Nickname,
			PortfolioValue:       equity,
			Cash:                 cash,
			BuyingPower:          parseFloat(a.BuyingPower),
			Holdings:             holdings,
		})
		totalPortfolio += equity
		totalCash += cash
	}

	return &Profile{
		TotalPortfolio: totalPortfolio,
		TotalCash:      totalCash,
		Accounts:       summaries,
		Timestamp:      time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

// GetAccountsSummary returns one entry per account with totals/cash but no
// holdings. Cheap call — useful as the entry point before drilling down into
// `GetHoldings(accountNumber)`.
func (c *Client) GetAccountsSummary() (*Profile, error) {
	accounts, err := c.listAllAccounts()
	if err != nil {
		return nil, err
	}
	summaries := make([]AccountSummary, 0, len(accounts))
	totalPortfolio := 0.0
	totalCash := 0.0

	for _, a := range accounts {
		var port portfolioResp
		if err := c.GetJSON(URL("/portfolios/"+a.AccountNumber+"/", nil), &port); err != nil {
			return nil, err
		}
		equity := parseFloat(port.Equity)
		if v := parseFloat(port.ExtendedHoursEquity); v > 0 {
			equity = v
		}
		cash := parseFloat(a.PortfolioCash)
		if cash == 0 {
			cash = parseFloat(a.Cash)
		}
		summaries = append(summaries, AccountSummary{
			AccountNumber:        a.AccountNumber,
			BrokerageAccountType: a.BrokerageAccountType,
			AccountType:          a.Type,
			Nickname:             a.Nickname,
			PortfolioValue:       equity,
			Cash:                 cash,
			BuyingPower:          parseFloat(a.BuyingPower),
		})
		totalPortfolio += equity
		totalCash += cash
	}

	return &Profile{
		TotalPortfolio: totalPortfolio,
		TotalCash:      totalCash,
		Accounts:       summaries,
		Timestamp:      time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

// GetHoldings returns the holdings for a single account, sorted by
// total_equity descending so the biggest positions are first.
func (c *Client) GetHoldings(accountNumber string) (*HoldingsResult, error) {
	if accountNumber == "" {
		return nil, errMsg("account_number is required")
	}
	positions, err := c.listAccountPositions(accountNumber)
	if err != nil {
		return nil, err
	}
	symbolSet := map[string]struct{}{}
	for i, p := range positions {
		if p.Symbol == "" {
			if sym, err := c.symbolFromInstrument(p.Instrument); err == nil {
				positions[i].Symbol = sym
			}
		}
		if positions[i].Symbol != "" {
			symbolSet[positions[i].Symbol] = struct{}{}
		}
	}

	quotes, err := c.batchQuotes(symbolKeys(symbolSet))
	if err != nil {
		return nil, err
	}

	holdings := make([]Holding, 0, len(positions))
	totalEquity := 0.0
	totalReturn := 0.0
	for _, p := range positions {
		qty := parseFloat(p.Quantity)
		if qty == 0 {
			continue
		}
		avg := parseFloat(p.AverageBuyPrice)
		if v := parseFloat(p.ClearingAverageCost); v > 0 {
			avg = v
		}
		current := 0.0
		if q := quotes[p.Symbol]; q != nil {
			current = parseFloat(q.LastTradePrice)
			if v := parseFloat(q.LastExtendedHoursTradePrice); v > 0 {
				current = v
			}
		}
		ret := (current - avg) * qty
		eq := qty * current
		holdings = append(holdings, Holding{
			Symbol:       p.Symbol,
			Shares:       qty,
			CurrentPrice: current,
			AvgCost:      avg,
			TotalReturn:  ret,
			TotalEquity:  eq,
		})
		totalEquity += eq
		totalReturn += ret
	}

	sort.Slice(holdings, func(i, j int) bool {
		return holdings[i].TotalEquity > holdings[j].TotalEquity
	})

	return &HoldingsResult{
		AccountNumber: accountNumber,
		Count:         len(holdings),
		TotalEquity:   totalEquity,
		TotalReturn:   totalReturn,
		Holdings:      holdings,
		Timestamp:     time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

func (c *Client) listAllAccounts() ([]accountRecord, error) {
	var out []accountRecord
	url := URL("/accounts/", map[string]string{"default_to_all_accounts": "true"})
	for url != "" {
		var page accountsResp
		if err := c.GetJSON(url, &page); err != nil {
			return nil, err
		}
		out = append(out, page.Results...)
		url = page.Next
	}
	return out, nil
}

func (c *Client) listAccountPositions(accountNumber string) ([]positionRecord, error) {
	var out []positionRecord
	url := URL("/positions/", map[string]string{
		"account_number": accountNumber,
		"nonzero":        "true",
	})
	for url != "" {
		var page positionsResp
		if err := c.GetJSON(url, &page); err != nil {
			return nil, err
		}
		out = append(out, page.Results...)
		url = page.Next
	}
	return out, nil
}

// batchQuotes batches symbols into ≤50-symbol chunks because Robinhood's
// quotes endpoint silently truncates very long query strings.
func (c *Client) batchQuotes(symbols []string) (map[string]*quoteRecord, error) {
	out := map[string]*quoteRecord{}
	if len(symbols) == 0 {
		return out, nil
	}
	const batch = 50
	for start := 0; start < len(symbols); start += batch {
		end := start + batch
		if end > len(symbols) {
			end = len(symbols)
		}
		var qr quotesResp
		if err := c.GetJSON(URL("/marketdata/quotes/", map[string]string{
			"symbols": strings.Join(symbols[start:end], ","),
		}), &qr); err != nil {
			return nil, err
		}
		for _, q := range qr.Results {
			if q != nil {
				out[q.Symbol] = q
			}
		}
	}
	return out, nil
}

func symbolKeys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	return out
}

// accountNumberFromURL turns ".../accounts/12345678/" into "12345678".
func accountNumberFromURL(u string) string {
	u = strings.TrimSuffix(u, "/")
	if i := strings.LastIndex(u, "/"); i >= 0 {
		return u[i+1:]
	}
	return u
}

func (c *Client) symbolFromInstrument(instrumentURL string) (string, error) {
	if instrumentURL == "" {
		return "", errMsg("empty instrument url")
	}
	var inst struct {
		Symbol string `json:"symbol"`
	}
	if err := c.GetJSON(instrumentURL, &inst); err != nil {
		return "", err
	}
	return inst.Symbol, nil
}

func parseFloat(s string) float64 {
	if s == "" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

type stringErr string

func (s stringErr) Error() string { return string(s) }

func errMsg(s string) error { return stringErr(s) }
