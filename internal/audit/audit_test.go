package audit

import (
	"bufio"
	"encoding/json"
	"os"
	"testing"

	"github.com/jackzhao/robinhood-cli/internal/config"
)

func TestTrade_AppendsLine(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	rec := TradeRecord{
		OrderID:  "abc",
		Account:  "597357623",
		Symbol:   "VOO",
		Side:     "buy",
		Type:     "MKT",
		TIF:      "GFD",
		Shares:   "0.077",
		Price:    "489.12",
		Notional: "37.66",
		State:    "confirmed",
	}
	if err := Trade(rec); err != nil {
		t.Fatal(err)
	}
	if err := Trade(rec); err != nil {
		t.Fatal(err)
	}
	p, _ := config.TradesLogPath()
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	lines := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines++
		var got TradeRecord
		if err := json.Unmarshal(sc.Bytes(), &got); err != nil {
			t.Fatalf("bad json: %v", err)
		}
		if got.Symbol != "VOO" {
			t.Errorf("got symbol %q", got.Symbol)
		}
		if got.Timestamp == "" {
			t.Errorf("missing timestamp")
		}
	}
	if lines != 2 {
		t.Errorf("want 2 lines, got %d", lines)
	}
}

func TestCommand_AppendsLine(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := Command(CommandRecord{Argv: []string{"rh", "version"}, ExitCode: 0, DurMs: 5}); err != nil {
		t.Fatal(err)
	}
	p, _ := config.AuditLogPath()
	b, _ := os.ReadFile(p)
	if len(b) == 0 {
		t.Fatal("audit log empty")
	}
	var rec CommandRecord
	if err := json.Unmarshal(b[:len(b)-1], &rec); err != nil {
		t.Fatal(err)
	}
	if rec.Argv[1] != "version" {
		t.Errorf("got argv %v", rec.Argv)
	}
	if rec.Timestamp == "" {
		t.Error("missing ts")
	}
}

func TestTrade_TimestampPreserved(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	rec := TradeRecord{
		Timestamp: "2026-04-20T12:00:00Z",
		Symbol:    "AAPL",
		Side:      "sell",
	}
	if err := Trade(rec); err != nil {
		t.Fatal(err)
	}
	p, _ := config.TradesLogPath()
	b, _ := os.ReadFile(p)
	var got TradeRecord
	if err := json.Unmarshal(b[:len(b)-1], &got); err != nil {
		t.Fatal(err)
	}
	if got.Timestamp != "2026-04-20T12:00:00Z" {
		t.Errorf("timestamp overwritten: %q", got.Timestamp)
	}
}
