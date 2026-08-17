// Copyright 2026 Kerry Morrison and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-trends/internal/cliutil"
)

// changesTermDelta describes one related-term row in an added/dropped/
// score_changed bucket. Fields are populated selectively: added/dropped rows
// carry Value; score_changed rows carry OldValue/NewValue/Delta.
type changesTermDelta struct {
	Term       string `json:"term"`
	Kind       string `json:"kind"`
	Facet      string `json:"facet"`
	Value      int    `json:"value,omitempty"`
	OldValue   int    `json:"old_value,omitempty"`
	NewValue   int    `json:"new_value,omitempty"`
	Delta      int    `json:"delta,omitempty"`
	IsBreakout bool   `json:"is_breakout,omitempty"`
}

type changesInterestDelta struct {
	OldValue int `json:"old_value"`
	NewValue int `json:"new_value"`
	Delta    int `json:"delta"`
}

type changesResult struct {
	Keyword       string                `json:"keyword"`
	Since         string                `json:"since"`
	Added         []changesTermDelta    `json:"added"`
	Dropped       []changesTermDelta    `json:"dropped"`
	ScoreChanged  []changesTermDelta    `json:"score_changed"`
	InterestDelta *changesInterestDelta `json:"interest_delta,omitempty"`
	Note          string                `json:"note,omitempty"`
}

// pickOlderSyncAt returns the most recent time in timesDesc (sorted
// most-recent-first, as returned by distinctSyncedAtDesc) that is at or
// before cutoff, skipping the first (most recent) entry entirely — that one
// is always the "recent" bucket, never a candidate "older" bucket.
func pickOlderSyncAt(timesDesc []time.Time, cutoff time.Time) (time.Time, bool) {
	for _, t := range timesDesc[1:] {
		if !t.After(cutoff) {
			return t, true
		}
	}
	return time.Time{}, false
}

// diffRelatedTerms buckets rows by their two most relevant sync instants
// (the latest sync, and the latest sync at or before cutoff) and fills
// result.Added/Dropped/ScoreChanged. Returns a human-readable note when
// there isn't enough history to diff (fewer than 2 usable sync instants).
func diffRelatedTerms(rows []gtRelatedTermRecord, cutoff time.Time, result *changesResult) string {
	if len(rows) == 0 {
		return "no related-term history yet for this keyword; run 'trends related' to start building it"
	}
	syncedAtValues := make([]string, 0, len(rows))
	for _, r := range rows {
		syncedAtValues = append(syncedAtValues, r.SyncedAt)
	}
	times := distinctSyncedAtDesc(syncedAtValues)
	if len(times) == 0 {
		return "related-term rows have no usable synced_at timestamps"
	}
	recentT := times[0]
	olderT, hasOlder := pickOlderSyncAt(times, cutoff)
	if !hasOlder {
		return "only one sync recorded within the comparison window; run 'trends related' again after some time to see changes"
	}

	recentByKey := map[string]gtRelatedTermRecord{}
	olderByKey := map[string]gtRelatedTermRecord{}
	for _, r := range rows {
		t, err := time.Parse(time.RFC3339, r.SyncedAt)
		if err != nil {
			continue
		}
		key := r.Kind + "|" + r.Facet + "|" + r.Term
		switch {
		case t.Equal(recentT):
			recentByKey[key] = r
		case t.Equal(olderT):
			olderByKey[key] = r
		}
	}

	for key, r := range recentByKey {
		if _, ok := olderByKey[key]; !ok {
			result.Added = append(result.Added, changesTermDelta{Term: r.Term, Kind: r.Kind, Facet: r.Facet, Value: r.Value, IsBreakout: r.IsBreakout})
		}
	}
	for key, r := range olderByKey {
		if _, ok := recentByKey[key]; !ok {
			result.Dropped = append(result.Dropped, changesTermDelta{Term: r.Term, Kind: r.Kind, Facet: r.Facet, Value: r.Value, IsBreakout: r.IsBreakout})
		}
	}
	for key, newRec := range recentByKey {
		if oldRec, ok := olderByKey[key]; ok && oldRec.Value != newRec.Value {
			result.ScoreChanged = append(result.ScoreChanged, changesTermDelta{
				Term: newRec.Term, Kind: newRec.Kind, Facet: newRec.Facet,
				OldValue: oldRec.Value, NewValue: newRec.Value, Delta: newRec.Value - oldRec.Value,
			})
		}
	}
	sort.Slice(result.Added, func(i, j int) bool { return result.Added[i].Term < result.Added[j].Term })
	sort.Slice(result.Dropped, func(i, j int) bool { return result.Dropped[i].Term < result.Dropped[j].Term })
	sort.Slice(result.ScoreChanged, func(i, j int) bool { return result.ScoreChanged[i].Term < result.ScoreChanged[j].Term })
	return ""
}

