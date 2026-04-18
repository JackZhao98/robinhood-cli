package client

import (
	"strings"
)

type Quote struct {
	Symbol             string   `json:"symbol"`
	LastPrice          *float64 `json:"last_price"`
	ExtendedHoursPrice *float64 `json:"extended_hours_price"`
	Bid                *float64 `json:"bid"`
	Ask                *float64 `json:"ask"`
	Is247Eligible      bool     `json:"is_24_7_eligible"`
	MarketCap          *float64 `json:"market_cap"`
	PERatio            *float64 `json:"pe_ratio"`
	DividendYield      *float64 `json:"dividend_yield"`
	Volume             *float64 `json:"volume"`
	AverageVolume      *float64 `json:"average_volume"`
	High               *float64 `json:"high"`
	Low                *float64 `json:"low"`
	Open               *float64 `json:"open"`
	High52Weeks        *float64 `json:"high_52_weeks"`
	Low52Weeks         *float64 `json:"low_52_weeks"`
	UpdatedAt          string   `json:"updated_at"`
}

type instrumentLite struct {
	ID     string `json:"id"`
	Symbol string `json:"symbol"`
}

type instrumentsResp struct {
	Results []instrumentLite `json:"results"`
}

type fullQuote struct {
	Symbol                       string `json:"symbol"`
	LastTradePrice               string `json:"last_trade_price"`
	LastExtendedHoursTradePrice  string `json:"last_extended_hours_trade_price"`
	BidPrice                     string `json:"bid_price"`
	AskPrice                     string `json:"ask_price"`
	Is247Eligible                bool   `json:"is_24_7_eligible"`
	UpdatedAt                    string `json:"updated_at"`
}

type fundamentals struct {
	Results []struct {
		MarketCap     string `json:"market_cap"`
		PERatio       string `json:"pe_ratio"`
		DividendYield string `json:"dividend_yield"`
		Volume        string `json:"volume"`
		AverageVolume string `json:"average_volume"`
		High          string `json:"high"`
		Low           string `json:"low"`
		Open          string `json:"open"`
		High52Weeks   string `json:"high_52_weeks"`
		Low52Weeks    string `json:"low_52_weeks"`
	} `json:"results"`
}

func (c *Client) GetQuote(symbol string) (*Quote, error) {
	symbol = strings.ToUpper(symbol)

	var instr instrumentsResp
	if err := c.GetJSON(URL("/instruments/", map[string]string{"symbols": symbol}), &instr); err != nil {
		return nil, err
	}
	if len(instr.Results) == 0 {
		return nil, errMsg("symbol not found: " + symbol)
	}
	id := instr.Results[0].ID

	var q struct {
		Results []fullQuote `json:"results"`
	}
	if err := c.GetJSON(URL("/marketdata/quotes/", map[string]string{
		"bounds": "24_5",
		"ids":    id,
	}), &q); err != nil {
		return nil, err
	}
	if len(q.Results) == 0 {
		return nil, errMsg("no quote returned")
	}
	r := q.Results[0]

	var f fundamentals
	if err := c.GetJSON(URL("/fundamentals/", map[string]string{"symbols": symbol}), &f); err != nil {
		return nil, err
	}
	var fr struct {
		MarketCap, PERatio, DividendYield, Volume, AverageVolume, High, Low, Open, High52Weeks, Low52Weeks string
	}
	if len(f.Results) > 0 {
		fr = struct {
			MarketCap, PERatio, DividendYield, Volume, AverageVolume, High, Low, Open, High52Weeks, Low52Weeks string
		}{
			f.Results[0].MarketCap, f.Results[0].PERatio, f.Results[0].DividendYield,
			f.Results[0].Volume, f.Results[0].AverageVolume,
			f.Results[0].High, f.Results[0].Low, f.Results[0].Open,
			f.Results[0].High52Weeks, f.Results[0].Low52Weeks,
		}
	}

	return &Quote{
		Symbol:             symbol,
		LastPrice:          optFloat(r.LastTradePrice),
		ExtendedHoursPrice: optFloat(r.LastExtendedHoursTradePrice),
		Bid:                optFloat(r.BidPrice),
		Ask:                optFloat(r.AskPrice),
		Is247Eligible:      r.Is247Eligible,
		MarketCap:          optFloat(fr.MarketCap),
		PERatio:            optFloat(fr.PERatio),
		DividendYield:      optFloat(fr.DividendYield),
		Volume:             optFloat(fr.Volume),
		AverageVolume:      optFloat(fr.AverageVolume),
		High:               optFloat(fr.High),
		Low:                optFloat(fr.Low),
		Open:               optFloat(fr.Open),
		High52Weeks:        optFloat(fr.High52Weeks),
		Low52Weeks:         optFloat(fr.Low52Weeks),
		UpdatedAt:          r.UpdatedAt,
	}, nil
}

func optFloat(s string) *float64 {
	if s == "" {
		return nil
	}
	v := parseFloat(s)
	return &v
}
