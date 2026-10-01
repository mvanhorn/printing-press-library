package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/nifty-onsen/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/nifty-onsen/internal/onsen"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, f *rootFlags) {
		bath, _, _ := root.Find([]string{"bath"})
		bath.AddCommand(newOnsenCoupons(f), newOnsenNearby(f), newOnsenCompare(f))
		root.AddCommand(newOnsenRegions(f), newOnsenFilters(f))
	})
}
func onsenClient(f *rootFlags) (*onsen.Client, error) {
	p, e := cliutil.CacheDir()
	if e != nil {
		return nil, e
	}
	source := f.dataSource
	if cliutil.IsDogfoodEnv() && source == "auto" {
		source = "live"
	} // A live read matrix must refresh the provider, even with a warm cache.
	return onsen.New(onsen.Options{CacheDir: filepath.Join(p, "parsed-onsen-v2"), DataSource: source, NoCache: f.noCache, MaxAge: f.maxAge, Rate: f.rateLimit}), nil
}
func onsenInput(args []string, id string) (string, error) {
	if len(args) > 1 || (len(args) > 0 && id != "") {
		return "", usageErr(fmt.Errorf("use exactly one facility ID/URL positional or --id"))
	}
	if len(args) == 1 {
		id = args[0]
	}
	if id == "" {
		return "", usageErr(fmt.Errorf("facility required: use --id=onsen012278 or a canonical Nifty facility URL"))
	}
	v, e := onsen.FacilityID(id)
	if e != nil {
		return "", usageErr(e)
	}
	return v, nil
}
func checkOnsenLimit(limit, max int) error {
	if limit < 1 || limit > max {
		return usageErr(fmt.Errorf("--limit must be 1..%d", max))
	}
	return nil
}
func onsenFilters(names []string, all bool) []string {
	if !all {
		names = append([]string{"day-use"}, names...)
	}
	return names
}

// onsenOutput projects domain rows while retaining freshness/coverage independently.
func onsenOutput(cmd *cobra.Command, f *rootFlags, data any, p onsen.Provenance, coverage map[string]any) error {
	for _, w := range p.Warnings {
		fmt.Fprintln(cmd.ErrOrStderr(), "warning:", w)
	}
	raw, e := json.Marshal(data)
	if e != nil {
		return e
	}
	var selectErr error
	if f.selectFields != "" {
		paths := strings.Split(f.selectFields, ",")
		for i, s := range paths {
			s = strings.TrimSpace(s)
			s = strings.TrimPrefix(s, "items.")
			s = strings.TrimPrefix(s, "results.")
			paths[i] = s
		}
		raw, selectErr = filterFieldsChecked(raw, strings.Join(paths, ","))
		if selectErr != nil {
			return selectErr
		}
	}
	source := "live"
	if p.Cache != "miss" {
		source = "local"
	}
	if p.Cache == "mixed" {
		source = "mixed"
	}
	if p.Cache == "computed" {
		source = "computed"
	}
	meta := map[string]any{"schema_version": onsen.SchemaVersion, "source": source, "provider": "Nifty Onsen", "source_url": p.URL, "freshness": p.Freshness, "coverage": coverage, "requests": p.Requests, "warnings": p.Warnings}
	opts := *f
	opts.selectFields = ""
	opts.compact = false
	if f.csv || f.plain || f.quiet {
		if e := printOutputWithFlags(cmd.OutOrStdout(), raw, &opts); e != nil {
			return e
		}
		return selectErr
	}
	envelope := map[string]any{"meta": meta, "results": json.RawMessage(raw)}
	// Use the common output pipeline, then compress agent JSON to a single line.
	var b bytes.Buffer
	if e = printJSONFiltered(&b, envelope, &opts); e != nil {
		return e
	}
	if f.agent || f.compact {
		var compact bytes.Buffer
		if e = json.Compact(&compact, b.Bytes()); e != nil {
			return e
		}
		compact.WriteByte('\n')
		_, e = cmd.OutOrStdout().Write(compact.Bytes())
	} else {
		_, e = cmd.OutOrStdout().Write(b.Bytes())
	}
	if e != nil {
		return e
	}
	return selectErr
}

