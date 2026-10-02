package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-guide/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/japan-guide/internal/guide"
	"github.com/spf13/cobra"
)

// pp:data-source live
func init() {
	entries := []whichEntry{
		{Command: "guide destinations", Description: "Find Japan Guide destinations by region and editorial recommendation", Group: "Japan Guide"},
		{Command: "guide attractions", Description: "Find destination attractions by source interest tags", Group: "Japan Guide"},
		{Command: "guide interests", Description: "List source sightseeing interests and canonical links", Group: "Japan Guide"},
		{Command: "guide inspect", Description: "Inspect temple, shrine, museum and garden visit facts: hours, admission, closures and access", Group: "Japan Guide", WhyItMatters: "Read facility-specific visit facts and offline source freshness"},
		{Command: "guide compare", Description: "Compare visit facts from a small shortlist with per-item errors", Group: "Japan Guide"},
		{Command: "guide itineraries", Description: "Discover regional or destination source itinerary links", Group: "Japan Guide"},
		{Command: "guide itinerary", Description: "Read source itinerary stop labels, visit durations and planning qualifiers", Group: "Japan Guide"},
	}
	kept := make([]whichEntry, 0, len(whichIndex)+len(entries))
	for _, entry := range whichIndex {
		if entry.Command != "source" && !strings.HasPrefix(entry.Command, "guide ") {
			kept = append(kept, entry)
		}
	}
	whichIndex = append(kept, entries...)
	registerNovelCommand(func(root *cobra.Command, f *rootFlags) {
		root.AddCommand(newGuideCmd(f))
		for _, cmd := range root.Commands() {
			if cmd.Name() == "source" {
				cmd.Hidden = true
				cmd.Annotations["mcp:hidden"] = "true"
				original := cmd.RunE
				cmd.RunE = func(cmd *cobra.Command, args []string) error {
					if dryRunOK(f) {
						return writeDryRun(cmd.OutOrStdout(), f, "source")
					}
					return original(cmd, args)
				}
			}
		}
	})
}
func newGuideCmd(f *rootFlags) *cobra.Command {
	p := &cobra.Command{Use: "guide", Short: "Find source destinations, attractions and bounded visit facts", Long: "Japan Guide public editorial source only. Facts retain seasonal qualifiers and source dates; open_now is unknown. Schedules refer to Japan local time (JST). No bookings or ticket inventory."}
	p.AddCommand(guideDestinationsCmd(f), guideAttractionsCmd(f), guideInterestsCmd(f), guideItinerariesCmd(f), guideInspectCmd(f), guideCompareCmd(f), guideItineraryCmd(f))
	return p
}
func guideAnnotations(happy string) map[string]string {
	return map[string]string{"pp:data-source": "live", "mcp:read-only": "true", "pp:happy-args": happy}
}
func guideClient(f *rootFlags) *guide.Client {
	c := guide.New()
	c.HTTP.Timeout = f.timeout
	if f.rateLimit >= 0 {
		c.Limiter = cliutil.NewAdaptiveLimiter(f.rateLimit)
	}
	return c
}
func guideErr(err error) error {
	var rate *cliutil.RateLimitError
	if errors.As(err, &rate) {
		return rateLimitErr(err)
	}
	return err
}
func guideOutput(cmd *cobra.Command, f *rootFlags, c *guide.Client, start time.Time, v map[string]any) error {
	c.Metrics.ElapsedMS = time.Since(start).Milliseconds()
	v["metrics"] = c.Metrics
	v["source"] = "Japan Guide"
	if _, exists := v["retrieved_at"]; !exists {
		v["retrieved_at"] = time.Now().UTC().Format(time.RFC3339)
	}
	f.asJSON = true
	if f.compact && !f.csv && !f.plain && !f.quiet {
		var out bytes.Buffer
		outputErr := printJSONFiltered(&out, v, f)
		if out.Len() > 0 {
			var compact bytes.Buffer
			if err := json.Compact(&compact, out.Bytes()); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), compact.String()); err != nil {
				return err
			}
		}
		return outputErr
	}
	return f.printJSON(cmd, v)
}

