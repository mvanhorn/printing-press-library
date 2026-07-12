// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/passive-indices/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/passive-indices/internal/niftyindices"
)

func newNovelIndexConstituentsDiffCmd(flags *rootFlags) *cobra.Command {
	var flagSince string

	cmd := &cobra.Command{
		Use:         "constituents-diff <index>",
		Short:       "See what changed in an index's constituent list (additions/removals) between two sync snapshots.",
		Example:     "  passive-indices-pp-cli index constituents-diff \"NIFTY 50\" --since 30d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would diff constituent snapshots")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("index name is required, e.g. \"NIFTY 50\""))
			}
			since := 90 * 24 * time.Hour
			if flagSince != "" {
				parsed, err := cliutil.ParseDurationLoose(flagSince)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --since %q: %w", flagSince, err))
				}
				since = parsed
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			slug := niftyindices.Slugify(args[0])
			c := newNiftyIndicesClient(flags)
			current, err := c.Constituents(ctx, slug)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			db, _, err := openStoreForCache(flags)
			if err != nil {
				return err
			}
			defer db.Close()

			now := time.Now().UTC()
			snapID := fmt.Sprintf("%s:%s", slug, now.Format(time.RFC3339))
			var snapshotWarning string
			if data, err := json.Marshal(current); err != nil {
				snapshotWarning = fmt.Sprintf("today's snapshot was not saved for future diffs: encoding failed: %v", err)
			} else if err := db.Upsert(resourceTypeIndexConstituentSnap, snapID, data); err != nil {
				snapshotWarning = fmt.Sprintf("today's snapshot was not saved for future diffs: %v", err)
			}

			cutoff := now.Add(-since)
			baseline, baselineTime, err := findConstituentSnapshotBefore(db, slug, cutoff)
			if err != nil {
				return err
			}
			if baseline == nil {
				result := map[string]any{
					"index":                     args[0],
					"note":                      fmt.Sprintf("no baseline snapshot found at or before %s — this command builds its own history over repeated runs; run it again after the --since period has elapsed to see a diff", cutoff.Format(time.RFC3339)),
					"current_constituent_count": len(current),
				}
				if snapshotWarning != "" {
					result["snapshot_warning"] = snapshotWarning
				}
				return flags.printJSON(cmd, result)
			}

			added, removed := diffConstituents(baseline, current)
			result := map[string]any{
				"index":          args[0],
				"baseline_as_of": baselineTime,
				"current_as_of":  now.Format(time.RFC3339),
				"added":          added,
				"removed":        removed,
			}
			if snapshotWarning != "" {
				result["snapshot_warning"] = snapshotWarning
			}
			return flags.printJSON(cmd, result)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "90d", "how far back to look for a baseline snapshot (e.g. 7d, 30d, 90d)")
	return cmd
}

// snapshotStore is the subset of *store.Store this helper needs, so it can
// be exercised without a real SQLite file in unit tests.
type snapshotStore interface {
	ListIDs(resourceType string) ([]string, error)
	Get(resourceType, id string) (json.RawMessage, error)
}

// findConstituentSnapshotBefore finds the most recent snapshot for slug
// whose embedded RFC3339 timestamp is at or before cutoff. Snapshot IDs are
// "<slug>:<RFC3339 timestamp>"; since RFC3339 timestamps sort lexically the
// same as chronologically, the best candidate is the lexically-greatest ID
// that is <= the cutoff-derived ID.
func findConstituentSnapshotBefore(db snapshotStore, slug string, cutoff time.Time) ([]niftyindices.ConstituentRow, string, error) {
	ids, err := db.ListIDs(resourceTypeIndexConstituentSnap)
	if err != nil {
		return nil, "", err
	}
	prefix := slug + ":"
	cutoffID := prefix + cutoff.Format(time.RFC3339)
	var bestID string
	for _, id := range ids {
		if len(id) <= len(prefix) || id[:len(prefix)] != prefix {
			continue
		}
		if id <= cutoffID && id > bestID {
			bestID = id
		}
	}
	if bestID == "" {
		return nil, "", nil
	}
	raw, err := db.Get(resourceTypeIndexConstituentSnap, bestID)
	if err != nil {
		return nil, "", err
	}
	var rows []niftyindices.ConstituentRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, "", fmt.Errorf("parsing cached snapshot %q: %w", bestID, err)
	}
	return rows, bestID[len(prefix):], nil
}

// diffConstituents compares two constituent snapshots by symbol.
func diffConstituents(baseline, current []niftyindices.ConstituentRow) ([]niftyindices.ConstituentRow, []niftyindices.ConstituentRow) {
	baseSymbols := make(map[string]bool, len(baseline))
	for _, r := range baseline {
		baseSymbols[r.Symbol] = true
	}
	curSymbols := make(map[string]bool, len(current))
	for _, r := range current {
		curSymbols[r.Symbol] = true
	}
	var added, removed []niftyindices.ConstituentRow
	for _, r := range current {
		if !baseSymbols[r.Symbol] {
			added = append(added, r)
		}
	}
	for _, r := range baseline {
		if !curSymbols[r.Symbol] {
			removed = append(removed, r)
		}
	}
	return added, removed
}
