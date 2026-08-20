// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.
// Shared local-corpus reading for the novel analytics commands (digest,
// author-compare). Both read the archived 'posts' resource type from the
// read-only store and reason over the same lightweight post projection.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// corpusPost is the projection digest/author-compare reason over. Host is the
// publication join key injected at archive time (_pp_source_host), falling back
// to the canonical_url host for posts archived before that tagging existed.
type corpusPost struct {
	Host     string
	Title    string
	Slug     string
	Audience string
	Date     string
	Parsed   time.Time
	HasDate  bool
}

// IsPaid reports whether the post is gated (audience != "everyone").
func (p corpusPost) IsPaid() bool {
	a := strings.TrimSpace(strings.ToLower(p.Audience))
	return a != "" && a != "everyone"
}

// corpusDateLayouts covers the post_date shapes Substack emits (ISO with/without
// fractional seconds or zone) plus a bare date.
var corpusDateLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05.000Z",
	"2006-01-02T15:04:05Z",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

func parseCorpusPost(raw json.RawMessage) (corpusPost, bool) {
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return corpusPost{}, false
	}
	strOf := func(k string) string {
		var s string
		if v, ok := m[k]; ok {
			_ = json.Unmarshal(v, &s)
		}
		return s
	}
	p := corpusPost{
		Host:     strOf("_pp_source_host"),
		Title:    strOf("title"),
		Slug:     strOf("slug"),
		Audience: strOf("audience"),
		Date:     strOf("post_date"),
	}
	if p.Host == "" {
		p.Host = hostFromURL(strOf("canonical_url"))
	}
	// Normalize the publication join key: Substack hosts are case-insensitive, so
	// lowercase the host used to group/compare posts. Without this, archiving
	// "AstralCodexTen" then running author-compare "astralcodexten" would match
	// zero posts (a stored-vs-read casing mismatch — a silent-empty result even
	// though the corpus holds every post).
	p.Host = strings.ToLower(p.Host)
	if p.Date != "" {
		for _, layout := range corpusDateLayouts {
			if t, err := time.Parse(layout, p.Date); err == nil {
				p.Parsed = t
				p.HasDate = true
				break
			}
		}
	}
	return p, true
}

func hostFromURL(u string) string {
	s := strings.TrimPrefix(u, "https://")
	s = strings.TrimPrefix(s, "http://")
	if i := strings.IndexByte(s, '/'); i >= 0 {
		s = s[:i]
	}
	return s
}

var errNoCorpus = fmt.Errorf("no local corpus yet — run 'substack-reader-pp-cli archive <publication>' first")

// loadCorpusPosts reads every archived post from the read-only local store.
func loadCorpusPosts(ctx context.Context) ([]corpusPost, error) {
	db, err := openStoreForRead(ctx, "substack-reader-pp-cli")
	if err != nil {
		return nil, fmt.Errorf("opening local corpus: %w", err)
	}
	if db == nil {
		return nil, errNoCorpus
	}
	defer db.Close()

	raws, err := db.List("posts", 0)
	if err != nil {
		return nil, fmt.Errorf("reading corpus: %w", err)
	}
	posts := make([]corpusPost, 0, len(raws))
	for _, r := range raws {
		if p, ok := parseCorpusPost(r); ok {
			posts = append(posts, p)
		}
	}
	return posts, nil
}