// pp:data-source auto
func newOnsenCoupons(f *rootFlags) *cobra.Command {
	var id string
	var limit int
	var full bool
	cmd := &cobra.Command{Use: "coupons [id]", Short: "Read public coupon terms, app/subscription restrictions and handoff links", Long: "Public information only. Never issues or redeems a coupon. Default returns 3 complete term sets; --full-text includes visible card text for unusual source layouts. Eligibility and current acceptance remain unknown.", Example: "  nifty-onsen-pp-cli bath coupons --id=onsen012278 --limit=3 --agent\n  nifty-onsen-pp-cli bath coupons onsen012278 --limit=12 --full-text --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "--id=onsen012278;--limit=3"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "GET public coupon information; no issuance/redemption")
		}
		id, e := onsenInput(args, id)
		if e != nil {
			return e
		}
		if e = checkOnsenLimit(limit, 20); e != nil {
			return e
		}
		c, e := onsenClient(f)
		if e != nil {
			return e
		}
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		v, p, e := c.Coupons(ctx, id)
		if e != nil {
			return e
		}
		count := len(v.Coupons)
		if count > limit {
			v.Coupons = v.Coupons[:limit]
		}
		for i := range v.Coupons {
			if !full {
				v.Coupons[i].SourceText = ""
				v.Coupons[i].SourceTextComplete = false
			}
		}
		return onsenOutput(cmd, f, v.Coupons, p, map[string]any{"scope": "public coupon cards", "facility_id": id, "source_count": count, "returned": len(v.Coupons), "truncated": count > limit, "exhaustive": false, "note": "Public terms only. Paid memberships, pair offers, age and holiday restrictions are not universal discounts. Source URL is an information handoff; no coupon is issued or redeemed."})
	}}
	cmd.Flags().StringVar(&id, "id", "", "Stable Nifty facility ID, for example onsen012278")
	cmd.Flags().IntVar(&limit, "limit", 3, "Maximum public coupons returned, from 1 to 20")
	cmd.Flags().BoolVar(&full, "full-text", false, "Include complete visible coupon card text in each result")
	return cmd
}

// pp:data-source auto
func newOnsenNearby(f *rootFlags) *cobra.Command {
	var lat, lon, radius float64
	var zoom, limit int
	var query string
	var filters []string
	var all bool
	cmd := &cobra.Command{Use: "nearby", Short: "Rank a bounded public map window by straight-line distance", Long: "Read-only source map query returns at most 20 candidates. This is a finite map window, not an exhaustive radius search. Zoom controls the source window; radius is a local distance filter. Default source filter is day-use. No walking routes or availability claims.", Example: "  nifty-onsen-pp-cli bath nearby --lat=35.6895 --lon=139.6917 --limit=5 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "--lat=35.6895;--lon=139.6917;--limit=3"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "GET public map context then POST read-only map query")
		}
		if len(args) > 0 {
			return usageErr(fmt.Errorf("nearby takes --lat and --lon flags"))
		}
		if !cmd.Flags().Changed("lat") || !cmd.Flags().Changed("lon") {
			return usageErr(fmt.Errorf("nearby requires --lat=35.6895 --lon=139.6917"))
		}
		if e := checkOnsenLimit(limit, 20); e != nil {
			return e
		}
		if radius != radius || radius < 0.1 || radius > 200 {
			return usageErr(fmt.Errorf("--radius-km must be 0.1..200"))
		}
		o := onsen.NearbyOptions{Latitude: lat, Longitude: lon, Zoom: zoom, Query: query, Filters: onsenFilters(filters, all)}
		if e := onsen.ValidateNearby(o); e != nil {
			return usageErr(e)
		}
		c, e := onsenClient(f)
		if e != nil {
			return e
		}
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		v, p, e := c.Nearby(ctx, o)
		if e != nil {
			return e
		}
		scanned := len(v)
		out := []onsen.Facility{}
		matches := 0
		for _, row := range v {
			if row.DistanceKM != nil && *row.DistanceKM <= radius {
				matches++
				if len(out) < limit {
					out = append(out, row)
				}
			}
		}
		return onsenOutput(cmd, f, out, p, map[string]any{"scope": "one source map window, max 20 candidates", "exhaustive": false, "scanned_items": scanned, "matching_items": matches, "returned": len(out), "source_cap": 20, "radius_km": radius, "zoom": zoom, "note": "Distances are straight-line. Source window is partial even below 20 results; change --zoom/coordinates or use bath search --region for broader discovery. Empty output is not proof no nearby facilities exist."})
	}}
	cmd.Flags().Float64Var(&lat, "lat", 0, "Japan latitude in decimal degrees, required")
	cmd.Flags().Float64Var(&lon, "lon", 0, "Japan longitude in decimal degrees, required")
	cmd.Flags().Float64Var(&radius, "radius-km", 20, "Local distance filter within source candidates, 0.1 to 200 km")
	cmd.Flags().IntVar(&zoom, "zoom", 10, "Source map zoom controlling candidate window, from 6 to 16")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum nearby rows returned, from 1 to 20")
	cmd.Flags().StringVar(&query, "query", "", "Optional Japanese keyword sent to source map query")
	cmd.Flags().StringSliceVar(&filters, "filter", nil, "Comma-separated source feature filters; see filters")
	cmd.Flags().BoolVar(&all, "all-types", false, "Omit default day-use source filter and include all types")
	return cmd
}

