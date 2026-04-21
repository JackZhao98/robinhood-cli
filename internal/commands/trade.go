package commands

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jackzhao/robinhood-cli/internal/client"
	"github.com/jackzhao/robinhood-cli/internal/config"
	"github.com/jackzhao/robinhood-cli/internal/output"
	"github.com/jackzhao/robinhood-cli/internal/tradebook"
)

// ─────────────────────────────────────────────────────────────────────────
//  Safety limits are loaded from ~/.config/rh/risk.json (see config.LoadRisk).
//  Account numbers remain hard-coded — they aren't safety caps.
// ─────────────────────────────────────────────────────────────────────────

const (
	defaultAccountNum = "597357623" // Trading (individual margin)
	rothAccountNum    = "647360304"
)

// acctShort returns the short label used in trades.csv for a given account num.
func acctShort(num string) string {
	switch num {
	case defaultAccountNum:
		return "Trading"
	case rothAccountNum:
		return "Roth"
	default:
		return num
	}
}

// ─────────────────────────────────────────────────────────────────────────
//  Command tree.
// ─────────────────────────────────────────────────────────────────────────

func newTradeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trade",
		Short: "Place / preview real equity orders with safety guards",
		Long: `Place real equity orders (or preview without sending).

Safety:
  - Default is preview-only; add --execute to actually submit.
  - Single order ≤ $5,000; daily total ≤ $10,000; max 5 orders/hour.
  - Every submitted order is appended to ~/Developer/Robinhood/tradebook/trades.csv.`,
	}
	cmd.AddCommand(newTradeBuyCmd(), newTradeSellCmd(), newTradeCancelCmd())
	return cmd
}

// Shared flag holder so buy/sell share the same parsing.
type tradeFlags struct {
	limit   float64
	tif     string
	account string
	notes   string
	execute bool
	shares  bool
}

func attachTradeFlags(cmd *cobra.Command, f *tradeFlags) {
	cmd.Flags().Float64Var(&f.limit, "limit", 0, "Limit price (default: market order)")
	cmd.Flags().StringVar(&f.tif, "tif", "gfd", "Time in force: gfd | gtc | ioc | fok")
	cmd.Flags().StringVar(&f.account, "account", defaultAccountNum,
		"Account number (default: Trading individual margin)")
	cmd.Flags().StringVar(&f.notes, "notes", "", "Free-form notes saved to trades.csv")
	cmd.Flags().BoolVar(&f.execute, "execute", false,
		"Actually submit the order. Without this flag the command only previews.")
	cmd.Flags().BoolVar(&f.shares, "shares", false,
		"Interpret AMOUNT as number of shares instead of dollars")
}

func newTradeBuyCmd() *cobra.Command {
	var f tradeFlags
	cmd := &cobra.Command{
		Use:   "buy SYMBOL AMOUNT",
		Short: "Buy equity (dollar amount by default, --shares for share count)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTrade("buy", args[0], args[1], f)
		},
	}
	attachTradeFlags(cmd, &f)
	return cmd
}

func newTradeSellCmd() *cobra.Command {
	var f tradeFlags
	cmd := &cobra.Command{
		Use:   "sell SYMBOL AMOUNT",
		Short: "Sell equity (shares by default for sell — use plain numbers)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// For sells, default to shares (safer — you sell what you own).
			if !cmd.Flags().Changed("shares") {
				f.shares = true
			}
			return runTrade("sell", args[0], args[1], f)
		},
	}
	attachTradeFlags(cmd, &f)
	return cmd
}

func newTradeCancelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cancel ORDER_ID",
		Short: "Cancel an open/queued order",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New()
			if err != nil {
				return err
			}
			url := client.URL("/orders/"+args[0]+"/cancel/", nil)
			var resp map[string]any
			status, err := c.PostJSON(url, map[string]any{}, &resp)
			if err != nil {
				return fmt.Errorf("cancel: %w", err)
			}
			if status < 200 || status >= 300 {
				return fmt.Errorf("cancel rejected (http %d): %v", status, resp)
			}
			return output.Emit(map[string]any{
				"cancelled":  true,
				"order_id":   args[0],
				"http":       status,
			})
		},
	}
	return cmd
}

// ─────────────────────────────────────────────────────────────────────────
//  Core trade execution logic.
// ─────────────────────────────────────────────────────────────────────────

