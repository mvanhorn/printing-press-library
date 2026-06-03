// Copyright 2026 Jan Medina and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored note parser for Perfumenz body_html (extracts Top/Heart/Base from the "Fragrance Notes" section).
// Used by sync (to enrich stored items) and by novel commands (overlap, recommend, etc).

package cli

import (
	"regexp"
	"strings"
)

var noteSectionRE = regexp.MustCompile(`(?i)(Top Notes?|Heart Notes?|Base Notes?|Middle Notes?)\s*[:\-–]\s*([^\n\r.]+)`)

// ParseNotes extracts normalized lower-case note tokens from the common "Fragrance Notes: Top Notes: a, b. Heart Notes: ..." text in body_html.
func ParseNotes(bodyHTML string) (top, heart, base []string) {
	if bodyHTML == "" {
		return nil, nil, nil
	}
	text := bodyHTML
	text = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(text, " ")
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&#39;", "'")
	text = strings.ReplaceAll(text, "&amp;", "&")
	matches := noteSectionRE.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		kind := strings.ToLower(strings.TrimSpace(m[1]))
		val := strings.TrimSpace(m[2])
		parts := regexp.MustCompile(`[,;]|\s+and\s+|\s+&\s+`).Split(val, -1)
		for _, p := range parts {
			p = strings.TrimSpace(strings.ToLower(p))
			p = strings.Trim(p, " .:-–—")
			if p == "" || len(p) < 2 {
				continue
			}
			switch {
			case strings.Contains(kind, "top"):
				top = append(top, p)
			case strings.Contains(kind, "heart") || strings.Contains(kind, "middle"):
				heart = append(heart, p)
			case strings.Contains(kind, "base"):
				base = append(base, p)
			}
		}
	}
	return dedup(lowerSlice(top)), dedup(lowerSlice(heart)), dedup(lowerSlice(base))
}

func dedup(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func lowerSlice(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = strings.ToLower(s)
	}
	return out
}

// NotesOverlap returns the count of shared notes across the three pyramids (used for similar/recommend scoring).
func NotesOverlap(aTop, aHeart, aBase, bTop, bHeart, bBase []string) int {
	score := 0
	score += overlap(aTop, bTop)
	score += overlap(aHeart, bHeart)
	score += overlap(aBase, bBase)
	return score
}

func overlap(x, y []string) int {
	if len(x) == 0 || len(y) == 0 {
		return 0
	}
	ym := map[string]bool{}
	for _, s := range y {
		ym[s] = true
	}
	c := 0
	for _, s := range x {
		if ym[s] {
			c++
		}
	}
	return c
}
