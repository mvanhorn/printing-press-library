// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package uberjobs

// Meta describes where an envelope's results came from.
type Meta struct {
	Source       string            `json:"source"`
	Command      string            `json:"command"`
	Fallback     bool              `json:"fallback"`
	FallbackFrom string            `json:"fallback_from,omitempty"`
	Complete     bool              `json:"complete"`
	Requests     int               `json:"requests"`
	Note         string            `json:"note,omitempty"`
	Cache        map[string]string `json:"cache,omitempty"`
	Unmapped     []string          `json:"unmapped_countries,omitempty"`
	Extra        map[string]any    `json:"extra,omitempty"`
}

// Envelope is the one top-level JSON object every command emits, for every
// data source: {meta:{source}, hits, returned, scanned, scan_cap_hit, results}.
type Envelope struct {
	Meta       Meta `json:"meta"`
	Hits       int  `json:"hits"`
	Returned   int  `json:"returned"`
	Scanned    int  `json:"scanned"`
	ScanCapHit bool `json:"scan_cap_hit"`
	Results    any  `json:"results"`
}

// NewEnvelope builds an envelope; results must already be a non-nil slice.
func NewEnvelope(command, source string, results any, returned int) Envelope {
	return Envelope{Meta: Meta{Source: source, Command: command}, Results: results, Returned: returned}
}