// pp:data-source live
func guideDestinationsCmd(f *rootFlags) *cobra.Command {
	return guideListCmd(f, "destinations", &cobra.Command{Use: "destinations"})
}

// pp:data-source live
func guideAttractionsCmd(f *rootFlags) *cobra.Command {
	return guideListCmd(f, "attractions", &cobra.Command{Use: "attractions [destination]"})
}

// pp:data-source live
func guideInterestsCmd(f *rootFlags) *cobra.Command {
	return guideListCmd(f, "interests", &cobra.Command{Use: "interests"})
}

// pp:data-source live
func guideItinerariesCmd(f *rootFlags) *cobra.Command {
	return guideListCmd(f, "itineraries", &cobra.Command{Use: "itineraries"})
}

// pp:data-source live
func guideListCmd(f *rootFlags, name string, cmd *cobra.Command) *cobra.Command {
	var limit, offset int
	var region, query, interest, destination string
	var dots int
	short := map[string]string{"destinations": "Find destinations by region and editorial recommendation", "attractions": "Find a destination's attractions and source interest tags", "interests": "List source travel interests and canonical links", "itineraries": "Discover source itinerary links"}[name]
	happy := map[string]string{"destinations": "--region=kanto;--limit=3", "attractions": "destination=e2164;--limit=3", "interests": "--limit=3", "itineraries": "--destination=e2164;--limit=3"}[name]
	cmd.Short = short
	cmd.Example = "  japan-guide-pp-cli guide " + name + map[string]string{"destinations": " --region kanto --limit 5 --agent", "attractions": " e2164 --interest temples --limit 5 --agent", "interests": " --limit 10 --agent", "itineraries": " --destination e2164 --limit 5 --agent"}[name]
	cmd.Annotations = guideAnnotations(happy)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if name == "attractions" && len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(f) {
			return f.printJSON(cmd, map[string]any{"dry_run": true, "operation": name, "max_requests": 1, "limit": limit, "items": []guide.Item{}})
		}
		if f.dataSource == "local" {
			return usageErr(fmt.Errorf("guide %s is a live source read; use guide inspect --offline for cached facts", name))
		}
		if limit < 1 || limit > 50 || offset < 0 || offset > 10000 || dots < 0 || dots > 3 {
			return usageErr(fmt.Errorf("use --limit 1..50, --offset 0..10000 and --min-dots 0..3"))
		}
		if name == "attractions" {
			if len(args) > 1 {
				return usageErr(fmt.Errorf("attractions accepts one destination ID"))
			}
			if len(args) == 1 {
				if destination != "" {
					return usageErr(fmt.Errorf("pass a destination argument or --destination, once"))
				}
				destination = args[0]
			}
			if destination == "" {
				return usageErr(fmt.Errorf("use guide attractions e2164; choose a destination with guide destinations"))
			}
		} else if len(args) > 0 {
			return usageErr(fmt.Errorf("guide %s takes flags only", name))
		}
		if destination != "" {
			if _, err := guide.Canonical(destination); err != nil {
				return usageErr(err)
			}
		}
		c := guideClient(f)
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		start := time.Now()
		var items []guide.Item
		var err error
		switch name {
		case "destinations":
			items, err = c.Destinations(ctx)
		case "attractions":
			items, err = c.Attractions(ctx, destination)
		case "interests":
			items, err = c.Interests(ctx)
		case "itineraries":
			items, err = c.Itineraries(ctx, destination)
		}
		if err != nil {
			return guideErr(err)
		}
		matches := []guide.Item{}
		for _, x := range items {
			if region != "" && !strings.EqualFold(x.Region, region) {
				continue
			}
			if query != "" && !strings.Contains(strings.ToLower(x.Name), strings.ToLower(query)) {
				continue
			}
			if interest != "" {
				found := false
				for _, v := range x.Interests {
					if strings.EqualFold(v, interest) {
						found = true
					}
				}
				if !found {
					continue
				}
			}
			if dots > 0 && (x.Recommendation == nil || x.Recommendation.Dots < dots) {
				continue
			}
			matches = append(matches, x)
		}
		end := min(offset+limit, len(matches))
		begin := min(offset, len(matches))
		next := any(nil)
		if end < len(matches) {
			next = end
		}
		view := map[string]any{"items": matches[begin:end], "total_matches": len(matches), "scanned_records": len(items), "max_source_pages": 1, "offset": offset, "limit": limit, "next_offset": next, "filters": map[string]any{"region": region, "query": query, "interest": interest, "min_editorial_dots": dots}, "scope": "Single source page; editorial dots are not visitor ratings"}
		if len(matches) == 0 {
			view["note"] = "No matching source records on this page; remove filters or choose another destination. No other source pages were scanned."
		}
		return guideOutput(cmd, f, c, start, view)
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum results to return, between 1 and 50")
	cmd.Flags().IntVar(&offset, "offset", 0, "Skip matching records; use next_offset to continue")
	cmd.Flags().StringVar(&query, "query", "", "Case-insensitive name substring, filtered within source records")
	if name == "destinations" {
		cmd.Flags().StringVar(&region, "region", "", "Source region: Hokkaido, Tohoku, Kanto, Chubu, Kansai, Chugoku, Shikoku, Kyushu or Okinawa")
	}
	if name == "attractions" {
		cmd.Flags().StringVar(&interest, "interest", "", "Exact source interest label, e.g. Temples or Gardens")
	}
	if name == "attractions" || name == "itineraries" {
		cmd.Flags().StringVar(&destination, "destination", "", "Source destination ID; e2164 is Tokyo")
	}
	if name == "destinations" || name == "attractions" {
		cmd.Flags().IntVar(&dots, "min-dots", 0, "Minimum Japan Guide editorial recommendation dots, 0..3")
	}
	return cmd
}

