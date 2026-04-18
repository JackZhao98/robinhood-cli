package client

import (
	"strings"
)

// --- search ---

type SymbolMatch struct {
	Symbol         string `json:"symbol"`
	Name           string `json:"name"`
	InstrumentID   string `json:"instrument_id"`
	State          string `json:"state"`
	Tradeable      bool   `json:"tradeable"`
	ListDate       string `json:"list_date"`
}

type SymbolSearchResult struct {
	Query   string        `json:"query"`
	Count   int           `json:"count"`
	Matches []SymbolMatch `json:"matches"`
}

type instrumentSearchRaw struct {
	Results []struct {
		Symbol       string `json:"symbol"`
		SimpleName   string `json:"simple_name"`
		Name         string `json:"name"`
		ID           string `json:"id"`
		State        string `json:"state"`
		Tradeable    bool   `json:"tradeable"`
		ListDate     string `json:"list_date"`
	} `json:"results"`
}

func (c *Client) SymbolSearch(query string) (*SymbolSearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return nil, errMsg("query is required")
	}
	var raw instrumentSearchRaw
	if err := c.GetJSON(URL("/instruments/", map[string]string{"query": query}), &raw); err != nil {
		return nil, err
	}
	matches := make([]SymbolMatch, 0, len(raw.Results))
	for _, r := range raw.Results {
		name := r.SimpleName
		if name == "" {
			name = r.Name
		}
		matches = append(matches, SymbolMatch{
			Symbol:       r.Symbol,
			Name:         name,
			InstrumentID: r.ID,
			State:        r.State,
			Tradeable:    r.Tradeable,
			ListDate:     r.ListDate,
		})
	}
	return &SymbolSearchResult{Query: query, Count: len(matches), Matches: matches}, nil
}

// --- news ---

type NewsItem struct {
	Title         string `json:"title"`
	Summary       string `json:"summary"`
	URL           string `json:"url"`
	Source        string `json:"source"`
	PublishedAt   string `json:"published_at"`
	PreviewImage  string `json:"preview_image_url,omitempty"`
}

type NewsResult struct {
	Symbol string     `json:"symbol"`
	Count  int        `json:"count"`
	News   []NewsItem `json:"news"`
}

type newsRaw struct {
	Results []struct {
		Title           string `json:"title"`
		Summary         string `json:"summary"`
		URL             string `json:"url"`
		Source          string `json:"source"`
		PublishedAt     string `json:"published_at"`
		PreviewImageURL string `json:"preview_image_url"`
	} `json:"results"`
}

func (c *Client) GetNews(symbol string) (*NewsResult, error) {
	symbol = strings.ToUpper(symbol)
	var raw newsRaw
	if err := c.GetJSON(URL("/midlands/news/"+symbol+"/", nil), &raw); err != nil {
		return nil, err
	}
	items := make([]NewsItem, 0, len(raw.Results))
	for _, r := range raw.Results {
		items = append(items, NewsItem{
			Title:        r.Title,
			Summary:      r.Summary,
			URL:          r.URL,
			Source:       r.Source,
			PublishedAt:  r.PublishedAt,
			PreviewImage: r.PreviewImageURL,
		})
	}
	return &NewsResult{Symbol: symbol, Count: len(items), News: items}, nil
}

// --- earnings ---

type EarningsEvent struct {
	Symbol      string  `json:"symbol"`
	Year        int     `json:"year"`
	Quarter     int     `json:"quarter"`
	ReportDate  string  `json:"report_date"`
	When        string  `json:"timing,omitempty"`
	EPSActual   float64 `json:"eps_actual,omitempty"`
	EPSEstimate float64 `json:"eps_estimate,omitempty"`
	RevActual   float64 `json:"revenue_actual,omitempty"`
	RevEstimate float64 `json:"revenue_estimate,omitempty"`
}

type EarningsResult struct {
	Symbol  string          `json:"symbol"`
	Count   int             `json:"count"`
	Events  []EarningsEvent `json:"events"`
}

type earningsRaw struct {
	Results []struct {
		Symbol  string `json:"symbol"`
		Year    int    `json:"year"`
		Quarter int    `json:"quarter"`
		EPS     struct {
			Actual   string `json:"actual"`
			Estimate string `json:"estimate"`
		} `json:"eps"`
		Report struct {
			Date    string `json:"date"`
			Timing  string `json:"timing"`
		} `json:"report"`
		Revenue struct {
			Actual   string `json:"actual"`
			Estimate string `json:"estimate"`
		} `json:"revenue"`
	} `json:"results"`
}

