// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command — NOT generator output.
// pp:data-source live
//
// pull builds (or incrementally refreshes) the local snip mirror from the Snipd
// Obsidian export API: fetch the episode catalog, POST the labelled-delimiter
// export templates per batch, parse the returned markdown, and upsert typed
// episode + snip JSON into the generic resources store (which FTS-indexes it).
// Ported from experiment/pull_corpus.py + parse_corpus.py.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/snipd/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/snipd/internal/config"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/snipd/internal/snipd"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/snipd/internal/store"
)

// pullMinTimeout is the floor for a bulk pull. The root --timeout default (60s)
// is tuned for quick reads; a full export (many episodes, server-rendered) needs
// far longer. A larger explicit --timeout still wins.
const pullMinTimeout = 15 * time.Minute

type pullResult struct {
	Episodes      int    `json:"episodes"`
	Snips         int    `json:"snips"`
	Batches       int    `json:"batches"`
	Cursor        string `json:"cursor,omitempty"`
	Curtailed     bool   `json:"curtailed,omitempty"`
	CurtailReason string `json:"curtail_reason,omitempty"`
	DB            string `json:"db"`
}

// fallbackSnipID builds a synthetic id for a snip that exported without a
// deep-link UUID. Such a snip has no server identity, so it is keyed by its
// POSITION — the episode plus the clip's start/end timestamps. Position is the
// right identity here because it is stable across content edits (editing a
// note/quote/transcript never moves the clip), so a re-pull of an edited snip
// updates the same row instead of duplicating it; and two snips at different
// moments get different ids. Content can't be part of the key without
// re-duplicating edited snips, and a bare start collides more than start+end, so
// start+end is the balance.
//
// When BOTH timestamps are empty (a snip exported with no start and no end),
// start+end degenerates to one key for every such snip in an episode, silently
// overwriting one with another. For that case only, the caller passes a
// non-negative ordinal — the snip's position among the both-empty snips in its
// episode, in export order — appended to keep them distinct. The ordinal is
// stable across content edits (editing never reorders the export), so it
// preserves the edit-stable property; its only cost is re-keying churn if a
// both-empty snip is inserted or removed, which yields a harmless duplicate row
// rather than data loss. Snips carrying any timestamp pass ordinal = -1 and keep
// their existing ids. (This whole path only touches the handful of UUID-less
// snips; the vast majority carry a stable deep-link UUID.)
func fallbackSnipID(s snipd.Snip, ordinal int) string {
	h := fnv.New64a()
	key := s.EpisodeID + "\x1f" + s.Start + "\x1f" + s.End
	if ordinal >= 0 {
		key += "\x1f" + strconv.Itoa(ordinal)
	}
	_, _ = h.Write([]byte(key))
	return fmt.Sprintf("%s#%x", s.EpisodeID, h.Sum64())
}

// tsLater reports whether timestamp a is strictly later than b. When both parse
// as timestamps it compares the parsed instants; otherwise it falls back to
// lexical order (e.g. the empty initial cursor, or an unexpected format). This
// keeps the incremental --updated-after cursor correct even if the API varies
// timezone offset or fractional-second precision between episodes.
func tsLater(a, b string) bool {
	ta, aok := parseSnipTS(a)
	tb, bok := parseSnipTS(b)
	if aok && bok {
		return ta.After(tb)
	}
	return a > b
}

