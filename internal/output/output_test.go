package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestEmitTableRendersKeyValueBox(t *testing.T) {
	var buf bytes.Buffer
	payload := map[string]any{
		"name":  "NVDA",
		"price": 199.745,
		"pe":    41.2544,
	}
	if err := emitTable(&buf, payload); err != nil {
		t.Fatalf("emitTable error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "price") {
		t.Fatalf("expected field name in table, got:\n%s", out)
	}
	if !strings.Contains(out, "╭") && !strings.Contains(out, "┌") {
		t.Fatalf("expected boxed table output, got:\n%s", out)
	}
}

func TestEmitTableReshapesQuoteDetails(t *testing.T) {
	var buf bytes.Buffer
	payload := map[string]any{
		"symbol":              "SOFI",
		"current_price":       19.134,
		"last_price":          19.134,
		"previous_close":      19.5,
		"previous_close_date": "2026-04-20",
		"day_change":          -0.366,
		"day_change_pct":      -1.876923,
		"bid":                 19.13,
		"ask":                 19.14,
		"open":                19.51,
	}
	if err := emitTable(&buf, payload); err != nil {
		t.Fatalf("emitTable error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "today_change") {
		t.Fatalf("expected synthesized today_change row, got:\n%s", out)
	}
	if !strings.Contains(out, "-0.3660 (-1.88%)") {
		t.Fatalf("expected combined day change display, got:\n%s", out)
	}
	if strings.Contains(out, "last_price") {
		t.Fatalf("expected quote details view to hide raw last_price, got:\n%s", out)
	}
}

func TestEmitTableRendersSliceTable(t *testing.T) {
	var buf bytes.Buffer
	payload := map[string]any{
		"timestamp": "2026-04-21 10:50:00",
		"accounts": []any{
			map[string]any{"nickname": "Trading", "cash": 7390.38, "portfolio_value": 8035.8022},
			map[string]any{"nickname": "Roth", "cash": 3933.33, "portfolio_value": 8890.2071},
		},
	}
	if err := emitTable(&buf, payload); err != nil {
		t.Fatalf("emitTable error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "accounts (2 rows)") {
		t.Fatalf("expected slice title, got:\n%s", out)
	}
	if !strings.Contains(out, "PORTFOLIO_VALUE") {
		t.Fatalf("expected column in table, got:\n%s", out)
	}
	if !strings.Contains(out, "summary") {
		t.Fatalf("expected scalar summary table, got:\n%s", out)
	}
}

func TestEmitTableReshapesOptionChainTable(t *testing.T) {
	var buf bytes.Buffer
	payload := map[string]any{
		"symbol":          "QQQ",
		"expiration_date": "2026-05-22",
		"option_type":     "call",
		"side":            "buy",
		"count":           float64(1),
		"options": []any{
			map[string]any{
				"instrument_id":          "abc-123",
				"strike":                 530.0,
				"bid":                    3.0,
				"ask":                    3.4,
				"last":                   3.0,
				"delta":                  0.7953,
				"gamma":                  0.0706,
				"iv":                     0.2123,
				"volume":                 120.0,
				"open_interest":          450.0,
				"chance_of_profit_long":  0.4096,
				"chance_of_profit_short": 0.5904,
			},
		},
	}
	if err := emitTable(&buf, payload); err != nil {
		t.Fatalf("emitTable error: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "instrument_id") || strings.Contains(out, "INSTRUMENT_ID") {
		t.Fatalf("instrument_id should be hidden in option table, got:\n%s", out)
	}
	if !strings.Contains(out, "CALL_PRICE") {
		t.Fatalf("expected call price column, got:\n%s", out)
	}
	if !strings.Contains(out, "BREAK_EVEN") {
		t.Fatalf("expected break-even column, got:\n%s", out)
	}
	if !strings.Contains(out, "ASK_MOVE_PCT") {
		t.Fatalf("expected ask move pct column, got:\n%s", out)
	}
}