func (c *Client) GetEarnings(symbol string) (*EarningsResult, error) {
	symbol = strings.ToUpper(symbol)
	var raw earningsRaw
	if err := c.GetJSON(URL("/marketdata/earnings/", map[string]string{"symbol": symbol}), &raw); err != nil {
		return nil, err
	}
	events := make([]EarningsEvent, 0, len(raw.Results))
	for _, r := range raw.Results {
		events = append(events, EarningsEvent{
			Symbol:      r.Symbol,
			Year:        r.Year,
			Quarter:     r.Quarter,
			ReportDate:  r.Report.Date,
			When:        r.Report.Timing,
			EPSActual:   parseFloat(r.EPS.Actual),
			EPSEstimate: parseFloat(r.EPS.Estimate),
			RevActual:   parseFloat(r.Revenue.Actual),
			RevEstimate: parseFloat(r.Revenue.Estimate),
		})
	}
	return &EarningsResult{Symbol: symbol, Count: len(events), Events: events}, nil
}

// --- ratings ---

type Rating struct {
	PublishedAt string `json:"published_at"`
	Type        string `json:"type"`
	Text        string `json:"text"`
}

type RatingsResult struct {
	Symbol      string   `json:"symbol"`
	NumBuy      int      `json:"num_buy"`
	NumHold     int      `json:"num_hold"`
	NumSell     int      `json:"num_sell"`
	BuyPercent  float64  `json:"buy_pct"`
	HoldPercent float64  `json:"hold_pct"`
	SellPercent float64  `json:"sell_pct"`
	Ratings     []Rating `json:"ratings"`
}

type ratingsRaw struct {
	Summary struct {
		NumBuyRatings  int `json:"num_buy_ratings"`
		NumHoldRatings int `json:"num_hold_ratings"`
		NumSellRatings int `json:"num_sell_ratings"`
	} `json:"summary"`
	Ratings []struct {
		PublishedAt string `json:"published_at"`
		Type        string `json:"type"`
		Text        string `json:"text"`
	} `json:"ratings"`
}

func (c *Client) GetRatings(symbol string) (*RatingsResult, error) {
	symbol = strings.ToUpper(symbol)
	id, err := c.instrumentIDFor(symbol)
	if err != nil {
		return nil, err
	}
	var raw ratingsRaw
	if err := c.GetJSON(URL("/midlands/ratings/"+id+"/", nil), &raw); err != nil {
		return nil, err
	}
	total := raw.Summary.NumBuyRatings + raw.Summary.NumHoldRatings + raw.Summary.NumSellRatings
	pct := func(n int) float64 {
		if total == 0 {
			return 0
		}
		return float64(n) / float64(total) * 100
	}
	rs := make([]Rating, 0, len(raw.Ratings))
	for _, r := range raw.Ratings {
		rs = append(rs, Rating{PublishedAt: r.PublishedAt, Type: r.Type, Text: r.Text})
	}
	return &RatingsResult{
		Symbol:      symbol,
		NumBuy:      raw.Summary.NumBuyRatings,
		NumHold:     raw.Summary.NumHoldRatings,
		NumSell:     raw.Summary.NumSellRatings,
		BuyPercent:  pct(raw.Summary.NumBuyRatings),
		HoldPercent: pct(raw.Summary.NumHoldRatings),
		SellPercent: pct(raw.Summary.NumSellRatings),
		Ratings:     rs,
	}, nil
}

// --- similar ---

type SimilarStock struct {
	Symbol     string  `json:"symbol"`
	Name       string  `json:"name,omitempty"`
	Tag        string  `json:"tag,omitempty"`
}

type SimilarResult struct {
	Symbol    string         `json:"symbol"`
	Count     int            `json:"count"`
	Similar   []SimilarStock `json:"similar"`
}

type tagInstrumentsRaw struct {
	Tags []struct {
		Slug        string   `json:"slug"`
		Name        string   `json:"name"`
		Instruments []string `json:"instruments"`
	} `json:"tags"`
}