// diffInterestPoints sets result.InterestDelta by comparing the latest
// interest-over-time value against the value dated closest to cutoff.
// Returns a human-readable note when there isn't enough history.
func diffInterestPoints(rows []gtInterestPointRecord, cutoff time.Time, result *changesResult) string {
	if len(rows) == 0 {
		return "no interest-over-time history yet for this keyword; run 'trends interest' to start building it"
	}
	if len(rows) < 2 {
		return "only one interest data point recorded; run 'trends interest' again after some time to see changes"
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Date < rows[j].Date })
	latest := rows[len(rows)-1]

	var closest gtInterestPointRecord
	found := false
	var bestDiff time.Duration
	for _, r := range rows[:len(rows)-1] {
		d, err := time.Parse("2006-01-02", r.Date)
		if err != nil {
			continue
		}
		diff := d.Sub(cutoff)
		if diff < 0 {
			diff = -diff
		}
		if !found || diff < bestDiff {
			bestDiff = diff
			closest = r
			found = true
		}
	}
	if !found {
		return "could not find a comparable earlier interest data point"
	}
	result.InterestDelta = &changesInterestDelta{OldValue: closest.Value, NewValue: latest.Value, Delta: latest.Value - closest.Value}
	return ""
}

// pp:data-source local
func newNovelTrendsChangesCmd(flags *rootFlags) *cobra.Command {
	var flagSince string

	cmd := &cobra.Command{
		Use:         "changes <keyword>",
		Short:       "See exactly what changed for a keyword's related terms and interest score since your last sync.",
		Example:     "  google-trends-pp-cli trends changes coffee --since 7d --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("keyword argument is required"))
			}
			keyword := args[0]
			sinceStr := flagSince
			if sinceStr == "" {
				sinceStr = "7d"
			}
			sinceDur, err := cliutil.ParseDurationLoose(sinceStr)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since %q: %w", flagSince, err))
			}

			result := changesResult{
				Keyword:      keyword,
				Since:        sinceStr,
				Added:        make([]changesTermDelta, 0),
				Dropped:      make([]changesTermDelta, 0),
				ScoreChanged: make([]changesTermDelta, 0),
			}

			ctx := cmd.Context()
			db, err := openStoreForRead(ctx, "google-trends-pp-cli")
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			if db == nil {
				result.Note = "no local mirror yet; run 'trends related' and 'trends interest' first to build history"
				return noLocalMirrorHint(cmd, flags, "google-trends-pp-cli trends related "+keyword+"' and 'google-trends-pp-cli trends interest "+keyword, result)
			}
			defer db.Close()

			cutoff := time.Now().Add(-sinceDur)

			relatedRows, err := queryRelatedTermsForKeyword(db, keyword)
			if err != nil {
				return fmt.Errorf("querying related terms: %w", err)
			}
			relatedRows = filterRelatedRowsToLatestScope(relatedRows)
			interestRows, err := queryInterestPointsForKeyword(db, keyword)
			if err != nil {
				return fmt.Errorf("querying interest points: %w", err)
			}
			interestRows = filterInterestRowsToLatestScope(interestRows)

			relatedNote := diffRelatedTerms(relatedRows, cutoff, &result)
			interestNote := diffInterestPoints(interestRows, cutoff, &result)
			notes := make([]string, 0, 2)
			if relatedNote != "" {
				notes = append(notes, relatedNote)
			}
			if interestNote != "" && interestNote != relatedNote {
				notes = append(notes, interestNote)
			}
			if len(notes) > 0 {
				result.Note = strings.Join(notes, "; ")
			}

			if len(relatedRows) == 0 && len(interestRows) == 0 {
				return notFoundErr(fmt.Errorf("no history at all for %q in the local store; run 'trends related %s' and 'trends interest %s' first", keyword, keyword, keyword))
			}
			return printLocalResult(cmd, flags, result)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Compare against the most recent sync at least this long ago (e.g. 7d, 24h); default 7d")
	return cmd
}