type tradePreview struct {
	Action        string  `json:"action"`
	Account       string  `json:"account"`
	Symbol        string  `json:"symbol"`
	OrderType     string  `json:"order_type"`
	TIF           string  `json:"tif"`
	Shares        string  `json:"shares"`
	RefPrice      float64 `json:"reference_price"`
	EstimatedCost float64 `json:"estimated_cost"`
	Bid           float64 `json:"bid"`
	Ask           float64 `json:"ask"`
	Last          float64 `json:"last"`
	DailyUsed     float64 `json:"daily_used_usd"`
	DailyLimit    float64 `json:"daily_limit_usd"`
	HourlyCount   int     `json:"hourly_order_count"`
	HourlyLimit   int     `json:"hourly_order_limit"`
	Mode          string  `json:"mode"`
}

type tradeResult struct {
	tradePreview
	Executed bool   `json:"executed"`
	OrderID  string `json:"order_id,omitempty"`
	State    string `json:"state,omitempty"`
	Notes    string `json:"notes,omitempty"`
	Error    string `json:"error,omitempty"`
}

func runTrade(action, symbol, amountStr string, f tradeFlags) error {
	symbol = strings.ToUpper(symbol)
	tif := strings.ToLower(f.tif)
	if tif != "gfd" && tif != "gtc" && tif != "ioc" && tif != "fok" && tif != "day" {
		return fmt.Errorf("invalid --tif %q (want gfd | gtc | ioc | fok)", f.tif)
	}
	if tif == "day" {
		tif = "gfd"
	}

	risk, err := config.LoadRisk()
	if err != nil {
		return fmt.Errorf("risk config: %w", err)
	}

	c, err := client.New()
	if err != nil {
		return err
	}

	// Fetch instrument + quote.
	inst, err := c.InstrumentBySymbol(symbol)
	if err != nil {
		return err
	}
	quote, err := c.QuoteSnapshot(inst.ID)
	if err != nil {
		return err
	}
	bid, _ := strconv.ParseFloat(quote.BidPrice, 64)
	ask, _ := strconv.ParseFloat(quote.AskPrice, 64)
	last, _ := strconv.ParseFloat(quote.LastTradePrice, 64)

	refPrice := ask
	if action == "sell" {
		refPrice = bid
	}
	if f.limit > 0 {
		refPrice = f.limit
	}

	// Resolve shares / dollar amount.
	amount, err := strconv.ParseFloat(strings.TrimPrefix(amountStr, "$"), 64)
	if err != nil {
		return fmt.Errorf("invalid AMOUNT %q: %w", amountStr, err)
	}
	var sharesF, dollarAmt float64
	if f.shares {
		sharesF = amount
		dollarAmt = sharesF * refPrice
	} else {
		dollarAmt = amount
		sharesF = dollarAmt / refPrice
	}
	sharesF = roundTo(sharesF, 6)
	dollarAmt = roundTo(sharesF*refPrice, 2)

	// Safety: read tradebook to compute daily / hourly usage.
	dailyUsed, hourlyCnt, err := computeUsage()
	if err != nil {
		return err
	}
	if dollarAmt > risk.MaxSingleOrder {
		return fmt.Errorf("order $%.2f exceeds single-order cap $%.0f", dollarAmt, risk.MaxSingleOrder)
	}
	if f.execute && dailyUsed+dollarAmt > risk.MaxDailyTotal {
		return fmt.Errorf("would exceed daily cap: used $%.2f + $%.2f > $%.0f",
			dailyUsed, dollarAmt, risk.MaxDailyTotal)
	}
	if f.execute && hourlyCnt >= risk.MaxHourlyOrders {
		return fmt.Errorf("hourly order count %d has hit cap %d — wait before next order",
			hourlyCnt, risk.MaxHourlyOrders)
	}

	preview := tradePreview{
		Action:        action,
		Account:       fmt.Sprintf("%s (%s)", acctShort(f.account), f.account),
		Symbol:        symbol,
		OrderType:     ifStr(f.limit > 0, fmt.Sprintf("LMT @ $%.2f", f.limit), "MKT"),
		TIF:           strings.ToUpper(tif),
		Shares:        strconv.FormatFloat(sharesF, 'f', -1, 64),
		RefPrice:      refPrice,
		EstimatedCost: dollarAmt,
		Bid:           bid,
		Ask:           ask,
		Last:          last,
		DailyUsed:     dailyUsed,
		DailyLimit:    risk.MaxDailyTotal,
		HourlyCount:   hourlyCnt,
		HourlyLimit:   risk.MaxHourlyOrders,
		Mode:          ifStr(f.execute, "EXECUTE", "PREVIEW-ONLY"),
	}

	if !f.execute {
		return output.Emit(tradeResult{
			tradePreview: preview,
			Executed:     false,
			Notes:        "Preview only — re-run with --execute to submit.",
		})
	}

	// Build + send the real order.
	req := client.EquityOrderRequest{
		Account:     client.AccountURL(f.account),
		Instrument:  inst.URL,
		Symbol:      symbol,
		TimeInForce: tif,
		Trigger:     "immediate",
		Quantity:    strconv.FormatFloat(sharesF, 'f', -1, 64),
		Price:       strconv.FormatFloat(refPrice, 'f', 2, 64),
		Side:        action,
		RefID:       fmt.Sprintf("rh-cli-%d", time.Now().UnixMilli()),
	}
	if f.limit > 0 {
		req.Type = "limit"
	} else {
		req.Type = "market"
		// Dollar-based only for buy market orders (fractional share support).
		if action == "buy" {
			req.DollarBased = &client.DollarAmount{
				Amount:       strconv.FormatFloat(dollarAmt, 'f', 2, 64),
				CurrencyCode: "USD",
			}
		}
	}

	placed, err := c.PlaceEquityOrder(req)
	if err != nil {
		return err
	}

	// Append to tradebook CSV.
	date, clock := tradebook.Now()
	side := "BTO"
	effect := "BUY"
	if action == "sell" {
		side = "STC"
		effect = "SELL"
	}
	row := tradebook.Trade{
		Date:     date,
		Time:     clock,
		Acct:     acctShort(f.account),
		Symbol:   symbol,
		Side:     side,
		Qty:      strconv.FormatFloat(sharesF, 'f', -1, 64),
		Price:    strconv.FormatFloat(refPrice, 'f', 4, 64),
		Type:     ifStr(f.limit > 0, "LMT", "MKT"),
		TIF:      strings.ToUpper(tif),
		Notional: strconv.FormatFloat(dollarAmt, 'f', 2, 64),
		Effect:   effect,
		Status:   tradebook.NormalizeState(placed.State),
		Mode:     "REAL",
		OrderID:  placed.ID,
		Notes:    f.notes,
	}
	if err := tradebook.Append(row); err != nil {
		// Don't fail the whole command if CSV write fails — order already sent.
		fmt.Fprintf(os.Stderr, "warning: tradebook append failed: %v\n", err)
	}

	return output.Emit(tradeResult{
		tradePreview: preview,
		Executed:     true,
		OrderID:      placed.ID,
		State:        placed.State,
		Notes:        f.notes,
	})
}

