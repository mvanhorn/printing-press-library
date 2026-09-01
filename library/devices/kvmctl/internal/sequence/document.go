package sequence

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// ParseDocument parses the stable JSON representation used by the CLI.
func ParseDocument(data []byte) (Plan, error) {
	var raw struct {
		Target                 string   `json:"target"`
		Actions                []Action `json:"actions"`
		MaxDurationMS          int      `json:"max_duration_ms"`
		UnexpectedScreenPolicy string   `json:"unexpected_screen_policy"`
	}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return Plan{}, fmt.Errorf("invalid sequence JSON: %w", err)
	}
	if raw.MaxDurationMS == 0 {
		raw.MaxDurationMS = 30000
	}
	p := Plan{Target: raw.Target, Actions: raw.Actions, MaxDuration: time.Duration(raw.MaxDurationMS) * time.Millisecond, UnexpectedScreenPolicy: raw.UnexpectedScreenPolicy}
	if err := p.validate(); err != nil {
		return Plan{}, err
	}
	return p, nil
}

// Validate checks a plan without performing any device operation.
func Validate(p Plan) error { return p.validate() }

// MarshalDocument returns the redaction-free plan document for local storage.
func MarshalDocument(p Plan) ([]byte, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		Target        string   `json:"target"`
		Actions       []Action `json:"actions"`
		MaxDurationMS int64    `json:"max_duration_ms"`
	}{p.Target, p.Actions, p.MaxDuration.Milliseconds()})
}

func ReadDocument(path string) (Plan, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return Plan{}, e
	}
	return ParseDocument(b)
}
