package client

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

type OptionChain struct {
	Symbol               string   `json:"symbol"`
	ChainID              string   `json:"chain_id"`
	ExpirationDates      []string `json:"expiration_dates"`
	TradeValueMultiplier float64  `json:"trade_value_multiplier"`
	CanOpenPosition      bool     `json:"can_open_position"`
}

type OptionInstrument struct {
	ID             string  `json:"id"`
	Symbol         string  `json:"symbol"`
	StrikePrice    float64 `json:"strike_price"`
	ExpirationDate string  `json:"expiration_date"`
	Type           string  `json:"type"`
	State          string  `json:"state"`
	Tradability    string  `json:"tradability"`
}

type OptionInstruments struct {
	Symbol         string             `json:"symbol"`
	ExpirationDate string             `json:"expiration_date"`
	OptionType     string             `json:"option_type,omitempty"`
	Count          int                `json:"count"`
	Instruments    []OptionInstrument `json:"instruments"`
}

type OptionGreek struct {
	Strike              float64  `json:"strike"`
	Type                string   `json:"type"`
	Bid                 *float64 `json:"bid"`
	Ask                 *float64 `json:"ask"`
	Last                *float64 `json:"last"`
	Mark                *float64 `json:"mark"`
	IV                  *float64 `json:"iv"`
	Delta               *float64 `json:"delta"`
	Theta               *float64 `json:"theta"`
	Gamma               *float64 `json:"gamma"`
	Vega                *float64 `json:"vega"`
	Rho                 *float64 `json:"rho"`
	OpenInterest        *float64 `json:"open_interest"`
	Volume              *float64 `json:"volume"`
	ChanceOfProfitLong  *float64 `json:"chance_of_profit_long"`
	ChanceOfProfitShort *float64 `json:"chance_of_profit_short"`
}

type OptionGreeksResult struct {
	Symbol         string        `json:"symbol"`
	ExpirationDate string        `json:"expiration_date"`
	OptionType     string        `json:"option_type"`
	Side           string        `json:"side"`
	Count          int           `json:"count"`
	Options        []OptionGreek `json:"options"`
	Error          string        `json:"error,omitempty"`
}

type OptionLeg struct {
	StrikePrice    float64 `json:"strike_price"`
	ExpirationDate string  `json:"expiration_date"`
	OptionType     string  `json:"option_type"`
	PositionType   string  `json:"position_type"`
	RatioQuantity  float64 `json:"ratio_quantity"`
}

type OptionPosition struct {
	ID                       string      `json:"id"`
	Symbol                   string      `json:"symbol"`
	Strategy                 string      `json:"strategy"`
	Direction                string      `json:"direction"`
	Quantity                 float64     `json:"quantity"`
	AverageOpenPrice         float64     `json:"average_open_price"`
	IntradayAverageOpenPrice float64     `json:"intraday_average_open_price"`
	Legs                     []OptionLeg `json:"legs"`
	TradeValueMultiplier     float64     `json:"trade_value_multiplier"`
	CreatedAt                string      `json:"created_at"`
	UpdatedAt                string      `json:"updated_at"`
}

type OptionPositionsResult struct {
	Count     int              `json:"count"`
	Positions []OptionPosition `json:"positions"`
	Timestamp string           `json:"timestamp"`
}

// --- chain ---

type chainResp struct {
	ID                   string   `json:"id"`
	ExpirationDates      []string `json:"expiration_dates"`
	TradeValueMultiplier string   `json:"trade_value_multiplier"`
	CanOpenPosition      bool     `json:"can_open_position"`
}

type instrumentExt struct {
	ID               string `json:"id"`
	Symbol           string `json:"symbol"`
	TradableChainID  string `json:"tradable_chain_id"`
}

type instrumentsExtResp struct {
	Results []instrumentExt `json:"results"`
}

