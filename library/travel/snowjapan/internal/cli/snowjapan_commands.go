// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source auto
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/snowjapan/internal/snowjapan"
	"github.com/mvanhorn/printing-press-library/library/travel/snowjapan/internal/store"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		if root.PersistentFlags().Lookup("db") == nil {
			root.PersistentFlags().String("db", "", "Local factual SQLite mirror path; default uses the CLI data directory")
		}
		if f := root.PersistentFlags().Lookup("rate-limit"); f != nil {
			f.Usage = "Maximum requests per second; factual source commands retain a 2 req/s source cap"
		}
		for _, command := range root.Commands() {
			switch command.Name() {
			case "resorts", "reports":
				for _, leaf := range command.Commands() {
					command.RemoveCommand(leaf)
				}
				if command.Name() == "resorts" {
					command.AddCommand(newSnowResortsList(flags, false), newSnowResortsSearchCmd(flags), newSnowGet(flags, "resorts"), newSnowCompare(flags))
				} else {
					command.AddCommand(newSnowReportsList(flags), newSnowGet(flags, "reports"))
				}
			case "seasons":
				root.RemoveCommand(command)
			}
		}
		seasonList := newSnowSeasonsList(flags)
		seasonParent := newSnowSeasonsList(flags)
		seasonParent.Use = "seasons"
		seasonParent.Annotations["pp:parent-group"] = "true"
		seasonParent.AddCommand(seasonList)
		root.AddCommand(seasonParent, newSnowSync(flags), newSnowSearch(flags))
	})
}

func snowDBPath(cmd *cobra.Command) string {
	path, _ := cmd.Root().PersistentFlags().GetString("db")
	if path == "" {
		path = defaultDBPath("snowjapan")
	}
	return path
}

// snowHappyArgs lets the live matrix use an explicitly prepared throwaway
// database through the existing public flag. Production examples stay portable.
func snowHappyArgs(base string) string {
	path := os.Getenv("PP_SNOWJAPAN_FIXTURE_DB")
	if filepath.IsAbs(path) && !strings.ContainsAny(path, ";\r\n\x00") {
		return base + ";--db=" + path
	}
	return base
}

func snowPrint(cmd *cobra.Command, flags *rootFlags, value any, origin string) error {
	if view, ok := value.(map[string]any); ok {
		if results, present := view["results"]; present {
			meta := map[string]any{"source": origin}
			for k, v := range view {
				if k != "results" {
					meta[k] = v
				}
			}
			value = map[string]any{"meta": meta, "results": results}
		}
	}
	b, e := json.Marshal(value)
	if e != nil {
		return e
	}
	// Every retained field is a bounded factual projection; keep the complete
	// domain view under --agent, while --select still has precedence.
	known := map[string]bool{}
	var collect func(any)
	collect = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, a := range x {
				known[k] = true
				collect(a)
			}
		case snowjapan.Fact:
			for k, a := range x {
				known[k] = true
				collect(a)
			}
		case []snowjapan.Fact:
			for _, a := range x {
				collect(a)
			}
		case []any:
			for _, a := range x {
				collect(a)
			}
		}
	}
	collect(value)
	keep := make([]string, 0, len(known))
	for k := range known {
		keep = append(keep, k)
	}
	f := *flags
	var selectErr error
	if flags.selectFields != "" {
		b, selectErr = filterFieldsChecked(b, flags.selectFields)
		f.selectFields = ""
		if flags.agent {
			var selected map[string]any
			if json.Unmarshal(b, &selected) == nil {
				if _, ok := selected["results"]; ok {
					meta, _ := selected["meta"].(map[string]any)
					if meta == nil {
						meta = map[string]any{}
					}
					meta["source"] = origin
					selected["meta"] = meta
					b, _ = json.Marshal(selected)
				}
			}
		}
	}
	if err := printOutputWithFlagsMetaAndKeep(cmd.OutOrStdout(), b, &f, map[string]any{"source": origin}, keep, known); err != nil {
		return err
	}
	return selectErr
}

