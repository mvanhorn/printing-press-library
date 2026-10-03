// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source local
package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/halal-gourmet-japan/internal/hgj"
	"github.com/mvanhorn/printing-press-library/library/travel/halal-gourmet-japan/internal/store"
	"github.com/spf13/cobra"
)

var hgjPlanDescriptions = map[string]string{
	"compare": "Use this command for comparing source-reported facts of selected places. For explicit requirements use 'plan match'; for missing planning evidence use 'plan gaps'.",
	"match":   "Use this command for testing explicit source condition requirements. For unrestricted facts use 'plan compare'; for certifier, validity, hours or access gaps use 'plan gaps'.",
	"gaps":    "Use this command for missing planning evidence and factual clarification questions. For condition selection use 'plan match'; for saved observation changes use 'plan changes'.",
	"pair":    "Use this command for source-coordinate straight-line proximity pairs. For condition evidence use 'plan compare'; for access and hours gaps use 'plan gaps'.",
	"changes": "Use this command for factual changes between two saved observations of the same place. For different-place facts use 'plan compare'; for missing latest evidence use 'plan gaps'.",
}

type hgjPlanOptions struct {
	restaurants, prayer, required []string
	dbPath                        string
	limit                         int
	maxKM                         float64
}

func configureHGJPlanCmd(cmd *cobra.Command, flags *rootFlags, name string, options *hgjPlanOptions) *cobra.Command {
	short := map[string]string{"compare": "Align selected full-detail food and prayer facts", "match": "Evaluate each requested source label independently", "gaps": "List missing certification, hours and access evidence", "pair": "Join food and prayer stops by straight-line proximity", "changes": "Compare the latest two successful factual observations"}[name]
	fixture := "--restaurants=300739;--prayer=838884"
	if name == "match" {
		fixture += ";--require=certified,prayer"
	}
	if name == "pair" {
		fixture += ";--max-km=5"
	}
	example := "  halal-gourmet-japan-pp-cli plan " + name + " --restaurants 300739 --prayer 838884"
	if name == "match" {
		example += " --require certified,prayer"
	}
	if name == "pair" {
		example += " --max-km 5"
	}
	example += " --agent"
	cmd.Short = short
	cmd.Long = hgjPlanDescriptions[name]
	cmd.Example = example
	cmd.Annotations = map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": fixture, "pp:typed-exit-codes": "0,2,10"}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		restaurants, prayer, required := options.restaurants, options.prayer, options.required
		dbPath, limit, maxKM := options.dbPath, options.limit, options.maxKM
		if len(args) == 0 && hgjHumanBareHelp(cmd, flags) {
			return cmd.Help()
		}
		if len(args) > 0 {
			return usageErr(fmt.Errorf("plan %s accepts --restaurants and --prayer IDs, not positionals", name))
		}
		if flags.dataSource == "live" {
			return usageErr(fmt.Errorf("plan %s has no live equivalent; run scoped get commands to save full details, then use --data-source local", name))
		}
		selections, err := hgj.ParseSelections(restaurants, prayer)
		if err != nil {
			return usageErr(err)
		}
		if len(selections) == 0 {
			return usageErr(fmt.Errorf("select at least one source ID with --restaurants or --prayer"))
		}
		if limit < 1 || limit > 50 {
			return usageErr(fmt.Errorf("--limit must be between 1 and 50"))
		}
		if name == "pair" && (len(restaurants) == 0 || len(prayer) == 0) {
			return usageErr(fmt.Errorf("plan pair requires both --restaurants and --prayer selections"))
		}
		if name == "match" {
			if _, err := hgj.Match(nil, required); err != nil {
				return usageErr(err)
			}
		}
		if name == "pair" {
			if _, err := hgj.Pair(nil, maxKM, limit); err != nil {
				return usageErr(err)
			}
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "plan "+name+" from saved detail observations")
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		loaded := hgj.LoadedSnapshots{Places: []hgj.Place{}, Missing: []hgj.MissingSnapshot{}}
		var db *store.Store
		path := hgjDBPath(dbPath)
		guard, err := hgj.BeginSavedReadContext(ctx, path)
		if err != nil {
			return configErr(err)
		}
		defer guard.Close()
		if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
			for _, s := range selections {
				resource := "restaurants"
				if s.Kind == hgj.Prayer {
					resource = "prayer"
				}
				loaded.Missing = append(loaded.Missing, hgj.MissingSnapshot{Selection: s, State: "detail_inspection_needed", Hint: fmt.Sprintf("run: %s %s get %s --data-source live", cmd.Root().Name(), resource, s.ID)})
			}
		} else {
			db, err = store.OpenReadOnlyContext(ctx, guard.Path())
			if err != nil {
				return configErr(err)
			}
			defer db.Close()
			loaded, err = hgj.LoadSnapshots(ctx, db.DB(), selections)
			if err != nil {
				return configErr(err)
			}
		}
		if len(loaded.Missing) > 0 {
			fmt.Fprintf(cmd.ErrOrStderr(), "%d selected places need full-detail inspection; planning uses %d saved observations\n", len(loaded.Missing), len(loaded.Places))
		}
		out := map[string]any{"results": []any{}, "missing_snapshots": loaded.Missing, "selected_count": len(selections), "loaded_count": len(loaded.Places), "source": "local", "note": "Only saved full-detail evidence is evaluated; directory labels do not guarantee dietary compliance, certification validity or current access."}
		switch name {
		case "compare":
			rows := loaded.Places
			if len(rows) > limit {
				rows = rows[:limit]
			}
			out["results"] = rows
			out["truncated"] = len(rows) < len(loaded.Places)
		case "match":
			rows, e := hgj.Match(loaded.Places, required)
			if e != nil {
				return usageErr(e)
			}
			if len(rows) > limit {
				out["truncated"] = true
				rows = rows[:limit]
			}
			out["results"] = rows
		case "gaps":
			rows, e := hgj.Gaps(loaded.Places, time.Now().UTC())
			if e != nil {
				return configErr(e)
			}
			if len(rows) > limit {
				out["truncated"] = true
				rows = rows[:limit]
			}
			out["results"] = rows
		case "pair":
			pairs, e := hgj.Pair(loaded.Places, maxKM, limit)
			if e != nil {
				return usageErr(e)
			}
			out["results"] = pairs.Results
			out["candidate_pairs"] = pairs.CandidatePairs
			out["skipped_coordinate_pairs"] = pairs.SkippedCoordinatePairs
			out["matched_pairs"] = pairs.MatchedPairs
			out["truncated"] = pairs.Truncated
			out["note"] = pairs.Note
		case "changes":
			rows := []hgj.ChangeRow{}
			for _, p := range loaded.Places {
				var before *hgj.Place
				if db != nil {
					b, ok, e := hgj.Snapshot(ctx, db.DB(), hgj.Selection{Kind: p.Kind, ID: p.ID}, 1)
					if e != nil {
						return configErr(e)
					}
					if ok {
						before = &b
					}
				}
				row, e := hgj.Changes(before, p)
				if e != nil {
					return configErr(e)
				}
				rows = append(rows, row)
			}
			if len(rows) > limit {
				out["truncated"] = true
				rows = rows[:limit]
			}
			out["results"] = rows
		}
		if err = guard.Check(); err != nil {
			return configErr(err)
		}
		for _, p := range loaded.Places {
			hgjStaleHint(cmd, flags, p)
		}
		flags.agentSource = "local"
		return flags.printJSON(cmd, out)
	}
	return cmd
}
