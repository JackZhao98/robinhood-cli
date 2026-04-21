package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// IndexRegistry maps upper-case index symbols (e.g. "VIX") to Robinhood
// internal instrument UUIDs. Robinhood exposes no public symbol→UUID lookup
// for indexes, so users must sniff the UUID from the RH web/app once per
// index. We persist the mapping at ~/.config/rh/indexes.json.
type IndexRegistry struct {
	Indexes map[string]string `json:"indexes"`
}

func indexesPath() (string, error) {
	d, err := RHConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "indexes.json"), nil
}

// SeedIndexes is the factory default. Known-good UUIDs sniffed from the
// Robinhood web client. Add more here as they're discovered.
var SeedIndexes = map[string]string{
	"VIX": "3b912aa2-88f9-4682-8ae3-e39520bdf4db",
}

// LoadIndexRegistry reads ~/.config/rh/indexes.json, falling back to
// SeedIndexes if the file does not exist yet. On first read, the seed is
// persisted so users can see and edit it.
func LoadIndexRegistry() (*IndexRegistry, error) {
	p, err := indexesPath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		seed := &IndexRegistry{Indexes: cloneMap(SeedIndexes)}
		if err := saveIndexRegistry(seed); err != nil {
			return nil, err
		}
		return seed, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", p, err)
	}
	var reg IndexRegistry
	if err := json.Unmarshal(b, &reg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", p, err)
	}
	if reg.Indexes == nil {
		reg.Indexes = map[string]string{}
	}
	return &reg, nil
}

// Register persists SYMBOL→UUID into indexes.json. Overwrites if symbol
// already known.
func (r *IndexRegistry) Register(symbol, uuid string) error {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	uuid = strings.TrimSpace(uuid)
	if symbol == "" || uuid == "" {
		return fmt.Errorf("symbol and uuid required")
	}
	if r.Indexes == nil {
		r.Indexes = map[string]string{}
	}
	r.Indexes[symbol] = uuid
	return saveIndexRegistry(r)
}

// Lookup returns the UUID for a symbol, or ("", false) if not registered.
func (r *IndexRegistry) Lookup(symbol string) (string, bool) {
	id, ok := r.Indexes[strings.ToUpper(strings.TrimSpace(symbol))]
	return id, ok
}

func saveIndexRegistry(r *IndexRegistry) error {
	p, err := indexesPath()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o600)
}

func cloneMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