func hintIfUnsynced(cmd *cobra.Command, db *store.Store, resource string) bool {
	_, at, _, e := db.GetSyncState(resource)
	if e != nil || at.IsZero() {
		fmt.Fprintf(cmd.ErrOrStderr(), "hint: no completed local %s sync; run snowjapan-pp-cli sync --resources %s with required source parameters\n", resource, resource)
		return true
	}
	return false
}
func hintIfStale(cmd *cobra.Command, db *store.Store, resource string, maxAge time.Duration) {
	if maxAge <= 0 {
		return
	}
	_, at, _, e := db.GetSyncState(resource)
	if e == nil && !at.IsZero() && time.Since(at) > maxAge {
		fmt.Fprintf(cmd.ErrOrStderr(), "hint: local %s facts were last synced at %s; explicitly sync to refresh\n", resource, at.UTC().Format(time.RFC3339))
	}
}

func snowLocal(ctx context.Context, cmd *cobra.Command, flags *rootFlags, resource, season string, catalog bool, detailID ...string) (result []snowjapan.Fact, captured bool, err error) {
	path := snowDBPath(cmd)
	if _, e := os.Stat(path); os.IsNotExist(e) {
		fmt.Fprintf(cmd.ErrOrStderr(), "hint: no local mirror; run snowjapan-pp-cli sync --resources resorts, then sync seasons with --resource-param seasons:season=2025-2026\n")
		return []snowjapan.Fact{}, false, nil
	}
	db, e := store.OpenSnowJapanReadOnlyContext(ctx, path)
	if e != nil {
		return nil, false, e
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			result, captured = nil, false
			err = errors.Join(err, closeErr)
		}
	}()
	var raw []json.RawMessage
	complete := false
	if resource == "seasons" && season != "" {
		var observed string
		raw, complete, observed, e = db.SnowJapanSeason(ctx, season)
		if e != nil {
			return nil, false, e
		}
		if !complete {
			fmt.Fprintf(cmd.ErrOrStderr(), "hint: requested season %s has no complete local capture; run snowjapan-pp-cli sync --resources seasons --resource-param seasons:season=%s\n", season, season)
			return []snowjapan.Fact{}, false, nil
		}
		at, parseErr := time.Parse(time.RFC3339, observed)
		if parseErr == nil && flags.maxAge > 0 && time.Since(at) > flags.maxAge {
			fmt.Fprintf(cmd.ErrOrStderr(), "hint: local %s season facts were captured at %s; explicitly sync that season to refresh\n", season, observed)
		}
	}
	if catalog {
		raw, complete, e = db.SnowJapanCatalog(ctx)
		if e != nil {
			return nil, false, e
		}
	}
	if !complete {
		query := `SELECT data FROM resources WHERE resource_type=?`
		params := []any{resource}
		if season != "" {
			query += ` AND json_extract(data,'$.season')=?`
			params = append(params, season)
		}
		query += ` ORDER BY id LIMIT 1501`
		rows, e := db.DB().QueryContext(ctx, query, params...)
		if e != nil {
			return nil, false, e
		}
		for rows.Next() {
			var s string
			if e = rows.Scan(&s); e != nil {
				return nil, false, errors.Join(e, rows.Close())
			}
			raw = append(raw, json.RawMessage(s))
		}
		e = errors.Join(rows.Err(), rows.Close())
		if e != nil {
			return nil, false, e
		}
		if len(raw) > snowjapan.MaxRecords {
			return nil, false, fmt.Errorf("local factual scan exceeds %d rows", snowjapan.MaxRecords)
		}
	}
	out := make([]snowjapan.Fact, 0, len(raw))
	for _, b := range raw {
		var f snowjapan.Fact
		if e = json.Unmarshal(b, &f); e != nil {
			return nil, false, fmt.Errorf("saved %s fact is invalid: %w", resource, e)
		}
		out = append(out, f)
	}
	if len(detailID) > 0 {
		// A detail read reports the age of its selected observation. Other
		// saved records still determine freshness for collection reads.
		found, e := snowSavedDetail(out, detailID[0], resource)
		if e != nil {
			return nil, false, e
		}
		out = []snowjapan.Fact{found}
	}
	if catalog && complete {
		firstObserved, _ := snowObservationRange(out)
		at, parseErr := time.Parse(time.RFC3339, firstObserved)
		if parseErr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: saved directory observation time is invalid; explicitly sync resorts to refresh\n")
		} else if flags.maxAge > 0 && time.Since(at) > flags.maxAge {
			fmt.Fprintf(cmd.ErrOrStderr(), "hint: complete local resort directory was captured at %s; explicitly sync --resources resorts to refresh\n", firstObserved)
		}
	} else if season == "" && !hintIfUnsynced(cmd, db.Store, resource) {
		if len(out) > 0 {
			// An exact capture refreshes only its own rows. The resource
			// sync timestamp cannot establish the age of other saved
			// observations or older list metadata.
			firstObserved, _ := snowObservationRange(out)
			at, parseErr := time.Parse(time.RFC3339, firstObserved)
			if parseErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: saved %s observation time is invalid; explicitly sync to refresh\n", resource)
			} else if flags.maxAge > 0 && time.Since(at) > flags.maxAge {
				fmt.Fprintf(cmd.ErrOrStderr(), "hint: local %s include observations from %s; explicitly capture the older records to refresh\n", resource, firstObserved)
			}
		} else {
			hintIfStale(cmd, db.Store, resource, flags.maxAge)
		}
	}
	return out, complete, nil
}

