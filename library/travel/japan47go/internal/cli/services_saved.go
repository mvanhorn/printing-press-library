// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source local
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/japan47go/internal/service"
	"github.com/spf13/cobra"
)

func newNovelServicesSavedCmd(f *rootFlags) *cobra.Command {
	var query string
	var limit int
	c := &cobra.Command{Use: "saved", Short: "Search saved service facts with their observation times", Long: "Read normalized local observations without any network request or cache migration. A missing store returns an explicit empty observation window. Successful live inspect/compare calls save at most 200 records unless --no-cache is used.", Example: "  japan47go-pp-cli services saved --query 妻籠 --limit 3 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "--query=妻籠;--limit=3"}}
	c.RunE = func(c *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(c.OutOrStdout(), f, "services saved")
		}
		if len(args) > 0 || f.dataSource == "live" || f.noCache || limit < 1 || limit > 50 {
			return serviceError(c, f, usageErr(fmt.Errorf("services saved takes --query and --limit 1..50; it conflicts with --data-source live or --no-cache")))
		}
		ctx, cancel := boundCtx(c.Context(), f)
		defer cancel()
		path, e := serviceCachePath()
		if e != nil {
			return serviceError(c, f, e)
		}
		out, scanned, e := service.Cached(ctx, path, query, limit)
		if e != nil {
			return serviceError(c, f, e)
		}
		for i := range out {
			out[i].Stale = f.maxAge > 0 && out[i].CacheAgeSeconds > int64(f.maxAge.Seconds())
		}
		note := "Saved observations only; run services inspect UUID --refresh to recheck source facts."
		if len(out) == 0 {
			note = "No matching saved observations in this local window; this does not establish source absence. Inspect a known UUID live first."
		}
		return serviceOutput(c, f, map[string]any{"service": out, "query": map[string]any{"keyword": query, "limit": limit}, "capacity": service.Capacity, "scanned_observations": scanned, "returned": len(out), "note": note, "source_boundary": "Offline saved evidence at each observed_at; no current-source or availability claim."})
	}
	c.Flags().StringVar(&query, "query", "", "Literal case-insensitive substring in saved IDs, names, prefectures and cities")
	c.Flags().IntVar(&limit, "limit", 10, "Maximum saved observations returned, from 1 to 50")
	return c
}
