package navitime

import (
	"encoding/json"
	"time"
)

// An absent snapshot has no source fetch time, rather than a year-0001 date.
func (m Metadata) MarshalJSON() ([]byte, error) {
	type plain Metadata
	var fetched *time.Time
	var sourceURL *string
	var age *float64
	if !m.FetchedAt.IsZero() {
		fetched = &m.FetchedAt
		age = &m.AgeSeconds
	}
	if m.SourceURL != "" {
		sourceURL = &m.SourceURL
	}
	return json.Marshal(struct {
		plain
		FetchedAt  *time.Time `json:"fetched_at"`
		SourceURL  *string    `json:"source_url"`
		AgeSeconds *float64   `json:"age_seconds"`
	}{plain(m), fetched, sourceURL, age})
}
