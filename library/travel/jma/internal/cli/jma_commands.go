// pp:data-source live
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/jma/internal/jma"
	"github.com/spf13/cobra"
)

func init() { registerNovelCommand(attachJMA) }
func attachJMA(root *cobra.Command, flags *rootFlags) {
	var cacheDir string
	var refresh, offline, detail bool
	d, _ := os.UserCacheDir()
	root.PersistentFlags().StringVar(&cacheDir, "cache-dir", filepath.Join(d, "jma-cli"), "JMA-specific HTTP and inventory cache directory")
	root.PersistentFlags().BoolVar(&refresh, "refresh", false, "Bypass HTTP cache and fetch current source documents")
	root.PersistentFlags().BoolVar(&offline, "offline", false, "Use inventory and fresh HTTP cache only")
	root.PersistentFlags().BoolVar(&detail, "detail", false, "Include source hazard properties or untimed analysis tracks")
	for _, name := range []string{"audit-dir", "client-profile", "config", "data-source", "deliver", "human-friendly", "max-age", "profile", "rate-limit", "receipt", "receipt-file", "csv", "plain", "quiet", "yes"} {
		if f := root.PersistentFlags().Lookup(name); f != nil {
			f.Hidden = true
		}
	}
	root.PersistentFlags().Lookup("compact").Usage = "Compact JSON is the default; use --detail for source properties"
	flags.timeout = 45 * time.Second
	root.PersistentFlags().Lookup("timeout").DefValue = "45s"
	root.PersistentFlags().Lookup("timeout").Usage = "Whole command request budget, greater than zero and at most 60s"
	// Retain generated framework utilities as hidden advanced local tools. Focused
	// weather commands use a separate bounded client and never open SQLite.
	for _, c := range root.Commands() {
		switch c.Name() {
		case "areas", "forecast", "warnings", "typhoons":
			root.RemoveCommand(c)
		case "doctor":
			if registeredPlatformSource == nil {
				root.RemoveCommand(c)
			}
		case "version", "help", "completion", "agent-context":
		default:
			c.Hidden = true
			if c.Annotations == nil {
				c.Annotations = map[string]string{}
			}
			c.Annotations["mcp:hidden"] = "true"
		}
	}
	run := func(fn func(context.Context, *jma.Client) (jma.Envelope, error)) func(*cobra.Command, []string) error {
		return func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return usageErr(fmt.Errorf("unexpected arguments %v; use %s --help", args, cmd.CommandPath()))
			}
			if flags.csv || flags.plain || flags.quiet {
				return usageErr(fmt.Errorf("JMA commands emit compact JSON; use --select for field projection"))
			}
			if flags.dryRun {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{"dry_run": true, "action": cmd.CommandPath(), "meta": map[string]any{"source": "dry-run", "requests": 0}, "results": map[string]any{"command": cmd.CommandPath(), "cache_dir": cacheDir}})
			}
			if flags.timeout <= 0 || flags.timeout > 60*time.Second {
				return usageErr(fmt.Errorf("--timeout must be greater than zero and at most 60s"))
			}
			if refresh && offline {
				return usageErr(fmt.Errorf("--refresh cannot be combined with --offline"))
			}
			if cacheDir == "" && !flags.noCache {
				return usageErr(fmt.Errorf("--cache-dir must be nonempty or use --no-cache"))
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), flags.timeout)
			defer cancel()
			requestTimeout := 10 * time.Second
			if flags.timeout < requestTimeout {
				requestTimeout = flags.timeout
			}
			c := jma.New(cacheDir, flags.noCache, refresh, offline, requestTimeout)
			v, e := fn(ctx, c)
			if e != nil {
				if v.Results != nil {
					if emitErr := emitJMA(cmd, v, flags.selectFields); emitErr != nil {
						return emitErr
					}
				}
				if je, ok := e.(*jma.Error); ok {
					return &cliError{code: je.Code, err: je}
				}
				return e
			}
			return emitJMA(cmd, v, flags.selectFields)
		}
	}
	command := func(use, short, example string, fn func(context.Context, *jma.Client) (jma.Envelope, error)) *cobra.Command {
		return &cobra.Command{Use: use, Short: short, Example: strings.Trim(example, "\n"), Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"}, RunE: run(fn)}
	}
	areas := &cobra.Command{Use: "areas", Short: "Search or resolve JMA source areas"}
	areas.AddCommand(newJMAAreaSearchCmd(run), newJMAAreaResolveCmd(run))
	root.AddCommand(areas)

	var sq string
	var so, sl int
	stations := &cobra.Command{Use: "stations", Short: "Discover forecast temperature reference stations"}
	ss := command("search", "Search forecast temperature reference stations by Japanese/English name or ID", "  jma-pp-cli stations search --query Tokyo", func(ctx context.Context, c *jma.Client) (jma.Envelope, error) {
		return c.StationSearch(ctx, sq, so, sl)
	})
	ss.Flags().StringVar(&sq, "query", "", "Japanese/English station name or source ID substring")
	ss.Flags().IntVar(&so, "offset", 0, "Local result offset, between zero and 100000")
	ss.Flags().IntVar(&sl, "limit", 20, "Maximum returned station records, between 1 and 100")
	stations.AddCommand(ss)
	root.AddCommand(stations)
	forecast := &cobra.Command{Use: "forecast", Short: "Official daily and weekly district/station forecasts"}
	forecast.AddCommand(newJMAForecastGetCmd(run))
	root.AddCommand(forecast)
	warnings := &cobra.Command{Use: "warnings", Short: "Current 2026 municipality weather warnings/advisories"}
	warnings.AddCommand(newJMAWarningsGetCmd(run, &detail))
	root.AddCommand(warnings)

	var to, tl int
	var id string
	var hours int
	typhoons := &cobra.Command{Use: "typhoons", Short: "Discover active cyclones before coherent detail retrieval"}
	tcList := command("list", "List current cyclone IDs and issue times with one source request", "  jma-pp-cli typhoons list --limit 10", func(ctx context.Context, c *jma.Client) (jma.Envelope, error) { return c.TyphoonList(ctx, to, tl) })
	tcList.Flags().IntVar(&to, "offset", 0, "Local cyclone index offset, between zero and 100000")
	tcList.Flags().IntVar(&tl, "limit", 20, "Maximum cyclone index entries, between 1 and 100")
	tcList.Annotations["pp:endpoint"] = "typhoons.list"
	tcGet := command("get", "Join coherent analysis, estimate and uncertain forecast detail", "  jma-pp-cli typhoons get --id TC2632 --hours 120", func(ctx context.Context, c *jma.Client) (jma.Envelope, error) {
		return c.Typhoon(ctx, id, detail, hours)
	})
	tcGet.Flags().StringVar(&id, "id", "", "Current JMA tropical cyclone ID from typhoons list")
	tcGet.Flags().IntVar(&hours, "hours", 120, "Maximum source forecast lead hours, 0..120")
	tcGet.Annotations["pp:endpoint"] = "typhoons.get"
	tcGet.Annotations["pp:happy-args"] = "--id=TC2632"
	typhoons.AddCommand(tcList, tcGet)
	root.AddCommand(typhoons)
	inventory := &cobra.Command{Use: "inventory", Short: "Explicitly refresh source area/station catalogs"}
	inventory.AddCommand(newJMAInventoryRefreshCmd(run))
	root.AddCommand(inventory)

	doctor := command("doctor", "Check JMA public index and resolved local inventory", "  jma-pp-cli doctor --dry-run", func(ctx context.Context, c *jma.Client) (jma.Envelope, error) {
		if _, e := c.Inventory(ctx, false); e != nil {
			return jma.Envelope{}, e
		}
		v, e := c.TyphoonList(ctx, 0, 1)
		if e != nil {
			return v, e
		}
		v.Results = map[string]any{"status": "ok", "authentication": "none", "read_only": true, "per_request_byte_limit": jma.MaxBody, "http_cache": "forecast 5m; warning/typhoon 60s; no stale fallback", "inventory_refresh": "explicit", "requests_sequential": true}
		return v, nil
	})
	if registeredPlatformSource == nil {
		root.AddCommand(doctor)
	}
}