// ─────────────────────────────────────────────────────────────────────────
//  Helpers.
// ─────────────────────────────────────────────────────────────────────────

func roundTo(v float64, decimals int) float64 {
	mul := 1.0
	for i := 0; i < decimals; i++ {
		mul *= 10
	}
	return float64(int64(v*mul+0.5)) / mul
}

func ifStr(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

// computeUsage reads trades.csv and sums REAL orders from today + counts
// orders in the last 60 minutes, to enforce rate limits.
func computeUsage() (float64, int, error) {
	trades, err := tradebook.Read()
	if err != nil {
		return 0, 0, err
	}
	now := time.Now()
	today := now.Format("2006-01-02")
	cutoff := now.Add(-time.Hour)

	var dailyUSD float64
	var hourlyCnt int
	for _, t := range trades {
		if t.Mode != "REAL" {
			continue
		}
		if t.Status == "REJ" || t.Status == "CXL" {
			continue
		}
		if t.Date == today {
			if v, err := strconv.ParseFloat(t.Notional, 64); err == nil {
				dailyUSD += v
			}
		}
		// hourly
		ts, err := time.ParseInLocation("2006-01-02 15:04:05", t.Date+" "+t.Time, now.Location())
		if err == nil && ts.After(cutoff) {
			hourlyCnt++
		}
	}
	return dailyUSD, hourlyCnt, nil
}