func snowFetch(ctx context.Context, cmd *cobra.Command, flags *rootFlags, resource, season string, fetch func(*snowjapan.Client) ([]snowjapan.Fact, error)) ([]snowjapan.Fact, string, error) {
	if flags.dataSource == "local" {
		v, _, e := snowLocal(ctx, cmd, flags, resource, season, resource == "resorts")
		return v, "local", e
	}
	v, e := fetch(snowjapan.NewWithRateLimit(flags.rateLimit))
	if e != nil {
		if flags.dataSource == "auto" && isNetworkError(e) {
			cached, _, localErr := snowLocal(ctx, cmd, flags, resource, season, resource == "resorts")
			if localErr == nil && len(cached) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: source network unavailable; returning explicitly dated local facts\n")
				return cached, "local", nil
			}
		}
		return nil, "live", e
	}
	return v, "live", nil
}

func snowName(f snowjapan.Fact, k string) string { s, _ := f[k].(string); return s }
func snowPrefecture(s string) string {
	return strings.TrimSpace(strings.TrimSuffix(strings.ToLower(s), " prefecture"))
}
func snowMatches(f snowjapan.Fact, q, pref string) bool {
	if pref != "" && snowPrefecture(snowName(f, "prefecture")) != snowPrefecture(pref) {
		return false
	}
	if q == "" {
		return true
	}
	blob := strings.ToLower(snowName(f, "name") + " " + snowName(f, "name_japanese") + " " + snowName(f, "town") + " " + snowName(f, "popular_region"))
	for _, part := range strings.Fields(strings.ToLower(q)) {
		if !strings.Contains(blob, part) {
			return false
		}
	}
	return true
}
func snowResolve(rows []snowjapan.Fact, id string) (snowjapan.Fact, error) {
	matches := make([]snowjapan.Fact, 0, 1)
	for _, r := range rows {
		if snowName(r, "id") == id || strings.HasSuffix(snowName(r, "id"), "/"+id) {
			matches = append(matches, r)
		}
	}
	if len(matches) != 1 {
		return nil, fmt.Errorf("resort %q has %d exact ID/slug matches; use a canonical id from resorts search", id, len(matches))
	}
	return matches[0], nil
}
func snowLimit(n int) error {
	if n < 1 || n > 200 {
		return usageErr(fmt.Errorf("--limit must be between 1 and 200"))
	}
	return nil
}

