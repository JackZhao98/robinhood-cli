package client

import (
	"fmt"
	"strings"
	"sync"
)

// IndexValue is the AI-friendly shape returned to callers.
// Top block = realtime quote (from /values/v1).
// Bottom block = daily session / 52-week fundamentals (from /fundamentals/v1).
// Fundamentals fields are pointers so a `--values-only` fetch can leave them nil.
type IndexValue struct {
	Symbol         string `json:"symbol"`
	UUID           string `json:"uuid"`
	Value          float64 `json:"value"`
	VenueTimestamp string  `json:"venue_timestamp"`
	UpdatedAt      string  `json:"updated_at"`
	State          string  `json:"state,omitempty"`

	// Fundamentals (omitted when empty / fetch skipped).
	Open              *float64 `json:"open,omitempty"`
	High              *float64 `json:"high,omitempty"`
	Low               *float64 `json:"low,omitempty"`
	PreviousClose     *float64 `json:"previous_close,omitempty"`
	PreviousCloseDate string   `json:"previous_close_date,omitempty"`
	High52Weeks       *float64 `json:"high_52_weeks,omitempty"`
	Low52Weeks        *float64 `json:"low_52_weeks,omitempty"`
	PERatio           *float64 `json:"pe_ratio,omitempty"`
	MarketCap         *float64 `json:"market_cap,omitempty"`
}

// indexValuesResp matches /marketdata/indexes/values/v1/.
type indexValuesResp struct {
	Data []struct {
		Status string `json:"status"`
		Data   struct {
			Value          string `json:"value"`
			VenueTimestamp string `json:"venue_timestamp"`
			Symbol         string `json:"symbol"`
			InstrumentID   string `json:"instrument_id"`
			State          string `json:"state"`
			UpdatedAt      string `json:"updated_at"`
		} `json:"data"`
	} `json:"data"`
}

// indexFundamentalsResp matches /marketdata/indexes/fundamentals/v1/.
type indexFundamentalsResp struct {
	Data []struct {
		Status string `json:"status"`
		Data   struct {
			ID                string `json:"id"`
			Symbol            string `json:"symbol"`
			High52Weeks       string `json:"high_52_weeks"`
			Low52Weeks        string `json:"low_52_weeks"`
			MarketCap         string `json:"market_cap"`
			PERatio           string `json:"pe_ratio"`
			High              string `json:"high"`
			Low               string `json:"low"`
			Open              string `json:"open"`
			PreviousClose     string `json:"previous_close"`
			PreviousCloseDate string `json:"previous_close_date"`
		} `json:"data"`
	} `json:"data"`
}

// GetIndexValues queries /marketdata/indexes/values/v1 (and optionally
// /marketdata/indexes/fundamentals/v1 in parallel) and merges by UUID.
// Robinhood has no public symbol→UUID lookup; callers must resolve UUIDs
// via the IndexRegistry in config.
func (c *Client) GetIndexValues(uuids []string, withFundamentals bool) ([]IndexValue, error) {
	if len(uuids) == 0 {
		return nil, fmt.Errorf("no uuids provided")
	}
	ids := strings.Join(uuids, ",")

	var (
		vResp    indexValuesResp
		fResp    indexFundamentalsResp
		vErr     error
		fErr     error
		wg       sync.WaitGroup
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		vErr = c.GetJSON(URL("/marketdata/indexes/values/v1/",
			map[string]string{"ids": ids}), &vResp)
	}()

	if withFundamentals {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fErr = c.GetJSON(URL("/marketdata/indexes/fundamentals/v1/",
				map[string]string{"ids": ids}), &fResp)
		}()
	}
	wg.Wait()

	if vErr != nil {
		return nil, fmt.Errorf("values: %w", vErr)
	}
	// fundamentals failure is non-fatal — still return realtime values.
	// Caller can see `pe_ratio` etc. are nil and decide.

	byUUID := make(map[string]*IndexValue, len(vResp.Data))
	order := make([]string, 0, len(vResp.Data))
	for _, item := range vResp.Data {
		if item.Status != "SUCCESS" {
			continue
		}
		iv := &IndexValue{
			Symbol:         item.Data.Symbol,
			UUID:           item.Data.InstrumentID,
			Value:          parseFloat(item.Data.Value),
			VenueTimestamp: item.Data.VenueTimestamp,
			UpdatedAt:      item.Data.UpdatedAt,
			State:          item.Data.State,
		}
		byUUID[iv.UUID] = iv
		order = append(order, iv.UUID)
	}

	if withFundamentals && fErr == nil {
		for _, item := range fResp.Data {
			if item.Status != "SUCCESS" {
				continue
			}
			iv, ok := byUUID[item.Data.ID]
			if !ok {
				continue
			}
			iv.Open = optFloat(item.Data.Open)
			iv.High = optFloat(item.Data.High)
			iv.Low = optFloat(item.Data.Low)
			iv.PreviousClose = optFloat(item.Data.PreviousClose)
			iv.PreviousCloseDate = item.Data.PreviousCloseDate
			iv.High52Weeks = optFloat(item.Data.High52Weeks)
			iv.Low52Weeks = optFloat(item.Data.Low52Weeks)
			iv.PERatio = optFloat(item.Data.PERatio)
			iv.MarketCap = optFloat(item.Data.MarketCap)
		}
	}

	out := make([]IndexValue, 0, len(order))
	for _, id := range order {
		out = append(out, *byUUID[id])
	}
	return out, nil
}
