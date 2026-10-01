package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/tokyo-art-beat/internal/tab"
	"github.com/spf13/cobra"
)

type tabFlags struct {
	cacheDir, fields string
	fresh, offline   bool
}

func init() { registerNovelCommand(registerTAB) }
func registerTAB(rootCmd *cobra.Command, f *rootFlags) {
	if registeredPlatformSource != nil {
		return
	}
	// Keep generated source intact; expose only the focused read-only product.
	for _, c := range rootCmd.Commands() {
		switch c.Name() {
		case "doctor", "agent-context", "version", "source", "which":
		default:
			rootCmd.RemoveCommand(c)
		}
	}
	for _, c := range rootCmd.Commands() {
		if c.Name() == "source" {
			c.Annotations["pp:happy-args"] = "--limit=2"
			c.Hidden = true
			c.RunE = func(cmd *cobra.Command, args []string) error {
				return tabRun(cmd, f, &tabFlags{}, func(ctx context.Context, c *tab.Client) (tab.Result, error) {
					limit, _ := cmd.Flags().GetInt("limit")
					include, _ := cmd.Flags().GetInt("include")
					typ, _ := cmd.Flags().GetString("content-type")
					return c.Source(ctx, typ, limit, include)
				})
			}
		}
	}
	rootCmd.Short = "Discover exhibitions, galleries and museums through Tokyo Art Beat"
	rootCmd.Long = "Read public Tokyo Art Beat exhibition and venue information. Date spans do not establish open days or ticket inventory. Japanese and English source fields are returned together. Start with events search or catalogs areas."
	rootCmd.SilenceErrors = true
	for _, name := range []string{"deliver", "profile", "client-profile", "receipt", "receipt-file", "audit-dir", "yes", "human-friendly", "config", "max-age", "rate-limit"} {
		_ = rootCmd.PersistentFlags().MarkHidden(name)
	}
	original := rootCmd.PersistentPreRunE
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Changed("rate-limit") {
			return tabError(cmd, tab.Fail("unsupported_option", "Request pacing is fixed at at most two per second for this public provider", 2))
		}
		if f.deliverSpec != "" && f.deliverSpec != "stdout" {
			return tabError(cmd, tab.Fail("unsupported_delivery", "This read-only CLI emits stdout; --deliver file/webhook is disabled", 2))
		}
		if f.runProfileName != "" || f.clientProfileName != "" || f.receiptEnabled || f.receiptFile != "" || f.auditDir != "" {
			return tabError(cmd, tab.Fail("unsupported_option", "Profiles and receipt writes are outside the focused read-only CLI; use --cache-dir", 2))
		}
		if f.timeout <= 0 || f.timeout > 2*time.Minute {
			return tabError(cmd, tab.Fail("invalid_timeout", "--timeout must be greater than zero and at most 2m", 2))
		}
		if original != nil {
			if err := original(cmd, args); err != nil {
				return tabError(cmd, tab.Fail("invalid_option", err.Error(), 2))
			}
		}
		return nil
	}
	t := &tabFlags{}
	rootCmd.PersistentFlags().StringVar(&t.cacheDir, "cache-dir", "", "Private response cache directory (default user cache)")
	rootCmd.PersistentFlags().BoolVar(&t.fresh, "fresh", false, "Bypass cached reads and refresh public source data")
	rootCmd.PersistentFlags().BoolVar(&t.offline, "offline", false, "Read exact cached queries only; report stale data")
	rootCmd.PersistentFlags().StringVar(&t.fields, "fields", "", "Comma-separated result fields, preserving metadata (id,name,starts)")
	rootCmd.PersistentFlags().Lookup("data-source").Usage = "Read strategy: auto uses TTL cache; live refreshes; local uses exact cached queries"
	rootCmd.PersistentFlags().Lookup("compact").Usage = "Use compact JSON; domain identity and freshness remain present"
	rootCmd.AddCommand(newTABEventsCmd(f, t))
	rootCmd.AddCommand(newTABVenuesCmd(f, t))
	rootCmd.AddCommand(newTABCatalogsCmd(f, t))
	rootCmd.AddCommand(newTABNearbyCmd(f, t))
	rootCmd.AddCommand(newTABCompareCmd(f, t))
}