// pp:data-source auto
func guideInspectCmd(f *rootFlags) *cobra.Command {
	var cache, offline bool
	var cacheDir string
	cmd := &cobra.Command{Use: "inspect [page]", Short: "Inspect scoped visit facts; optionally save an extracted local snapshot", Example: "  japan-guide-pp-cli guide inspect e3001 --cache --agent\n  japan-guide-pp-cli guide inspect e3001 --offline --agent", Annotations: guideAnnotations("page=e3001"), RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "guide inspect")
		}
		if len(args) != 1 {
			return usageErr(fmt.Errorf("use guide inspect e3001 or a canonical Japan Guide URL"))
		}
		if _, err := guide.Canonical(args[0]); err != nil {
			return usageErr(err)
		}
		if cache && offline {
			return usageErr(fmt.Errorf("choose --cache for a live snapshot or --offline to read it"))
		}
		if f.dataSource == "local" {
			if cache {
				return usageErr(fmt.Errorf("--cache fetches live facts and conflicts with --data-source local"))
			}
			offline = true
		}
		if offline && f.dataSource == "live" {
			return usageErr(fmt.Errorf("--offline conflicts with --data-source live"))
		}
		c := guideClient(f)
		c.Offline = offline
		fallback := f.dataSource == "auto" && !f.noCache
		if cache || offline || fallback {
			if cacheDir == "" {
				base, err := cliutil.CacheDir()
				if err != nil {
					return err
				}
				cacheDir = filepath.Join(base, "guide-facts")
			}
			if cache || offline {
				c.CacheDir = cacheDir
			}
		}
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		start := time.Now()
		d, err := c.Inspect(ctx, args[0])
		liveFailure := ""
		var saveErr *guide.SnapshotWriteError
		if err != nil && fallback && !offline && !errors.As(err, &saveErr) {
			var rate *cliutil.RateLimitError
			if errors.As(err, &rate) {
				return guideErr(err)
			}
			liveFailure = err.Error()
			c.CacheDir = cacheDir
			c.Offline = true
			var cacheErr error
			d, cacheErr = c.Inspect(ctx, args[0])
			if cacheErr != nil {
				return fmt.Errorf("%w; automatic snapshot fallback unavailable: %v", err, cacheErr)
			}
			err = nil
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: live source read failed; returning cached facts retrieved at %s\n", d.RetrievedAt)
		}
		if err != nil {
			return guideErr(err)
		}
		if c.Offline {
			f.agentSource = "local"
			if retrieved, err := time.Parse(time.RFC3339, d.RetrievedAt); err == nil && f.maxAge > 0 && time.Since(retrieved) > f.maxAge {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: cached facts were retrieved at %s; refresh with guide inspect %s --cache when live access is available\n", d.RetrievedAt, d.ID)
			}
		}
		view := map[string]any{"item": d, "data_source": d.Freshness, "retrieved_at": d.RetrievedAt}
		if liveFailure != "" {
			view["live_failure"] = liveFailure
		}
		return guideOutput(cmd, f, c, start, view)
	}}
	cmd.Annotations["pp:data-source"] = "auto"
	cmd.Annotations["pp:method"] = "GET"     // External acquisition is GET; optional local saves remain may-write in MCP.
	delete(cmd.Annotations, "mcp:read-only") // --cache can replace a user-selected snapshot.
	cmd.Flags().BoolVar(&cache, "cache", false, "Save only extracted facts locally for later offline inspection")
	cmd.Flags().BoolVar(&offline, "offline", false, "Read the cached snapshot and preserve original freshness timestamps")
	cmd.Flags().StringVar(&cacheDir, "cache-dir", "", "Override the local extracted-facts cache directory")
	return cmd
}

