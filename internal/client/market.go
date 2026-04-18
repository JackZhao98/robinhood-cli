package client

import (
	"strings"
	"time"
)

// --- market hours ---

type MarketStatus struct {
	IsOpenNow         bool   `json:"is_open_now"`
	Today             string `json:"today"`
	OpenAt            string `json:"open_at,omitempty"`
	CloseAt           string `json:"close_at,omitempty"`
	ExtendedOpenAt    string `json:"extended_open_at,omitempty"`
	ExtendedCloseAt   string `json:"extended_close_at,omitempty"`
	NextOpenDate      string `json:"next_open_date,omitempty"`
	Mic               string `json:"mic"`
	OperatingMIC      string `json:"operating_mic,omitempty"`
}

type marketsResp struct {
	Results []struct {
		MIC          string `json:"mic"`
		OperatingMIC string `json:"operating_mic"`
		Acronym      string `json:"acronym"`
		Name         string `json:"name"`
		TodaysHours  string `json:"todays_hours"`
	} `json:"results"`
}

type hoursResp struct {
	IsOpen          bool   `json:"is_open"`
	Date            string `json:"date"`
	OpensAt         string `json:"opens_at"`
	ClosesAt        string `json:"closes_at"`
	ExtendedOpensAt string `json:"extended_opens_at"`
	ExtendedClosesAt string `json:"extended_closes_at"`
	NextOpenHours   string `json:"next_open_hours"`
}

func (c *Client) GetMarketStatus() (*MarketStatus, error) {
	var ms marketsResp
	if err := c.GetJSON(URL("/markets/", nil), &ms); err != nil {
		return nil, err
	}
	// Use XNAS (NASDAQ) by convention for "is the market open".
	mic := "XNAS"
	hoursURL := ""
	for _, m := range ms.Results {
		if m.MIC == mic && m.TodaysHours != "" {
			hoursURL = m.TodaysHours
			break
		}
	}
	if hoursURL == "" && len(ms.Results) > 0 {
		hoursURL = ms.Results[0].TodaysHours
		mic = ms.Results[0].MIC
	}
	if hoursURL == "" {
		return nil, errMsg("no market hours URL")
	}
	var h hoursResp
	if err := c.GetJSON(hoursURL, &h); err != nil {
		return nil, err
	}

	isOpenNow := false
	if h.IsOpen && h.OpensAt != "" && h.ClosesAt != "" {
		now := time.Now().UTC()
		open, err1 := time.Parse(time.RFC3339Nano, h.OpensAt)
		close_, err2 := time.Parse(time.RFC3339Nano, h.ClosesAt)
		if err1 == nil && err2 == nil {
			isOpenNow = !now.Before(open) && now.Before(close_)
		}
	}

	return &MarketStatus{
		IsOpenNow:        isOpenNow,
		Today:            h.Date,
		OpenAt:           h.OpensAt,
		CloseAt:          h.ClosesAt,
		ExtendedOpenAt:   h.ExtendedOpensAt,
		ExtendedCloseAt:  h.ExtendedClosesAt,
		NextOpenDate:     h.NextOpenHours,
		Mic:              mic,
	}, nil
}

// --- movers (S&P 500 top gainers/losers) ---

type Mover struct {
	Symbol            string  `json:"symbol"`
	Description       string  `json:"description,omitempty"`
	PriceMovement     float64 `json:"price_movement_pct"`
	UpdatedAt         string  `json:"updated_at"`
}

type MoversResult struct {
	Direction string  `json:"direction"`
	Count     int     `json:"count"`
	Movers    []Mover `json:"movers"`
}

type moversRaw struct {
	Results []struct {
		Symbol      string `json:"symbol"`
		Description string `json:"description"`
		UpdatedAt   string `json:"updated_at"`
		PriceMovement struct {
			MarketHoursLastMovementPct string `json:"market_hours_last_movement_pct"`
			MarketHoursLastPrice       string `json:"market_hours_last_price"`
		} `json:"price_movement"`
	} `json:"results"`
}

