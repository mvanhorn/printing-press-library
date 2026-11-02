package cli

import (
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/config"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/govcache"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/govdata"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/store"
	"github.com/spf13/cobra"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// pp:data-source live
func init() {
	registerNovelCommand(func(root *cobra.Command, f *rootFlags) {
		root.AddCommand(newGovCatalog(f), newGovDataset(f), newGovResource(f))
		for _, old := range root.Commands() {
			if old.Name() == "sync" {
				root.RemoveCommand(old)
				break
			}
		}
		root.AddCommand(newGovSync(f))
	})
}
func govClient(f *rootFlags, demo bool) (*govdata.Client, error) {
	cfg, e := config.Load(f.configPath)
	if e != nil {
		return nil, e
	}
	key := cfg.AuthHeader()
	if demo && key == "" {
		key = "DEMO_KEY"
	}
	httpClient, e := govHTTPClient(f)
	if e != nil {
		return nil, e
	}
	c := govdata.New(key)
	c.ConfigureHTTP(httpClient, f.rateLimit)
	return c, nil
}
func govAnnotations() map[string]string {
	return map[string]string{"pp:data-source": "live", "mcp:read-only": "true"}
}
func newGovCatalog(f *rootFlags) *cobra.Command {
	parent := &cobra.Command{Use: "catalog", Short: "Search and export California and federal metadata"}
	parent.AddCommand(newGovCatalogFind(f))
	export := newGovCatalogFind(f)
	export.Use = "export [query]"
	export.Example = "  government-data-pp-cli catalog export water --source california --limit 2 --json"
	export.Aliases = nil
	parent.AddCommand(export)
	for _, mode := range []string{"organizations", "topics"} {
		mode := mode
		var source string
		var demo bool
		var limit int
		cmd := &cobra.Command{Use: mode, Short: "List source-native catalog discovery metadata", Example: "  government-data-pp-cli catalog " + mode + " --source california --limit 5 --json", Annotations: govAnnotations(), RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(f) {
				return writeDryRun(cmd.OutOrStdout(), f, "catalog read")
			}
			if limit < 1 || limit > 1000 {
				return usageErr(fmt.Errorf("--limit must be 1..1000"))
			}
			srcs, e := govdata.Sources(source)
			if e != nil {
				return usageErr(e)
			}
			c, e := govClient(f, demo)
			if e != nil {
				return e
			}
			ctx, cancel := boundCtx(cmd.Context(), f)
			defer cancel()
			out := map[string]any{}
			for _, src := range srcs {
				path := "/organization_list"
				if mode == "topics" {
					path = "/group_list"
				}
				if src == "federal" {
					path = "/organizations"
					if mode == "topics" {
						path = "/keywords"
					}
				}
				m, e := c.Fetch(ctx, src, path, url.Values{})
				if e != nil {
					return e
				}
				var v any = m
				if src == "california" {
					v = m["result"]
				}
				if a, ok := v.([]any); ok && len(a) > limit {
					v = a[:limit]
				}
				out[src] = map[string]any{"data": govBoundDiscovery(v, limit), "limit": limit, "note": "Bounded discovery values; not a complete publisher/topic inventory."}
			}
			return f.printJSON(cmd, out)
		}}
		cmd.Flags().StringVar(&source, "source", "california", "Catalog source: both, california, or federal")
		cmd.Flags().BoolVar(&demo, "demo", false, "Use public federal DEMO_KEY for limited exploration")
		cmd.Flags().IntVar(&limit, "limit", 25, "Maximum discovery values returned per source")
		parent.AddCommand(cmd)
	}
	return parent
}
func newGovDataset(f *rootFlags) *cobra.Command {
	p := &cobra.Command{Use: "dataset", Short: "Inspect a catalog dataset and its resource links"}
	for _, mode := range []string{"show", "resources"} {
		mode := mode
		var source string
		var demo bool
		cmd := &cobra.Command{Use: mode + " [id]", Short: "Read one dataset by source-native identifier", Example: "  government-data-pp-cli dataset " + mode + " water-plan-water-balance-data --source california --json", Annotations: govAnnotations(), RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(f) {
				return writeDryRun(cmd.OutOrStdout(), f, "catalog read")
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("provide one dataset identifier"))
			}
			if source != "california" && source != "federal" {
				return usageErr(fmt.Errorf("--source must be california or federal"))
			}
			c, e := govClient(f, demo)
			if e != nil {
				return e
			}
			ctx, cancel := boundCtx(cmd.Context(), f)
			defer cancel()
			d, e := c.Show(ctx, source, args[0])
			if e != nil {
				return e
			}
			if mode == "resources" {
				return f.printJSON(cmd, map[string]any{"source": source, "dataset_id": d.ID, "resources": d.Resources})
			}
			return f.printJSON(cmd, d)
		}}
		cmd.Flags().StringVar(&source, "source", "california", "Catalog containing this dataset identifier")
		cmd.Flags().BoolVar(&demo, "demo", false, "Use public federal DEMO_KEY for limited exploration")
		p.AddCommand(cmd)
	}
	return p
}
func newGovResource(f *rootFlags) *cobra.Command {
	p := &cobra.Command{Use: "resource", Short: "Download an explicitly selected dataset resource"}
	var u, output string
	var max int64
	cmd := &cobra.Command{Use: "download", Short: "Download one HTTPS resource to an explicit new file", Example: "  government-data-pp-cli resource download --url https://water.ca.gov/-/media/DWR-Website/Web-Pages/Programs/California-Water-Plan/Docs/Update2023/Supporting-Documents/Water-Portfolios-and-Balances.pdf --output resource-download.bin --json", Annotations: map[string]string{"pp:data-source": "live", "mcp:write-flags": "output", "pp:live-happy-path": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "catalog read")
		}
		if u == "" || output == "" || max < 1 {
			return usageErr(fmt.Errorf("provide --url, --output and positive --max-bytes"))
		}
		if _, e := os.Lstat(output); e == nil {
			return fmt.Errorf("output already exists: %s", output)
		} else if !os.IsNotExist(e) {
			return e
		}
		file, e := os.CreateTemp(filepath.Dir(output), ".government-data-*.part")
		if e != nil {
			return e
		}
		defer func() { _ = file.Close(); _ = os.Remove(file.Name()) }()
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		c, e := govClient(f, false)
		if e != nil {
			return e
		}
		n, e := c.Download(ctx, u, file, max)
		if e != nil {
			return e
		}
		if e = file.Close(); e != nil {
			return e
		}
		if e = os.Link(file.Name(), output); e != nil {
			return e
		}
		return f.printJSON(cmd, map[string]any{"url": u, "output": output, "bytes": n})
	}}
	cmd.Flags().StringVar(&u, "url", "", "Exact HTTPS download URL from dataset resource metadata")
	cmd.Flags().StringVar(&output, "output", "", "Explicit new output file; existing files are refused")
	cmd.Flags().Int64Var(&max, "max-bytes", 100<<20, "Maximum allowed download bytes before removing partial output")
	p.AddCommand(cmd)
	return p
}

