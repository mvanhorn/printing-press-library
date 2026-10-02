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
	"errors"
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
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

var errExportCountMismatch = errors.New("export snip count differs from metadata")

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

// validateExportBatch makes sure the ZIP is a complete representation of the
// requested episodes before any stored snips can be removed. A parseable but
// truncated export must never be treated as an authoritative deletion list.
func validateExportBatch(expected []snipd.MetaEpisode, episodes []snipd.Episode, snips []snipd.Snip) error {
	wanted := make(map[string]int, len(expected))
	for _, episode := range expected {
		if episode.EpisodeID == "" || episode.TotalSnipCount < 0 {
			return fmt.Errorf("invalid episode metadata for %q", episode.EpisodeID)
		}
		if _, duplicate := wanted[episode.EpisodeID]; duplicate {
			return fmt.Errorf("duplicate episode %s in export metadata", episode.EpisodeID)
		}
		wanted[episode.EpisodeID] = episode.TotalSnipCount
	}
	seen := make(map[string]bool, len(episodes))
	for _, episode := range episodes {
		count, requested := wanted[episode.EpisodeID]
		if !requested {
			return fmt.Errorf("unexpected episode %s in export", episode.EpisodeID)
		}
		if seen[episode.EpisodeID] {
			return fmt.Errorf("duplicate episode %s in export", episode.EpisodeID)
		}
		seen[episode.EpisodeID] = true
		if episode.SnipCount != count {
			return fmt.Errorf("%w for episode %s: metadata lists %d snips, ZIP contains %d", errExportCountMismatch, episode.EpisodeID, count, episode.SnipCount)
		}
	}
	if len(seen) != len(wanted) {
		return fmt.Errorf("incomplete export: requested %d episodes, ZIP contains %d", len(wanted), len(seen))
	}
	actual := make(map[string]int, len(episodes))
	seenSnipIDs := make(map[string]bool, len(snips))
	emptyTimeSeq := map[string]int{}
	for _, snip := range snips {
		if !seen[snip.EpisodeID] {
			return fmt.Errorf("snip belongs to unexpected episode %s", snip.EpisodeID)
		}
		if snip.SnipID == "" && strings.TrimSpace(snip.URL) == "" {
			return fmt.Errorf("snip in episode %s has no identifying URL", snip.EpisodeID)
		}
		id := effectiveSnipID(snip, emptyTimeSeq)
		if seenSnipIDs[id] {
			return fmt.Errorf("duplicate snip ID %s in export", id)
		}
		seenSnipIDs[id] = true
		actual[snip.EpisodeID]++
	}
	for id, count := range wanted {
		if actual[id] != count {
			return fmt.Errorf("%w for episode %s: metadata lists %d snips, parsed %d", errExportCountMismatch, id, count, actual[id])
		}
	}
	return nil
}

func effectiveSnipID(snip snipd.Snip, emptyTimeSeq map[string]int) string {
	if snip.SnipID != "" {
		return snip.SnipID
	}
	ordinal := -1
	if snip.Start == "" && snip.End == "" {
		ordinal = emptyTimeSeq[snip.EpisodeID]
		emptyTimeSeq[snip.EpisodeID]++
	}
	return fallbackSnipID(snip, ordinal)
}

// A separate metadata and ZIP request can straddle a user edit. Refresh the
// metadata once on a count mismatch, then retry the affected export once if
// the first ZIP still disagrees. Identity or template failures are not retried.
func fetchValidatedBatch(ctx context.Context, c *snipd.Client, updatedAfter string, expected []snipd.MetaEpisode) ([]snipd.Episode, []snipd.Snip, []snipd.MetaEpisode, error) {
	eids := make([]string, 0, len(expected))
	for _, episode := range expected {
		eids = append(eids, episode.EpisodeID)
	}
	export := func() ([]snipd.Episode, []snipd.Snip, error) {
		zipBytes, err := c.ExportEpisodes(ctx, eids)
		if err != nil {
			return nil, nil, err
		}
		return snipd.ParseZip(zipBytes)
	}
	eps, snips, err := export()
	if err != nil {
		return nil, nil, nil, err
	}
	if err := validateExportBatch(expected, eps, snips); err == nil {
		return eps, snips, expected, nil
	} else if !errors.Is(err, errExportCountMismatch) {
		return nil, nil, nil, err
	}
	metadata, err := c.FetchMetadata(ctx, updatedAfter)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("refreshing export metadata after count change: %w", err)
	}
	byID := map[string]snipd.MetaEpisode{}
	for _, batch := range metadata.EpisodeBatches {
		for _, episode := range batch.Episodes {
			if _, duplicate := byID[episode.EpisodeID]; duplicate {
				return nil, nil, nil, fmt.Errorf("duplicate episode %s in refreshed metadata", episode.EpisodeID)
			}
			byID[episode.EpisodeID] = episode
		}
	}
	refreshed := make([]snipd.MetaEpisode, 0, len(expected))
	for _, id := range eids {
		episode, ok := byID[id]
		if !ok {
			return nil, nil, nil, fmt.Errorf("episode %s missing from refreshed metadata", id)
		}
		refreshed = append(refreshed, episode)
	}
	if err := validateExportBatch(refreshed, eps, snips); err == nil {
		return eps, snips, refreshed, nil
	} else if !errors.Is(err, errExportCountMismatch) {
		return nil, nil, nil, err
	}
	eps, snips, err = export()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("retrying export after metadata change: %w", err)
	}
	if err := validateExportBatch(refreshed, eps, snips); err != nil {
		return nil, nil, nil, err
	}
	return eps, snips, refreshed, nil
}