// GetSimilar returns up to 20 peer instruments derived from the tags the
// target symbol shares on Robinhood's discovery taxonomy. The previous
// /midlands/tags/similar/<id>/ endpoint was removed — tags are the closest
// remaining replacement.
func (c *Client) GetSimilar(symbol string) (*SimilarResult, error) {
	symbol = strings.ToUpper(symbol)
	id, err := c.instrumentIDFor(symbol)
	if err != nil {
		return nil, err
	}
	var raw tagInstrumentsRaw
	if err := c.GetJSON(URL("/midlands/tags/instrument/"+id+"/", nil), &raw); err != nil {
		return nil, err
	}

	// Collect peer instrument URLs, tagged with the first category they
	// appear in. Skip this symbol and generic popularity tags ("100 Most
	// Popular" / "upcoming-earnings") which are noise.
	const skipSlug = "100-most-popular"
	tagByURL := map[string]string{}
	order := []string{}
	for _, t := range raw.Tags {
		if t.Slug == skipSlug || strings.HasPrefix(t.Slug, "upcoming-") {
			continue
		}
		for _, iURL := range t.Instruments {
			instID := lastPathSegment(strings.TrimSuffix(iURL, "/"))
			if instID == id || instID == "" {
				continue
			}
			if _, seen := tagByURL[instID]; !seen {
				tagByURL[instID] = t.Name
				order = append(order, instID)
			}
		}
	}
	// Cap to keep the payload useful, not overwhelming.
	const max = 20
	if len(order) > max {
		order = order[:max]
	}

	// Batch-resolve instrument IDs → symbols.
	symbols := map[string]string{}
	const batch = 50
	for start := 0; start < len(order); start += batch {
		end := start + batch
		if end > len(order) {
			end = len(order)
		}
		var resp instrumentsExtResp
		if err := c.GetJSON(URL("/instruments/", map[string]string{
			"ids": strings.Join(order[start:end], ","),
		}), &resp); err != nil {
			return nil, err
		}
		for _, inst := range resp.Results {
			symbols[inst.ID] = inst.Symbol
		}
	}

	out := make([]SimilarStock, 0, len(order))
	for _, instID := range order {
		sym := symbols[instID]
		if sym == "" {
			continue
		}
		out = append(out, SimilarStock{Symbol: sym, Tag: tagByURL[instID]})
	}
	return &SimilarResult{Symbol: symbol, Count: len(out), Similar: out}, nil
}

// --- tags ---

type Tag struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type TagsResult struct {
	Symbol string `json:"symbol"`
	Count  int    `json:"count"`
	Tags   []Tag  `json:"tags"`
}

type tagsRaw struct {
	Tags []struct {
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"tags"`
}

func (c *Client) GetTags(symbol string) (*TagsResult, error) {
	symbol = strings.ToUpper(symbol)
	id, err := c.instrumentIDFor(symbol)
	if err != nil {
		return nil, err
	}
	var raw tagsRaw
	if err := c.GetJSON(URL("/midlands/tags/instrument/"+id+"/", nil), &raw); err != nil {
		return nil, err
	}
	out := make([]Tag, 0, len(raw.Tags))
	for _, t := range raw.Tags {
		out = append(out, Tag{Slug: t.Slug, Name: t.Name, Description: t.Description})
	}
	return &TagsResult{Symbol: symbol, Count: len(out), Tags: out}, nil
}

// --- splits ---

type Split struct {
	ExecutionDate string  `json:"execution_date"`
	Multiplier    float64 `json:"multiplier"`
	Divisor       float64 `json:"divisor"`
}

type SplitsResult struct {
	Symbol string  `json:"symbol"`
	Count  int     `json:"count"`
	Splits []Split `json:"splits"`
}

type splitsRaw struct {
	Results []struct {
		ExecutionDate string `json:"execution_date"`
		Multiplier    string `json:"multiplier"`
		Divisor       string `json:"divisor"`
	} `json:"results"`
}

func (c *Client) GetSplits(symbol string) (*SplitsResult, error) {
	symbol = strings.ToUpper(symbol)
	id, err := c.instrumentIDFor(symbol)
	if err != nil {
		return nil, err
	}
	var raw splitsRaw
	if err := c.GetJSON(URL("/instruments/"+id+"/splits/", nil), &raw); err != nil {
		return nil, err
	}
	out := make([]Split, 0, len(raw.Results))
	for _, s := range raw.Results {
		out = append(out, Split{
			ExecutionDate: s.ExecutionDate,
			Multiplier:    parseFloat(s.Multiplier),
			Divisor:       parseFloat(s.Divisor),
		})
	}
	return &SplitsResult{Symbol: symbol, Count: len(out), Splits: out}, nil
}

// instrumentIDFor caches symbol → instrument id lookups locally per command.
func (c *Client) instrumentIDFor(symbol string) (string, error) {
	var resp instrumentsExtResp
	if err := c.GetJSON(URL("/instruments/", map[string]string{"symbols": strings.ToUpper(symbol)}), &resp); err != nil {
		return "", err
	}
	if len(resp.Results) == 0 {
		return "", errMsg("symbol not found: " + symbol)
	}
	return resp.Results[0].ID, nil
}