// Projection retains provenance and pagination; requested result paths are strict.
func emitJMA(cmd *cobra.Command, v jma.Envelope, fields string) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	var obj map[string]any
	if e = json.Unmarshal(b, &obj); e != nil {
		return e
	}
	if fields != "" {
		result := map[string]any{"meta": obj["meta"]}
		if p, ok := obj["page"]; ok {
			result["page"] = p
		}
		tree := map[string]any{}
		for _, path := range strings.Split(fields, ",") {
			path = strings.TrimSpace(path)
			if path == "" {
				return usageErr(fmt.Errorf("--select contains an empty field"))
			}
			if path != "results" && !strings.HasPrefix(path, "results.") && !strings.HasPrefix(path, "meta.") {
				path = "results." + path
			}
			parts := strings.Split(path, ".")
			if !hasPath(obj, parts) && !emptyJMAPath(cmd, obj, parts) {
				return usageErr(fmt.Errorf("--select path %q matched no field", path))
			}
			addProjection(tree, parts)
		}
		for k, x := range project(obj, tree).(map[string]any) {
			if k != "meta" {
				result[k] = x
			}
		}
		obj = result
	}
	return json.NewEncoder(cmd.OutOrStdout()).Encode(obj)
}
func addProjection(t map[string]any, path []string) {
	if len(path) == 1 {
		t[path[0]] = nil
		return
	}
	if x, ok := t[path[0]]; ok && x == nil {
		return
	}
	m, ok := t[path[0]].(map[string]any)
	if !ok {
		m = map[string]any{}
		t[path[0]] = m
	}
	addProjection(m, path[1:])
}
func hasPath(v any, path []string) bool {
	if len(path) == 0 {
		return true
	}
	switch x := v.(type) {
	case map[string]any:
		a, ok := x[path[0]]
		return ok && hasPath(a, path[1:])
	case []any:
		for _, a := range x {
			if hasPath(a, path) {
				return true
			}
		}
	}
	return false
}
func project(v any, t map[string]any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, sub := range t {
			if a, ok := x[k]; ok {
				if sub == nil {
					out[k] = a
				} else {
					out[k] = project(a, sub.(map[string]any))
				}
			}
		}
		return out
	case []any:
		out := []any{}
		for _, a := range x {
			out = append(out, project(a, t))
		}
		return out
	}
	return nil
}