// pp:data-source auto
func tabRun(cmd *cobra.Command, f *rootFlags, t *tabFlags, fn func(context.Context, *tab.Client) (tab.Result, error)) error {
	preview := dryRunOK(f)
	if t.fresh && t.offline {
		return tabError(cmd, tab.Fail("conflicting_options", "--fresh and --offline cannot be combined", 2))
	}
	offline := t.offline
	fresh := t.fresh
	if f.dataSource == "local" {
		offline = true
	}
	if f.dataSource == "live" {
		fresh = true
	}
	if offline && (fresh || f.noCache) {
		return tabError(cmd, tab.Fail("conflicting_options", "Offline/local reads cannot combine with --fresh, --no-cache or --data-source live", 2))
	}
	if f.selectFields != "" && (f.csv || f.plain || f.quiet) {
		return tabError(cmd, tab.Fail("invalid_select", "--select projects the JSON envelope; use --fields with CSV/plain/quiet output", 2))
	}
	cacheDir := t.cacheDir
	if cacheDir == "" && f.homePath != "" {
		cacheDir = filepath.Join(f.homePath, "cache", "tab")
	}
	ctx, cancel := boundCtx(cmd.Context(), f)
	defer cancel()
	c := tab.NewClient(tab.Options{ValidateOnly: preview, CacheDir: cacheDir, Fresh: fresh, Offline: offline, NoCache: f.noCache, Timeout: min(f.timeout, 15*time.Second)})
	r, err := fn(ctx, c)
	if preview {
		if err != nil {
			e := tab.Classify(err)
			if e.Code == "compare_failed" {
				for _, failure := range e.Failures {
					if failure.Code != "dry_run" {
						return tabError(cmd, failure)
					}
				}
			} else if e.Code != "dry_run" {
				return tabError(cmd, err)
			}
		}
		return writeDryRun(cmd.OutOrStdout(), f, cmd.CommandPath())
	}
	if err != nil {
		return tabError(cmd, err)
	}
	if len(r.Errors) > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d source fetch or resolution failures; inspect errors and meta.partial\n", len(r.Errors))
	}
	if t.fields != "" {
		raw, er := json.Marshal(r.Results)
		if er != nil {
			return er
		}
		raw, er = filterFieldsChecked(raw, t.fields)
		if er != nil {
			return tabError(cmd, tab.Fail("invalid_fields", er.Error(), 2))
		}
		r.Results = json.RawMessage(raw)
	}
	// Domain cards are already bounded. Generic compact projection would drop dates/hours.
	copy := *f
	copy.compact = false
	if copy.csv || copy.plain || copy.quiet {
		return printJSONFiltered(cmd.OutOrStdout(), r.Results, &copy)
	}
	// Compact JSON is the default in every surface, including a terminal.
	if copy.selectFields != "" {
		raw, er := json.Marshal(r)
		if er != nil {
			return er
		}
		projected, er := filterFieldsChecked(raw, copy.selectFields)
		if er != nil {
			return tabError(cmd, tab.Fail("invalid_select", er.Error(), 2))
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(json.RawMessage(projected))
	}
	return json.NewEncoder(cmd.OutOrStdout()).Encode(r)
}
func tabError(cmd *cobra.Command, err error) error {
	e := tab.Classify(err)
	_ = json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{"error": e})
	fmt.Fprintln(cmd.ErrOrStderr(), e.Message)
	return &cliError{code: e.Exit, err: e}
}
func noTABArgs(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return tab.Fail("invalid_arguments", "Unexpected positional arguments; use named filters from --help", 2)
	}
	return nil
}
func tabSearchFlags(cmd *cobra.Command, s *tab.Search) {
	cmd.Flags().StringVar(&s.Query, "query", "", "Full-text source search across exhibition content")
	cmd.Flags().StringVar(&s.Artist, "artist", "", "Match source artist text in either supplied language")
	cmd.Flags().StringVar(&s.Area, "area", "", "Exact area/prefecture name or catalogs areas source ID")
	cmd.Flags().StringVar(&s.Category, "category", "", "Exact category name or catalogs categories source ID")
	cmd.Flags().StringVar(&s.Venue, "venue", "", "Venue source ID or an unambiguous venue name")
	cmd.Flags().StringVar(&s.From, "from", "", "Inclusive first trip date YYYY-MM-DD, including year")
	cmd.Flags().StringVar(&s.To, "to", "", "Inclusive last trip date; defaults to --from")
	cmd.Flags().StringVar(&s.Relation, "relation", "overlap", "Date-span relation: overlap, starts or ends")
	cmd.Flags().StringVar(&s.Status, "status", "", "Relative span filter: active, upcoming, past or all (default not ended)")
	cmd.Flags().StringVar(&s.Sort, "sort", "starts", "Source ordering: starts, ends or updated; ID tiebreak within each page")
	cmd.Flags().IntVar(&s.Limit, "limit", 10, "Maximum returned exhibitions, between 1 and 50")
	cmd.Flags().IntVar(&s.MaxScanPages, "max-scan-pages", 3, "Maximum source pages for local artist filtering, between 1 and 5")
	cmd.Flags().IntVar(&s.Offset, "offset", 0, "Source pagination offset; use meta.pagination.next_offset")
}
func newTABEventsCmd(f *rootFlags, t *tabFlags) *cobra.Command {
	group := &cobra.Command{Use: "events", Short: "Search and inspect exhibition editions with bilingual source identities"}
	group.AddCommand(newTABEventsSearchCmd(f, t))
	group.AddCommand(newTABEventsDetailCmd(f, t))
	return group
}
func newTABEventsSearchCmd(f *rootFlags, t *tabFlags) *cobra.Command {
	var s tab.Search
	search := &cobra.Command{Use: "search", Short: "Find exhibitions by geography, dates, category, artist or venue", Example: "  tokyo-art-beat-pp-cli events search --area Roppongi --from 2026-10-01 --to 2026-10-07 --limit 5 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "--limit=2"}, RunE: func(cmd *cobra.Command, args []string) error {
		return tabRun(cmd, f, t, func(ctx context.Context, c *tab.Client) (tab.Result, error) {
			if err := noTABArgs(cmd, args); err != nil {
				return tab.Result{}, err
			}
			return c.SearchEvents(ctx, s)
		})
	}}
	tabSearchFlags(search, &s)
	return search
}
func newTABEventsDetailCmd(f *rootFlags, t *tabFlags) *cobra.Command {
	var on string
	detail := &cobra.Command{Use: "detail [source-id-or-url]", Short: "Inspect one edition, separate event/venue hours and assess a date", Example: "  tokyo-art-beat-pp-cli events detail 2ccd6619-6e5e-410c-851a-56bf5d4662dc --on 2027-05-03 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "event=2ccd6619-6e5e-410c-851a-56bf5d4662dc"}, RunE: func(cmd *cobra.Command, args []string) error {
		return tabRun(cmd, f, t, func(ctx context.Context, c *tab.Client) (tab.Result, error) {
			if len(args) != 1 {
				return tab.Result{}, tab.Fail("missing_identity", "events detail requires one exhibition source ID or URL", 2)
			}
			return c.EventDetail(ctx, args[0], on)
		})
	}}
	detail.Flags().StringVar(&on, "on", "", "Assess a JST date YYYY-MM-DD without claiming actual opening")
	return detail
}
func newTABVenuesCmd(f *rootFlags, t *tabFlags) *cobra.Command {
	group := &cobra.Command{Use: "venues", Short: "Discover galleries and museums, inspect defaults and exhibition schedules"}
	var query, area, typ string
	var limit, offset int
	search := &cobra.Command{Use: "search", Short: "Search gallery and museum locations by area and venue type", Example: "  tokyo-art-beat-pp-cli venues search --query Mori --limit 5 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "--query=Mori;--limit=2"}, RunE: func(cmd *cobra.Command, args []string) error {
		return tabRun(cmd, f, t, func(ctx context.Context, c *tab.Client) (tab.Result, error) {
			if err := noTABArgs(cmd, args); err != nil {
				return tab.Result{}, err
			}
			return c.SearchVenues(ctx, query, area, typ, limit, offset)
		})
	}}
	search.Flags().StringVar(&query, "query", "", "Full-text source search across venue content")
	search.Flags().StringVar(&area, "area", "", "Exact area name or catalogs areas source ID")
	search.Flags().StringVar(&typ, "type", "", "Exact venue type name or catalogs types source ID")
	search.Flags().IntVar(&limit, "limit", 10, "Maximum returned venues, between 1 and 50")
	search.Flags().IntVar(&offset, "offset", 0, "Source pagination offset; use meta.pagination.next_offset")
	detail := &cobra.Command{Use: "detail [source-id-or-url]", Short: "Inspect venue location, official links, fees and default hours", Example: "  tokyo-art-beat-pp-cli venues detail import_venue_record__61183FDF --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "venue=import_venue_record__61183FDF"}, RunE: func(cmd *cobra.Command, args []string) error {
		return tabRun(cmd, f, t, func(ctx context.Context, c *tab.Client) (tab.Result, error) {
			if len(args) != 1 {
				return tab.Result{}, tab.Fail("missing_identity", "venues detail requires one venue source ID or URL", 2)
			}
			return c.VenueDetail(ctx, args[0])
		})
	}}
	var s tab.Search
	events := &cobra.Command{Use: "events [source-id-or-name]", Short: "List exhibition editions linked to an exact venue", Example: "  tokyo-art-beat-pp-cli venues events import_venue_record__61183FDF --status all --limit 5 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "venue=import_venue_record__61183FDF;--limit=2"}, RunE: func(cmd *cobra.Command, args []string) error {
		return tabRun(cmd, f, t, func(ctx context.Context, c *tab.Client) (tab.Result, error) {
			if len(args) != 1 {
				return tab.Result{}, tab.Fail("missing_identity", "venues events requires one venue source ID or unambiguous name", 2)
			}
			copy := s
			copy.Venue = args[0]
			return c.SearchEvents(ctx, copy)
		})
	}}
	tabSearchFlags(events, &s)
	_ = events.Flags().MarkHidden("venue")
	group.AddCommand(search, detail, events)
	return group
}
func newTABCatalogsCmd(f *rootFlags, t *tabFlags) *cobra.Command {
	group := &cobra.Command{Use: "catalogs", Short: "Resolve area, category and venue type names to stable source IDs"}
	for _, kind := range []string{"areas", "categories", "types"} {
		var query string
		var limit, offset int
		cmd := &cobra.Command{Use: kind, Short: "List bilingual " + kind + " source names and IDs", Example: "  tokyo-art-beat-pp-cli catalogs " + kind + " --limit 10 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "--limit=5"}, RunE: func(cmd *cobra.Command, args []string) error {
			return tabRun(cmd, f, t, func(ctx context.Context, c *tab.Client) (tab.Result, error) {
				if err := noTABArgs(cmd, args); err != nil {
					return tab.Result{}, err
				}
				return c.Catalog(ctx, kind, query, limit, offset)
			})
		}}
		cmd.Flags().StringVar(&query, "query", "", "Match source catalog names using full-text search")
		cmd.Flags().IntVar(&limit, "limit", 100, "Maximum catalog rows, between 1 and 300")
		cmd.Flags().IntVar(&offset, "offset", 0, "Source pagination offset for the finite catalog")
		group.AddCommand(cmd)
	}
	return group
}