// newGovSync replaces generic resource inference with explicit bounded catalog sync.
func newGovSync(f *rootFlags) *cobra.Command {
	var cmd *cobra.Command
	for _, c := range newGovCatalog(f).Commands() {
		if c.Name() == "find" {
			cmd = c
			break
		}
	}
	original := cmd.RunE
	cmd.Use = "sync [query]"
	cmd.Aliases = nil
	cmd.Short = "Save a bounded catalog search snapshot and update offline search"
	cmd.Example = "  government-data-pp-cli sync --resources california,federal --param q=water --max-pages 1 --demo"
	var resources, query, since string
	var params []string
	var full bool
	cmd.Flags().StringVar(&resources, "resources", "", "Comma-separated catalog resources: california,federal")
	cmd.Flags().StringVar(&query, "query", "", "Topic query whose metadata should be saved locally")
	cmd.Flags().StringArrayVar(&params, "param", nil, "Query parameter key=value; supported keys q, format, organization")
	cmd.Flags().StringVar(&since, "since", "", "Incremental duration is not supported for immutable query snapshots")
	cmd.Flags().BoolVar(&full, "full", false, "Capture a fresh bounded snapshot rather than reuse saved metadata")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "bounded catalog sync")
		}
		if since != "" {
			return usageErr(fmt.Errorf("--since is unavailable for bounded catalog snapshots; use two named snapshots and changes"))
		}
		if resources != "" {
			r := strings.Split(resources, ",")
			src := ""
			ca, us := false, false
			for _, v := range r {
				switch strings.TrimSpace(v) {
				case "california":
					ca = true
				case "federal":
					us = true
				default:
					return usageErr(fmt.Errorf("--resources accepts california,federal only"))
				}
			}
			if ca && us {
				src = "both"
			} else if ca {
				src = "california"
			} else {
				src = "federal"
			}
			if e := cmd.Flags().Set("source", src); e != nil {
				return e
			}
		}
		for _, v := range params {
			kv := strings.SplitN(v, "=", 2)
			if len(kv) != 2 {
				return usageErr(fmt.Errorf("--param expects key=value"))
			}
			switch kv[0] {
			case "q":
				if query != "" && query != kv[1] {
					return usageErr(fmt.Errorf("conflicting --query and --param q"))
				}
				query = kv[1]
			case "format", "organization":
				if e := cmd.Flags().Set(kv[0], kv[1]); e != nil {
					return e
				}
			default:
				return usageErr(fmt.Errorf("unsupported --param %s; use q, format, organization", kv[0]))
			}
		}
		if len(args) > 0 {
			if len(args) != 1 || query != "" {
				return usageErr(fmt.Errorf("provide query once, positional or --query/--param q"))
			}
			query = args[0]
		}
		if query == "" {
			if cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			return usageErr(fmt.Errorf("provide --query water or --param q=water"))
		}
		if v, _ := cmd.Flags().GetString("save"); v == "" {
			if e := cmd.Flags().Set("save", "sync-"+time.Now().UTC().Format("20060102T150405.000000000")); e != nil {
				return e
			}
		}
		return original(cmd, []string{query})
	}
	return cmd
}

