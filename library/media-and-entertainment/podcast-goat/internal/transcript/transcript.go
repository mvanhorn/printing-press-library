// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.
// PATCH: v0.1 canonical transcript shape (every adapter normalizes here).

package transcript

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Tier captures which dispatch lane an adapter sits in.
type Tier string

const (
	TierCookie Tier = "cookie"
	TierFree   Tier = "free"
	TierPaid   Tier = "paid"
)

// Segment is one speaker turn in canonical shape.
type Segment struct {
	TsSec   int    `json:"ts_sec"`
	Speaker string `json:"speaker"`
	Text    string `json:"text"`
}

// SectionMark is an h2/section header in the transcript (Dwarkesh H2 timestamps,
// spoken.md chapter markers). Optional. Survives round-trip to markdown.
type SectionMark struct {
	TsSec int    `json:"ts_sec"`
	Title string `json:"title"`
}

// Transcript is the canonical shape every adapter normalizes to.
type Transcript struct {
	ID                string        `json:"id"`
	Source            string        `json:"source"`
	Show              string        `json:"show"`
	Tier              Tier          `json:"tier"`
	URL               string        `json:"url"`
	Title             string        `json:"title"`
	Host              string        `json:"host"`
	Guests            []string      `json:"guests"`
	Published         string        `json:"published_at"`
	DurationSec       int           `json:"duration_sec"`
	Provider          string        `json:"provider"`
	CostCredits       float64       `json:"cost_credits"`
	Segments          []Segment     `json:"segments"`
	SectionTimestamps []SectionMark `json:"section_timestamps,omitempty"`
	FetchedAt         time.Time     `json:"fetched_at"`
}

// IDFor returns the canonical episode id for a URL (SHA-256 hex).
func IDFor(url string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(url)))
	return hex.EncodeToString(sum[:])
}

// FmtTime renders a ts_sec as MM:SS or HH:MM:SS depending on magnitude.
func FmtTime(ts int) string {
	if ts < 0 {
		ts = 0
	}
	h := ts / 3600
	m := (ts % 3600) / 60
	s := ts % 60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

// CanonicalMarkdown renders the speaker-labeled markdown.
//
//	**Speaker** (MM:SS)
//
//	body text
func (t *Transcript) CanonicalMarkdown() string {
	var b strings.Builder
	if t.Title != "" {
		b.WriteString("# ")
		b.WriteString(t.Title)
		b.WriteString("\n\n")
	}
	if t.Show != "" || t.Host != "" || len(t.Guests) > 0 {
		if t.Show != "" {
			b.WriteString("**Show:** ")
			b.WriteString(t.Show)
			b.WriteString("  \n")
		}
		if t.Host != "" {
			b.WriteString("**Host:** ")
			b.WriteString(t.Host)
			b.WriteString("  \n")
		}
		if len(t.Guests) > 0 {
			b.WriteString("**Guests:** ")
			b.WriteString(strings.Join(t.Guests, ", "))
			b.WriteString("  \n")
		}
		if t.URL != "" {
			b.WriteString("**URL:** ")
			b.WriteString(t.URL)
			b.WriteString("  \n")
		}
		if t.Provider != "" {
			b.WriteString("**Provider:** ")
			b.WriteString(t.Provider)
			b.WriteString("  \n")
		}
		b.WriteString("\n")
	}

	// Interleave section marks with segments when both are present.
	secs := make([]SectionMark, len(t.SectionTimestamps))
	copy(secs, t.SectionTimestamps)
	si := 0
	for _, seg := range t.Segments {
		for si < len(secs) && secs[si].TsSec <= seg.TsSec {
			b.WriteString("## ")
			b.WriteString(secs[si].Title)
			b.WriteString(" (")
			b.WriteString(FmtTime(secs[si].TsSec))
			b.WriteString(")\n\n")
			si++
		}
		b.WriteString("**")
		b.WriteString(seg.Speaker)
		b.WriteString("** (")
		b.WriteString(FmtTime(seg.TsSec))
		b.WriteString(")\n\n")
		b.WriteString(strings.TrimSpace(seg.Text))
		b.WriteString("\n\n")
	}
	for si < len(secs) {
		b.WriteString("## ")
		b.WriteString(secs[si].Title)
		b.WriteString(" (")
		b.WriteString(FmtTime(secs[si].TsSec))
		b.WriteString(")\n\n")
		si++
	}
	return b.String()
}

// PlainText renders an unstructured text view (no markdown).
func (t *Transcript) PlainText() string {
	var b strings.Builder
	for _, seg := range t.Segments {
		b.WriteString(seg.Speaker)
		b.WriteString(" (")
		b.WriteString(FmtTime(seg.TsSec))
		b.WriteString("): ")
		b.WriteString(strings.TrimSpace(seg.Text))
		b.WriteString("\n\n")
	}
	return b.String()
}

// JSONL renders one segment per line.
func (t *Transcript) JSONL() string {
	var b strings.Builder
	for _, seg := range t.Segments {
		out, _ := json.Marshal(seg)
		b.Write(out)
		b.WriteByte('\n')
	}
	return b.String()
}

// JSON renders the full transcript as pretty JSON.
func (t *Transcript) JSON() string {
	out, _ := json.MarshalIndent(t, "", "  ")
	return string(out)
}

// Speakers returns the distinct ordered list of speakers in the transcript.
func (t *Transcript) Speakers() []string {
	seen := map[string]bool{}
	var out []string
	for _, seg := range t.Segments {
		if seg.Speaker == "" || seen[seg.Speaker] {
			continue
		}
		seen[seg.Speaker] = true
		out = append(out, seg.Speaker)
	}
	return out
}

// TokenEstimate is a coarse estimator: ~4 chars/token.
func (t *Transcript) TokenEstimate() int {
	total := 0
	for _, seg := range t.Segments {
		total += len(seg.Text)
	}
	return total / 4
}
