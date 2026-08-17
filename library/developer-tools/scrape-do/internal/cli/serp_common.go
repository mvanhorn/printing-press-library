// Copyright 2026 Charles Garrison and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored shared helpers for the governor + SERP commands: output
// emission, SERP param-hash canonicalization, organic-result extraction, and
// relative-time parsing. Hand file (no generator header) so it survives regen.

package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-do/internal/store"
	"github.com/spf13/cobra"
)

// emitGov prints JSON when --json/--agent is set, otherwise the supplied text.
func emitGov(cmd *cobra.Command, flags *rootFlags, v any, text string) error {
	if flags.asJSON {
		return flags.printJSON(cmd, v)
	}
	fmt.Fprintln(cmd.OutOrStdout(), text)
	return nil
}

// serpParamHash canonicalizes the identity parameters of a Google SERP into a
// stable hash so `drift`/`movers` can group snapshots of the same query+locale.
// Pagination/result-count params are intentionally excluded — they don't change
// the query identity.
func serpParamHash(query, gl, hl, googleDomain, device string) string {
	norm := func(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
	gd := norm(googleDomain)
	if gd == "" {
		gd = "google.com"
	}
	dev := norm(device)
	if dev == "" {
		dev = "desktop"
	}
	key := strings.Join([]string{norm(query), norm(gl), norm(hl), gd, dev}, "\x1f")
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// extractOrganic pulls the flattened organic results out of a Scrape.do Google
// SERP JSON payload. Every accessor is defensive: the documented shape is
// {position,title,link,source,snippet} but fields may be absent.
func extractOrganic(raw []byte) []store.OrganicRow {
	var resp struct {
		OrganicResults []struct {
			Position int    `json:"position"`
			Title    string `json:"title"`
			Link     string `json:"link"`
			Source   string `json:"source"`
			Snippet  string `json:"snippet"`
		} `json:"organic_results"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil
	}
	out := make([]store.OrganicRow, 0, len(resp.OrganicResults))
	for i, r := range resp.OrganicResults {
		pos := r.Position
		if pos == 0 {
			pos = i + 1
		}
		domain := strings.ToLower(strings.TrimSpace(r.Source))
		if domain == "" {
			domain = hostOf(r.Link)
		}
		out = append(out, store.OrganicRow{
			Position: pos,
			Title:    r.Title,
			Link:     r.Link,
			Domain:   domain,
			Snippet:  r.Snippet,
		})
	}
	return out
}

// hostOf returns the lowercased host of a URL, or "" on parse failure.
func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimPrefix(u.Hostname(), "www."))
}

// parseSinceDur parses a relative-time window: "today", "month", "Nd" (days),
// "Nh" (hours), "Nw" (weeks), or any Go duration. Returns the cutoff time.
func parseSinceDur(s string) (time.Time, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	now := time.Now().UTC()
	switch s {
	case "", "all":
		return time.Time{}, nil
	case "today":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), nil
	case "month":
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC), nil
	}
	// Nd / Nw shorthands that time.ParseDuration doesn't accept.
	if strings.HasSuffix(s, "d") {
		if n, err := atoiTrim(strings.TrimSuffix(s, "d")); err == nil {
			return now.Add(-time.Duration(n) * 24 * time.Hour), nil
		}
	}
	if strings.HasSuffix(s, "w") {
		if n, err := atoiTrim(strings.TrimSuffix(s, "w")); err == nil {
			return now.Add(-time.Duration(n) * 7 * 24 * time.Hour), nil
		}
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid --since %q: use today, month, 7d, 24h, 2w, or a Go duration", s)
	}
	return now.Add(-d), nil
}

func atoiTrim(s string) (int, error) {
	s = strings.TrimSpace(s)
	n := 0
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("not a number: %q", s)
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}