// Empty lists still have a declared item shape; validate known list fields
// without fabricating items or accepting typos against an empty live index.
func emptyJMAPath(cmd *cobra.Command, obj any, path []string) bool {
	full := strings.Join(path, ".")
	var walk func(any, []string) bool
	walk = func(v any, parts []string) bool {
		if len(parts) == 0 {
			return false
		}
		switch x := v.(type) {
		case map[string]any:
			a, ok := x[parts[0]]
			return ok && walk(a, parts[1:])
		case []any:
			if len(x) == 0 {
				return knownJMAListPath(cmd, full)
			}
			for _, a := range x {
				if walk(a, parts) {
					return true
				}
			}
		}
		return false
	}
	return walk(obj, path)
}
func knownJMAListPath(cmd *cobra.Command, path string) bool {
	parent := ""
	if cmd.Parent() != nil {
		parent = cmd.Parent().Name()
	}
	key := parent + " " + cmd.Name()
	fields := map[string]string{
		"areas search":    "id,name_ja,name_en,kind,parent_id,office_id,forecast_district_ids,url",
		"stations search": "id,name_ja,name_en,role,district_ids,office_ids,url",
		"typhoons list":   "id,number,category,issued_at,issue_age_hours,url",
	}
	if list, ok := fields[key]; ok {
		for _, f := range strings.Split(list, ",") {
			if path == "results."+f {
				return true
			}
		}
	}
	if key == "warnings get" {
		prefix := "results.municipalities."
		if strings.HasPrefix(path, prefix) {
			tail := strings.TrimPrefix(path, prefix)
			if flag := cmd.Flag("detail"); flag != nil && flag.Value.String() == "true" && (tail == "events.source_properties" || tail == "events.additions_ja") {
				return true
			}
			for _, f := range strings.Split("area_id,name_ja,name_en,state,has_active_hazards,complete_municipality_products,no_warning_product_ids,missing_product_ids,not_applicable_product_ids,events,url", ",") {
				if tail == f {
					return true
				}
			}
			for _, f := range strings.Split("hazard_id,status_ja,lifecycle,in_effect,product_id,issued_at,valid_until,hazard,hazard.name_ja,hazard.name_en,hazard.severity,hazard.alert_level", ",") {
				if tail == "events."+f {
					return true
				}
			}
		}
	}
	return false
}