// pp:data-source auto
func newTABNearbyCmd(f *rootFlags, t *tabFlags) *cobra.Command {
	var lat, lon, radius float64
	var candidates int
	var s tab.Search
	cmd := &cobra.Command{Use: "nearby", Short: "Rank a bounded exhibition shortlist by straight-line venue distance", Example: "  tokyo-art-beat-pp-cli nearby --lat 35.6605 --lon 139.7292 --radius-km 2 --limit 5 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "--lat=35.6605;--lon=139.7292;--radius-km=2;--limit=2;--candidates=100"}, RunE: func(cmd *cobra.Command, args []string) error {
		return tabRun(cmd, f, t, func(ctx context.Context, c *tab.Client) (tab.Result, error) {
			if err := noTABArgs(cmd, args); err != nil {
				return tab.Result{}, err
			}
			if !cmd.Flags().Changed("lat") || !cmd.Flags().Changed("lon") {
				return tab.Result{}, tab.Fail("missing_coordinates", "nearby requires both --lat and --lon", 2)
			}
			return c.Nearby(ctx, tab.Geo{Lat: lat, Lon: lon}, radius, candidates, s)
		})
	}}
	tabSearchFlags(cmd, &s)
	cmd.Flags().Float64Var(&lat, "lat", 0, "Center latitude in decimal degrees, required with --lon")
	cmd.Flags().Float64Var(&lon, "lon", 0, "Center longitude in decimal degrees, required with --lat")
	cmd.Flags().Float64Var(&radius, "radius-km", 2, "Straight-line search radius in kilometers, greater than 0 and at most 50")
	cmd.Flags().IntVar(&candidates, "candidates", 100, "Maximum source venue candidates examined, between 1 and 100")
	return cmd
}

// pp:data-source auto
func newTABCompareCmd(f *rootFlags, t *tabFlags) *cobra.Command {
	var on string
	cmd := &cobra.Command{Use: "compare [source-id-or-url ...]", Short: "Compare 2 to 4 exhibition editions, retaining fees, hours and source links", Example: "  tokyo-art-beat-pp-cli compare 2ccd6619-6e5e-410c-851a-56bf5d4662dc import_event_record__2004_41B6 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "event1=2ccd6619-6e5e-410c-851a-56bf5d4662dc;event2=import_event_record__2004_41B6"}, RunE: func(cmd *cobra.Command, args []string) error {
		return tabRun(cmd, f, t, func(ctx context.Context, c *tab.Client) (tab.Result, error) { return c.Compare(ctx, args, on) })
	}}
	cmd.Flags().StringVar(&on, "on", "", "Assess the same JST date YYYY-MM-DD for each exhibition")
	return cmd
}

func tabUsageError(cmd *cobra.Command, err error) error {
	return tabError(cmd, tab.Fail("invalid_arguments", err.Error()+"; run the command with --help for valid arguments", 2))
}