func govBoundDiscovery(v any, limit int) any {
	switch x := v.(type) {
	case []any:
		if len(x) > limit {
			return x[:limit]
		}
		return x
	case map[string]any:
		o := map[string]any{}
		for k, z := range x {
			o[k] = govBoundDiscovery(z, limit)
		}
		return o
	default:
		return v
	}
}

func newGovCatalogFind(f *rootFlags) *cobra.Command {
	mode := "find"
	var source, format, org, save, dbPath string
	var limit, pages int
	var demo bool
	cmd := &cobra.Command{Use: "find [query]", Short: "Fetch bounded catalog metadata with per-source provenance", Example: "  government-data-pp-cli catalog " + mode + " water --source california --limit 5 --agent", Annotations: govAnnotations(), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "catalog read")
		}
		if save != "" && localDataDisabled() {
			return fmt.Errorf("--save is disabled while %s=1: it would persist catalog metadata", noLocalDataEnvVar)
		}
		if limit < 1 || limit > 1000 || pages < 1 || pages > 100 {
			return usageErr(fmt.Errorf("--limit must be 1..1000 and --max-pages must be 1..100"))
		}
		if _, err := govdata.Sources(source); err != nil {
			return usageErr(err)
		}

		if len(args) != 1 {
			return usageErr(fmt.Errorf("provide one query, for example catalog %s water", mode))
		}
		if cliutil.IsDogfoodEnv() {
			pages = 1
			if limit > 5 {
				limit = 5
			}
		}
		c, e := govClient(f, demo)
		if e != nil {
			return e
		}
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		r, e := c.Find(ctx, args[0], source, format, org, limit, pages)
		if e != nil {
			return e
		}
		if len(r.FetchFailures) > 0 {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d source requests failed; results include successful sources only\n", len(r.FetchFailures))
		}
		if save != "" {
			if dbPath == "" {
				dbPath = defaultDBPath("government-data-pp-cli")
			}
			db, e := store.OpenWithContext(ctx, dbPath)
			if e != nil {
				return e
			}
			defer db.Close()
			snap := govdata.Snapshot{ID: save, Query: args[0], Source: source, CapturedAt: time.Now().UTC().Format(time.RFC3339Nano), Datasets: r.Results, Complete: r.Complete, Failures: r.FetchFailures, Scope: map[string]any{"format": format, "organization": org, "limit_per_source": limit, "max_pages_per_source": pages, "scanned_pages": r.ScannedPages, "next": r.Next}}
			r.SnapshotID = save
			if e = govcache.Save(ctx, db.DB(), snap); e != nil {
				return e
			}
			for _, d := range r.Results {
				b, e := json.Marshal(d)
				if e != nil {
					return e
				}
				if e = saveGovDataset(db, d, b); e != nil {
					return e
				}
			}
		}
		return f.printJSON(cmd, r)
	}}
	if mode == "find" {
		cmd.Aliases = []string{"capture"}
	}
	cmd.Flags().StringVar(&source, "source", "both", "Catalog source: both, california, or federal")
	cmd.Flags().StringVar(&format, "format", "", "Filter declared resource format, such as CSV")
	cmd.Flags().StringVar(&org, "organization", "", "Source-native publishing organization slug to filter")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum returned records from each selected catalog")
	cmd.Flags().IntVar(&pages, "max-pages", 1, "Maximum scanned pages from each selected catalog")
	cmd.Flags().BoolVar(&demo, "demo", false, "Use public federal DEMO_KEY for limited exploration")
	cmd.Flags().StringVar(&save, "save", "", "Persist immutable bounded snapshot with this identifier")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite path used when saving a snapshot")
	cmd.Annotations["mcp:read-only-unless-flags"] = "save"
	return cmd
}
