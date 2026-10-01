// pp:data-source auto
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/omakase/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/omakase/internal/omakase"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"time"
	"unicode/utf8"
)

type planningOptions struct {
	lang             string
	refresh, offline bool
}
type planningSession struct {
	client  *omakase.Client
	started time.Time
}

func planningFlags(c *cobra.Command, o *planningOptions) {
	c.Flags().StringVar(&o.lang, "lang", "en", "Source language: en or ja")
	c.Flags().BoolVar(&o.refresh, "refresh", false, "Fetch current source instead of cache")
	c.Flags().BoolVar(&o.offline, "offline", false, "Read only cached source; explicitly mark stale data")
}
func newPlanningSession(f *rootFlags, o planningOptions) (*planningSession, error) {
	if _, e := omakase.LocalePath(o.lang); e != nil {
		return nil, usageErr(e)
	}
	if (o.offline || f.dataSource == "local") && o.refresh {
		return nil, usageErr(fmt.Errorf("--offline and --refresh conflict"))
	}
	if (o.offline || f.dataSource == "local") && (f.noCache || f.dataSource == "live") {
		return nil, usageErr(fmt.Errorf("offline/local needs cache and conflicts with live/no-cache"))
	}
	if f.maxAge < 0 {
		return nil, usageErr(fmt.Errorf("--max-age must be nonnegative"))
	}
	dir, e := cliutil.CacheDir()
	if e != nil {
		return nil, configErr(e)
	}
	c := omakase.New(filepath.Join(dir, "omakase", "documents"))
	c.Refresh = o.refresh || f.dataSource == "live"
	c.Offline = o.offline || f.dataSource == "local"
	c.NoCache = f.noCache
	c.MaxAge = f.maxAge
	if f.rateLimit == 0 {
		c.Pace = 0
	} else if f.rateLimit > 0 {
		c.Pace = time.Duration(float64(time.Second) / f.rateLimit)
		if c.Pace < 250*time.Millisecond {
			c.Pace = 250 * time.Millisecond
		}
	}
	return &planningSession{c, time.Now()}, nil
}
func planningError(e error) error {
	var he *omakase.HTTPError
	var rate *cliutil.RateLimitError
	if errors.As(e, &rate) {
		return rateLimitErr(e)
	}
	if errors.As(e, &he) {
		if he.Status == 404 {
			return notFoundErr(e)
		}
		if he.Status == 401 || he.Status == 403 {
			return authErr(e)
		}
	}
	return apiErr(e)
}
func planningOutput(c *cobra.Command, f *rootFlags, v any) error {
	// Preserve explicit nulls and nested source terms; generated gravity compaction
	// is unsuitable for course/release data. --select still uses framework projection.
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	if f.selectFields != "" {
		b, e = filterFieldsChecked(b, f.selectFields)
		if e != nil {
			return e
		}
	}
	if f.csv || f.plain || f.quiet {
		copyFlags := *f
		copyFlags.agent = false
		copyFlags.compact = false
		copyFlags.selectFields = ""
		copyFlags.asJSON = true
		return printOutputWithFlags(c.OutOrStdout(), b, &copyFlags)
	}
	_, e = fmt.Fprintln(c.OutOrStdout(), string(b))
	return e
}
func (s *planningSession) emit(c *cobra.Command, f *rootFlags, results any, partial bool, extra map[string]any) error {
	meta := map[string]any{"provider": "OMAKASE", "source": "live", "access": "anonymous_public", "retrieved_at": time.Now().UTC(), "requests": s.client.Stats.Requests, "cache_hits": s.client.Stats.CacheHits, "response_bytes": s.client.Stats.ResponseBytes, "elapsed_ms": time.Since(s.started).Milliseconds(), "partial": partial, "seat_coverage": "exact seats not exposed publicly"}
	if s.client.Stats.Requests == 0 {
		meta["transport"] = "cache"
	} else {
		meta["transport"] = "http"
	}
	for k, v := range extra {
		meta[k] = v
	}
	return planningOutput(c, f, map[string]any{"meta": meta, "results": results})
}
func planAnnotation(happy string) map[string]string {
	return map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": happy}
}
func oneID(args []string) error {
	if len(args) != 1 {
		return usageErr(fmt.Errorf("provide one OMAKASE restaurant ID"))
	}
	if e := omakase.ValidateID(args[0]); e != nil {
		return usageErr(e)
	}
	return nil
}
func publicDetail(ctx context.Context, s *planningSession, id, lang string, japanese bool) (omakase.Detail, bool, error) {
	d, e := s.client.Restaurant(ctx, id, lang)
	if e != nil {
		return d, false, e
	}
	partial := false
	if japanese && lang == "en" {
		jp, e := s.client.Restaurant(ctx, id, "ja")
		if e != nil {
			partial = true
			d.Warnings = append(d.Warnings, "Japanese name unavailable: "+e.Error())
		} else {
			d.NameJA = jp.NameJA
			d.Evidence = append(d.Evidence, jp.Evidence...)
		}
	}
	return d, partial, nil
}
func sourceStale(evs []omakase.Evidence) bool {
	for _, e := range evs {
		if e.Stale {
			return true
		}
	}
	return false
}

