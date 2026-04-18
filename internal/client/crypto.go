package client

import (
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/jackzhao/robinhood-cli/internal/config"
)

type CryptoHolding struct {
	Symbol       string  `json:"symbol"`
	CurrencyCode string  `json:"currency_code"`
	Quantity     float64 `json:"quantity"`
	CostBasis    float64 `json:"cost_basis"`
	MarkPrice    float64 `json:"mark_price"`
	MarketValue  float64 `json:"market_value"`
	TotalReturn  float64 `json:"total_return"`
}

type CryptoHoldingsResult struct {
	Count     int             `json:"count"`
	TotalEq   float64         `json:"total_market_value"`
	TotalRet  float64         `json:"total_return"`
	Holdings  []CryptoHolding `json:"holdings"`
	Timestamp string          `json:"timestamp"`
}

type cryptoHoldingsRaw struct {
	Results []struct {
		ID                          string `json:"id"`
		Quantity                    string `json:"quantity"`
		QuantityAvailable           string `json:"quantity_available"`
		CostBases                   []struct {
			DirectCostBasis string `json:"direct_cost_basis"`
		} `json:"cost_bases"`
		Currency struct {
			Code string `json:"code"`
		} `json:"currency"`
	} `json:"results"`
	Next string `json:"next"`
}

type cryptoCurrencyPair struct {
	ID     string `json:"id"`
	Symbol string `json:"symbol"`
	AssetCurrency struct {
		Code string `json:"code"`
	} `json:"asset_currency"`
}

type cryptoPairsResp struct {
	Results []cryptoCurrencyPair `json:"results"`
	Next    string               `json:"next"`
}

type cryptoMarketdata struct {
	MarkPrice  string `json:"mark_price"`
	BidPrice   string `json:"bid_price"`
	AskPrice   string `json:"ask_price"`
	HighPrice  string `json:"high_price"`
	LowPrice   string `json:"low_price"`
	OpenPrice  string `json:"open_price"`
	Volume     string `json:"volume"`
	Symbol     string `json:"symbol"`
}

type CryptoQuote struct {
	Symbol    string  `json:"symbol"`
	Mark      float64 `json:"mark_price"`
	Bid       float64 `json:"bid_price"`
	Ask       float64 `json:"ask_price"`
	Open      float64 `json:"open_price"`
	High      float64 `json:"high_price"`
	Low       float64 `json:"low_price"`
	Volume    float64 `json:"volume"`
	Timestamp string  `json:"timestamp"`
}

var (
	cryptoPairsOnce  sync.Once
	cryptoPairsErr   error
	cryptoPairsBySym map[string]cryptoCurrencyPair
	cryptoPairsByAsset map[string]cryptoCurrencyPair
	cryptoPairsByID  map[string]cryptoCurrencyPair
)

func (c *Client) loadCryptoPairs() error {
	cryptoPairsOnce.Do(func() {
		cryptoPairsBySym = map[string]cryptoCurrencyPair{}
		cryptoPairsByAsset = map[string]cryptoCurrencyPair{}
		cryptoPairsByID = map[string]cryptoCurrencyPair{}
		u := config.NummusAPI + "/currency_pairs/"
		for u != "" {
			var page cryptoPairsResp
			if err := c.GetJSON(u, &page); err != nil {
				cryptoPairsErr = err
				return
			}
			for _, p := range page.Results {
				cryptoPairsBySym[strings.ToUpper(p.Symbol)] = p
				cryptoPairsByAsset[strings.ToUpper(p.AssetCurrency.Code)] = p
				cryptoPairsByID[p.ID] = p
			}
			u = page.Next
		}
	})
	return cryptoPairsErr
}