func newSnowResortsList(flags *rootFlags, search bool) *cobra.Command {
	cmd := &cobra.Command{Use: "list"}
	configureSnowResortsList(cmd, flags, search)
	return cmd
}

func configureSnowResortsList(cmd *cobra.Command, flags *rootFlags, search bool) {
	var q, pref string
	limit, maxScan := 20, 1500
	var minVertical, minLifts float64
	name := "list"
	if search {
		name = "search"
	}
	cmd.Use = name
	cmd.Short = "Search a bounded factual directory; installed lifts do not indicate today's operations."
	cmd.Example = "  snowjapan-pp-cli resorts " + name + " --prefecture Nagano --limit 5 --agent"
	cmd.Annotations = map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "--prefecture=Nagano;--limit=5"}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "resorts "+name)
		}
		if len(args) > 0 {
			return usageErr(fmt.Errorf("use --query for text search"))
		}
		if e := snowLimit(limit); e != nil {
			return e
		}
		if maxScan < 1 || maxScan > 1500 {
			return usageErr(fmt.Errorf("--max-scan-records must be 1..1500"))
		}
		if minVertical < 0 || minLifts < 0 || math.IsNaN(minVertical) || math.IsInf(minVertical, 0) || math.IsNaN(minLifts) || math.IsInf(minLifts, 0) {
			return usageErr(fmt.Errorf("numeric minimum filters must be nonnegative"))
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		rows, origin, e := snowFetch(ctx, cmd, flags, "resorts", "", func(c *snowjapan.Client) ([]snowjapan.Fact, error) {
			return c.Catalog(ctx)
		})
		if e != nil {
			return classifyAPIError(cmd.OutOrStdout(), e, flags)
		}
		sort.Slice(rows, func(i, j int) bool { return snowName(rows[i], "id") < snowName(rows[j], "id") })
		out := make([]snowjapan.Fact, 0)
		scanned := 0
		for _, f := range rows {
			if scanned == maxScan {
				break
			}
			scanned++
			if !snowMatches(f, q, pref) {
				continue
			}
			if minVertical > 0 {
				v, ok := snowNumber(f["vertical_m"])
				if !ok || v < minVertical {
					continue
				}
			}
			if minLifts > 0 {
				v, ok := snowNumber(f["installed_lifts"])
				if !ok || v < minLifts {
					continue
				}
			}
			out = append(out, f)
		}
		matched := len(out)
		if len(out) > limit {
			out = out[:limit]
		}
		view := map[string]any{"results": out, "matched_records": matched, "scanned_records": scanned, "source_records": len(rows), "scan_cap_hit": scanned < len(rows)}
		if len(out) == 0 {
			view["note"] = "No match in the scanned factual records; a capped scan can be widened with --max-scan-records."
		}
		return snowPrint(cmd, flags, view, origin)
	}
	cmd.Flags().StringVar(&q, "query", "", "Match words in source name, Japanese name when available, town or recorded region")
	cmd.Flags().StringVar(&pref, "prefecture", "", "Exact prefecture name; the optional Prefecture suffix is accepted")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum matching factual records to return, 1..200")
	cmd.Flags().IntVar(&maxScan, "max-scan-records", 1500, "Maximum source records to examine independently of output limit, 1..1500")
	cmd.Flags().Float64Var(&minVertical, "min-vertical", 0, "Minimum published vertical drop in metres; missing fields do not match")
	cmd.Flags().Float64Var(&minLifts, "min-lifts", 0, "Minimum installed lift count; this does not filter current operating lifts")
}

