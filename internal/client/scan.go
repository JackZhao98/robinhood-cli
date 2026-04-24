package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/jackzhao/robinhood-cli/internal/config"
)

// bonfireScanURL is the Bonfire SDUI screener endpoint. Sniffed from the web
// app — there is no public equivalent. Response shape is Server-Driven UI
// components; we flatten it into plain rows before returning.
const bonfireScanURL = "https://bonfire.robinhood.com/screeners/scan/"

// --- request types ---

type ScanRequest struct {
	Columns       []string        `json:"columns"`
	Indicators    []ScanIndicator `json:"indicators"`
	IsPollable    bool            `json:"is_pollable"`
	SortBy        string          `json:"sort_by"`
	SortDirection string          `json:"sort_direction"`
}

type ScanIndicator struct {
	Key      string     `json:"key"`
	Filter   ScanFilter `json:"filter"`
	IsHidden bool       `json:"is_hidden"`
}

type ScanFilter struct {
	Type      string        `json:"type"`
	Selection ScanSelection `json:"selection"`
}

type ScanSelection struct {
	ID              string         `json:"id"`
	SecondaryFilter *ScanSecondary `json:"secondary_filter,omitempty"`
}

type ScanSecondary struct {
	Type string   `json:"type"`
	Min  *float64 `json:"min"`
	Max  *float64 `json:"max"`
}

// BuildScan1dChangeRequest builds the common "screen by today's % change"
// request that matches the shape sniffed from the RH web app. Passing nil
// for either bound sends JSON null, which the server treats as "no bound".
func BuildScan1dChangeRequest(min, max *float64, sortDir string) *ScanRequest {
	dir := strings.ToUpper(strings.TrimSpace(sortDir))
	if dir == "" {
		dir = "DESC"
	}
	return &ScanRequest{
		Columns:    []string{"sparkline", "1d_price_change", "price", "todays_volume", "market_cap"},
		IsPollable: true,
		SortBy:     "1d_price_change",
		SortDirection: dir,
		Indicators: []ScanIndicator{{
			Key: "1d_price_change",
			Filter: ScanFilter{
				Type: "SINGLE_SELECT",
				Selection: ScanSelection{
					ID: "custom",
					SecondaryFilter: &ScanSecondary{
						Type: "PERCENT_RANGE",
						Min:  min,
						Max:  max,
					},
				},
			},
		}},
	}
}

// --- response types ---

// ScanRow is the AI-friendly flattened row.
// Price/Volume/MarketCap are kept as strings because the SDUI response
// hands them back pre-formatted (e.g. "$0.5249", "580.54M", "25.22B", "—");
// parsing loses fidelity and trips on the em-dash placeholder.
type ScanRow struct {
	Symbol       string  `json:"symbol"`
	Name         string  `json:"name,omitempty"`
	InstrumentID string  `json:"instrument_id"`
	Change1dPct  float64 `json:"change_1d_pct"`
	Direction    string  `json:"direction,omitempty"`
	Price        string  `json:"price,omitempty"`
	Volume       string  `json:"volume,omitempty"`
	MarketCap    string  `json:"market_cap,omitempty"`
}

type ScanResult struct {
	Count    int       `json:"count"`
	Subtitle string    `json:"subtitle,omitempty"`
	Rows     []ScanRow `json:"rows"`
}

// Scan POSTs to the Bonfire screener and flattens the SDUI response.
func (c *Client) Scan(req *ScanRequest) (*ScanResult, error) {
	if err := c.ensureFresh(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal scan request: %w", err)
	}
	httpReq, err := http.NewRequest("POST", bonfireScanURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	// Bonfire needs these web-style headers in addition to the standard bearer.
	httpReq.Header.Set("Accept", "*/*")
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", config.UserAgent)
	httpReq.Header.Set("Authorization", "Bearer "+c.creds.AccessToken)
	httpReq.Header.Set("Origin", "https://robinhood.com")
	httpReq.Header.Set("Referer", "https://robinhood.com/")
	httpReq.Header.Set("X-Hyper-Ex", "enabled")
	httpReq.Header.Set("X-TimeZone-Id", "America/Los_Angeles")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("unauthorized — try `rh login` again")
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("POST %s: http %d: %s", bonfireScanURL, resp.StatusCode, truncate(respBody, 400))
	}
	return parseScanResponse(respBody)
}

// --- SDUI parser ---

type rawScanResp struct {
	Subtitle string `json:"subtitle"`
	Rows     []struct {
		InstrumentID     string `json:"instrument_id"`
		InstrumentSymbol string `json:"instrument_symbol"`
		Items            []struct {
			Component json.RawMessage `json:"component"`
		} `json:"items"`
	} `json:"rows"`
}

func parseScanResponse(body []byte) (*ScanResult, error) {
	var r rawScanResp
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("decode scan response: %w (body=%s)", err, truncate(body, 300))
	}
	rows := make([]ScanRow, 0, len(r.Rows))
	for _, row := range r.Rows {
		// The header row has empty symbol / instrument_id.
		if row.InstrumentSymbol == "" {
			continue
		}
		out := ScanRow{
			Symbol:       row.InstrumentSymbol,
			InstrumentID: row.InstrumentID,
		}
		textIdx := 0 // TEXT components appear in the order: volume, market_cap
		for _, it := range row.Items {
			extractScanField(&out, it.Component, &textIdx)
		}
		rows = append(rows, out)
	}
	return &ScanResult{
		Count:    len(rows),
		Subtitle: r.Subtitle,
		Rows:     rows,
	}, nil
}

// extractScanField inspects one SDUI component by sdui_component_type.
// textIdx is incremented on every TEXT component so volume lands before
// market_cap without relying on absolute indexing (robust to column reorder).
func extractScanField(out *ScanRow, raw json.RawMessage, textIdx *int) {
	var probe struct {
		Type string `json:"sdui_component_type"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return
	}
	switch probe.Type {
	case "TABLE_INSTRUMENT_NAME":
		var c struct {
			Name   string `json:"name"`
			Symbol string `json:"symbol"`
		}
		if err := json.Unmarshal(raw, &c); err == nil {
			out.Name = c.Name
			if out.Symbol == "" {
				out.Symbol = c.Symbol
			}
		}
	case "TABLE_1D_CHANGE_ITEM":
		var c struct {
			DefaultValue struct {
				Direction string `json:"direction"`
				Value     string `json:"value"`
			} `json:"default_value"`
		}
		if err := json.Unmarshal(raw, &c); err == nil {
			out.Direction = c.DefaultValue.Direction
			pct := parseFloat(strings.TrimSuffix(c.DefaultValue.Value, "%"))
			if c.DefaultValue.Direction == "down" && pct > 0 {
				pct = -pct
			}
			out.Change1dPct = pct
		}
	case "TABLE_SHARE_PRICE_ITEM":
		var c struct {
			DefaultValue string `json:"default_value"`
		}
		if err := json.Unmarshal(raw, &c); err == nil {
			out.Price = c.DefaultValue
		}
	case "TEXT":
		var c struct {
			Text struct {
				Text string `json:"text"`
			} `json:"text"`
		}
		if err := json.Unmarshal(raw, &c); err != nil {
			return
		}
		switch *textIdx {
		case 0:
			out.Volume = c.Text.Text
		case 1:
			out.MarketCap = c.Text.Text
		}
		*textIdx++
	}
}