// storeAndReconcileSnips persists one successfully parsed export batch, then
// removes local snips that no longer appear in each exported episode. The
// reconciliation runs only after every returned snip was stored successfully;
// a parse or upsert failure therefore cannot turn a partial response into an
// authoritative deletion set.
func storeAndReconcileSnips(db *store.Store, episodes []snipd.Episode, snips []snipd.Snip) error {
	seenByEpisode := make(map[string][]string, len(episodes))
	unsafeToReconcile := make(map[string]bool, len(episodes))
	for _, episode := range episodes {
		seenByEpisode[episode.EpisodeID] = []string{}
	}

	// emptyTimeSeq gives a per-episode ordinal to UUID-less snips whose export
	// omitted both timestamps; without it they'd share episode+""+"" and
	// overwrite each other. An episode's snips never span export batches.
	emptyTimeSeq := map[string]int{}
	for _, exportedSnip := range snips {
		// A non-empty URL without a UUID can be stored under a positional
		// fallback, but cannot prove that a formerly UUID-keyed snip was
		// removed. Preserve old rows for this episode until a clean export.
		if exportedSnip.SnipID == "" {
			unsafeToReconcile[exportedSnip.EpisodeID] = true
		}
		id := effectiveSnipID(exportedSnip, emptyTimeSeq)
		exportedSnip.SnipID = id
		raw, err := json.Marshal(exportedSnip)
		if err != nil {
			return fmt.Errorf("marshaling snip %s: %w", id, err)
		}
		if err := db.Upsert("snips", id, raw); err != nil {
			return fmt.Errorf("storing snip %s: %w", id, err)
		}
		seenByEpisode[exportedSnip.EpisodeID] = append(seenByEpisode[exportedSnip.EpisodeID], id)
	}

	for _, episode := range episodes {
		if unsafeToReconcile[episode.EpisodeID] {
			continue
		}
		if _, err := db.ReconcilePartition(
			"snips", "$.episode_id", episode.EpisodeID,
			seenByEpisode[episode.EpisodeID], "", nil,
		); err != nil {
			return fmt.Errorf("reconciling snips for episode %s: %w", episode.EpisodeID, err)
		}
	}
	return nil
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
				return writeDryRunShim(cmd.OutOrStdout(), flags, "pull")
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			token := resolveSnipdToken(cfg)
			if token == "" {
				return usageErr(fmt.Errorf("no Snipd token configured — the export API requires your Snipd account token.\n" +
					"Get your token from Snipd's browser sign-in (no Obsidian needed; see README -> Authentication), then: snipd-pp-cli auth login  (or auth set-token <token>, or export SNIPD_TOKEN=<token>)"))
			}

			// Generous timeout floor for a bulk pull; a larger explicit --timeout wins.
			timeout := pullMinTimeout
			if flags.timeout > timeout {
				timeout = flags.timeout
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			c := snipd.NewClient(cfg.BaseURL, token)
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
					// The initial metadata is the pull's cursor snapshot. A
					// later metadata refresh may observe an edit to an earlier
					// batch that has already been exported; advancing past it
					// would make the next incremental pull skip that edit.
					if tsLater(e.LatestSnipUpdateTS, newestCursor) {
						newestCursor = e.LatestSnipUpdateTS
					}
				}

				eps, snips, _, err := fetchValidatedBatch(ctx, c, updatedAfter, fetched)
				if err != nil {
					return fmt.Errorf("fetching and validating batch %d export: %w", batch.Index, err)
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
				if err := storeAndReconcileSnips(db, eps, snips); err != nil {
					return fmt.Errorf("storing batch %d snips: %w", batch.Index, err)
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

// resolveSnipdToken returns the raw Snipd bearer token, mirroring the precedence
// config.AuthHeader() applies everywhere else in the CLI: the SNIPD_TOKEN env var
// wins, then whatever was persisted to the credentials file.
//
// The fallback is load-bearing, not defensive. `auth set-token` routes through the
// generated config.SaveTokens, whose OAuth-shaped signature files the value under
// AccessToken. cfg.SnipdToken maps to the `token` key and CAN be loaded from the
// credentials file, but no command ever writes that key — so in practice the env var
// is its only populated source. Reading it alone meant `doctor` reported "Auth:
// configured" from the saved credential while every pull rejected it: the generated
// half of the CLI honoured the token and this hand-built half did not.
//
// AuthHeaderVal is deliberately NOT consulted: it holds a complete header value
// ("Bearer x"), while snipd.NewClient wants the bare token. `auth set-token`
// clears it for exactly this reason.
func resolveSnipdToken(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	if cfg.SnipdToken != "" {
		return cfg.SnipdToken
	}
	return cfg.AccessToken
}