// snowSavedDetail rejects a list projection instead of presenting it as an
// inspected record. Automatic network fallback uses the same detail contract.
func snowSavedDetail(rows []snowjapan.Fact, id, resource string) (snowjapan.Fact, error) {
	var found snowjapan.Fact
	var err error
	if resource == "resorts" {
		found, err = snowResolve(rows, id)
	} else {
		for _, row := range rows {
			if snowName(row, "id") == id {
				found = row
				break
			}
		}
		if found == nil {
			err = fmt.Errorf("report is absent from the local mirror")
		}
	}
	if err != nil {
		return nil, err
	}
	expected, selector := "detail-v1", "--resorts"
	if resource == "reports" {
		expected, selector = "report-observations-v1", "--reports"
	}
	if snowName(found, "projection") != expected {
		return nil, fmt.Errorf("detail_not_captured: saved %s projection is %q; capture it with sync --resources %s %s %s", resource, snowName(found, "projection"), resource, selector, snowName(found, "id"))
	}
	return found, nil
}

func newSnowGet(flags *rootFlags, resource string) *cobra.Command {
	sample := "nagano-prefecture/hakuba-village/able-hakuba-goryu"
	if resource == "reports" {
		sample = "hakuba-now-1st-october-2026"
	}
	cmd := &cobra.Command{Use: "get [id]", Short: "Inspect an exact source identity and its factual fields.", Example: "  snowjapan-pp-cli " + resource + " get " + sample + " --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "id=" + sample},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, resource+" get")
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("get requires exactly one canonical source id"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if flags.dataSource == "local" {
				rows, _, e := snowLocal(ctx, cmd, flags, resource, "", false, args[0])
				if e != nil {
					return e
				}
				found, e := snowSavedDetail(rows, args[0], resource)
				if e != nil {
					return e
				}
				return snowPrint(cmd, flags, found, "local")
			}
			c := snowjapan.NewWithRateLimit(flags.rateLimit)
			var fact snowjapan.Fact
			var e error
			if resource == "resorts" {
				fact, e = c.Inspect(ctx, args[0])
			} else {
				fact, e = c.Report(ctx, args[0])
			}
			if e != nil {
				if flags.dataSource == "auto" && isNetworkError(e) {
					rows, _, localErr := snowLocal(ctx, cmd, flags, resource, "", false, args[0])
					var cached snowjapan.Fact
					if localErr == nil {
						cached, localErr = snowSavedDetail(rows, args[0], resource)
					}
					if localErr == nil && cached != nil {
						fmt.Fprintln(cmd.ErrOrStderr(), "warning: source network unavailable; returning explicitly dated local facts")
						return snowPrint(cmd, flags, cached, "local")
					}
				}
				return classifyAPIError(cmd.OutOrStdout(), e, flags)
			}
			return snowPrint(cmd, flags, fact, "live")
		}}
	return cmd
}

func newSnowCompare(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "compare <first> <second> [third] [fourth]", Short: "Compare two to four exact resorts by terrain, installed lifts and source update status.", Example: "  snowjapan-pp-cli resorts compare able-hakuba-goryu hakuba-happo-one --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "first=able-hakuba-goryu;second=hakuba-happo-one"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "resorts compare")
			}
			if len(args) < 2 || len(args) > 4 {
				return usageErr(fmt.Errorf("compare requires two to four canonical ids or unique exact slugs"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := snowjapan.NewWithRateLimit(flags.rateLimit)
			rows, origin, e := snowFetch(ctx, cmd, flags, "resorts", "", func(_ *snowjapan.Client) ([]snowjapan.Fact, error) { return c.Catalog(ctx) })
			if e != nil {
				return classifyAPIError(cmd.OutOrStdout(), e, flags)
			}
			out := make([]snowjapan.Fact, 0, len(args))
			seen := map[string]bool{}
			for _, id := range args {
				r, e := snowResolve(rows, id)
				if e != nil {
					return usageErr(e)
				}
				key := snowName(r, "id")
				if seen[key] {
					return usageErr(fmt.Errorf("duplicate resort %q", id))
				}
				seen[key] = true
				if origin == "live" {
					r, e = c.Inspect(ctx, key)
					if e != nil {
						return classifyAPIError(cmd.OutOrStdout(), e, flags)
					}
				}
				out = append(out, r)
			}
			return snowPrint(cmd, flags, map[string]any{"results": out, "compared": len(out), "lift_operation_status": "unknown", "basis": "published source facts; no suitability or daily operation guarantee"}, origin)
		}}
	return cmd
}

