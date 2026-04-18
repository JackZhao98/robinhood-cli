package client

import (
	"strings"
	"time"
)

type Bar struct {
	Time   string  `json:"time"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume int64   `json:"volume"`
}

type HistoricalResult struct {
	Symbol   string `json:"symbol"`
	Data     []Bar  `json:"data"`
	Count    int    `json:"count"`
	Interval string `json:"interval"`
	Span     string `json:"span"`
	Error    string `json:"error,omitempty"`
}

type historicalRaw struct {
	Historicals []struct {
		BeginsAt   string `json:"begins_at"`
		OpenPrice  string `json:"open_price"`
		HighPrice  string `json:"high_price"`
		LowPrice   string `json:"low_price"`
		ClosePrice string `json:"close_price"`
		Volume     int64  `json:"volume"`
	} `json:"historicals"`
}

func (c *Client) GetHistorical(symbol, beginDate, endDate, interval string) (*HistoricalResult, error) {
	symbol = strings.ToUpper(symbol)

	begin, err := time.Parse("2006-01-02", beginDate)
	if err != nil {
		return nil, errMsg("invalid begin_date, expected YYYY-MM-DD")
	}
	if _, err := time.Parse("2006-01-02", endDate); err != nil {
		return nil, errMsg("invalid end_date, expected YYYY-MM-DD")
	}
	daysDiff := int(time.Since(begin).Hours() / 24)

	// Coerce interval to something Robinhood will return for the requested span.
	switch {
	case daysDiff > 30 && (interval == "5minute" || interval == "10minute" || interval == "hour"):
		interval = "day"
	case daysDiff > 7 && (interval == "5minute" || interval == "10minute"):
		interval = "hour"
	}

	span := chooseSpan(daysDiff)

	var raw historicalRaw
	if err := c.GetJSON(URL("/marketdata/historicals/"+symbol+"/", map[string]string{
		"interval": interval,
		"span":     span,
		"bounds":   "regular",
	}), &raw); err != nil {
		return nil, err
	}

	bars := make([]Bar, 0, len(raw.Historicals))
	for _, h := range raw.Historicals {
		dayPart := h.BeginsAt
		if len(dayPart) >= 10 {
			dayPart = h.BeginsAt[:10]
		}
		if dayPart < beginDate || dayPart > endDate {
			continue
		}
		t := h.BeginsAt
		if len(t) >= 16 {
			t = strings.Replace(t[:16], "T", " ", 1)
		}
		bars = append(bars, Bar{
			Time:   t,
			Open:   parseFloat(h.OpenPrice),
			High:   parseFloat(h.HighPrice),
			Low:    parseFloat(h.LowPrice),
			Close:  parseFloat(h.ClosePrice),
			Volume: h.Volume,
		})
	}

	res := &HistoricalResult{
		Symbol:   symbol,
		Data:     bars,
		Count:    len(bars),
		Interval: interval,
		Span:     span,
	}
	if len(bars) == 0 {
		res.Error = "no data in requested date range"
	}
	return res, nil
}

func chooseSpan(daysDiff int) string {
	switch {
	case daysDiff <= 1:
		return "day"
	case daysDiff <= 7:
		return "week"
	case daysDiff <= 31:
		return "month"
	case daysDiff <= 90:
		return "3month"
	case daysDiff <= 365:
		return "year"
	default:
		return "5year"
	}
}