// pp:data-source live
func guideCompareCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "compare [pages...]", Short: "Compare facts for up to five space-separated page IDs; preserve partial failures", Example: "  japan-guide-pp-cli guide compare e3001 e3002 --agent", Annotations: guideAnnotations("page=e3001;page=e3002"), RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(f) {
			return f.printJSON(cmd, map[string]any{"dry_run": true, "max_requests": 5})
		}
		pages := []string{}
		for _, value := range args {
			pages = append(pages, strings.Fields(value)...)
		}
		args = pages
		if len(args) < 1 || len(args) > 5 {
			return usageErr(fmt.Errorf("compare accepts 1..5 source page IDs"))
		}
		if f.dataSource == "local" {
			return usageErr(fmt.Errorf("compare reads live source facts"))
		}
		for _, a := range args {
			if _, err := guide.Canonical(a); err != nil {
				return usageErr(err)
			}
		}
		c := guideClient(f)
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		start := time.Now()
		items := []guide.Detail{}
		failures := []map[string]string{}
		for _, a := range args {
			d, err := c.Inspect(ctx, a)
			if err != nil {
				var rate *cliutil.RateLimitError
				if errors.As(err, &rate) {
					return guideErr(err)
				}
				failures = append(failures, map[string]string{"id": a, "error": err.Error()})
				continue
			}
			items = append(items, d)
		}
		if len(failures) > 0 {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d source reads failed; %d successful items returned\n", len(failures), len(args), len(items))
		}
		if len(items) == 0 {
			return fmt.Errorf("all comparison source reads failed: %v", failures)
		}
		return guideOutput(cmd, f, c, start, map[string]any{"items": items, "fetch_failures": failures, "partial": len(failures) > 0, "requested": len(args), "successful": len(items)})
	}}
}

// pp:data-source live
func guideItineraryCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "itinerary [page]", Short: "Inspect source itinerary day or stop labels and canonical links", Example: "  japan-guide-pp-cli guide itinerary e2400_kanto --agent", Annotations: guideAnnotations("page=e2400_kanto"), RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(f) {
			return f.printJSON(cmd, map[string]any{"dry_run": true, "max_requests": 1, "max_stops": 30})
		}
		if len(args) != 1 {
			return usageErr(fmt.Errorf("use guide itinerary e2400_kanto; select a source plan with guide itineraries"))
		}
		if _, err := guide.Canonical(args[0]); err != nil {
			return usageErr(err)
		}
		if f.dataSource == "local" {
			return usageErr(fmt.Errorf("itinerary reads live source labels"))
		}
		c := guideClient(f)
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		start := time.Now()
		v, err := c.Itinerary(ctx, args[0])
		if err != nil {
			return guideErr(err)
		}
		return guideOutput(cmd, f, c, start, v)
	}}
}
