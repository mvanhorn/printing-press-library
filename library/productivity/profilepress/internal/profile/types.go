package profile

import "time"

// Section is a visible LinkedIn profile section captured into the local mirror.
type Section struct {
	Name       string `json:"name"`
	RawText    string `json:"raw_text"`
	Normalized string `json:"normalized"`
	Source     string `json:"source"`
}

// Snapshot is an immutable capture of profile sections at a point in time.
type Snapshot struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Source    string    `json:"source"`
	Sections  []Section `json:"sections"`
}
