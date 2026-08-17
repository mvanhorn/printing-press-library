// Copyright 2026 ganes-j and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: undervalued detection vs local trailing baseline.
//
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/discogs/internal/cliutil"

	"github.com/spf13/cobra"
)

type undervaluedRow struct {
	ReleaseID int64   `json:"release_id"`
	Title     string  `json:"title,omitempty"`
	Current   float64 `json:"current_lowest"`
	Baseline  float64 `json:"baseline_median"`
	PctBelow  float64 `json:"pct_below"`
}

type undervaluedView struct {
	Undervalued []undervaluedRow `json:"undervalued"`
	Scanned     int              `json:"scanned"`
	NoBaseline  int              `json:"no_baseline"`
	Note        string           `json:"note,omitempty"`
}

func newNovelUndervaluedCmd(flags *rootFlags) *cobra.Command {
	var scope string
	var currency string
	var threshold float64
	var maxScan int

	cmd := &cobra.Command{
		Use:   "undervalued",
		Short: "Flag releases whose current asking price sits below their own trailing/suggested baseline.",
		Long: "Compares each release's live marketplace lowest asking against its own trailing median from " +
			"local price snapshots. Needs some price history to have a baseline — run 'sync' or this command " +
			"a few times first.\n\nUse this for market-wide mispricing with no preset limit. For wantlist items " +
			"hitting a limit you set, use 'fills'.",
		Example:     "  discogs-pp-cli undervalued --scope wantlist --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := checkDataSource(flags, "live"); err != nil {
				return err
			}
			if scope != "wantlist" && scope != "collection" {
				return usageErr(fmt.Errorf("--scope must be wantlist or collection, got %q", scope))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			st, err := openDiscogsStore(ctx, flags)
			if err != nil {
				return err
			}
			defer st.Close()

			// Collect (release_id, title) for the scope (drain-first).
			type target struct {
				id    int64
				title string
			}
			var targets []target
			rows, err := st.DB().QueryContext(ctx, `SELECT id, data FROM resources WHERE resource_type = ?`, scope)
			if err != nil {
				return fmt.Errorf("reading %s: %w", scope, err)
			}
			// Collection rows are one-per-instance, so a multi-copy release
			// appears N times; dedup by release_id to avoid N redundant stats
			// calls and N duplicate snapshots polluting the trailing median.
			seen := map[int64]bool{}
			for rows.Next() {
				var idStr string
				var data []byte
				if err := rows.Scan(&idStr, &data); err != nil {
					continue
				}
				var bi struct {
					BasicInfo discogsBasicInfo `json:"basic_information"`
				}
				_ = json.Unmarshal(data, &bi)
				rid := bi.BasicInfo.ID
				if rid == 0 && scope != "collection" {
					// Only wantlist rows are keyed by release id, so parsing the
					// store key back into a release id is safe there. Collection
					// rows are keyed by instance_id, so the same fallback would
					// fetch stats and read/write snapshots for the wrong release —
					// skip the row instead of guessing.
					if v, perr := strconv.ParseInt(idStr, 10, 64); perr == nil {
						rid = v
					}
				}
				if rid == 0 || seen[rid] {
					continue
				}
				seen[rid] = true
				targets = append(targets, target{id: rid, title: bi.BasicInfo.display()})
			}
			_ = rows.Err()
			_ = rows.Close()

			view := undervaluedView{Undervalued: make([]undervaluedRow, 0)}
			if len(targets) == 0 {
				view.Note = fmt.Sprintf("no %s synced — run 'sync --resources %s'", scope, scope)
				return emitUndervalued(cmd, flags, view)
			}

			scanMax := len(targets)
			if maxScan > 0 && maxScan < scanMax {
				scanMax = maxScan
			}
			if cliutil.IsDogfoodEnv() && scanMax > 3 {
				scanMax = 3
			}

			for i := 0; i < scanMax; i++ {
				t := targets[i]
				med, hasMed := trailingMedian(ctx, st, t.id) // baseline from prior history
				stats, cerr := captureSnapshot(ctx, c, st, t.id, currency)
				view.Scanned++
				if cerr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not check release %d: %v\n", t.id, cerr)
					continue
				}
				if !hasMed {
					view.NoBaseline++
					continue
				}
				if stats.LowestPrice == nil || stats.LowestPrice.Value == nil {
					continue
				}
				cur := *stats.LowestPrice.Value
				if cur < med*(1-threshold/100.0) {
					pct := 0.0
					if med > 0 {
						pct = (med - cur) / med * 100.0
					}
					view.Undervalued = append(view.Undervalued, undervaluedRow{
						ReleaseID: t.id, Title: t.title, Current: cur, Baseline: med, PctBelow: pct,
					})
				}
			}
			if view.NoBaseline > 0 && len(view.Undervalued) == 0 {
				view.Note = fmt.Sprintf("%d release(s) have no price baseline yet — run 'sync' or 'undervalued' a few times to build history", view.NoBaseline)
			}
			return emitUndervalued(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&scope, "scope", "wantlist", "Which set to scan: wantlist or collection")
	cmd.Flags().StringVar(&currency, "currency", "", "Currency for marketplace checks (e.g. USD)")
	cmd.Flags().Float64Var(&threshold, "threshold", 0, "Only flag items at least this percent below baseline")
	cmd.Flags().IntVar(&maxScan, "max", 0, "Maximum releases to scan (0 = all)")
	return cmd
}

func emitUndervalued(cmd *cobra.Command, flags *rootFlags, view undervaluedView) error {
	if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
		return printJSONFiltered(cmd.OutOrStdout(), view, flags)
	}
	for _, r := range view.Undervalued {
		title := r.Title
		if title == "" {
			title = fmt.Sprintf("release %d", r.ReleaseID)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%-40s  %.2f (%.0f%% below %.2f)\n", truncate(title, 40), r.Current, r.PctBelow, r.Baseline)
	}
	if view.Note != "" {
		fmt.Fprintln(cmd.OutOrStdout(), view.Note)
	}
	return nil
}
