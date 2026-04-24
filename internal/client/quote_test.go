package client

import "testing"

func TestMapFundamentalsByResolvedOrderUsesResolvedSymbols(t *testing.T) {
	f := fundamentals{
		Results: []struct {
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
		}{
			{MarketCap: "12345", PERatio: "33.1"},
		},
	}

	got := mapFundamentalsByResolvedOrder(f, []string{"NVDA"})
	if _, ok := got["BAD"]; ok {
		t.Fatalf("unexpected fundamentals mapped to unresolved symbol BAD")
	}
	row, ok := got["NVDA"]
	if !ok {
		t.Fatalf("expected NVDA fundamentals to be present")
	}
	if row.MarketCap != "12345" || row.PERatio != "33.1" {
		t.Fatalf("wrong fundamentals mapped to NVDA: %+v", row)
	}
}
