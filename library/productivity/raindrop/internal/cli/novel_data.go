// Copyright 2026 srijits and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored Raindrop.io local-analysis helpers. Preserved across reprints.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/mvanhorn/printing-press-library/library/productivity/raindrop/internal/store"
)

type localBookmark struct {
	ID           string           `json:"id"`
	Title        string           `json:"title"`
	Link         string           `json:"link"`
	Excerpt      string           `json:"excerpt,omitempty"`
	Note         string           `json:"note,omitempty"`
	Domain       string           `json:"domain,omitempty"`
	Tags         []string         `json:"tags,omitempty"`
	CollectionID int64            `json:"collection_id,omitempty"`
	Important    bool             `json:"important,omitempty"`
	Created      time.Time        `json:"created,omitempty"`
	LastUpdate   time.Time        `json:"last_update,omitempty"`
	Highlights   []map[string]any `json:"highlights,omitempty"`
	Raw          json.RawMessage  `json:"-"`
}

func openNovelStore(ctx context.Context, dbPath string) (*store.Store, string, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("raindrop-pp-cli")
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, dbPath, fmt.Errorf("opening local database: %w; run 'raindrop-pp-cli sync' first", err)
	}
	return db, dbPath, nil
}

func loadLocalBookmarks(db *store.Store) ([]localBookmark, error) {
	rows, err := db.List("raindrops", 0)
	if err != nil {
		return nil, err
	}
	items := make([]localBookmark, 0, len(rows))
	for _, raw := range rows {
		var obj map[string]any
		if json.Unmarshal(raw, &obj) != nil {
			continue
		}
		item := localBookmark{
			ID:         valueID(obj["_id"]),
			Title:      valueString(obj["title"]),
			Link:       valueString(obj["link"]),
			Excerpt:    valueString(obj["excerpt"]),
			Note:       valueString(obj["note"]),
			Domain:     valueString(obj["domain"]),
			Important:  valueBool(obj["important"]),
			Created:    valueTime(obj["created"]),
			LastUpdate: valueTime(obj["lastUpdate"]),
			Raw:        append(json.RawMessage(nil), raw...),
		}
		if item.ID == "" {
			item.ID = valueID(obj["id"])
		}
		if tags, ok := obj["tags"].([]any); ok {
			for _, tag := range tags {
				if s := strings.TrimSpace(valueString(tag)); s != "" {
					item.Tags = append(item.Tags, s)
				}
			}
		}
		if collection, ok := obj["collection"].(map[string]any); ok {
			item.CollectionID, _ = strconv.ParseInt(valueID(collection["$id"]), 10, 64)
		}
		if highlights, ok := obj["highlights"].([]any); ok {
			for _, rawHighlight := range highlights {
				if highlight, ok := rawHighlight.(map[string]any); ok {
					item.Highlights = append(item.Highlights, highlight)
				}
			}
		}
		items = append(items, item)
	}
	return items, nil
}

func valueString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case json.Number:
		return x.String()
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case nil:
		return ""
	default:
		return fmt.Sprint(x)
	}
}

func valueID(v any) string { return strings.TrimSuffix(valueString(v), ".0") }

func valueBool(v any) bool {
	b, _ := v.(bool)
	return b
}

func valueTime(v any) time.Time {
	s := valueString(v)
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, s); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func parseAge(raw string, fallback time.Duration) (time.Duration, error) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return fallback, nil
	}
	if strings.HasSuffix(raw, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(raw, "d"))
		if err != nil || days < 0 {
			return 0, fmt.Errorf("invalid day duration %q", raw)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	if strings.HasSuffix(raw, "w") {
		weeks, err := strconv.Atoi(strings.TrimSuffix(raw, "w"))
		if err != nil || weeks < 0 {
			return 0, fmt.Errorf("invalid week duration %q", raw)
		}
		return time.Duration(weeks) * 7 * 24 * time.Hour, nil
	}
	return time.ParseDuration(raw)
}

var nonTagRune = regexp.MustCompile(`[^\pL\pN]+`)

func normalizedTag(tag string) string {
	return strings.Trim(nonTagRune.ReplaceAllString(strings.ToLower(tag), "-"), "-")
}

func canonicalBookmarkURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return strings.TrimSpace(strings.ToLower(raw))
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""
	q := u.Query()
	for key := range q {
		lower := strings.ToLower(key)
		if strings.HasPrefix(lower, "utm_") || lower == "fbclid" || lower == "gclid" || lower == "mc_cid" || lower == "mc_eid" {
			q.Del(key)
		}
	}
	u.RawQuery = q.Encode()
	if u.Path != "/" {
		u.Path = strings.TrimSuffix(u.Path, "/")
	}
	return u.String()
}

func bookmarkRichness(item localBookmark) int {
	return len(item.Tags)*4 + len(item.Highlights)*8 + len(item.Note)/40 + len(item.Excerpt)/80 + boolScore(item.Important)
}

func boolScore(v bool) int {
	if v {
		return 10
	}
	return 0
}

func words(s string) map[string]struct{} {
	result := map[string]struct{}{}
	var current strings.Builder
	flush := func() {
		if current.Len() >= 3 {
			result[current.String()] = struct{}{}
		}
		current.Reset()
	}
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return result
}

func overlapScore(a, b localBookmark) float64 {
	at := map[string]struct{}{}
	for _, tag := range a.Tags {
		at[normalizedTag(tag)] = struct{}{}
	}
	bt := map[string]struct{}{}
	for _, tag := range b.Tags {
		bt[normalizedTag(tag)] = struct{}{}
	}
	sharedTags := 0
	for tag := range at {
		if _, ok := bt[tag]; ok && tag != "" {
			sharedTags++
		}
	}
	aw := words(a.Title + " " + a.Excerpt + " " + a.Note)
	bw := words(b.Title + " " + b.Excerpt + " " + b.Note)
	sharedWords := 0
	for word := range aw {
		if _, ok := bw[word]; ok {
			sharedWords++
		}
	}
	score := float64(sharedTags*5 + sharedWords)
	if a.Domain != "" && strings.EqualFold(a.Domain, b.Domain) {
		score += 2
	}
	return score
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