func (c *Client) GetMovers(direction string) (*MoversResult, error) {
	d := strings.ToLower(direction)
	if d == "" {
		d = "up"
	}
	if d != "up" && d != "down" {
		return nil, errMsg("direction must be 'up' or 'down'")
	}
	var raw moversRaw
	if err := c.GetJSON(URL("/midlands/movers/sp500/", map[string]string{"direction": d}), &raw); err != nil {
		return nil, err
	}
	out := make([]Mover, 0, len(raw.Results))
	for _, r := range raw.Results {
		out = append(out, Mover{
			Symbol:        r.Symbol,
			Description:   r.Description,
			PriceMovement: parseFloat(r.PriceMovement.MarketHoursLastMovementPct),
			UpdatedAt:     r.UpdatedAt,
		})
	}
	return &MoversResult{Direction: d, Count: len(out), Movers: out}, nil
}

// --- watchlists ---

type WatchlistItem struct {
	Symbol       string `json:"symbol"`
	InstrumentID string `json:"instrument_id"`
	CreatedAt    string `json:"created_at,omitempty"`
}

type Watchlist struct {
	Name      string          `json:"name"`
	ID        string          `json:"id,omitempty"`
	Count     int             `json:"count"`
	Items     []WatchlistItem `json:"items"`
}

type WatchlistsResult struct {
	Count      int         `json:"count"`
	Watchlists []Watchlist `json:"watchlists"`
}

type watchlistsRaw struct {
	Results []struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
		Name        string `json:"name"`
	} `json:"results"`
}

type watchlistItemsRaw struct {
	Results []struct {
		Symbol    string `json:"symbol"`
		ObjectID  string `json:"object_id"`
		CreatedAt string `json:"created_at"`
	} `json:"results"`
	Next string `json:"next"`
}

func (c *Client) ListWatchlists() (*WatchlistsResult, error) {
	// Robinhood's list endpoint now requires `owner_type`. "custom" matches
	// user-created watchlists (and some Robinhood-provided ones the user
	// follows) which is what the previous behavior intended.
	var raw watchlistsRaw
	if err := c.GetJSON(URL("/midlands/lists/", map[string]string{"owner_type": "custom"}), &raw); err != nil {
		return nil, err
	}
	out := make([]Watchlist, 0, len(raw.Results))
	for _, w := range raw.Results {
		name := w.DisplayName
		if name == "" {
			name = w.Name
		}
		out = append(out, Watchlist{Name: name, ID: w.ID})
	}
	return &WatchlistsResult{Count: len(out), Watchlists: out}, nil
}

func (c *Client) ShowWatchlist(name string) (*Watchlist, error) {
	if name == "" {
		name = "Default"
	}
	// Find the list ID by name.
	var raw watchlistsRaw
	if err := c.GetJSON(URL("/midlands/lists/", map[string]string{"owner_type": "custom"}), &raw); err != nil {
		return nil, err
	}
	listID := ""
	displayName := ""
	for _, w := range raw.Results {
		dn := w.DisplayName
		if dn == "" {
			dn = w.Name
		}
		if strings.EqualFold(dn, name) {
			listID = w.ID
			displayName = dn
			break
		}
	}
	if listID == "" {
		return nil, errMsg("watchlist not found: " + name + " (try `rh watchlist list` to see names)")
	}

	items := []WatchlistItem{}
	u := URL("/midlands/lists/items/", map[string]string{"list_id": listID})
	for u != "" {
		var page watchlistItemsRaw
		if err := c.GetJSON(u, &page); err != nil {
			return nil, err
		}
		for _, it := range page.Results {
			items = append(items, WatchlistItem{
				Symbol:       it.Symbol,
				InstrumentID: it.ObjectID,
				CreatedAt:    it.CreatedAt,
			})
		}
		u = page.Next
	}
	return &Watchlist{Name: displayName, ID: listID, Count: len(items), Items: items}, nil
}
