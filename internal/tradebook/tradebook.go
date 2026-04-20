// Package tradebook maintains the master CSV of all executed trades.
//
// CSV schema(18 columns, pro-trader format):
//
//	Date,Time,Acct,Symbol,Side,Qty,Price,Type,TIF,Strike,Expiry,CP,
//	Notional,Effect,Status,Mode,OrderID,Notes
package tradebook

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TradesCSVPath returns the path to the master trades CSV.
// Override with env var ROBINHOOD_TRADES_CSV for testing.
func TradesCSVPath() string {
	if p := os.Getenv("ROBINHOOD_TRADES_CSV"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Developer", "Robinhood", "tradebook", "trades.csv")
}

// Header is the exact 18-column header the CSV uses.
var Header = []string{
	"Date", "Time", "Acct", "Symbol", "Side", "Qty", "Price", "Type", "TIF",
	"Strike", "Expiry", "CP", "Notional", "Effect", "Status", "Mode", "OrderID", "Notes",
}

// Trade is a single row of the tradebook. Equity trades leave Strike/Expiry/CP empty.
type Trade struct {
	Date     string // YYYY-MM-DD
	Time     string // HH:MM:SS
	Acct     string // Trading | Roth | Paper
	Symbol   string
	Side     string // BTO | STO | BTC | STC
	Qty      string // decimal as string to preserve precision
	Price    string // decimal as string
	Type     string // MKT | LMT | STOP | STP-LMT
	TIF      string // GFD | GTC | FOK | IOC
	Strike   string // empty for equity
	Expiry   string // empty for equity
	CP       string // C | P | "" for equity
	Notional string
	Effect   string // BUY | SELL | DR | CR
	Status   string // QUEUED | OPEN | FILLED | PART | CXL | REJ | EXP
	Mode     string // REAL | PAPER
	OrderID  string
	Notes    string
}

// Row flattens t into the 18-column order.
func (t Trade) Row() []string {
	return []string{
		t.Date, t.Time, t.Acct, t.Symbol, t.Side, t.Qty, t.Price, t.Type, t.TIF,
		t.Strike, t.Expiry, t.CP, t.Notional, t.Effect, t.Status, t.Mode, t.OrderID, t.Notes,
	}
}

// Append writes t to the tradebook CSV, creating the file + header if needed.
func Append(t Trade) error {
	path := TradesCSVPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir tradebook: %w", err)
	}

	// Create with header if missing.
	fileExists := true
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fileExists = false
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open tradebook: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if !fileExists {
		if err := w.Write(Header); err != nil {
			return fmt.Errorf("write header: %w", err)
		}
	}
	if err := w.Write(t.Row()); err != nil {
		return fmt.Errorf("append row: %w", err)
	}
	return nil
}

// Read returns all trades from the CSV (skipping header).
func Read() ([]Trade, error) {
	path := TradesCSVPath()
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open tradebook: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read tradebook: %w", err)
	}
	if len(rows) <= 1 {
		return nil, nil
	}

	out := make([]Trade, 0, len(rows)-1)
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		if len(row) < len(Header) {
			continue // skip malformed
		}
		out = append(out, Trade{
			Date: row[0], Time: row[1], Acct: row[2], Symbol: row[3], Side: row[4],
			Qty: row[5], Price: row[6], Type: row[7], TIF: row[8],
			Strike: row[9], Expiry: row[10], CP: row[11],
			Notional: row[12], Effect: row[13], Status: row[14],
			Mode: row[15], OrderID: row[16], Notes: row[17],
		})
	}
	return out, nil
}

// Now returns ("YYYY-MM-DD", "HH:MM:SS") in local time for CSV timestamps.
func Now() (string, string) {
	t := time.Now()
	return t.Format("2006-01-02"), t.Format("15:04:05")
}

// NormalizeState converts Robinhood API state values to pro-trader status codes.
func NormalizeState(state string) string {
	switch strings.ToLower(state) {
	case "queued":
		return "QUEUED"
	case "confirmed", "unconfirmed", "partially_filled":
		if state == "partially_filled" {
			return "PART"
		}
		return "OPEN"
	case "filled":
		return "FILLED"
	case "cancelled", "canceled":
		return "CXL"
	case "rejected":
		return "REJ"
	case "expired":
		return "EXP"
	default:
		return strings.ToUpper(state)
	}
}
