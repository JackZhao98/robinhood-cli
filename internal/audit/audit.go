// Package audit writes append-only JSONL logs under ~/.config/rh.
package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/jackzhao/robinhood-cli/internal/config"
)

type TradeRecord struct {
	Timestamp string `json:"ts"`
	OrderID   string `json:"rh_order_id,omitempty"`
	Account   string `json:"account_number"`
	Symbol    string `json:"symbol"`
	Side      string `json:"side"`
	Type      string `json:"type"`
	TIF       string `json:"tif"`
	Shares    string `json:"shares"`
	Price     string `json:"price"`
	Notional  string `json:"notional_usd"`
	State     string `json:"state"`
	Notes     string `json:"notes,omitempty"`
}

type CommandRecord struct {
	Timestamp string   `json:"ts"`
	Argv      []string `json:"argv"`
	ExitCode  int      `json:"exit_code"`
	DurMs     int64    `json:"duration_ms"`
}

func appendJSONL(path string, v any) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal jsonl record: %w", err)
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return f.Sync()
}

// Trade appends one line to ~/.config/rh/trades.jsonl.
func Trade(rec TradeRecord) error {
	if rec.Timestamp == "" {
		rec.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	p, err := config.TradesLogPath()
	if err != nil {
		return fmt.Errorf("trades log path: %w", err)
	}
	return appendJSONL(p, rec)
}

// Command appends one line to ~/.config/rh/audit.jsonl.
func Command(rec CommandRecord) error {
	if rec.Timestamp == "" {
		rec.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	p, err := config.AuditLogPath()
	if err != nil {
		return fmt.Errorf("audit log path: %w", err)
	}
	return appendJSONL(p, rec)
}
