// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source auto
package cli

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/halal-gourmet-japan/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/halal-gourmet-japan/internal/config"
	"github.com/mvanhorn/printing-press-library/library/travel/halal-gourmet-japan/internal/hgj"
	"github.com/mvanhorn/printing-press-library/library/travel/halal-gourmet-japan/internal/store"
	"github.com/spf13/cobra"
)

// The generated runtime hook supplies source-specific global flag semantics.
func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		if f := root.PersistentFlags().Lookup("max-age"); f != nil {
			flags.maxAge = 24 * time.Hour
			f.DefValue = "24h"
			f.Usage = "Maximum saved-detail age before a stderr recheck hint; 0 disables"
		}
		if f := root.PersistentFlags().Lookup("data-source"); f != nil {
			f.Usage = "Detail reads: auto/live refresh source, local reads saved evidence; search requires live; plans use local"
		}
		if f := root.PersistentFlags().Lookup("no-cache"); f != nil {
			f.Usage = "Do not save successful source detail observations in the local database"
		}
		if f := root.PersistentFlags().Lookup("rate-limit"); f != nil {
			f.Usage = "Source requests per second (auto starts at 1; positive values capped at 2; 0 disables)"
		}
		if plan, _, err := root.Find([]string{"plan"}); err == nil && plan != root {
			plan.Short = "Compare and evaluate selected saved full-detail evidence"
		}
	})
}
func hgjDBPath(path string) string {
	if path != "" {
		return path
	}
	return defaultDBPath("halal-gourmet-japan")
}
func hgjSourceClient(flags *rootFlags) (*hgj.Client, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return nil, configErr(err)
	}
	return hgj.NewClient(cfg.BaseURL, flags.timeout, flags.rateLimit), nil
}
func hgjSourceError(cmd *cobra.Command, flags *rootFlags, err error) error {
	code := 5
	var r *cliutil.RateLimitError
	var h *hgj.HTTPError
	if errors.As(err, &r) {
		code = 7
	} else if errors.As(err, &h) && h.Status == 404 {
		code = 3
	}
	writeAPIErrorEnvelope(cmd.OutOrStdout(), flags, err, code)
	if code == 7 {
		return rateLimitErr(err)
	}
	if code == 3 {
		return notFoundErr(err)
	}
	return apiErr(err)
}
func hgjStaleHint(cmd *cobra.Command, flags *rootFlags, p hgj.Place) {
	if flags.maxAge <= 0 {
		return
	}
	at, err := time.Parse(time.RFC3339Nano, p.ObservedAt)
	if err == nil && time.Since(at) > flags.maxAge {
		resource := "restaurants"
		if p.Kind == hgj.Prayer {
			resource = "prayer"
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "saved %s:%s was observed %s; recheck with %s %s get %s --data-source live\n", p.Kind, p.ID, p.ObservedAt, cmd.Root().Name(), resource, p.ID)
	}
}

