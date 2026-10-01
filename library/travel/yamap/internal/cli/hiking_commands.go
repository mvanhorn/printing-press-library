package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/yamap/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/yamap/internal/hiking"
	"github.com/spf13/cobra"
)

type hikeOptions struct {
	refresh     bool
	diagnostics bool
}
type hikeView struct {
	Meta    map[string]any `json:"meta"`
	Results []hiking.Row   `json:"results"`
}

func init() { registerNovelCommand(attachHiking) }

// pp:data-source live
func attachHiking(rootCmd *cobra.Command, f *rootFlags) {
	prior := rootCmd.PersistentPreRunE
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		parts := strings.Fields(cmd.CommandPath())
		if len(parts) > 1 && strings.HasPrefix(parts[1], "source-") {
			for _, item := range []struct {
				name string
				max  int
			}{{"per", 20}, {"page", 2000}} {
				if cmd.Flags().Lookup(item.name) != nil {
					n, err := strconv.Atoi(cmd.Flags().Lookup(item.name).Value.String())
					if err != nil || n < 1 || n > item.max {
						return usageErr(fmt.Errorf("--%s must be 1–%d", item.name, item.max))
					}
				}
			}
		}
		if len(parts) > 1 {
			switch parts[1] {
			case "mountains", "routes", "reports", "maps", "inventory":
				if !cmd.Flags().Changed("no-learn") {
					f.noLearn = true
				}
			}
		}
		return prior(cmd, args)
	}
	opts := &hikeOptions{}
	rootCmd.PersistentFlags().BoolVar(&opts.refresh, "refresh", false, "Refresh exact request cache from YAMAP; never crawl full inventory")
	rootCmd.PersistentFlags().BoolVar(&opts.diagnostics, "diagnostics", false, "Write request count, response bytes and latency to stderr")
	f.timeout = 30 * time.Second
	rootCmd.PersistentFlags().Lookup("timeout").DefValue = "30s"
	f.maxAge = 15 * time.Minute
	rootCmd.PersistentFlags().Lookup("max-age").DefValue = "15m0s"
	for _, cmd := range rootCmd.Commands() {
		if strings.HasPrefix(cmd.Name(), "source-") {
			hideHikingTree(cmd, f)
		}
	}
	rootCmd.AddCommand(newHikingMountainsCmd(f, opts))
	rootCmd.AddCommand(newHikingRoutesCmd(f, opts))
	rootCmd.AddCommand(newHikingReportsCmd(f, opts))
	rootCmd.AddCommand(newHikingMapsCmd(f, opts))
	rootCmd.AddCommand(newHikingInventoryCmd(f, opts))
}
func newHikingMountainsCmd(f *rootFlags, o *hikeOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "mountains", Example: "  yamap-pp-cli mountains search 高尾山 --limit 3 --agent", Short: "Discover named mountains and source relationships", RunE: parentNoSubcommandRunE(f)}
	cmd.AddCommand(newHikingMountainSearch(f, o))
	cmd.AddCommand(newHikingMountainGet(f, o))
	cmd.AddCommand(newHikingMountainRoutes(f, o))
	cmd.AddCommand(newHikingMountainReports(f, o))
	return cmd
}
func newHikingRoutesCmd(f *rootFlags, o *hikeOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "routes", Example: "  yamap-pp-cli routes get 1771 --agent", Short: "Inspect planned model courses", RunE: parentNoSubcommandRunE(f)}
	cmd.AddCommand(newHikingRouteSearch(f, o))
	cmd.AddCommand(newHikingRouteGet(f, o))
	cmd.AddCommand(hikingCompare(f, o))
	return cmd
}
func newHikingReportsCmd(f *rootFlags, o *hikeOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "reports", Example: "  yamap-pp-cli reports recent 高尾山 --limit 3 --agent", Short: "Read activity logs and contributor observations", RunE: parentNoSubcommandRunE(f)}
	cmd.AddCommand(newHikingReportSearch(f, o))
	cmd.AddCommand(newHikingReportGet(f, o))
	cmd.AddCommand(hikingRecent(f, o))
	cmd.AddCommand(newHikingObservationsCmd(f, o))
	return cmd
}
func newHikingObservationsCmd(f *rootFlags, o *hikeOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "observations [id-or-source-url]", Short: "Read dated contributor observation text; no closure or safety inference"}
	base := hikingDetail(f, o, "report", "observations")
	cmd.RunE = base.RunE
	cmd.Example = base.Example
	cmd.Annotations = base.Annotations
	return cmd
}
func newHikingMapsCmd(f *rootFlags, o *hikeOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "maps", Example: "  yamap-pp-cli maps coverage 77 --agent", Short: "Inspect map areas and coverage limits", RunE: parentNoSubcommandRunE(f)}
	cmd.AddCommand(newHikingMapSearch(f, o))
	cmd.AddCommand(newHikingMapGet(f, o))
	cmd.AddCommand(newHikingCoverageCmd(f, o))
	return cmd
}
func newHikingCoverageCmd(f *rootFlags, o *hikeOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "coverage [id-or-source-url]", Short: "Inspect source map bounds and limits; offline navigation unavailable"}
	base := hikingDetail(f, o, "map", "coverage")
	cmd.RunE = base.RunE
	cmd.Example = base.Example
	cmd.Annotations = base.Annotations
	return cmd
}
func newHikingInventoryCmd(f *rootFlags, o *hikeOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "inventory", Example: "  yamap-pp-cli inventory status --agent", Short: "Inspect the bounded exact-request cache", RunE: parentNoSubcommandRunE(f)}
	cmd.AddCommand(hikingInventory(f, o))
	return cmd
}
func hideHikingTree(c *cobra.Command, f *rootFlags) {
	c.Hidden = true
	if c.Annotations == nil {
		c.Annotations = map[string]string{}
	}
	c.Annotations["mcp:hidden"] = "true"
	if run := c.RunE; run != nil {
		c.RunE = func(cmd *cobra.Command, args []string) error {
			if dryRunOK(f) {
				return writeDryRun(cmd.OutOrStdout(), f, cmd.CommandPath())
			}
			return run(cmd, args)
		}
	}
	for _, child := range c.Commands() {
		hideHikingTree(child, f)
	}
}
func hikingAnnotations(happy string, origin string) map[string]string {
	return map[string]string{"mcp:read-only": "true", "pp:data-source": origin, "pp:typed-exit-codes": "0,2,3,4,5,7,10", "pp:happy-args": happy}
}
func hikingClient(f *rootFlags, opts *hikeOptions) (*hiking.Client, error) {
	if f.timeout <= 0 || f.timeout > 2*time.Minute {
		return nil, usageErr(fmt.Errorf("--timeout must be positive and at most 2m"))
	}
	if f.maxAge <= 0 || f.maxAge > 24*time.Hour {
		return nil, usageErr(fmt.Errorf("--max-age must be positive and at most 24h"))
	}
	if f.csv || f.plain || f.quiet {
		return nil, usageErr(fmt.Errorf("focused hiking commands emit compact JSON; use --select to narrow fields"))
	}
	if f.dataSource == "local" && (opts.refresh || f.noCache) {
		return nil, usageErr(fmt.Errorf("--data-source local cannot combine with --refresh or --no-cache"))
	}
	dir, err := cliutil.KindDir(cliutil.PathKindCache)
	if err != nil {
		return nil, configErr(err)
	}
	c := hiking.NewClient(filepath.Join(dir, "hiking-v1"), f.rateLimit)
	c.Refresh = opts.refresh || f.dataSource == "live"
	c.NoCache = f.noCache
	c.Offline = f.dataSource == "local"
	c.TTL = f.maxAge
	if cliutil.IsDogfoodEnv() {
		c.Refresh = true
		c.Offline = false
	}
	return c, nil
}
func hikingError(err error) error {
	if err == nil {
		return nil
	}
	var rate *cliutil.RateLimitError
	if errors.As(err, &rate) {
		return rateLimitErr(err)
	}
	var source *hiking.SourceError
	if errors.As(err, &source) {
		switch source.Kind {
		case "usage":
			return usageErr(err)
		case "not_found":
			return notFoundErr(err)
		case "restricted":
			return authErr(err)
		}
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return apiErr(fmt.Errorf("YAMAP request budget ended: %w", err))
	}
	return apiErr(err)
}
func hikingEmit(cmd *cobra.Command, f *rootFlags, opts *hikeOptions, c *hiking.Client, start time.Time, rows []hiking.Row, fetches []hiking.FetchMeta, extra map[string]any) error {
	if rows == nil {
		rows = []hiking.Row{}
	}
	for i, r := range rows {
		projected, err := hiking.Project(r, f.selectFields)
		if err != nil {
			return usageErr(err)
		}
		rows[i] = projected
	}
	meta := map[string]any{"provider": "YAMAP", "source": "first_party_public_api", "retrieved_at": time.Now().UTC().Format(time.RFC3339), "fetches": fetches, "request_count": c.Requests, "cache_hits": c.CacheHits, "response_bytes": c.ResponseBytes, "returned": len(rows), "access": "public_anonymous", "official_closure_coverage": "unknown", "safety_status": nil, "partial_coverage": true, "units": "m, seconds; timestamps UTC; activity_date_jst Asia/Tokyo", "membership_limits": "no GPX, offline maps, account data or multi-landmark premium filtering"}
	for k, v := range extra {
		meta[k] = v
	}
	if len(c.Warnings) > 0 {
		meta["warnings"] = c.Warnings
		for _, w := range c.Warnings {
			fmt.Fprintln(cmd.ErrOrStderr(), "warning: "+w)
		}
	}
	for _, fetch := range fetches {
		if fetch.Stale {
			fmt.Fprintln(cmd.ErrOrStderr(), "warning: cached source response is stale; use --refresh online")
		}
	}
	if opts.diagnostics {
		fmt.Fprintf(cmd.ErrOrStderr(), "yamap requests=%d cache_hits=%d upstream_bytes=%d elapsed_ms=%d\n", c.Requests, c.CacheHits, c.ResponseBytes, time.Since(start).Milliseconds())
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetEscapeHTML(false)
	return enc.Encode(hikeView{Meta: meta, Results: rows})
}
func hikingArgs(args []string, n int) error {
	if len(args) != n {
		return usageErr(fmt.Errorf("expected %d argument(s); see --help for a source ID or Japanese name example", n))
	}
	return nil
}
func hikingQuery(s string) error {
	if strings.TrimSpace(s) == "" || len([]rune(s)) > 120 {
		return usageErr(fmt.Errorf("query must contain 1–120 characters; Japanese source names give better matches"))
	}
	return nil
}
func hikingPagination(page, limit, pages int) error {
	if page < 1 || page > 2000 {
		return usageErr(fmt.Errorf("--page must be 1–2000"))
	}
	if limit < 1 || limit > 20 {
		return usageErr(fmt.Errorf("--limit must be 1–20"))
	}
	if pages < 1 || pages > 5 {
		return usageErr(fmt.Errorf("page scan budget must be 1–5"))
	}
	return nil
}
func hikingPath(kind string) (string, string, string) {
	switch kind {
	case "mountain":
		return "mountains", "mountains", "mountain"
	case "route":
		return "model_courses", "model_courses", "model_course"
	case "report":
		return "activities", "activities", "activity"
	default:
		return "maps", "maps", "map"
	}
}
func hikingSearch(f *rootFlags, opts *hikeOptions, kind, name, scope string) *cobra.Command {
	page, limit, pages := 1, 5, 1
	happy := "query=高尾山;--limit=2"
	arg := "query"
	short := "Search source " + kind + " summaries by Japanese name or text"
	if scope == "mountain" {
		happy = "mountain=108;--limit=2"
		arg = "mountain-id"
		short = "Read source " + kind + " relationships for an exact mountain ID"
	}
	cmd := &cobra.Command{Use: name + " [" + arg + "]", Short: short, Annotations: hikingAnnotations(happy, "live"), Example: "  yamap-pp-cli " + map[string]string{"mountain": "mountains", "route": "routes", "report": "reports", "map": "maps"}[kind] + " search 高尾山 --limit 3 --agent", RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, cmd.CommandPath())
		}
		if err := hikingArgs(args, 1); err != nil {
			return err
		}
		if err := hikingPagination(page, limit, pages); err != nil {
			return err
		}
		path, key, _ := hikingPath(kind)
		params := url.Values{}
		coverage := "source_text_or_name_match; no traversal verification"
		if scope == "mountain" {
			id, e := hiking.ParseID(args[0], "mountains")
			if e != nil {
				return usageErr(e)
			}
			path = "mountains/" + id + "/" + path
			coverage = "source_mountain_relationship; not exact route overlap"
		} else {
			if err := hikingQuery(args[0]); err != nil {
				return err
			}
			wire := "keyword"
			if kind == "route" {
				wire = "name"
			}
			params.Set(wire, args[0])
			if kind == "route" {
				path += "/"
			} else {
				path += "/search"
			}
		}
		c, e := hikingClient(f, opts)
		if e != nil {
			return e
		}
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		start := time.Now()
		rows := []hiking.Row{}
		fetches := []hiking.FetchMeta{}
		scanned := 0
		next := any(nil)
		var sourceMeta any
		if cliutil.IsDogfoodEnv() {
			pages = 1
		}
		seen := map[string]bool{}
		requested := page
		for i := 0; i < pages; i++ {
			params.Set("page", strconv.Itoa(requested))
			params.Set("per", strconv.Itoa(limit))
			obj, fm, err := c.Fetch(ctx, "/"+path, params)
			if err != nil {
				return hikingError(err)
			}
			items, err := hiking.Extract(obj, key)
			if err != nil {
				return hikingError(err)
			}
			fetches = append(fetches, fm)
			scanned += len(items)
			sourceMeta = obj["meta"]
			for _, m := range items {
				r := hiking.Summary(kind, m, false)
				id := r["id"].(string)
				if !seen[id] && len(rows) < limit {
					rows = append(rows, r)
					seen[id] = true
				}
			}
			np := hikeNext(obj["meta"])
			next = nil
			if np > requested {
				next = np
			}
			if np <= requested || len(items) == 0 || len(rows) >= limit {
				break
			}
			requested = np
		}
		return hikingEmit(cmd, f, opts, c, start, rows, fetches, map[string]any{"match_basis": coverage, "source_pagination": sourceMeta, "next_page": next, "scanned_records": scanned, "page_budget": pages, "source_result_window_cap": func() any {
			if kind == "report" {
				return 10000
			}
			return nil
		}(), "coverage_note": "Source result counts describe its search window, not a complete inventory; --page follows explicit pagination"})
	}}
	if scope == "mountain" {
		cmd.Example = "  yamap-pp-cli mountains " + name + " 108 --limit 3 --agent"
	}
	cmd.Flags().IntVar(&page, "page", 1, "One-based source result page, maximum 2000")
	cmd.Flags().IntVar(&limit, "limit", 5, "Maximum result count and source page size, 1–20")
	cmd.Flags().IntVar(&pages, "max-pages", 1, "Maximum pages examined for short pages, 1–5; output still bounded by limit")
	return cmd
}
func hikeNext(meta any) int {
	m, _ := meta.(map[string]any)
	switch n := m["next_page"].(type) {
	case json.Number:
		i, _ := strconv.Atoi(n.String())
		return i
	case float64:
		return int(n)
	}
	return 0
}
func hikingDetail(f *rootFlags, opts *hikeOptions, kind, name string) *cobra.Command {
	id := map[string]string{"mountain": "108", "route": "1771", "report": "51497803", "map": "77"}[kind]
	parent := map[string]string{"mountain": "mountains", "route": "routes", "report": "reports", "map": "maps"}[kind]
	cmd := &cobra.Command{Use: name + " [id-or-source-url]", Short: "Read one public " + kind + " detail with metrics and evidence limits", Example: "  yamap-pp-cli " + parent + " " + name + " " + id + " --agent", Annotations: hikingAnnotations("id="+id, "live"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, cmd.CommandPath())
		}
		if err := hikingArgs(args, 1); err != nil {
			return err
		}
		path, _, key := hikingPath(kind)
		urlKind := path
		if kind == "route" {
			urlKind = "model-courses"
		}
		parsed, err := hiking.ParseID(args[0], urlKind)
		if err != nil {
			return usageErr(err)
		}
		c, err := hikingClient(f, opts)
		if err != nil {
			return err
		}
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		start := time.Now()
		obj, fm, err := c.Fetch(ctx, "/"+path+"/"+parsed, nil)
		if err != nil {
			return hikingError(err)
		}
		m, err := hiking.Detail(obj, key, parsed)
		if err != nil {
			return hikingError(err)
		}
		r := hiking.Summary(kind, m, true)
		note := "publisher source facts; flags and map coverage do not establish open/safe status"
		if kind == "report" {
			note = "contributor text and recorded metrics; no authoritative trail or closure advice"
		}
		return hikingEmit(cmd, f, opts, c, start, []hiking.Row{r}, []hiking.FetchMeta{fm}, map[string]any{"coverage_note": note, "detail": true})
	}}
	return cmd
}
func hikingRecent(f *rootFlags, opts *hikeOptions) *cobra.Command {
	limit, pages, days := 5, 1, 30
	var since, mountain string
	cmd := &cobra.Command{Use: "recent [query]", Short: "Find reports by recorded trip date within bounded source candidates", Long: "Scan source candidates and filter by trip start date, not publication date. Default one page of twenty candidates. Results are sorted within scanned pages only. Known planned and future activities are excluded. A missing planned flag remains unknown until detail retrieval. No match does not establish absence of recent hiking or trail conditions.", Example: "  yamap-pp-cli reports recent 高尾山 --days 30 --limit 3 --agent\n  yamap-pp-cli reports recent --mountain-id 108 --max-scan-pages 2 --agent", Annotations: hikingAnnotations("query=高尾山;--limit=2", "live"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, cmd.CommandPath())
		}
		if len(args) > 1 || (mountain == "" && len(args) != 1) || (mountain != "" && len(args) > 0) {
			return usageErr(fmt.Errorf("provide one query OR --mountain-id, not both"))
		}
		if err := hikingPagination(1, limit, pages); err != nil {
			return err
		}
		now := time.Now()
		cutoff, err := hiking.Since(since, days, now)
		if err != nil {
			return usageErr(err)
		}
		if since != "" && cmd.Flags().Changed("days") {
			return usageErr(fmt.Errorf("use --since OR --days"))
		}
		path := "/activities/search"
		params := url.Values{"per": {"20"}}
		basis := "source_text_match; mountain traversal unverified"
		if mountain != "" {
			id, e := hiking.ParseID(mountain, "mountains")
			if e != nil {
				return usageErr(e)
			}
			path = "/mountains/" + id + "/activities"
			basis = "source_mountain_relationship"
		} else {
			if err := hikingQuery(args[0]); err != nil {
				return err
			}
			params.Set("keyword", args[0])
		}
		c, err := hikingClient(f, opts)
		if err != nil {
			return err
		}
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		start := time.Now()
		fetches := []hiking.FetchMeta{}
		matches := []hiking.Row{}
		scanned := 0
		next := 1
		exhausted := false
		seen := map[string]bool{}
		if cliutil.IsDogfoodEnv() {
			pages = 1
		}
		for p := 0; p < pages; p++ {
			params.Set("page", strconv.Itoa(next))
			obj, fm, e := c.Fetch(ctx, path, params)
			if e != nil {
				return hikingError(e)
			}
			items, e := hiking.Extract(obj, "activities")
			if e != nil {
				return hikingError(e)
			}
			fetches = append(fetches, fm)
			scanned += len(items)
			for _, m := range items {
				if hiking.Recent(m, cutoff, now) && !seen[hiking.ID(m["id"])] {
					matches = append(matches, m)
					seen[hiking.ID(m["id"])] = true
				}
			}
			np := hikeNext(obj["meta"])
			if np <= next || len(items) == 0 {
				exhausted = true
				next = 0
				break
			}
			next = np
		}
		sort.SliceStable(matches, func(i, j int) bool {
			return fmt.Sprint(hiking.Stamp(matches[i]["start_at"])) > fmt.Sprint(hiking.Stamp(matches[j]["start_at"]))
		})
		if len(matches) > limit {
			matches = matches[:limit]
		}
		rows := []hiking.Row{}
		for _, m := range matches {
			r := hiking.Summary("report", m, false)
			r["recency_basis"] = "activity_start_date"
			rows = append(rows, r)
		}
		note := "Recent only among scanned source candidates; source order is not guaranteed by activity date"
		if len(rows) == 0 {
			note = "No matching recorded trips among scanned candidates; raise --max-scan-pages or widen --days. No trail condition or closure inference."
		}
		return hikingEmit(cmd, f, opts, c, start, rows, fetches, map[string]any{"since": cutoff.UTC().Format(time.RFC3339), "as_of": now.UTC().Format(time.RFC3339), "scanned_records": scanned, "max_scan_pages": pages, "source_exhausted": exhausted, "next_page": next, "match_basis": basis, "coverage_note": note, "partial_coverage": true})
	}}
	cmd.Flags().IntVar(&limit, "limit", 5, "Maximum recent reports to return, 1–20")
	cmd.Flags().IntVar(&pages, "max-scan-pages", 1, "Maximum candidate pages to inspect, 1–5; twenty candidates per page")
	cmd.Flags().IntVar(&days, "days", 30, "Look back by trip date, 1–366 days")
	cmd.Flags().StringVar(&since, "since", "", "Earliest trip date in Japan, YYYY-MM-DD; replaces days")
	cmd.Flags().StringVar(&mountain, "mountain-id", "", "Exact source mountain ID, alternative to broad text query")
	return cmd
}
func hikingCompare(f *rootFlags, opts *hikeOptions) *cobra.Command {
	return &cobra.Command{Use: "compare [model-course-id] [activity-id]", Short: "Compare planned route metrics against a recorded trip", Long: "Distance and elevation differences are numeric facts. The CLI does not compare tracks, establish route equivalence, infer pace, determine official closures or assess safety. Standard model-course time differs from recorded elapsed and source active time.", Example: "  yamap-pp-cli routes compare 1771 51497803 --agent", Annotations: hikingAnnotations("route=1771;activity=51497803", "computed"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, cmd.CommandPath())
		}
		if err := hikingArgs(args, 2); err != nil {
			return err
		}
		rid, e := hiking.ParseID(args[0], "model-courses")
		if e != nil {
			return usageErr(e)
		}
		aid, e := hiking.ParseID(args[1], "activities")
		if e != nil {
			return usageErr(e)
		}
		c, e := hikingClient(f, opts)
		if e != nil {
			return e
		}
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		start := time.Now()
		r, rf, e := c.Fetch(ctx, "/model_courses/"+rid, nil)
		if e != nil {
			return hikingError(e)
		}
		rm, e := hiking.Detail(r, "model_course", rid)
		if e != nil {
			return hikingError(e)
		}
		a, af, e := c.Fetch(ctx, "/activities/"+aid, nil)
		if e != nil {
			return hikingError(e)
		}
		am, e := hiking.Detail(a, "activity", aid)
		if e != nil {
			return hikingError(e)
		}
		row, e := hiking.Compare(rm, am)
		if e != nil {
			return hikingError(e)
		}
		return hikingEmit(cmd, f, opts, c, start, []hiking.Row{row}, []hiking.FetchMeta{rf, af}, map[string]any{"coverage_note": "numeric comparison only; route equivalence and official closures unknown"})
	}}
}
func hikingInventory(f *rootFlags, opts *hikeOptions) *cobra.Command {
	return &cobra.Command{Use: "status", Short: "Inspect cached request counts, storage bounds and freshness", Example: "  yamap-pp-cli inventory status --agent", Annotations: hikingAnnotations("", "local"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "inventory status")
		}
		if err := hikingArgs(args, 0); err != nil {
			return err
		}
		if opts.refresh {
			return usageErr(fmt.Errorf("inventory status does not crawl; refresh the chosen source command with --refresh"))
		}
		c, e := hikingClient(f, opts)
		if e != nil {
			return e
		}
		start := time.Now()
		row, e := c.Inventory()
		if e != nil {
			return configErr(e)
		}
		return hikingEmit(cmd, f, opts, c, start, []hiking.Row{row}, []hiking.FetchMeta{}, map[string]any{"source": "local_exact_request_cache", "coverage_note": "Only requests you fetched are present; no comprehensive offline inventory"})
	}}
}

