package config

import (
	"os"
	"path/filepath"
	"testing"
)

func withTempHome(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
}

func TestLoadRisk_MissingReturnsDefaults(t *testing.T) {
	withTempHome(t)
	r, err := LoadRisk()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r != DefaultRisk() {
		t.Errorf("expected defaults, got %+v", r)
	}
}

func TestLoadRisk_OverridesDefaults(t *testing.T) {
	withTempHome(t)
	p, err := RiskPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(`{"schema_version":1,"max_single_order_usd":123}`), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := LoadRisk()
	if err != nil {
		t.Fatal(err)
	}
	if r.MaxSingleOrder != 123 {
		t.Errorf("want 123, got %v", r.MaxSingleOrder)
	}
	// Untouched fields should keep defaults.
	if r.MaxDailyTotal != DefaultRisk().MaxDailyTotal {
		t.Errorf("daily total should fall back to default, got %v", r.MaxDailyTotal)
	}
	if r.MaxHourlyOrders != DefaultRisk().MaxHourlyOrders {
		t.Errorf("hourly orders should fall back to default, got %v", r.MaxHourlyOrders)
	}
}

func TestLoadRisk_ErrorCases(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"malformed json", `{not json`},
		{"unknown version", `{"schema_version":99}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withTempHome(t)
			p, err := RiskPath()
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadRisk(); err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}

func TestDefaultRisk_MatchesLegacyConstants(t *testing.T) {
	d := DefaultRisk()
	if d.MaxSingleOrder != 5000 {
		t.Errorf("MaxSingleOrder: want 5000, got %v", d.MaxSingleOrder)
	}
	if d.MaxDailyTotal != 10000 {
		t.Errorf("MaxDailyTotal: want 10000, got %v", d.MaxDailyTotal)
	}
	if d.MaxHourlyOrders != 5 {
		t.Errorf("MaxHourlyOrders: want 5, got %v", d.MaxHourlyOrders)
	}
	if d.SchemaVersion != 1 {
		t.Errorf("SchemaVersion: want 1, got %v", d.SchemaVersion)
	}
}