func init() {
	registerNovelCommand(func(root *cobra.Command, f *rootFlags) {
		root.AddCommand(newRestaurantsCmd(f), newFiltersCmd(f), newMembershipCmd(f))
	})
}
func newRestaurantsCmd(f *rootFlags) *cobra.Command {
	p := &cobra.Command{Use: "restaurants", Short: "Find public restaurant summaries or fetch lazy source details"}
	p.AddCommand(newRestaurantsFindCmd(f))
	show := newRestaurantsShowCmd(f)
	p.AddCommand(show)
	return p
}
func newRestaurantsFindCmd(f *rootFlags) *cobra.Command {
	var o planningOptions
	var query, area, cuisine string
	page, limit, offset := 1, 10, 0
	c := &cobra.Command{Use: "find", Short: "Find name-relevant public summaries on one source page", Long: "Read one source page only. --query applies a literal name substring filter after the provider's broad fuzzy search. Coverage remains partial when source pages remain. Use --offset for remaining matches on the same page, then --page for the next source page. Japanese names on English summary cards are null until detail retrieval.", Example: "  omakase-pp-cli restaurants find --query Sugita --limit 10 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "--query=Sugita"}, RunE: func(c *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(c.OutOrStdout(), f, "read public restaurant summaries")
		}
		if len(args) > 0 {
			return usageErr(fmt.Errorf("use --query for a name"))
		}
		if limit < 1 || limit > 50 || offset < 0 || offset > 31 || page < 1 || page > 25 || utf8.RuneCountInString(query) > 50 {
			return usageErr(fmt.Errorf("limit 1..50, offset 0..31, page 1..25, query at most 50 characters"))
		}
		s, e := newPlanningSession(f, o)
		if e != nil {
			return e
		}
		ctx, cancel := boundCtx(c.Context(), f)
		defer cancel()
		p, ev, e := s.client.Catalogue(ctx, o.lang, page, query, area, cuisine)
		if e != nil {
			return planningError(e)
		}
		for key, value := range map[string]string{"area": area, "cuisine": cuisine} {
			if value != "" {
				valid := false
				for _, op := range p.Filters[key] {
					if op.Value == value {
						valid = true
					}
				}
				if !valid {
					return usageErr(fmt.Errorf("unknown %s %q; use filters for source values", key, value))
				}
			}
		}
		matches := []omakase.Summary{}
		for _, row := range p.Results {
			if omakase.NameMatches(row.Name, query) {
				matches = append(matches, row)
			}
		}
		start := offset
		if start > len(matches) {
			start = len(matches)
		}
		end := start + limit
		if end > len(matches) {
			end = len(matches)
		}
		var nextOffset, nextPage *int
		if end < len(matches) {
			v := end
			nextOffset = &v
		} else if p.Next != nil {
			v := page + 1
			nextPage = &v
		}
		partial := p.Next != nil || nextOffset != nil || ev.Stale
		return s.emit(c, f, matches[start:end], partial, map[string]any{"evidence": ev, "source_total": p.Total, "source_page": page, "source_page_rows": len(p.Results), "name_matches_on_page": len(matches), "next_offset": nextOffset, "next_page": nextPage, "next_url": p.Next, "query": query, "match_policy": "literal_name_substring"})
	}}
	planningFlags(c, &o)
	c.Flags().StringVar(&query, "query", "", "Name substring (literal filter after source fuzzy search)")
	c.Flags().StringVar(&area, "area", "", "Source area value; see filters")
	c.Flags().StringVar(&cuisine, "cuisine", "", "Source cuisine value; see filters")
	c.Flags().IntVar(&page, "page", 1, "Source page 1..25")
	c.Flags().IntVar(&limit, "limit", 10, "Maximum returned matches 1..50")
	c.Flags().IntVar(&offset, "offset", 0, "Offset in matched rows on this source page 0..31")
	return c
}
func configurePlanningDetailCmd(f *rootFlags, c *cobra.Command, kind string, date *string, party *int) *cobra.Command {
	var o planningOptions
	var japanese bool
	if date == nil {
		v := ""
		date = &v
	}
	if party == nil {
		v := 0
		party = &v
	}
	happy := "id=hc778124"
	if kind == "release" {
		happy = "id=jv742052"
	}
	if kind == "availability" {
		happy += ";--date=2026-11-01;--party=2"
	}
	c.Annotations = planAnnotation(happy)
	c.Example = "  omakase-pp-cli " + kind + " hc778124 --agent"
	if kind == "show" {
		c.Example = "  omakase-pp-cli restaurants show hc778124 --agent"
	}
	c.RunE = func(c *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(c.OutOrStdout(), f, "inspect public restaurant "+kind)
		}
		if e := oneID(args); e != nil {
			return e
		}
		if kind == "availability" {
			if (*date == "") != (*party == 0) {
				return usageErr(fmt.Errorf("provide --date and --party together"))
			}
			if *date != "" {
				if _, e := time.Parse("2006-01-02", *date); e != nil {
					return usageErr(fmt.Errorf("date must be a valid YYYY-MM-DD"))
				}
				if *party < 1 || *party > 20 {
					return usageErr(fmt.Errorf("party must be 1..20"))
				}
			}
		}
		s, e := newPlanningSession(f, o)
		if e != nil {
			return e
		}
		ctx, cancel := boundCtx(c.Context(), f)
		defer cancel()
		d, partial, e := publicDetail(ctx, s, args[0], o.lang, japanese)
		if e != nil {
			return planningError(e)
		}
		partial = partial || sourceStale(d.Evidence)
		base := map[string]any{"id": d.ID, "name": d.Name, "name_ja": d.NameJA, "url": d.URL, "evidence": d.Evidence}
		var out any = d
		switch kind {
		case "courses":
			base["courses"] = d.Courses
			base["course_notes"] = d.CourseNotes
			base["service_charge"] = d.ServiceCharge
			base["reservation_fee"] = d.ReservationFee
			base["cancellation"] = d.Cancellation
			base["cancellation_note"] = d.CancellationNote
			out = base
		case "release":
			base["release"] = d.Release
			base["booking_method"] = d.BookingMethod
			base["action_raw"] = d.ActionRaw
			base["seat_state"] = "unknown"
			base["warnings"] = d.Warnings
			out = base
		case "availability":
			base["state"] = "unknown"
			base["date"] = nil
			if *date != "" {
				base["date"] = *date
			}
			base["party"] = nil
			if *party > 0 {
				base["party"] = *party
			}
			base["query_evaluated"] = false
			base["seats"] = nil
			base["access"] = d.Access
			base["booking_method"] = d.BookingMethod
			base["release"] = d.Release
			base["action_raw"] = d.ActionRaw
			base["reason"] = "Exact date/party seats are not exposed on anonymous public pages; release, request and waitlist do not prove availability."
			out = base
			partial = true
		}
		return s.emit(c, f, out, partial, map[string]any{"stale": sourceStale(d.Evidence)})
	}
	planningFlags(c, &o)
	if kind == "show" {
		c.Flags().BoolVar(&japanese, "japanese", true, "Also lazily retrieve the first-party Japanese name (one additional document)")
	}
	return c
}
func newPlanningCompareCmd(f *rootFlags) *cobra.Command {
	var o planningOptions
	var allowPartial bool
	c := &cobra.Command{Use: "compare [id...]", Short: "Compare public courses, fees, cancellation and release for 2..5 restaurants", Example: "  omakase-pp-cli compare hc778124 qt951856 --agent", Annotations: planAnnotation("hc778124;qt951856"), RunE: func(c *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(c.OutOrStdout(), f, "compare public restaurant details")
		}
		if len(args) < 2 || len(args) > 5 {
			return usageErr(fmt.Errorf("compare requires 2..5 source restaurant IDs"))
		}
		seen := map[string]bool{}
		for _, id := range args {
			if e := omakase.ValidateID(id); e != nil {
				return usageErr(e)
			}
			if seen[id] {
				return usageErr(fmt.Errorf("duplicate restaurant ID"))
			}
			seen[id] = true
		}
		s, e := newPlanningSession(f, o)
		if e != nil {
			return e
		}
		ctx, cancel := boundCtx(c.Context(), f)
		defer cancel()
		out := []map[string]any{}
		errs := []map[string]any{}
		partial := false
		for _, id := range args {
			d, e := s.client.Restaurant(ctx, id, o.lang)
			if e != nil {
				errs = append(errs, map[string]any{"id": id, "error": e.Error()})
				partial = true
				continue
			}
			partial = partial || sourceStale(d.Evidence)
			out = append(out, map[string]any{"id": d.ID, "name": d.Name, "name_ja": d.NameJA, "url": d.URL, "courses": d.Courses, "course_notes": d.CourseNotes, "service_charge": d.ServiceCharge, "reservation_fee": d.ReservationFee, "cancellation": d.Cancellation, "cancellation_note": d.CancellationNote, "release": d.Release, "seat_state": d.SeatState, "evidence": d.Evidence})
		}
		if e = s.emit(c, f, out, partial, map[string]any{"errors": errs, "comparison_count": len(out), "requested_count": len(args)}); e != nil {
			return e
		}
		if len(errs) > 0 && !allowPartial {
			return apiErr(fmt.Errorf("OMAKASE comparison incomplete; see per-ID errors (use --allow-partial to accept)"))
		}
		return nil
	}}
	planningFlags(c, &o)
	c.Flags().BoolVar(&allowPartial, "allow-partial", false, "Exit 0 with explicit per-ID errors when some sources fail")
	return c
}
func newFiltersCmd(f *rootFlags) *cobra.Command {
	var o planningOptions
	c := &cobra.Command{Use: "filters", Short: "Read current first-party area and cuisine option values", Annotations: planAnnotation(""), RunE: func(c *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(c.OutOrStdout(), f, "read source filters")
		}
		if len(args) != 0 {
			return usageErr(fmt.Errorf("filters accepts no positional arguments"))
		}
		s, e := newPlanningSession(f, o)
		if e != nil {
			return e
		}
		ctx, cancel := boundCtx(c.Context(), f)
		defer cancel()
		p, ev, e := s.client.Catalogue(ctx, o.lang, 1, "", "", "")
		if e != nil {
			return planningError(e)
		}
		return s.emit(c, f, p.Filters, ev.Stale, map[string]any{"evidence": ev})
	}}
	planningFlags(c, &o)
	return c
}
func newMembershipCmd(f *rootFlags) *cobra.Command {
	var o planningOptions
	c := &cobra.Command{Use: "membership", Short: "Inspect public Premium features, prices and eligibility boundaries", Long: "Reads the first-party English Premium page. This command cannot determine the caller's account eligibility or access Premium seats, calendar or exact date/party search. No registration or purchase is performed.", Annotations: planAnnotation(""), RunE: func(c *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(c.OutOrStdout(), f, "inspect public membership terms")
		}
		if len(args) != 0 {
			return usageErr(fmt.Errorf("membership accepts no positional arguments"))
		}
		o.lang = "en"
		s, e := newPlanningSession(f, o)
		if e != nil {
			return e
		}
		ctx, cancel := boundCtx(c.Context(), f)
		defer cancel()
		m, ev, e := s.client.Premium(ctx)
		if e != nil {
			return planningError(e)
		}
		return s.emit(c, f, m, true, map[string]any{"evidence": ev, "account_eligibility": "unknown"})
	}}
	c.Flags().BoolVar(&o.refresh, "refresh", false, "Read current source instead of cache")
	c.Flags().BoolVar(&o.offline, "offline", false, "Read cached public membership terms")
	return c
}

