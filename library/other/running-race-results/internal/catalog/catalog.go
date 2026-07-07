// internal/catalog/catalog.go
package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed catalog.json
var raw []byte

// Entry maps a known race edition to a provider + event identifier.
type Entry struct {
	Race     string   `json:"race"`
	Aliases  []string `json:"aliases"`
	Provider string   `json:"provider"`
	EventID  string   `json:"event_id"`
	Year     int      `json:"year"`
	BaseURL  string   `json:"base_url"`
}

// Load parses the embedded catalog.
func Load() ([]Entry, error) {
	var entries []Entry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("parse catalog: %w", err)
	}
	return entries, nil
}