func newSnowSeasonsList(flags *rootFlags) *cobra.Command {
	var season, pref string
	limit := 20
	cmd := &cobra.Command{Use: "list", Short: "Inspect recorded first/last endpoints for a completed season; span days are calendar days.", Example: "  snowjapan-pp-cli seasons list --season 2025-2026 --limit 5 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "--season=2025-2026;--limit=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "seasons list")
			}
			if len(args) != 0 {
				return usageErr(fmt.Errorf("use --season YYYY-YYYY"))
			}
			if e := snowjapan.ValidateSeason(season); e != nil {
				return usageErr(e)
			}
			if e := snowLimit(limit); e != nil {
				return e
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			rows, origin, e := snowFetch(ctx, cmd, flags, "seasons", season, func(c *snowjapan.Client) ([]snowjapan.Fact, error) { return c.Seasons(ctx, season) })
			if e != nil {
				return classifyAPIError(cmd.OutOrStdout(), e, flags)
			}
			out := make([]snowjapan.Fact, 0)
			inconsistent := 0
			for _, r := range rows {
				if state := snowName(r, "endpoint_evidence_state"); state != "" && state != "recorded_span" {
					inconsistent++
				}
				if snowMatches(r, "", pref) {
					out = append(out, r)
				}
			}
			total := len(out)
			if len(out) > limit {
				out = out[:limit]
			}
			return snowPrint(cmd, flags, map[string]any{"results": out, "season": season, "matched_records": total, "source_records": len(rows), "inconsistent_source_rows": inconsistent, "requested_season_captured": len(rows) > 0, "continuous_operation": "unknown", "missing_endpoint_rows": "not a closure indicator"}, origin)
		}}
	cmd.Flags().StringVar(&season, "season", "", "Completed published winter YYYY-YYYY; upcoming seasons are unsupported")
	cmd.Flags().StringVar(&pref, "prefecture", "", "Exact prefecture name; the optional Prefecture suffix is accepted")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum confirmed endpoint records to return, 1..200")
	return cmd
}

func newSnowReportsList(flags *rootFlags) *cobra.Command {
	var region string
	limit := 20
	cmd := &cobra.Command{Use: "list", Short: "List dated regional report metadata; coverage is named reporter regions.", Example: "  snowjapan-pp-cli reports list --region hakuba --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "--region=hakuba"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "reports list")
			}
			if len(args) > 0 {
				return usageErr(fmt.Errorf("reports list uses --region"))
			}
			if e := snowLimit(limit); e != nil {
				return e
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			rows, origin, e := snowFetch(ctx, cmd, flags, "reports", "", func(c *snowjapan.Client) ([]snowjapan.Fact, error) { return c.Reports(ctx) })
			if e != nil {
				return classifyAPIError(cmd.OutOrStdout(), e, flags)
			}
			out := make([]snowjapan.Fact, 0)
			for _, r := range rows {
				if region == "" || strings.EqualFold(snowName(r, "region"), region) {
					out = append(out, r)
				}
			}
			total := len(out)
			if len(out) > limit {
				out = out[:limit]
			}
			return snowPrint(cmd, flags, map[string]any{"results": out, "matched_records": total, "source_records": len(rows), "lift_operation_status": "unknown"}, origin)
		}}
	cmd.Flags().StringVar(&region, "region", "", "Exact source region slug, such as hakuba, niseko, geto-kogen or yuzawa")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum regional report metadata rows to return, 1..200")
	return cmd
}

