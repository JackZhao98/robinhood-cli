package config

import (
	"os"
	"path/filepath"
)

// RHConfigDir is XDG-style ~/.config/rh, created on demand.
// Kept separate from the legacy ConfigDir() (~/.robinhood-cli) so
// credentials are never moved — this dir holds risk/trades/audit only.
func RHConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "rh")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func RiskPath() (string, error) {
	d, err := RHConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "risk.json"), nil
}

func TradesLogPath() (string, error) {
	d, err := RHConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "trades.jsonl"), nil
}

func AuditLogPath() (string, error) {
	d, err := RHConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "audit.jsonl"), nil
}
