package client

import (
	"strconv"
	"strings"
)

// --- documents (1099 etc) ---

type Document struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Date    string `json:"date"`
	URL     string `json:"download_url"`
	Account string `json:"account,omitempty"`
}

type DocumentsResult struct {
	Count     int        `json:"count"`
	Documents []Document `json:"documents"`
}

type docsRaw struct {
	Results []struct {
		ID          string `json:"id"`
		Type        string `json:"type"`
		// API field is `date` (YYYY-MM-DD), not `display_date`.
		Date        string `json:"date"`
		DownloadURL string `json:"download_url"`
		URL         string `json:"url"`
		Account     string `json:"account"`
	} `json:"results"`
	Next string `json:"next"`
}

// GetDocuments lists account documents. `taxYear` filters to documents
// dated within that calendar year. The `tax_year` query param is still
// sent for forward compatibility, but the upstream API currently ignores
// it — so the filter is applied client-side against each document's
// `date` field (the actual RH field name; `display_date` does not exist).
func (c *Client) GetDocuments(taxYear int) (*DocumentsResult, error) {
	out := []Document{}
	q := map[string]string{}
	if taxYear > 0 {
		q["tax_year"] = strconv.Itoa(taxYear)
	}
	u := URL("/documents/", q)
	for u != "" {
		var page docsRaw
		if err := c.GetJSON(u, &page); err != nil {
			return nil, err
		}
		for _, r := range page.Results {
			if taxYear > 0 && !strings.HasPrefix(r.Date, strconv.Itoa(taxYear)) {
				continue
			}
			dl := r.DownloadURL
			if dl == "" {
				dl = r.URL
			}
			out = append(out, Document{
				ID:      r.ID,
				Type:    r.Type,
				Date:    r.Date,
				URL:     dl,
				Account: r.Account,
			})
		}
		u = page.Next
		if len(out) >= 500 {
			break
		}
	}
	return &DocumentsResult{Count: len(out), Documents: out}, nil
}

// --- gold subscription ---

type GoldStatus struct {
	IsSubscribed bool    `json:"is_subscribed"`
	Tier         string  `json:"tier,omitempty"`
	State        string  `json:"state,omitempty"`
	StartedAt    string  `json:"started_at,omitempty"`
	NextBillDate string  `json:"next_bill_date,omitempty"`
	MonthlyCost  float64 `json:"monthly_cost,omitempty"`
}

type goldRaw struct {
	Results []struct {
		ID   string `json:"id"`
		Plan struct {
			Type        string `json:"type"`          // e.g. "24_karat"
			Name        string `json:"name"`          // e.g. "cc_annual_24_karat"
			MonthlyCost string `json:"monthly_cost"`  // e.g. "4.17"
		} `json:"plan"`
		// Robinhood exposes "status" (not "state") on the subscription.
		Status       string `json:"status"`
		CreatedAt    string `json:"created_at"`
		EndedAt      *string `json:"ended_at"`
		RenewalDate  string `json:"renewal_date"`
	} `json:"results"`
}

func (c *Client) GetGold() (*GoldStatus, error) {
	var raw goldRaw
	if err := c.GetJSON(URL("/subscription/subscriptions/", map[string]string{"active": "true"}), &raw); err != nil {
		return nil, err
	}
	if len(raw.Results) == 0 {
		return &GoldStatus{IsSubscribed: false}, nil
	}
	r := raw.Results[0]
	// A subscription is active when the server didn't set an end date.
	// `status` values observed: "created", "active". Neither implies a
	// cancelled sub on its own, so `ended_at == null` is the reliable
	// signal.
	active := r.EndedAt == nil || *r.EndedAt == ""
	return &GoldStatus{
		IsSubscribed: active,
		Tier:         r.Plan.Type,
		State:        r.Status,
		StartedAt:    r.CreatedAt,
		NextBillDate: r.RenewalDate,
		MonthlyCost:  parseFloat(r.Plan.MonthlyCost),
	}, nil
}

// --- margin / day-trade / PDT ---

type MarginInfo struct {
	AccountNumber       string  `json:"account_number"`
	MarginEquity        float64 `json:"margin_equity,omitempty"`
	OvernightBuyingPower float64 `json:"overnight_buying_power,omitempty"`
	DayTradeBuyingPower float64 `json:"day_trade_buying_power,omitempty"`
	UnclearedDeposits   float64 `json:"uncleared_deposits,omitempty"`
	OutstandingInterest float64 `json:"outstanding_interest,omitempty"`
}

type MarginResult struct {
	Count   int          `json:"count"`
	Margins []MarginInfo `json:"margins"`
}

type accountWithMarginRaw struct {
	Results []struct {
		AccountNumber  string `json:"account_number"`
		MarginBalances struct {
			MarginEquity         string `json:"margin_equity"`
			OvernightBuyingPower string `json:"overnight_buying_power"`
			DayTradeBuyingPower  string `json:"day_trade_buying_power"`
			UnclearedDeposits    string `json:"uncleared_deposits"`
			OutstandingInterest  string `json:"outstanding_interest"`
		} `json:"margin_balances"`
	} `json:"results"`
	Next string `json:"next"`
}