func inventoryPath() (string, error) {
	p, e := cliutil.DataDir()
	if e != nil {
		return "", e
	}
	return filepath.Join(p, "omakase", "inventory.json"), nil
}
func newPlanningInventoryCmd(f *rootFlags) *cobra.Command {
	status := func(c *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(c.OutOrStdout(), f, "inspect local inventory")
		}
		if len(args) != 0 {
			return usageErr(fmt.Errorf("inventory status accepts no positional arguments"))
		}
		if f.dataSource == "live" {
			return usageErr(fmt.Errorf("inventory status is local; use inventory refresh"))
		}
		path, e := inventoryPath()
		if e != nil {
			return configErr(e)
		}
		in, e := omakase.ReadInventory(path)
		if errors.Is(e, os.ErrNotExist) {
			return planningOutput(c, f, map[string]any{"meta": map[string]any{"source": "local", "requests": 0, "partial": true}, "results": map[string]any{"exists": false, "count": 0, "path": path, "refresh_command": "omakase-pp-cli inventory refresh"}})
		}
		if e != nil {
			return configErr(e)
		}
		return planningOutput(c, f, map[string]any{"meta": map[string]any{"source": "local", "requests": 0, "partial": !in.Complete, "stale": time.Since(in.FetchedAt) > 24*time.Hour}, "results": map[string]any{"exists": true, "count": len(in.Results), "path": path, "fetched_at": in.FetchedAt, "pages": in.Pages, "source_total": in.SourceTotal, "complete": in.Complete, "locale": in.Locale}})
	}
	p := &cobra.Command{Use: "inventory", Short: "Explicit bounded summary refresh and offline inventory lookup", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": ""}, RunE: status}
	p.AddCommand(&cobra.Command{Use: "status", Short: "Inspect local inventory coverage and freshness", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"}, RunE: status})
	var o planningOptions
	pages := 1
	refresh := &cobra.Command{Use: "refresh", Short: "Replace local summary inventory after a successful bounded public read", Annotations: map[string]string{"mcp:local-write": "true", "pp:data-source": "live", "pp:happy-args": "--pages=1"}, RunE: func(c *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(c.OutOrStdout(), f, "refresh local summary inventory")
		}
		if len(args) != 0 || pages < 1 || pages > 25 {
			return usageErr(fmt.Errorf("inventory refresh uses --pages 1..25 and no positional arguments"))
		}
		if f.dataSource == "local" {
			return usageErr(fmt.Errorf("inventory refresh needs live source access"))
		}
		s, e := newPlanningSession(f, o)
		if e != nil {
			return e
		}
		path, e := inventoryPath()
		if e != nil {
			return configErr(e)
		}
		ctx, cancel := boundCtx(c.Context(), f)
		defer cancel()
		in, e := s.client.RefreshInventory(ctx, path, o.lang, pages)
		if e != nil {
			return planningError(e)
		}
		return s.emit(c, f, map[string]any{"count": len(in.Results), "pages": in.Pages, "complete": in.Complete, "source_total": in.SourceTotal, "fetched_at": in.FetchedAt, "path": path}, !in.Complete, nil)
	}}
	refresh.Flags().StringVar(&o.lang, "lang", "en", "Source language en or ja")
	refresh.Flags().IntVar(&pages, "pages", 1, "Source pages to read 1..25; bounded, explicit refresh")
	p.AddCommand(refresh)
	var query string
	limit, offset := 10, 0
	find := &cobra.Command{Use: "find", Short: "Search literal names in the existing local summary inventory", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": ""}, RunE: func(c *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(c.OutOrStdout(), f, "find local inventory names")
		}
		if len(args) != 0 || limit < 1 || limit > 50 || offset < 0 || offset > 800 {
			return usageErr(fmt.Errorf("use --query, --limit 1..50 and --offset 0..800"))
		}
		if f.dataSource == "live" {
			return usageErr(fmt.Errorf("inventory find is local; use restaurants find for live reads"))
		}
		path, e := inventoryPath()
		if e != nil {
			return configErr(e)
		}
		in, e := omakase.ReadInventory(path)
		if errors.Is(e, os.ErrNotExist) {
			return planningOutput(c, f, map[string]any{"meta": map[string]any{"source": "local", "requests": 0, "partial": true, "inventory_exists": false}, "results": []omakase.Summary{}})
		}
		if e != nil {
			return configErr(e)
		}
		matches := []omakase.Summary{}
		for _, r := range in.Results {
			if omakase.NameMatches(r.Name, query) {
				matches = append(matches, r)
			}
		}
		start := offset
		if start > len(matches) {
			start = len(matches)
		}
		end := start + limit
		if end > len(matches) {
			end = len(matches)
		}
		var next *int
		if end < len(matches) {
			v := end
			next = &v
		}
		return planningOutput(c, f, map[string]any{"meta": map[string]any{"source": "local", "requests": 0, "partial": !in.Complete || next != nil, "fetched_at": in.FetchedAt, "stale": time.Since(in.FetchedAt) > 24*time.Hour, "source_total": in.SourceTotal, "inventory_count": len(in.Results), "match_count": len(matches), "next_offset": next}, "results": matches[start:end]})
	}}
	find.Flags().StringVar(&query, "query", "", "Literal name substring")
	find.Flags().IntVar(&limit, "limit", 10, "Maximum returned rows 1..50")
	find.Flags().IntVar(&offset, "offset", 0, "Offset into local matches")
	p.AddCommand(find)
	return p
}