func newSnowSync(flags *rootFlags) *cobra.Command {
	var resourceCSV, resortCSV, reportCSV, param string
	var resourceParams []string
	cmd := &cobra.Command{Use: "sync", Short: "Explicitly save bounded source facts and resort observation history to local SQLite.", Example: "  snowjapan-pp-cli sync --resources resorts\n  snowjapan-pp-cli sync --resources seasons --resource-param seasons:season=2025-2026\n  snowjapan-pp-cli sync --resources resorts --resorts nagano-prefecture/hakuba-village/able-hakuba-goryu\n  snowjapan-pp-cli sync --resources reports --reports hakuba-now-1st-october-2026", Annotations: map[string]string{"mcp:local-write": "true", "pp:data-source": "live", "pp:happy-args": snowHappyArgs("--resources=resorts,seasons;--resource-param=seasons:season=2025-2026"), "pp:live-happy-path": "true"},
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "sync factual mirror")
			}
			if len(args) > 0 {
				return usageErr(fmt.Errorf("sync accepts --resources and optional source parameters"))
			}
			if flags.dataSource == "local" {
				return usageErr(fmt.Errorf("sync needs live public source reads"))
			}
			selected := strings.Split(resourceCSV, ",")
			if len(selected) > 3 {
				return usageErr(fmt.Errorf("--resources supports resorts,seasons,reports once each"))
			}
			season := ""
			for _, s := range append(resourceParams, param) {
				if s == "" {
					continue
				}
				s = strings.TrimPrefix(s, "seasons:")
				if strings.HasPrefix(s, "season=") {
					season = strings.TrimPrefix(s, "season=")
				} else {
					return usageErr(fmt.Errorf("only --resource-param seasons:season=YYYY-YYYY or --param season=YYYY-YYYY is supported"))
				}
			}
			seen := map[string]bool{}
			for _, r := range selected {
				if (r != "resorts" && r != "seasons" && r != "reports") || seen[r] {
					return usageErr(fmt.Errorf("--resources must contain unique resorts,seasons,reports values"))
				}
				seen[r] = true
			}
			if seen["seasons"] {
				if e := snowjapan.ValidateSeason(season); e != nil {
					return usageErr(e)
				}
			}
			if resortCSV != "" && (!seen["resorts"] || len(selected) != 1) {
				return usageErr(fmt.Errorf("--resorts detail capture requires --resources resorts only"))
			}
			if reportCSV != "" {
				if !seen["reports"] || len(selected) != 1 {
					return usageErr(fmt.Errorf("--reports detail capture requires --resources reports only"))
				}
				ids := strings.Split(reportCSV, ",")
				if len(ids) > 4 {
					return usageErr(fmt.Errorf("--reports capture accepts at most four exact dated report ids"))
				}
				unique := map[string]bool{}
				for _, id := range ids {
					if id == "" || unique[id] {
						return usageErr(fmt.Errorf("--reports requires distinct nonempty dated report ids"))
					}
					unique[id] = true
				}
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := snowjapan.NewWithRateLimit(flags.rateLimit)
			db, e := store.OpenSnowJapanWritableContext(ctx, snowDBPath(cmd))
			if e != nil {
				return e
			}
			defer func() { err = errors.Join(err, db.Close()) }()
			summary := map[string]any{}
			for _, resource := range selected {
				var facts []snowjapan.Fact
				switch resource {
				case "resorts":
					if resortCSV == "" {
						facts, e = c.Catalog(ctx)
					} else {
						ids := strings.Split(resortCSV, ",")
						if len(ids) > 4 {
							return usageErr(fmt.Errorf("--resorts capture accepts at most four canonical ids"))
						}
						for _, id := range ids {
							f, er := c.Inspect(ctx, id)
							if er != nil {
								e = er
								break
							}
							facts = append(facts, f)
						}
					}
				case "seasons":
					facts, e = c.Seasons(ctx, season)
				case "reports":
					if reportCSV == "" {
						facts, e = c.Reports(ctx)
					} else {
						for _, id := range strings.Split(reportCSV, ",") {
							f, er := c.Report(ctx, id)
							if er != nil {
								e = er
								break
							}
							facts = append(facts, f)
						}
					}
				}
				if e != nil {
					return classifyAPIError(cmd.OutOrStdout(), e, flags)
				}
				raw := make([]json.RawMessage, 0, len(facts))
				for _, f := range facts {
					b, er := json.Marshal(f)
					if er != nil {
						return er
					}
					raw = append(raw, b)
				}
				stored, failed, e := db.UpsertBatch(resource, raw)
				if e != nil {
					return e
				}
				if failed != 0 {
					return fmt.Errorf("sync failed to retain %d source identities", failed)
				}
				if resource == "resorts" {
					if e = db.CaptureSnowJapan(ctx, raw, resortCSV == ""); e != nil {
						return fmt.Errorf("saving source observation history: %w", e)
					}
				}
				if resource == "seasons" {
					if e = db.CaptureSnowJapanSeason(ctx, season, raw); e != nil {
						return fmt.Errorf("saving complete season capture: %w", e)
					}
				}
				if e = db.SaveSyncState(resource, "", stored); e != nil {
					return e
				}
				summary[resource] = map[string]any{"stored": stored, "complete_directory": resource == "resorts" && resortCSV == ""}
			}
			if e = db.Close(); e != nil {
				return e
			}
			return snowPrint(cmd, flags, map[string]any{"synced": summary, "season": season, "source": "public factual projections only"}, "live")
		}}
	cmd.Flags().StringVar(&resourceCSV, "resources", "resorts", "Comma-separated factual resources to save: resorts,seasons,reports")
	cmd.Flags().StringVar(&resortCSV, "resorts", "", "At most four exact canonical resort ids for detailed snapshot capture")
	cmd.Flags().StringVar(&reportCSV, "reports", "", "At most four exact dated report ids to capture base/town snow observations")
	cmd.Flags().StringVar(&param, "param", "", "Source parameter season=YYYY-YYYY for historical seasonal sync")
	cmd.Flags().StringArrayVar(&resourceParams, "resource-param", nil, "Resource source parameter; supported form seasons:season=YYYY-YYYY")
	return cmd
}

