// Copyright 2026 david. Licensed under Apache-2.0. See LICENSE.

// digest.go implements `slickdeals-pp-cli digest`: reads the local
// deal_snapshots table, returns the top-N deals (by thumbs) captured since
// a cutoff. Optional --merchant-cap caps items per merchant; --grouped-by
// buckets the output by category or merchant. Empty store is a valid
// state — we print a helpful stderr hint and return an empty envelope.
//
// We use raw SQL against db.DB() because the Snapshots-Analytics engineer's
// `store.QuerySnapshotsSince` API isn't available yet. CREATE TABLE IF NOT
// EXISTS makes the path safe even before any rows have been written.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/slickdeals/internal/store"

	"github.com/spf13/cobra"
)

// digestSnapshot is the row shape we read out of deal_snapshots for digest
// output. Projects store.DealSnapshot down to the fields the CLI surfaces.
type digestSnapshot struct {
	DealID     string    `json:"deal_id"`
	CapturedAt time.Time `json:"captured_at"`
	Thumbs     int       `json:"thumbs"`
	Merchant   string    `json:"merchant,omitempty"`
	Category   string    `json:"category,omitempty"`
	Title      string    `json:"title,omitempty"`
}

// projectSnapshots maps store.DealSnapshot rows to the CLI-facing
// digestSnapshot shape and sorts by thumbs DESC, captured_at DESC.
func projectSnapshots(snaps []store.DealSnapshot) []digestSnapshot {
	out := make([]digestSnapshot, 0, len(snaps))
	for _, s := range snaps {
		out = append(out, digestSnapshot{
			DealID:     s.DealID,
			CapturedAt: s.CapturedAt,
			Thumbs:     s.Thumbs,
			Merchant:   s.Merchant,
			Category:   s.Category,
			Title:      s.Title,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Thumbs != out[j].Thumbs {
			return out[i].Thumbs > out[j].Thumbs
		}
		return out[i].CapturedAt.After(out[j].CapturedAt)
	})
	return out
}

// applyMerchantCap drops items once a merchant has appeared `cap` times.
// Items with empty merchant are never capped (no merchant attribution).
// `in` must already be sorted by thumbs DESC; this preserves that order.
func applyMerchantCap(in []digestSnapshot, capN int) []digestSnapshot {
	if capN <= 0 {
		return in
	}
	seen := map[string]int{}
	out := make([]digestSnapshot, 0, len(in))
	for _, s := range in {
		if s.Merchant == "" {
			out = append(out, s)
			continue
		}
		if seen[s.Merchant] >= capN {
			continue
		}
		seen[s.Merchant]++
		out = append(out, s)
	}
	return out
}

// groupSnapshots buckets items by group key (merchant or category). Returns
// a map keyed by the group with each bucket preserving the input order. The
// bucket order in the returned struct is sorted alphabetically for stable
// JSON output.
func groupSnapshots(in []digestSnapshot, groupBy string) map[string][]digestSnapshot {
	groups := map[string][]digestSnapshot{}
	for _, s := range in {
		var key string
		switch groupBy {
		case "merchant":
			key = s.Merchant
		case "category":
			key = s.Category
		}
		if key == "" {
			key = "(unspecified)"
		}
		groups[key] = append(groups[key], s)
	}
	return groups
}

func newDigestCmd(flags *rootFlags) *cobra.Command {
	var since string
	var top int
	var merchantCap int
	var groupedBy string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "digest",
		Short:       "Summarize the top deals captured locally over a recent window",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Read the local deal_snapshots store and return the top deals captured
within a recent window, sorted by thumbs. Snapshots are populated by
'slickdeals-pp-cli watch <deal-id> --persist' and (when implemented)
'sync'. Empty store is a valid state — digest prints a hint and returns
an empty envelope.`,
		Example: `  # Top 10 deals from the last 24 hours
  slickdeals-pp-cli digest --since 24h --top 10 --json

  # Bucket the last 4 hours by merchant
  slickdeals-pp-cli digest --since 4h --grouped-by merchant --json

  # Cap any one merchant at 3 deals
  slickdeals-pp-cli digest --since 7d --top 20 --merchant-cap 3 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			switch groupedBy {
			case "", "merchant", "category":
				// ok
			default:
				return usageErr(fmt.Errorf("--grouped-by must be one of merchant, category, or empty; got %q", groupedBy))
			}

			cutoff, err := parseSinceDuration(since)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since value %q: %w", since, err))
			}

			if dbPath == "" {
				dbPath = defaultDBPath("slickdeals-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			rows, err := db.QuerySnapshotsSince(cutoff, 0)
			if err != nil {
				return fmt.Errorf("querying snapshots: %w", err)
			}
			snaps := projectSnapshots(rows)
			if snaps == nil {
				snaps = []digestSnapshot{} // render JSON [] not null for empty results
			}

			if len(snaps) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(),
					"No snapshots in the local store yet. Run `slickdeals-pp-cli watch <deal-id> --persist` or `slickdeals-pp-cli sync` to populate the snapshot store.")
			}

			// Already sorted by thumbs DESC from the SQL. Apply merchant cap
			// before limiting to top — cap is a per-merchant dedup, top is a
			// hard cut.
			snaps = applyMerchantCap(snaps, merchantCap)
			if top > 0 && len(snaps) > top {
				snaps = snaps[:top]
			}

			var payload any = snaps
			if groupedBy != "" {
				grouped := groupSnapshots(snaps, groupedBy)
				// Materialize an ordered view so JSON output is stable.
				keys := make([]string, 0, len(grouped))
				for k := range grouped {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				ordered := make(map[string][]digestSnapshot, len(keys))
				for _, k := range keys {
					ordered[k] = grouped[k]
				}
				payload = ordered
			}

			// Pick SyncedAt from the most recent snapshot if we have any.
			var syncedAt *time.Time
			if len(snaps) > 0 {
				latest := snaps[0].CapturedAt
				for _, s := range snaps[1:] {
					if s.CapturedAt.After(latest) {
						latest = s.CapturedAt
					}
				}
				syncedAt = &latest
			} else {
				now := time.Now().UTC()
				syncedAt = &now
			}

			raw, err := json.Marshal(payload)
			if err != nil {
				return err
			}
			envelope, err := wrapWithProvenance(raw, DataProvenance{
				Source:       "local",
				SyncedAt:     syncedAt,
				ResourceType: "digest",
			})
			if err != nil {
				return err
			}
			printProvenance(cmd, len(snaps), DataProvenance{Source: "local", SyncedAt: syncedAt, ResourceType: "digest"})
			return printJSONFiltered(cmd.OutOrStdout(), json.RawMessage(envelope), flags)
		},
	}

	cmd.Flags().StringVar(&since, "since", "24h", "Window for snapshot inclusion (e.g. 24h, 7d, 30m, 1w)")
	cmd.Flags().IntVar(&top, "top", 20, "Maximum number of deals in the result (0 = no limit)")
	cmd.Flags().IntVar(&merchantCap, "merchant-cap", 0, "Maximum deals per merchant (0 = no cap)")
	cmd.Flags().StringVar(&groupedBy, "grouped-by", "", "Bucket output: merchant, category, or empty for flat list")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/slickdeals-pp-cli/data.db)")

	return cmd
}
