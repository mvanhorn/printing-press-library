// Copyright 2026 dhilip-subramanian. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads/internal/store"

	"github.com/spf13/cobra"
)

type overlapPair struct {
	AudienceA  string  `json:"audience_a"`
	AudienceB  string  `json:"audience_b"`
	OverlapPct float64 `json:"overlap_pct,omitempty"`
	Verdict    string  `json:"verdict"`
	Note       string  `json:"note,omitempty"`
}

type overlapView struct {
	Audiences []string      `json:"audiences"`
	Pairs     []overlapPair `json:"pairs"`
	Note      string        `json:"note,omitempty"`
}

func newNovelOverlapCmd(flags *rootFlags) *cobra.Command {
	var flagAudience []string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "overlap",
		Short: "Pairwise overlap percentages across custom audiences.",
		Long: `For each pair of supplied custom audience IDs, look up the overlap percentage
in the local store (synced via Meta's audience_overlap endpoint). Flags pairs
with >30% overlap as cannibalization risk.

Specify at least two audiences with repeated --audience flags.`,
		Example: `  meta-ads-pp-cli overlap --audience 23847001 --audience 23847002 --audience 23847003 --agent
  meta-ads-pp-cli overlap --audience 23847001 --audience 23847002 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute pairwise overlap from local audience_overlap store")
				return nil
			}
			if len(flagAudience) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("at least two --audience flags are required"))
			}

			if dbPath == "" {
				dbPath = defaultDBPath("meta-ads-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			view := overlapView{
				Audiences: flagAudience,
				Pairs:     make([]overlapPair, 0),
			}

			// Walk every unordered pair
			for i := 0; i < len(flagAudience); i++ {
				for j := i + 1; j < len(flagAudience); j++ {
					a := flagAudience[i]
					b := flagAudience[j]
					pct, ok := lookupOverlap(cmd.Context(), db, a, b)
					pair := overlapPair{AudienceA: a, AudienceB: b}
					if !ok {
						pair.Verdict = "no-data"
						pair.Note = "no overlap row in local store; sync audience_overlap resource for this pair"
					} else {
						pair.OverlapPct = pct
						if pct >= 30 {
							pair.Verdict = "cannibalization-risk"
							pair.Note = "consider consolidating or excluding one audience from the other's targeting"
						} else if pct >= 15 {
							pair.Verdict = "moderate-overlap"
						} else {
							pair.Verdict = "low-overlap"
						}
					}
					view.Pairs = append(view.Pairs, pair)
				}
			}

			missing := 0
			for _, p := range view.Pairs {
				if p.Verdict == "no-data" {
					missing++
				}
			}
			if missing == len(view.Pairs) {
				view.Note = "no audience_overlap data in local store. Meta's audience_overlap endpoint needs to be called separately and persisted before this command can compute overlap."
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringSliceVar(&flagAudience, "audience", nil, "Custom audience ID (specify two or more)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (defaults to ~/.meta-ads-pp-cli/data.db)")
	return cmd
}

func lookupOverlap(ctx context.Context, db *store.Store, a, b string) (float64, bool) {
	q := `SELECT data FROM resources
		WHERE resource_type IN ('audience_overlap', 'customaudiences_overlap')
		  AND ((json_extract(data, '$.audience_a') = ? AND json_extract(data, '$.audience_b') = ?)
		    OR (json_extract(data, '$.audience_a') = ? AND json_extract(data, '$.audience_b') = ?))
		LIMIT 1`
	row := db.DB().QueryRowContext(ctx, q, a, b, b, a)
	var data []byte
	if err := row.Scan(&data); err != nil {
		return 0, false
	}
	var raw struct {
		OverlapPct float64 `json:"overlap_pct"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return 0, false
	}
	return raw.OverlapPct, true
}
