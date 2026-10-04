// Copyright 2026 Jet Sng and contributors. Licensed under Apache-2.0.
// pp:data-source auto
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/wheelog/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/wheelog/internal/store"
	"github.com/mvanhorn/printing-press-library/library/travel/wheelog/internal/wheelog"
	"github.com/spf13/cobra"
)

func newNovelShortlistChangesCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var options wheelogReadOptions
	cmd := &cobra.Command{Use: "changes", Short: "Compare saved source observations or fetch a bounded current observation.",
		Long:        "Show exact source field and aggregate-count changes. Auto prefers a new anonymous detail observation and labels saved-pair fallback; live requires source success; local compares the last stored pair. A failed source response never replaces the saved baseline. Count changes do not establish a physical change. For recheck triage without refreshing use shortlist list --audit.",
		Example:     "  wheelog-pp-cli shortlist changes --data-source live --limit 3 --agent\n  wheelog-pp-cli shortlist changes --data-source local --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:local-write": "true", "pp:live-happy-path": "true", "pp:data-source": "auto", "pp:happy-args": "--data-source=local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "compare saved WheeLog observations")
			}
			if len(args) > 0 {
				return usageErr(fmt.Errorf("shortlist changes accepts flags, not positional arguments"))
			}
			maxAge, err := validateWheelogOptions(options)
			if err != nil {
				return err
			}
			mode, err := wheelogMode(flags)
			if err != nil {
				return err
			}
			if limit < 1 || limit > 50 || (mode != "local" && limit > 5) {
				return usageErr(fmt.Errorf("--limit must be 1..50 for local comparisons and 1..5 for source reads"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			var db *store.Store
			var saved []store.WheelogObservation
			if mode == "local" {
				saved, err = savedWheelog(ctx, options.DB)
			} else {
				db, err = openWheelogStore(ctx, options.DB)
				if err != nil {
					return err
				}
				defer db.Close()
				saved, err = db.WheelogList(ctx)
			}
			if err != nil {
				return err
			}
			if mode == "local" {
				wheelogLocalHint(cmd, saved, maxAge)
			}
			var c *client.Client
			if mode != "local" && len(saved) > 0 {
				c, err = newWheelogClient(flags)
				if err != nil {
					return err
				}
			}
			rows := make([]map[string]any, 0)
			failures := make([]wheelogFailure, 0)
			live := 0
			for index, entry := range saved {
				if index >= limit {
					break
				}
				latest := entry.Latest
				previous := entry.Previous
				source := "local"
				var fallback string
				if mode != "local" {
					fresh, err := fetchWheelogSpot(ctx, c, entry.Latest.ID)
					if err != nil {
						if isWheelogThrottle(err) {
							return wheelogError(err)
						}
						failures = append(failures, wheelogFailure{entry.Latest.ID, err.Error()})
						if mode == "live" {
							rows = append(rows, map[string]any{"id": entry.Latest.ID, "status": "unavailable", "source_url": entry.Latest.SourceURL})
							continue
						}
						fallback = err.Error()
					} else {
						old := latest
						previous = &old
						latest = fresh
						source = "live"
						live++
						if err := db.ObserveWheelog(ctx, fresh); err != nil {
							return configErr(err)
						}
					}
				}
				changes := make([]wheelog.Change, 0)
				status := "no_baseline"
				var previousTime *string
				if previous != nil {
					previousTime = &previous.ObservedAt
					changes = wheelog.Changes(*previous, latest)
					status = "unchanged"
					if len(changes) > 0 {
						status = "changed"
					}
				}
				assessment := viewWheelog(latest, options.Questions, maxAge)
				row := map[string]any{"id": latest.ID, "name": latest.Name, "source_url": latest.SourceURL, "status": status, "changes": changes, "previous_retrieved_at": previousTime, "current_retrieved_at": latest.ObservedAt, "data_source": source, "requirements": assessment.Requirements, "recheck_reasons": assessment.Recheck, "record_update_age_days": assessment.UpdateAge, "cache_age_days": assessment.CacheAge}
				if fallback != "" {
					row["fallback_reason"] = fallback
				}
				rows = append(rows, row)
			}
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d source reads failed; %d new observations checked, saved baselines retained for failures\n", len(failures), live)
			}
			if mode == "live" && len(saved) > 0 && live == 0 {
				return apiErr(fmt.Errorf("no selected source spot could be refreshed; saved observations were retained"))
			}
			source := "local"
			if live > 0 {
				source = "live"
				if mode == "auto" && len(failures) > 0 {
					source = "auto"
				}
			}
			return emitWheelog(cmd, flags, map[string]any{"results": rows, "fetch_failures": failures, "coverage": map[string]any{"scope": "selected_saved_spots", "saved": len(saved), "examined": len(rows), "live_observations": live, "output_limit": limit}, "note": "Changes describe recorded public-place fields and aggregate reports, not a physical improvement or deterioration. Local mode shows the last saved transition."}, source)
		}}
	cmd.Flags().IntVar(&limit, "limit", 5, "Maximum selected spots checked; up to 5 source reads or 50 local comparisons.")
	addWheelogReadFlags(cmd, &options)
	return cmd
}