func (c *Client) GetMargin() (*MarginResult, error) {
	out := []MarginInfo{}
	u := URL("/accounts/", map[string]string{"default_to_all_accounts": "true"})
	for u != "" {
		var page accountWithMarginRaw
		if err := c.GetJSON(u, &page); err != nil {
			return nil, err
		}
		for _, a := range page.Results {
			info := MarginInfo{
				AccountNumber:        a.AccountNumber,
				MarginEquity:         parseFloat(a.MarginBalances.MarginEquity),
				OvernightBuyingPower: parseFloat(a.MarginBalances.OvernightBuyingPower),
				DayTradeBuyingPower:  parseFloat(a.MarginBalances.DayTradeBuyingPower),
				UnclearedDeposits:    parseFloat(a.MarginBalances.UnclearedDeposits),
				OutstandingInterest:  parseFloat(a.MarginBalances.OutstandingInterest),
			}
			out = append(out, info)
		}
		u = page.Next
	}
	return &MarginResult{Count: len(out), Margins: out}, nil
}

type PDTStatus struct {
	AccountNumber           string  `json:"account_number"`
	IsPatternDayTrader      bool    `json:"is_pattern_day_trader"`
	DayTradeCount           int     `json:"day_trade_count"`
	DayTradesProtection     bool    `json:"day_trades_protection"`
	DayTradeBuyingPower     float64 `json:"day_trade_buying_power"`
}

type PDTResult struct {
	Count   int         `json:"count"`
	Statuses []PDTStatus `json:"statuses"`
}

type accountPDTRaw struct {
	Results []struct {
		AccountNumber       string `json:"account_number"`
		IsPinned            bool   `json:"is_pinned"`
		IsPDT               bool   `json:"is_pattern_day_trader"`
		DayTradesProtection bool   `json:"day_trades_protection"`
		MarginBalances      struct {
			DayTradeBuyingPower string `json:"day_trade_buying_power"`
			DayTradeCount       int    `json:"day_trade_count"`
		} `json:"margin_balances"`
	} `json:"results"`
	Next string `json:"next"`
}

func (c *Client) GetPDT() (*PDTResult, error) {
	out := []PDTStatus{}
	u := URL("/accounts/", map[string]string{"default_to_all_accounts": "true"})
	for u != "" {
		var page accountPDTRaw
		if err := c.GetJSON(u, &page); err != nil {
			return nil, err
		}
		for _, a := range page.Results {
			out = append(out, PDTStatus{
				AccountNumber:       a.AccountNumber,
				IsPatternDayTrader:  a.IsPDT,
				DayTradeCount:       a.MarginBalances.DayTradeCount,
				DayTradesProtection: a.DayTradesProtection,
				DayTradeBuyingPower: parseFloat(a.MarginBalances.DayTradeBuyingPower),
			})
		}
		u = page.Next
	}
	return &PDTResult{Count: len(out), Statuses: out}, nil
}

// --- notifications ---

type Notification struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
	ReadAt    string `json:"read_at,omitempty"`
}

type NotificationsResult struct {
	Count         int            `json:"count"`
	Notifications []Notification `json:"notifications"`
}

type notificationsRaw struct {
	Results []struct {
		ID        string `json:"id"`
		Type      string `json:"type"`
		Title     string `json:"title"`
		Message   string `json:"message"`
		CreatedAt string `json:"time"`
		ReadAt    string `json:"read_time"`
	} `json:"results"`
	Next string `json:"next"`
}

func (c *Client) GetNotifications(limit int) (*NotificationsResult, error) {
	if limit <= 0 {
		limit = 50
	}
	out := []Notification{}
	u := URL("/notifications/devices/", nil)
	// Try the inbox endpoint first; fall back if not available.
	if err := c.GetJSON(URL("/notifications/", nil), &struct{}{}); err == nil {
		u = URL("/notifications/", nil)
	}
	for u != "" && len(out) < limit {
		var page notificationsRaw
		if err := c.GetJSON(u, &page); err != nil {
			return &NotificationsResult{Count: len(out), Notifications: out}, nil // best-effort
		}
		for _, r := range page.Results {
			out = append(out, Notification{
				ID:        r.ID,
				Type:      r.Type,
				Title:     r.Title,
				Message:   r.Message,
				CreatedAt: r.CreatedAt,
				ReadAt:    r.ReadAt,
			})
			if len(out) >= limit {
				break
			}
		}
		u = page.Next
	}
	return &NotificationsResult{Count: len(out), Notifications: out}, nil
}

// --- single order detail ---