func parseSnipTS(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func newNovelPullCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var updatedAfter string
	var limit int

	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Pull your Snipd snips into a local SQLite mirror you can search and query offline — the app has no export-to-query path.",
		Long: "Build (or refresh) the local snip corpus from your Snipd account.\n\n" +
			"pull is the populate step every other command reads from: it fetches the\n" +
			"export catalog, downloads your snips through the sanctioned Obsidian export\n" +
			"API, and upserts them into a local SQLite mirror with a full-text index.\n" +
			"Run it once to seed the corpus, then again (optionally with --updated-after)\n" +
			"to pull in what changed. Read-only against Snipd; requires SNIPD_TOKEN.",
		Example: "  snipd-pp-cli pull\n  snipd-pp-cli pull --updated-after 2026-07-01\n  snipd-pp-cli pull --limit 5 --agent",
		// pull WRITES the local store, so it is intentionally NOT annotated
		// mcp:read-only.
		RunE: func(cmd *cobra.Command, args []string) error {
			// A bulk network write cannot be exercised against the mock verify
			// server (the export response is a ZIP), so short-circuit under both
			// --dry-run and PRINTING_PRESS_VERIFY. Live dogfood (real API) still
			// runs, curtailed below.
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would pull episodes and snips into the local mirror")
				return nil
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			if cfg.SnipdToken == "" {
				return usageErr(fmt.Errorf("no SNIPD_TOKEN set — the export API requires your Snipd account token.\n" +
					"Get your token from Snipd's browser sign-in (no Obsidian needed; see README -> Authentication), then: snipd-pp-cli auth set-token <token>  (or export SNIPD_TOKEN=<token>)"))
			}

			// Generous timeout floor for a bulk pull; a larger explicit --timeout wins.
			timeout := pullMinTimeout
			if flags.timeout > timeout {
				timeout = flags.timeout
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			c := snipd.NewClient(cfg.BaseURL, cfg.SnipdToken)
			meta, err := c.FetchMetadata(ctx, updatedAfter)
			if err != nil {
				return err
			}

			// Curtail work to fit the live-dogfood per-command timeout.
			capEpisodes := 0 // 0 = all
			curtailed := false
			curtailReason := ""
			if cliutil.IsDogfoodEnv() {
				capEpisodes = 1
				curtailed = true
				curtailReason = "dogfood: limited to 1 episode"
			} else if limit > 0 {
				capEpisodes = limit
			}

			if dbPath == "" {
				dbPath = defaultDBPath("snipd-pp-cli")
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			res := pullResult{DB: dbPath, Curtailed: curtailed, CurtailReason: curtailReason}
			remaining := capEpisodes
			var newestCursor string

			for _, batch := range meta.EpisodeBatches {
				if capEpisodes > 0 && remaining <= 0 {
					break
				}
				take := len(batch.Episodes)
				if capEpisodes > 0 && take > remaining {
					take = remaining
				}
				if take <= 0 {
					continue
				}
				fetched := batch.Episodes[:take]
				eids := make([]string, 0, take)
				for _, e := range fetched {
					eids = append(eids, e.EpisodeID)
					// Advance the cursor only over episodes actually fetched, never
					// over the ones a --limit/dogfood run skipped. Compare as real
					// instants (not lexically) so a valid timestamp with a different
					// offset or fractional precision can't sort as "newer" by string
					// order and park the cursor before episodes it should cover.
					if tsLater(e.LatestSnipUpdateTS, newestCursor) {
						newestCursor = e.LatestSnipUpdateTS
					}
				}

				zipBytes, err := c.ExportEpisodes(ctx, eids)
				if err != nil {
					return fmt.Errorf("exporting batch %d: %w", batch.Index, err)
				}
				eps, snips, err := snipd.ParseZip(zipBytes)
				if err != nil {
					return fmt.Errorf("parsing batch %d export: %w", batch.Index, err)
				}

				for _, ep := range eps {
					raw, err := json.Marshal(ep)
					if err != nil {
						return fmt.Errorf("marshaling episode %s: %w", ep.EpisodeID, err)
					}
					if err := db.Upsert("episodes", ep.EpisodeID, raw); err != nil {
						return fmt.Errorf("storing episode %s: %w", ep.EpisodeID, err)
					}
				}
				// emptyTimeSeq gives a per-episode ordinal to UUID-less snips whose
				// export omitted both timestamps; without it they'd share
				// episode+""+"" and overwrite each other. Reset per batch — an
				// episode's snips never span batches.
				emptyTimeSeq := map[string]int{}
				for _, s := range snips {
					id := s.SnipID
					if id == "" {
						ordinal := -1
						if s.Start == "" && s.End == "" {
							ordinal = emptyTimeSeq[s.EpisodeID]
							emptyTimeSeq[s.EpisodeID]++
						}
						id = fallbackSnipID(s, ordinal)
					}
					raw, err := json.Marshal(s)
					if err != nil {
						return fmt.Errorf("marshaling snip %s: %w", id, err)
					}
					if err := db.Upsert("snips", id, raw); err != nil {
						return fmt.Errorf("storing snip %s: %w", id, err)
					}
				}

				res.Episodes += len(eps)
				res.Snips += len(snips)
				res.Batches++
				if capEpisodes > 0 {
					remaining -= len(eids)
				}
			}

			// The incremental cursor is only meaningful after a FULL pull; a
			// curtailed/--limit run must not advance it past episodes it never
			// fetched (a later --updated-after would then skip them silently).
			stampCursor := ""
			if capEpisodes == 0 {
				stampCursor = newestCursor
			}
			res.Cursor = stampCursor
			// Stamp freshness with the TRUE mirror totals (not this run's delta),
			// so doctor / provenance report the full corpus after an incremental
			// pull (meta-pattern 8: a novel populate path stamps sync_state itself).
			epTotal, _ := db.Count("episodes")
			snipTotal, _ := db.Count("snips")
			_ = db.SaveSyncState("episodes", stampCursor, epTotal)
			_ = db.SaveSyncState("snips", stampCursor, snipTotal)

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Pulled %d episodes and %d snips (%d batches) into %s\n",
				res.Episodes, res.Snips, res.Batches, res.DB)
			if curtailed {
				fmt.Fprintf(cmd.OutOrStdout(), "(%s)\n", curtailReason)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Now try: snipd-pp-cli search \"<topic>\"")
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite mirror path (default: per-user data dir)")
	cmd.Flags().StringVar(&updatedAfter, "updated-after", "", "Only pull episodes updated after this ISO-8601 time (incremental sync)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max episodes to pull this run (0 = all)")
	return cmd
}