func (c *Client) GetOptionChain(symbol string) (*OptionChain, error) {
	symbol = strings.ToUpper(symbol)

	var instr instrumentsExtResp
	if err := c.GetJSON(URL("/instruments/", map[string]string{"symbols": symbol}), &instr); err != nil {
		return nil, err
	}
	if len(instr.Results) == 0 {
		return nil, errMsg("symbol not found: " + symbol)
	}
	chainID := instr.Results[0].TradableChainID
	if chainID == "" {
		return nil, errMsg(symbol + " has no tradable options chain")
	}

	var chain chainResp
	if err := c.GetJSON(URL("/options/chains/"+chainID+"/", nil), &chain); err != nil {
		return nil, err
	}

	mult := parseFloat(chain.TradeValueMultiplier)
	if mult == 0 {
		mult = 100
	}
	return &OptionChain{
		Symbol:               symbol,
		ChainID:              chainID,
		ExpirationDates:      chain.ExpirationDates,
		TradeValueMultiplier: mult,
		CanOpenPosition:      chain.CanOpenPosition,
	}, nil
}

// --- instruments (per expiration) ---

type optInstrumentResp struct {
	Results []struct {
		ID             string `json:"id"`
		ChainSymbol    string `json:"chain_symbol"`
		StrikePrice    string `json:"strike_price"`
		ExpirationDate string `json:"expiration_date"`
		Type           string `json:"type"`
		State          string `json:"state"`
		Tradability    string `json:"tradability"`
	} `json:"results"`
}