type OrderDetail struct {
	ID                 string                  `json:"id"`
	AssetClass         string                  `json:"asset_class"`
	Symbol             string                  `json:"symbol"`
	Side               string                  `json:"side"`
	State              string                  `json:"state"`
	Type               string                  `json:"order_type"`
	Quantity           float64                 `json:"quantity"`
	CumulativeQuantity float64                 `json:"cumulative_quantity"`
	AveragePrice       float64                 `json:"average_price"`
	CreatedAt          string                  `json:"created_at"`
	UpdatedAt          string                  `json:"updated_at"`
	Executions         []OrderExecution        `json:"executions"`
}

type OrderExecution struct {
	Quantity      float64 `json:"quantity"`
	Price         float64 `json:"price"`
	Timestamp     string  `json:"timestamp"`
	SettlementDate string `json:"settlement_date,omitempty"`
}

type orderDetailRaw struct {
	ID                 string `json:"id"`
	Side               string `json:"side"`
	State              string `json:"state"`
	Type               string `json:"type"`
	Quantity           string `json:"quantity"`
	CumulativeQuantity string `json:"cumulative_quantity"`
	AveragePrice       string `json:"average_price"`
	Instrument         string `json:"instrument"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
	Executions         []struct {
		Quantity       string `json:"quantity"`
		Price          string `json:"price"`
		Timestamp      string `json:"timestamp"`
		SettlementDate string `json:"settlement_date"`
	} `json:"executions"`
}

func (c *Client) GetOrder(id string) (*OrderDetail, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errMsg("order id is required")
	}
	var raw orderDetailRaw
	if err := c.GetJSON(URL("/orders/"+id+"/", nil), &raw); err != nil {
		return nil, err
	}
	sym := ""
	if raw.Instrument != "" {
		sym, _ = c.symbolFromInstrument(raw.Instrument)
	}
	exes := make([]OrderExecution, 0, len(raw.Executions))
	for _, e := range raw.Executions {
		exes = append(exes, OrderExecution{
			Quantity:       parseFloat(e.Quantity),
			Price:          parseFloat(e.Price),
			Timestamp:      e.Timestamp,
			SettlementDate: e.SettlementDate,
		})
	}
	return &OrderDetail{
		ID:                 raw.ID,
		AssetClass:         "equity",
		Symbol:             sym,
		Side:               raw.Side,
		State:              raw.State,
		Type:               raw.Type,
		Quantity:           parseFloat(raw.Quantity),
		CumulativeQuantity: parseFloat(raw.CumulativeQuantity),
		AveragePrice:       parseFloat(raw.AveragePrice),
		CreatedAt:          raw.CreatedAt,
		UpdatedAt:          raw.UpdatedAt,
		Executions:         exes,
	}, nil
}

// --- option historicals ---

type OptionHistoryBar struct {
	BeginsAt string  `json:"begins_at"`
	Open     float64 `json:"open"`
	High     float64 `json:"high"`
	Low      float64 `json:"low"`
	Close    float64 `json:"close"`
	Volume   int     `json:"volume"`
}

type OptionHistoryResult struct {
	InstrumentID string             `json:"instrument_id"`
	Span         string             `json:"span"`
	Interval     string             `json:"interval"`
	Count        int                `json:"count"`
	Bars         []OptionHistoryBar `json:"bars"`
}

type optionHistoricalRaw struct {
	DataPoints []struct {
		BeginsAt   string `json:"begins_at"`
		OpenPrice  string `json:"open_price"`
		HighPrice  string `json:"high_price"`
		LowPrice   string `json:"low_price"`
		ClosePrice string `json:"close_price"`
		Volume     int    `json:"volume"`
	} `json:"data_points"`
}

func (c *Client) GetOptionHistory(instrumentID, span, interval string) (*OptionHistoryResult, error) {
	if instrumentID == "" {
		return nil, errMsg("instrument_id is required")
	}
	if span == "" {
		span = "week"
	}
	if interval == "" {
		switch span {
		case "day":
			interval = "5minute"
		case "week":
			interval = "hour"
		default:
			interval = "day"
		}
	}
	var raw optionHistoricalRaw
	if err := c.GetJSON(URL("/marketdata/options/historicals/"+instrumentID+"/", map[string]string{
		"span":     span,
		"interval": interval,
	}), &raw); err != nil {
		return nil, err
	}
	bars := make([]OptionHistoryBar, 0, len(raw.DataPoints))
	for _, p := range raw.DataPoints {
		bars = append(bars, OptionHistoryBar{
			BeginsAt: p.BeginsAt,
			Open:     parseFloat(p.OpenPrice),
			High:     parseFloat(p.HighPrice),
			Low:      parseFloat(p.LowPrice),
			Close:    parseFloat(p.ClosePrice),
			Volume:   p.Volume,
		})
	}
	return &OptionHistoryResult{
		InstrumentID: instrumentID,
		Span:         span,
		Interval:     interval,
		Count:        len(bars),
		Bars:         bars,
	}, nil
}