func newSnowSearch(flags *rootFlags) *cobra.Command {
	var typ string
	limit := 20
	// Any free-text term is valid; zero matching local records is a normal
	// success, so an arbitrary nonsense term is not an error-path fixture.
	cmd := &cobra.Command{Use: "search <term>", Short: "Search concise saved factual projections in the local mirror.", Example: "  snowjapan-pp-cli search Hakuba --type resorts --limit 5 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": snowHappyArgs("term=Hakuba;--type=resorts;--limit=5"), "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "search saved facts")
			}
			if e := validateDataSourceStrategy(flags, "local"); e != nil {
				return usageErr(e)
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("search requires one term; quote a multi-word query"))
			}
			if e := snowLimit(limit); e != nil {
				return e
			}
			if typ != "resorts" && typ != "seasons" && typ != "reports" {
				return usageErr(fmt.Errorf("--type must be resorts, seasons or reports"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			rows, _, e := snowLocal(ctx, cmd, flags, typ, "", false)
			if e != nil {
				return e
			}
			out := make([]snowjapan.Fact, 0)
			for _, r := range rows {
				if snowMatches(r, args[0], "") {
					out = append(out, r)
				}
			}
			total := len(out)
			if len(out) > limit {
				out = out[:limit]
			}
			return snowPrint(cmd, flags, map[string]any{"results": out, "matched_records": total, "scanned_records": len(rows), "note": "Saved observations retain their own source dates; cache presence does not establish current operation."}, "local")
		}}
	cmd.Flags().StringVar(&typ, "type", "resorts", "Saved factual resource to search: resorts, seasons or reports")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum matching saved factual records to return, 1..200")
	return cmd
}