// GetCryptoHoldings returns all crypto positions.
func (c *Client) GetCryptoHoldings() (*CryptoHoldingsResult, error) {
	if err := c.loadCryptoPairs(); err != nil {
		return nil, err
	}

	var raw cryptoHoldingsRaw
	if err := c.GetJSON(config.NummusAPI+"/holdings/?nonzero=true", &raw); err != nil {
		return nil, err
	}

	holdings := make([]CryptoHolding, 0, len(raw.Results))
	totalEq := 0.0
	totalRet := 0.0
	for _, r := range raw.Results {
		qty := parseFloat(r.Quantity)
		if qty == 0 {
			continue
		}
		cost := 0.0
		for _, cb := range r.CostBases {
			cost += parseFloat(cb.DirectCostBasis)
		}
		code := strings.ToUpper(r.Currency.Code)

		mark := 0.0
		symbol := code
		if pair, ok := cryptoPairsByAsset[code]; ok {
			symbol = pair.Symbol
			if q, err := c.fetchCryptoMarketdata(pair.ID); err == nil {
				mark = parseFloat(q.MarkPrice)
			}
		}
		mv := qty * mark
		holdings = append(holdings, CryptoHolding{
			Symbol:       symbol,
			CurrencyCode: code,
			Quantity:     qty,
			CostBasis:    cost,
			MarkPrice:    mark,
			MarketValue:  mv,
			TotalReturn:  mv - cost,
		})
		totalEq += mv
		totalRet += mv - cost
	}
	return &CryptoHoldingsResult{
		Count:     len(holdings),
		TotalEq:   totalEq,
		TotalRet:  totalRet,
		Holdings:  holdings,
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

func (c *Client) fetchCryptoMarketdata(pairID string) (*cryptoMarketdata, error) {
	var md cryptoMarketdata
	if err := c.GetJSON(config.APIBase+"/marketdata/forex/quotes/"+pairID+"/", &md); err != nil {
		return nil, err
	}
	return &md, nil
}

// GetCryptoQuote returns a real-time quote for a crypto symbol like "BTC" or "BTCUSD".
func (c *Client) GetCryptoQuote(symbol string) (*CryptoQuote, error) {
	if err := c.loadCryptoPairs(); err != nil {
		return nil, err
	}
	sym := strings.ToUpper(symbol)
	pair, ok := cryptoPairsBySym[sym]
	if !ok {
		pair, ok = cryptoPairsByAsset[sym]
	}
	if !ok {
		// Try common variants like "BTC-USD"
		alt := strings.ReplaceAll(sym, "-", "")
		pair, ok = cryptoPairsBySym[alt]
	}
	if !ok {
		return nil, errMsg("crypto pair not found: " + symbol)
	}
	md, err := c.fetchCryptoMarketdata(pair.ID)
	if err != nil {
		return nil, err
	}
	return &CryptoQuote{
		Symbol:    pair.Symbol,
		Mark:      parseFloat(md.MarkPrice),
		Bid:       parseFloat(md.BidPrice),
		Ask:       parseFloat(md.AskPrice),
		Open:      parseFloat(md.OpenPrice),
		High:      parseFloat(md.HighPrice),
		Low:       parseFloat(md.LowPrice),
		Volume:    parseFloat(md.Volume),
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

// --- crypto orders / activity ---

type cryptoOrderRaw struct {
	ID              string `json:"id"`
	AccountID       string `json:"account_id"`
	CurrencyPairID  string `json:"currency_pair_id"`
	Side            string `json:"side"`
	State           string `json:"state"`
	Type            string `json:"type"`
	Quantity        string `json:"quantity"`
	CumulativeQuantity string `json:"cumulative_quantity"`
	AveragePrice    string `json:"average_price"`
	Price           string `json:"price"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

type cryptoOrdersResp struct {
	Results []cryptoOrderRaw `json:"results"`
	Next    string           `json:"next"`
}

func (c *Client) fetchCryptoOrders(maxRows int, since time.Time) ([]ActivityEntry, error) {
	if err := c.loadCryptoPairs(); err != nil {
		return nil, err
	}
	out := []ActivityEntry{}
	u := config.NummusAPI + "/orders/"
	for u != "" && len(out) < maxRows {
		var page cryptoOrdersResp
		if err := c.GetJSON(u, &page); err != nil {
			return nil, err
		}
		stop := false
		for _, r := range page.Results {
			if !since.IsZero() {
				t, err := time.Parse(time.RFC3339Nano, r.CreatedAt)
				if err == nil && t.Before(since) {
					stop = true
					break
				}
			}
			qty := parseFloat(r.CumulativeQuantity)
			if qty == 0 {
				qty = parseFloat(r.Quantity)
			}
			price := parseFloat(r.AveragePrice)
			if price == 0 {
				price = parseFloat(r.Price)
			}
			sym := r.CurrencyPairID
			if p, ok := cryptoPairsByID[r.CurrencyPairID]; ok {
				sym = p.Symbol
			}
			out = append(out, ActivityEntry{
				ID:           r.ID,
				AssetClass:   "crypto",
				Symbol:       sym,
				Side:         r.Side,
				Quantity:     qty,
				AveragePrice: price,
				TotalValue:   qty * price,
				OrderType:    r.Type,
				State:        r.State,
				CreatedAt:    r.CreatedAt,
				UpdatedAt:    r.UpdatedAt,
			})
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

// helper for query string building
var _ = url.Values{}
