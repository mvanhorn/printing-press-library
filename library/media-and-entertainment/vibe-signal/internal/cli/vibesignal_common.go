// Copyright 2026 not0xjarvis and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored aggregator runtime shared by report/evidence/sources commands.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/vibe-signal/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/vibe-signal/internal/source"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/vibe-signal/internal/store"
)

// coverageEntry reports the outcome of syncing one source for a topic.
type coverageEntry struct {
	Source string `json:"source"`
	Status string `json:"status"` // ok | partial | failed
	Count  int    `json:"count"`
	Error  string `json:"error,omitempty"`
}

// openSignalStore opens the local store read-write and ensures the
// vibe-signal schema exists.
func openSignalStore(ctx context.Context, dbPath string) (*store.Store, error) {
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	if err := db.EnsureVibeSignalSchema(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// selectedSources resolves the source filter to a concrete set. An empty
// filter selects all registered sources.
func selectedSources(filter string) ([]source.Source, error) {
	if filter == "" {
		return source.All(), nil
	}
	s, ok := source.Lookup(filter)
	if !ok {
		names := make([]string, 0)
		for _, src := range source.All() {
			names = append(names, src.Name())
		}
		return nil, usageErr(fmt.Errorf("unknown source %q; available: %s", filter, strings.Join(names, ", ")))
	}
	return []source.Source{s}, nil
}

// syncSources fans out the topic across the given sources, returning all
// collected signals plus a per-source coverage table. A failed source never
// becomes silent emptiness: its error is preserved in the coverage entry and a
// rate-limit error is surfaced verbatim so throttling is distinguishable from
// "no data".
func syncSources(ctx context.Context, sources []source.Source, opts source.SyncOptions) ([]source.Signal, []coverageEntry) {
	var (
		all      []source.Signal
		coverage = make([]coverageEntry, 0, len(sources))
	)
	for _, src := range sources {
		signals, err := src.Sync(ctx, opts)
		entry := coverageEntry{Source: src.Name(), Count: len(signals)}
		switch {
		case err != nil:
			entry.Status = "failed"
			var rle *cliutil.RateLimitError
			if errors.As(err, &rle) {
				entry.Error = "rate limited: " + rle.Error()
			} else {
				entry.Error = err.Error()
			}
		case len(signals) == 0:
			entry.Status = "ok" // reached the source, no matching items
		default:
			entry.Status = "ok"
		}
		all = append(all, signals...)
		coverage = append(coverage, entry)
	}
	return all, coverage
}

// liveSyncAndStore fans the topic out across the selected sources, persists a
// snapshot to the store, and returns the freshly synced signals plus coverage.
// Used by report/sources-sync and by evidence's fetch-on-miss path so a topic
// can be queried without a prior explicit sync.
func liveSyncAndStore(ctx context.Context, db *store.Store, topic, sourceFilter string, since time.Time, windowDays, limit int) ([]source.Signal, []coverageEntry, string, error) {
	sources, err := selectedSources(sourceFilter)
	if err != nil {
		return nil, nil, "", err
	}
	if cliutil.IsDogfoodEnv() && limit > 5 {
		limit = 5 // curtail live fan-out to fit the dogfood timeout
	}
	signals, coverage := syncSources(ctx, sources, source.SyncOptions{Query: topic, Since: since, Limit: limit})
	runID := newRunID(topic)
	coverageJSON, _ := json.Marshal(coverage)
	if err := db.RecordRun(ctx, runID, topic, windowDays, string(coverageJSON)); err != nil {
		return signals, coverage, runID, err
	}
	if err := db.UpsertSignals(ctx, runID, signalsToRows(topic, signals)); err != nil {
		return signals, coverage, runID, err
	}
	return signals, coverage, runID, nil
}

// signalsToRows maps source signals to store rows for a topic.
func signalsToRows(query string, signals []source.Signal) []store.SignalRow {
	rows := make([]store.SignalRow, 0, len(signals))
	for _, s := range signals {
		rows = append(rows, store.SignalRow{
			Source:      s.Source,
			SourceID:    s.ID,
			Query:       query,
			Title:       s.Title,
			URL:         s.URL,
			Author:      s.Author,
			Points:      s.Points,
			Comments:    s.Comments,
			PublishedAt: s.PublishedAt,
			Excerpt:     s.Excerpt,
			RawJSON:     s.RawJSON,
		})
	}
	return rows
}

// parseWindow turns a window flag ("30d", "14d", "48h", "") into a lower-bound
// timestamp and a day count. An empty window defaults to 30 days.
func parseWindow(s string) (time.Time, int, error) {
	if strings.TrimSpace(s) == "" {
		s = "30d"
	}
	dur, err := cliutil.ParseDurationLoose(s)
	if err != nil {
		return time.Time{}, 0, usageErr(fmt.Errorf("invalid --window %q: use forms like 7d, 30d, 48h", s))
	}
	if dur <= 0 {
		return time.Time{}, 0, usageErr(fmt.Errorf("--window must be positive"))
	}
	days := int(dur.Hours() / 24)
	return time.Now().Add(-dur).UTC(), days, nil
}

// newRunID returns a sortable run identifier for a snapshot.
func newRunID(query string) string {
	stamp := time.Now().UTC().Format("20060102-150405")
	slug := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r >= 'A' && r <= 'Z':
			return r + 32
		default:
			return '-'
		}
	}, query)
	slug = strings.Trim(slug, "-")
	if len(slug) > 32 {
		slug = slug[:32]
	}
	if slug == "" {
		slug = "topic"
	}
	return stamp + "-" + slug
}

// keywordTally counts significant title words across signals. It is a
// mechanical frequency count, explicitly NOT synthesis — the report labels it
// as such so observed evidence is never presented as an interpreted theme.
func keywordTally(signals []source.Signal, top int) []keywordCount {
	counts := map[string]int{}
	for _, s := range signals {
		for _, w := range strings.Fields(strings.ToLower(s.Title)) {
			w = strings.Trim(w, ".,:;!?\"'()[]{}")
			if len(w) <= 3 || stopwords[w] {
				continue
			}
			counts[w]++
		}
	}
	out := make([]keywordCount, 0, len(counts))
	for w, c := range counts {
		if c < 2 {
			continue
		}
		out = append(out, keywordCount{Term: w, Count: c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Term < out[j].Term
	})
	if top > 0 && len(out) > top {
		out = out[:top]
	}
	return out
}

type keywordCount struct {
	Term  string `json:"term"`
	Count int    `json:"count"`
}

var stopwords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "that": true,
	"this": true, "from": true, "your": true, "have": true, "will": true,
	"what": true, "when": true, "into": true, "over": true, "they": true,
	"are": true, "was": true, "how": true, "why": true, "who": true,
}
