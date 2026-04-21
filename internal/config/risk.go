package config

import (
	"encoding/json"
	"errors"
	"os"
)

// Risk caps enforced by `rh trade`. Values are defaults if risk.json
// is missing; any field present in the file overrides its default.
type Risk struct {
	SchemaVersion   int     `json:"schema_version"`
	MaxSingleOrder  float64 `json:"max_single_order_usd"`
	MaxDailyTotal   float64 `json:"max_daily_total_usd"`
	MaxHourlyOrders int     `json:"max_hourly_order_count"`
	MaxPctPerSymbol float64 `json:"max_pct_per_symbol"` // 0 = disabled
	MaxPctPerSector float64 `json:"max_pct_per_sector"` // 0 = disabled
}

// DefaultRisk matches the previous hard-coded values in trade.go.
func DefaultRisk() Risk {
	return Risk{
		SchemaVersion:   1,
		MaxSingleOrder:  5000,
		MaxDailyTotal:   10000,
		MaxHourlyOrders: 5,
		MaxPctPerSymbol: 0,
		MaxPctPerSector: 0,
	}
}

// LoadRisk reads risk.json from ~/.config/rh. Missing file → defaults.
// Malformed file → error (fail-closed; we do NOT silently fall back so
// that a typo doesn't disable safety).
func LoadRisk() (Risk, error) {
	p, err := RiskPath()
	if err != nil {
		return Risk{}, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultRisk(), nil
		}
		return Risk{}, err
	}
	r := DefaultRisk() // start with defaults, then overlay
	if err := json.Unmarshal(b, &r); err != nil {
		return Risk{}, err
	}
	if r.SchemaVersion != 1 {
		return Risk{}, errors.New("unsupported risk schema_version; want 1")
	}
	return r, nil
}