func configureHGJGetCmd(cmd *cobra.Command, flags *rootFlags, kind string, dbPath *string) *cobra.Command {
	resource, path, id := "restaurants", "/restaurant/{id}", "300739"
	if kind == hgj.Prayer {
		resource, path, id = "prayer", "/pray/{id}", "838884"
	}
	cmd.Short = "Inspect full source conditions and save a bounded factual observation"
	cmd.Example = "  halal-gourmet-japan-pp-cli " + resource + " get " + id + " --agent\n  halal-gourmet-japan-pp-cli " + resource + " get " + id + " --data-source local --agent"
	cmd.Annotations = map[string]string{"mcp:read-only": "false", "mcp:local-write": "true", "pp:domain-endpoint": resource + ".get", "pp:method": "GET", "pp:path": path, "pp:data-source": "auto", "pp:happy-args": "id=" + id, "pp:live-happy-path": "true", "pp:typed-exit-codes": "0,2,3,5,7,10"}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && hgjHumanBareHelp(cmd, flags) {
			return cmd.Help()
		}
		if len(args) != 1 {
			return usageErr(fmt.Errorf("provide exactly one numeric source ID: %s get %s", resource, id))
		}
		if _, err := hgj.CanonicalURL(kind, args[0]); err != nil {
			return usageErr(err)
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "inspect "+kind+" full-detail page")
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		selection := hgj.Selection{Kind: kind, ID: args[0]}
		if flags.dataSource == "local" {
			path := hgjDBPath(*dbPath)
			guard, err := hgj.BeginSavedReadContext(ctx, path)
			if err != nil {
				return configErr(err)
			}
			defer guard.Close()
			if _, err := os.Stat(path); os.IsNotExist(err) {
				return notFoundErr(fmt.Errorf("no saved detail for %s:%s; run %s %s get %s --data-source live", kind, args[0], cmd.Root().Name(), resource, args[0]))
			}
			db, err := store.OpenReadOnlyContext(ctx, guard.Path())
			if err != nil {
				return configErr(err)
			}
			defer db.Close()
			p, ok, err := hgj.Snapshot(ctx, db.DB(), selection, 0)
			if err != nil {
				return configErr(err)
			}
			if err = guard.Check(); err != nil {
				return configErr(err)
			}
			if !ok {
				return notFoundErr(fmt.Errorf("no saved detail for %s:%s; run %s %s get %s --data-source live", kind, args[0], cmd.Root().Name(), resource, args[0]))
			}
			flags.agentSource = "local"
			hgjStaleHint(cmd, flags, p)
			return hgjPrintDetail(cmd, flags, p)
		}
		c, err := hgjSourceClient(flags)
		if err != nil {
			return err
		}
		p, err := c.Detail(ctx, kind, args[0])
		if err != nil {
			return hgjSourceError(cmd, flags, err)
		}
		if !flags.noCache {
			writeGuard, err := hgj.BeginSavedWrite(hgjDBPath(*dbPath))
			if err != nil {
				return configErr(err)
			}
			db, err := store.OpenWithContext(ctx, writeGuard.Path())
			if err != nil {
				return configErr(err)
			}
			defer db.Close()
			if err = writeGuard.Check(); err != nil {
				return configErr(err)
			}
			if err = hgj.SaveSnapshot(ctx, db.DB(), p); err != nil {
				return configErr(err)
			}
			if err = writeGuard.Check(); err != nil {
				return configErr(err)
			}
		}
		flags.agentSource = "live"
		return hgjPrintDetail(cmd, flags, p)
	}
	return cmd
}
func configureHGJSearchCmd(cmd *cobra.Command, flags *rootFlags, kind string, o *hgj.SearchOptions) *cobra.Command {
	o.Kind = kind
	resource := "restaurants"
	if kind == hgj.Prayer {
		resource = "prayer"
	}
	cmd.Short = "Discover source cards; inspect full details before requirement matching"
	cmd.Example = "  halal-gourmet-japan-pp-cli " + resource + " search --prefecture Tokyo --limit 5 --agent"
	cmd.Annotations = map[string]string{"mcp:read-only": "true", "pp:domain-endpoint": resource + ".search", "pp:method": "GET", "pp:path": "/search", "pp:data-source": "live", "pp:happy-args": "--prefecture=Tokyo;--limit=5", "pp:typed-exit-codes": "0,2,3,5,7,10"}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && hgjHumanBareHelp(cmd, flags) {
			return cmd.Help()
		}
		if len(args) != 0 {
			return usageErr(fmt.Errorf("search uses --query or --prefecture, not positional arguments"))
		}
		if flags.dataSource == "local" {
			return usageErr(fmt.Errorf("source search has no local equivalent; inspect candidates and use plan commands for saved detail evidence"))
		}
		if _, err := o.Values(); err != nil {
			return usageErr(err)
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "GET source search cards")
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		c, err := hgjSourceClient(flags)
		if err != nil {
			return err
		}
		out, err := c.Search(ctx, *o)
		if err != nil {
			return hgjSourceError(cmd, flags, err)
		}
		flags.agentSource = "live"
		return flags.printJSON(cmd, out)
	}
	return cmd
}

// Human bare invocations can explore help; machine calls must satisfy command inputs.
func hgjHumanBareHelp(cmd *cobra.Command, flags *rootFlags) bool {
	return cmd.Flags().NFlag() == 0 && !flags.dryRun && !flags.asJSON && !flags.noInput && !flags.agent && os.Getenv("HALAL_GOURMET_JAPAN_LEARN_SURFACE") != "mcp"
}
func hgjPrintDetail(cmd *cobra.Command, flags *rootFlags, p hgj.Place) error {
	if flags.quiet || flags.plain || flags.csv {
		return flags.printJSON(cmd, []hgj.Place{p})
	}
	return flags.printJSON(cmd, p)
}