func newHikingMountainSearch(f *rootFlags, o *hikeOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "search [query]"}
	return configureHikingAlias(cmd, hikingSearch(f, o, "mountain", "search", ""))
}

func newHikingMountainGet(f *rootFlags, o *hikeOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "get [id-or-source-url]"}
	return configureHikingAlias(cmd, hikingDetail(f, o, "mountain", "get"))
}

func newHikingMountainRoutes(f *rootFlags, o *hikeOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "routes [mountain-id]"}
	return configureHikingAlias(cmd, hikingSearch(f, o, "route", "routes", "mountain"))
}

func newHikingMountainReports(f *rootFlags, o *hikeOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "reports [mountain-id]"}
	return configureHikingAlias(cmd, hikingSearch(f, o, "report", "reports", "mountain"))
}

func newHikingRouteSearch(f *rootFlags, o *hikeOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "search [query]"}
	return configureHikingAlias(cmd, hikingSearch(f, o, "route", "search", ""))
}

func newHikingRouteGet(f *rootFlags, o *hikeOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "get [id-or-source-url]"}
	return configureHikingAlias(cmd, hikingDetail(f, o, "route", "get"))
}

func newHikingReportSearch(f *rootFlags, o *hikeOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "search [query]"}
	return configureHikingAlias(cmd, hikingSearch(f, o, "report", "search", ""))
}

func newHikingReportGet(f *rootFlags, o *hikeOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "get [id-or-source-url]"}
	return configureHikingAlias(cmd, hikingDetail(f, o, "report", "get"))
}

func newHikingMapSearch(f *rootFlags, o *hikeOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "search [query]"}
	return configureHikingAlias(cmd, hikingSearch(f, o, "map", "search", ""))
}

func newHikingMapGet(f *rootFlags, o *hikeOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "get [id-or-source-url]"}
	return configureHikingAlias(cmd, hikingDetail(f, o, "map", "get"))
}

func configureHikingAlias(cmd, base *cobra.Command) *cobra.Command {
	cmd.Short = base.Short
	cmd.Long = base.Long
	cmd.RunE = base.RunE
	cmd.Example = base.Example
	cmd.Annotations = base.Annotations
	cmd.Flags().AddFlagSet(base.Flags())
	return cmd
}