func (c *Client) GetOptionInstruments(symbol, expirationDate, optionType string, strikePrice *float64) (*OptionInstruments, error) {
	chain, err := c.GetOptionChain(symbol)
	if err != nil {
		return nil, err
	}
	q := map[string]string{
		"chain_id":         chain.ChainID,
		"expiration_dates": expirationDate,
		"state":            "active",
	}
	if optionType != "" {
		q["type"] = strings.ToLower(optionType)
	}
	if strikePrice != nil {
		q["strike_price"] = trimFloat(*strikePrice)
	}

	var data optInstrumentResp
	if err := c.GetJSON(URL("/options/instruments/", q), &data); err != nil {
		return nil, err
	}

	out := make([]OptionInstrument, 0, len(data.Results))
	for _, r := range data.Results {
		out = append(out, OptionInstrument{
			ID:             r.ID,
			Symbol:         r.ChainSymbol,
			StrikePrice:    parseFloat(r.StrikePrice),
			ExpirationDate: r.ExpirationDate,
			Type:           r.Type,
			State:          r.State,
			Tradability:    r.Tradability,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		return out[i].StrikePrice < out[j].StrikePrice
	})
	return &OptionInstruments{
		Symbol:         strings.ToUpper(symbol),
		ExpirationDate: expirationDate,
		OptionType:     optionType,
		Count:          len(out),
		Instruments:    out,
	}, nil
}

// --- greeks (chunked marketdata fetch) ---

type optMarketResp struct {
	Results []*struct {
		InstrumentID         string `json:"instrument_id"`
		BidPrice             string `json:"bid_price"`
		AskPrice             string `json:"ask_price"`
		LastTradePrice       string `json:"last_trade_price"`
		MarkPrice            string `json:"mark_price"`
		ImpliedVolatility    string `json:"implied_volatility"`
		Delta                string `json:"delta"`
		Theta                string `json:"theta"`
		Gamma                string `json:"gamma"`
		Vega                 string `json:"vega"`
		Rho                  string `json:"rho"`
		OpenInterest         int    `json:"open_interest"`
		Volume               int    `json:"volume"`
		ChanceOfProfitLong   string `json:"chance_of_profit_long"`
		ChanceOfProfitShort  string `json:"chance_of_profit_short"`
	} `json:"results"`
}

func (c *Client) GetOptionGreeks(symbol, expirationDate, optionType, side string) (*OptionGreeksResult, error) {
	if optionType == "" {
		optionType = "call"
	}
	if side == "" {
		side = "buy"
	}
	insts, err := c.GetOptionInstruments(symbol, expirationDate, optionType, nil)
	if err != nil {
		return nil, err
	}
	if len(insts.Instruments) == 0 {
		return &OptionGreeksResult{
			Symbol:         strings.ToUpper(symbol),
			ExpirationDate: expirationDate,
			OptionType:     optionType,
			Side:           side,
			Error:          "no option instruments matched",
		}, nil
	}

	idMap := map[string]OptionInstrument{}
	ids := make([]string, 0, len(insts.Instruments))
	for _, i := range insts.Instruments {
		idMap[i.ID] = i
		ids = append(ids, i.ID)
	}

	// Robinhood marketdata accepts batches of ~30 IDs comfortably.
	const batch = 25
	combined := []OptionGreek{}
	for start := 0; start < len(ids); start += batch {
		end := start + batch
		if end > len(ids) {
			end = len(ids)
		}
		var md optMarketResp
		if err := c.GetJSON(URL("/marketdata/options/", map[string]string{
			"ids": strings.Join(ids[start:end], ","),
		}), &md); err != nil {
			return nil, err
		}
		for _, r := range md.Results {
			if r == nil {
				continue
			}
			inst, ok := idMap[r.InstrumentID]
			if !ok {
				continue
			}
			combined = append(combined, OptionGreek{
				Strike:              inst.StrikePrice,
				Type:                inst.Type,
				Bid:                 optFloat(r.BidPrice),
				Ask:                 optFloat(r.AskPrice),
				Last:                optFloat(r.LastTradePrice),
				Mark:                optFloat(r.MarkPrice),
				IV:                  optFloat(r.ImpliedVolatility),
				Delta:               optFloat(r.Delta),
				Theta:               optFloat(r.Theta),
				Gamma:               optFloat(r.Gamma),
				Vega:                optFloat(r.Vega),
				Rho:                 optFloat(r.Rho),
				OpenInterest:        intPtrFloat(r.OpenInterest),
				Volume:              intPtrFloat(r.Volume),
				ChanceOfProfitLong:  optFloat(r.ChanceOfProfitLong),
				ChanceOfProfitShort: optFloat(r.ChanceOfProfitShort),
			})
		}
	}

	sort.Slice(combined, func(i, j int) bool {
		return combined[i].Strike < combined[j].Strike
	})

	return &OptionGreeksResult{
		Symbol:         strings.ToUpper(symbol),
		ExpirationDate: expirationDate,
		OptionType:     optionType,
		Side:           side,
		Count:          len(combined),
		Options:        combined,
	}, nil
}

func intPtrFloat(v int) *float64 {
	f := float64(v)
	return &f
}

// --- option positions ---

type aggregatePositionsResp struct {
	Results []struct {
		ID                       string `json:"id"`
		Symbol                   string `json:"symbol"`
		Strategy                 string `json:"strategy"`
		Direction                string `json:"direction"`
		Quantity                 string `json:"quantity"`
		AverageOpenPrice         string `json:"average_open_price"`
		IntradayAverageOpenPrice string `json:"intraday_average_open_price"`
		TradeValueMultiplier     string `json:"trade_value_multiplier"`
		CreatedAt                string `json:"created_at"`
		UpdatedAt                string `json:"updated_at"`
		Legs                     []struct {
			StrikePrice    string `json:"strike_price"`
			ExpirationDate string `json:"expiration_date"`
			OptionType     string `json:"option_type"`
			PositionType   string `json:"position_type"`
			RatioQuantity  string `json:"ratio_quantity"`
		} `json:"legs"`
	} `json:"results"`
}

func (c *Client) GetOptionPositions() (*OptionPositionsResult, error) {
	var data aggregatePositionsResp
	if err := c.GetJSON(URL("/options/aggregate_positions/", map[string]string{
		"nonzero": "True",
	}), &data); err != nil {
		return nil, err
	}

	positions := make([]OptionPosition, 0, len(data.Results))
	for _, r := range data.Results {
		legs := make([]OptionLeg, 0, len(r.Legs))
		for _, l := range r.Legs {
			legs = append(legs, OptionLeg{
				StrikePrice:    parseFloat(l.StrikePrice),
				ExpirationDate: l.ExpirationDate,
				OptionType:     l.OptionType,
				PositionType:   l.PositionType,
				RatioQuantity:  parseFloat(l.RatioQuantity),
			})
		}
		mult := parseFloat(r.TradeValueMultiplier)
		if mult == 0 {
			mult = 100
		}
		positions = append(positions, OptionPosition{
			ID:                       r.ID,
			Symbol:                   r.Symbol,
			Strategy:                 r.Strategy,
			Direction:                r.Direction,
			Quantity:                 parseFloat(r.Quantity),
			AverageOpenPrice:         parseFloat(r.AverageOpenPrice),
			IntradayAverageOpenPrice: parseFloat(r.IntradayAverageOpenPrice),
			Legs:                     legs,
			TradeValueMultiplier:     mult,
			CreatedAt:                r.CreatedAt,
			UpdatedAt:                r.UpdatedAt,
		})
	}
	return &OptionPositionsResult{
		Count:     len(positions),
		Positions: positions,
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

func trimFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}
