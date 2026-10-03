// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// configurePlanningCache replaces generic provider-sync assumptions with the
// actual manual normalized snapshot contract. It never queries the provider.
func configurePlanningCache(root *cobra.Command, flags *rootFlags) {
	if source := findSubcommand(root, "source"); source != nil {
		examples := map[string]string{
			"availability": "  hostelworld-pp-cli source availability 67481 --check-in 2026-11-10 --nights 3 --guests 2 --agent",
			"city-search":  "  hostelworld-pp-cli source city-search 452 --check-in 2026-11-10 --nights 3 --guests 2 --page-size 5 --agent",
			"locations":    "  hostelworld-pp-cli source locations --query Osaka --agent",
			"property":     "  hostelworld-pp-cli source property 67481 --agent",
		}
		for name, example := range examples {
			if cmd := findSubcommand(source, name); cmd != nil {
				cmd.Example = example
			}
		}
	}
	if cmd := findSubcommand(root, "search"); cmd != nil {
		cmd.Short = "Search explicitly saved planning observations offline; dated prices are stale"
		cmd.Long = "Search the bounded local planning_snapshot cache populated by hostels inspect/offers --save. Auto and local both read saved observations. Live discovery uses destinations search and hostels search instead."
		cmd.Example = "  hostelworld-pp-cli search Nui --agent --limit 5"
		cmd.Annotations["pp:data-source"] = "local"
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "search manually saved planning snapshots")
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			limit, _ := cmd.Flags().GetInt("limit")
			if len(args) != 1 || limit < 1 || limit > 50 || len(args[0]) > 200 {
				return usageErr(fmt.Errorf("search takes one query of at most 200 characters; --limit must be 1–50"))
			}
			resource, _ := cmd.Flags().GetString("type")
			if resource != "" && resource != "planning_snapshot" {
				return usageErr(fmt.Errorf("--type must be planning_snapshot for the manual cache"))
			}
			path, _ := cmd.Flags().GetString("db")
			out := []any{}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, guard, err := OpenPlanningReadOnly(ctx, path)
			if err != nil {
				return err
			}
			defer guard.Close()
			if db != nil {
				defer db.Close()
				rows, err := db.Search(args[0], limit, "planning_snapshot")
				if err != nil {
					return fmt.Errorf("search saved snapshots: %w", err)
				}
				for _, raw := range rows {
					var v map[string]any
					if err := json.Unmarshal(raw, &v); err != nil {
						return fmt.Errorf("saved snapshot is invalid")
					}
					v["freshness"] = "stale"
					out = append(out, v)
				}
			}
			if err := guard.Check(); err != nil {
				return err
			}
			return flags.printJSON(cmd, map[string]any{"results": out, "freshness": "stale", "population": "hostels inspect/offers --save", "notice": "saved observations are incomplete and dated prices must be refreshed live"})
		}
	}
	if parent := findSubcommand(root, "workflow"); parent != nil {
		if cmd := findSubcommand(parent, "archive"); cmd != nil {
			cmd.Hidden = true
			if cmd.Annotations == nil {
				cmd.Annotations = map[string]string{}
			}
			cmd.Annotations["mcp:hidden"] = "true"
			cmd.Annotations["pp:typed-exit-codes"] = "0,2"
			cmd.Short = "Provider archive is unsupported; save individual planning observations"
			cmd.Long = cmd.Short
			cmd.Example = "  hostelworld-pp-cli hostels inspect 67481 --save --agent"
			cmd.RunE = func(cmd *cobra.Command, args []string) error {
				if dryRunOK(flags) {
					return writeDryRun(cmd.OutOrStdout(), flags, "provider archive is unsupported")
				}
				return usageErr(fmt.Errorf("provider archive is unsupported; use hostels inspect/offers --save then hostels saved; saved dated observations are stale"))
			}
		}
	}
	// The framework workflow status describes only this local cache.
	if parent := findSubcommand(root, "workflow"); parent != nil {
		if cmd := findSubcommand(parent, "status"); cmd != nil {
			cmd.Short = "Inspect local cache status; no provider snapshot refresh"
			cmd.Long = "Inspect the selected profile's manual observation counts. No provider snapshot or archive is available; populate with hostels inspect/offers --save."
			cmd.Example = "  hostelworld-pp-cli workflow status --agent"
			cmd.Annotations["pp:data-source"] = "local"
			cmd.RunE = func(cmd *cobra.Command, args []string) error {
				if len(args) != 0 {
					return usageErr(fmt.Errorf("workflow status takes no arguments"))
				}
				if err := validateDataSourceStrategy(flags, "local"); err != nil {
					return usageErr(err)
				}
				if dryRunOK(flags) {
					return writeDryRun(cmd.OutOrStdout(), flags, "manual cache status")
				}
				ctx, cancel := boundCtx(cmd.Context(), flags)
				defer cancel()
				requested, _ := cmd.Flags().GetString("db")
				db, guard, err := OpenPlanningReadOnly(ctx, requested)
				if err != nil {
					return err
				}
				defer guard.Close()
				counts := map[string]int{}
				if db != nil {
					defer db.Close()
					counts, err = db.Status()
					if err != nil {
						return err
					}
				}
				if err := guard.Check(); err != nil {
					return err
				}
				return flags.printJSON(cmd, map[string]any{"status": "local_cache_only", "provider_snapshot_refreshed": false, "counts": counts, "freshness": "stale", "population": "hostels inspect/offers --save", "retention_limit": 200})
			}
		}
	}
}