// pp:data-source auto
func newOnsenCompare(f *rootFlags) *cobra.Command {
	var ids []string
	cmd := &cobra.Command{Use: "compare [facility...]", Short: "Inspect 2 to 5 facilities side by side with per-item freshness and failures", Example: "  nifty-onsen-pp-cli bath compare --ids=onsen012278,onsen001483 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "--ids=onsen012278,onsen001483"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "GET 2 to 5 public facility details")
		}
		if len(args) > 0 && len(ids) > 0 {
			return usageErr(fmt.Errorf("use positional facilities or --ids, not both"))
		}
		input := ids
		if len(args) > 0 {
			input = args
		}
		if len(input) < 2 || len(input) > 5 {
			return usageErr(fmt.Errorf("compare requires 2..5 facility IDs/URLs"))
		}
		seen := map[string]bool{}
		clean := []string{}
		for _, s := range input {
			id, e := onsen.FacilityID(s)
			if e != nil {
				return usageErr(e)
			}
			if seen[id] {
				return usageErr(fmt.Errorf("duplicate compare ID %s", id))
			}
			seen[id] = true
			clean = append(clean, id)
		}
		c, e := onsenClient(f)
		if e != nil {
			return e
		}
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		rows := []map[string]any{}
		failures := []map[string]string{}
		perItem := map[string]any{}
		modes := map[string]bool{}
		p := onsen.Provenance{Warnings: []string{}, URL: onsen.Origin, Freshness: onsen.Freshness{Cache: "miss", Timezone: "Asia/Tokyo"}}
		for _, id := range clean {
			v, fp, e := c.Detail(ctx, id)
			if e != nil {
				failures = append(failures, map[string]string{"id": id, "error": e.Error()})
				continue
			}
			rows = append(rows, map[string]any{"facility": v, "freshness": fp.Freshness})
			p.Warnings = append(p.Warnings, fp.Warnings...)
			perItem[id] = map[string]any{"freshness": fp.Freshness, "source_url": v.URL}
			modes[fp.Cache] = true
			p.Stale = p.Stale || fp.Stale
			if fp.AgeSeconds > p.AgeSeconds {
				p.AgeSeconds = fp.AgeSeconds
			}
			if p.FetchedAt == "" || fp.FetchedAt < p.FetchedAt {
				p.FetchedAt = fp.FetchedAt
			}
		}
		p.Requests = c.Requests()
		if len(modes) > 1 {
			p.Cache = "mixed"
		} else {
			for mode := range modes {
				p.Cache = mode
			}
		}
		if len(failures) > 0 {
			p.Warnings = append(p.Warnings, fmt.Sprintf("%d of %d fetches failed; comparison includes %d successful facilities", len(failures), len(clean), len(rows)))
		}
		if len(rows) == 0 {
			return fmt.Errorf("all %d facility fetches failed: %s", len(clean), failures[0]["error"])
		}
		return onsenOutput(cmd, f, rows, p, map[string]any{"scope": "explicit comparison shortlist", "exhaustive": false, "requested": len(clean), "returned": len(rows), "fetch_failures": failures, "item_provenance": perItem, "note": "Per-ID freshness/source URLs survive projection in metadata; top freshness describes the oldest successful facts; no normalized cheapest claim because fee basis and eligibility vary. Use facility official_url/source URL to confirm terms."})
	}}
	cmd.Flags().StringSliceVar(&ids, "ids", nil, "Two to five stable facility IDs, separated by commas")
	return cmd
}

// pp:data-source computed
func newOnsenRegions(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "regions", Short: "List source prefecture slugs, Japanese names and geographic groups", Example: "  nifty-onsen-pp-cli regions --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "computed", "pp:happy-args": ""}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "list embedded source prefectures")
		}
		if f.dataSource == "live" {
			return usageErr(fmt.Errorf("regions is an embedded catalog; use --data-source=auto or local"))
		}
		if len(args) > 0 {
			return usageErr(fmt.Errorf("regions takes no positional arguments"))
		}
		return onsenOutput(cmd, f, onsen.Prefectures, onsen.Provenance{URL: onsen.Origin, Freshness: onsen.Freshness{Cache: "computed", Timezone: "Asia/Tokyo"}, Warnings: []string{}}, map[string]any{"scope": "47 source prefectures", "exhaustive": true, "catalog_verified_at": "2026-10-01"})
	}}
	return cmd
}

// pp:data-source computed
func newOnsenFilters(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "filters", Short: "List verified source filters with Japanese labels and domain caveats", Example: "  nifty-onsen-pp-cli filters --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "computed", "pp:happy-args": ""}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "list embedded source filters")
		}
		if f.dataSource == "live" {
			return usageErr(fmt.Errorf("filters is an embedded catalog; use --data-source=auto or local"))
		}
		if len(args) > 0 {
			return usageErr(fmt.Errorf("filters takes no positional arguments"))
		}
		return onsenOutput(cmd, f, onsen.Filters, onsen.Provenance{URL: onsen.Origin + "/search/", Freshness: onsen.Freshness{Cache: "computed", Timezone: "Asia/Tokyo"}, Warnings: []string{}}, map[string]any{"scope": "supported source filter subset", "exhaustive": false, "catalog_verified_at": "2026-10-01", "note": "Features are source classifications, not confirmed admission policies or reservable capacity. Multiple same-group filters use source bitmask semantics."})
	}}
	return cmd
}
