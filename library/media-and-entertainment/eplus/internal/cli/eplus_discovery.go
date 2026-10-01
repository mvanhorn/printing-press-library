package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/eplus/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/eplus/internal/discovery"
	"github.com/spf13/cobra"
	"path/filepath"
	"strings"
	"time"
)

// init overlays the generated HTML mirrors with domain-aware, bounded reads.
// The generated source and provenance remain intact for reprint comparison.
func init() {
	for _, entry := range []whichEntry{
		{Command: "events search", Description: "Search domestic events by artist, date, venue, region and category"},
		{Command: "events detail", Description: "Inspect domestic performances, lottery deadlines and sale booking links"},
		{Command: "international search", Description: "Search the separate overseas international catalog"},
		{Command: "international detail", Description: "Inspect international product prices and eligibility conditions"},
		{Command: "compare", Description: "Compare performances and sale rounds from up to four public detail IDs"},
		{Command: "policies", Description: "Read current international eligibility, payment and collection guidance"},
	} {
		found := false
		for _, existing := range whichIndex {
			if existing.Command == entry.Command {
				found = true
				break
			}
		}
		if !found {
			whichIndex = append(whichIndex, entry)
		}
	}
	registerNovelCommand(func(root *cobra.Command, f *rootFlags) {
		events, _, _ := root.Find([]string{"events"})
		for _, c := range events.Commands() {
			events.RemoveCommand(c)
		}
		events.AddCommand(newEventsSearchAdapterCmd(f), newDiscoveryDetail(f, false))
		ib := &cobra.Command{Use: "international", Short: "Discover the separate international eplus catalog", Annotations: map[string]string{"mcp:read-only": "true", "pp:parent-group": "true"}, RunE: parentNoSubcommandRunE(f)}
		ib.AddCommand(newInternationalSearchCmd(f), newDiscoveryDetail(f, true))
		root.AddCommand(ib, newPolicies(f), newCompare(f))
		// These generated surfaces serve an archive/general API that this provider does not expose.
		for _, c := range root.Commands() {
			switch c.Name() {
			case "tail", "workflow", "feedback":
				root.RemoveCommand(c)
			}
		}
		pre := root.PersistentPreRunE
		root.PersistentPreRunE = func(c *cobra.Command, a []string) error {
			if f.deliverSpec != "" && f.deliverSpec != "stdout" && !strings.HasPrefix(f.deliverSpec, "file:") {
				return usageErr(fmt.Errorf("read-only eplus discovery supports --deliver stdout or file:<path>"))
			}
			return pre(c, a)
		}
	})
}

type discoveryFlags struct {
	Fresh    bool
	CacheDir string
}

func addDiscoveryFlags(c *cobra.Command, o *discoveryFlags) {
	c.Flags().BoolVar(&o.Fresh, "fresh", false, "Bypass the two-minute public response cache")
	c.Flags().StringVar(&o.CacheDir, "cache-dir", "", "Directory for timestamped public-response cache files")
}
func newDiscoveryClient(f *rootFlags, o discoveryFlags) (*discovery.Client, error) {
	if f.dataSource != "auto" && f.dataSource != "live" {
		return nil, usageErr(fmt.Errorf("public discovery supports --data-source auto or live; no offline mirror is shipped"))
	}
	if f.timeout <= 0 {
		return nil, usageErr(fmt.Errorf("--timeout must be a positive duration"))
	}
	cache := o.CacheDir
	if cache == "" {
		p, e := cliutil.CacheDir()
		if e != nil {
			return nil, e
		}
		cache = filepath.Join(p, "public")
	}
	fresh := o.Fresh || f.noCache || f.dataSource == "live"
	if f.noCache {
		cache = ""
	}
	c := discovery.NewClient(cache, fresh, f.timeout)
	if f.rateLimit > 0 {
		c.SetRateLimit(f.rateLimit)
	}
	return c, nil
}
func discoveryContext(c *cobra.Command, f *rootFlags) (context.Context, context.CancelFunc) {
	ctx, cancel := boundCtx(c.Context(), f)
	budget := 20 * time.Second
	if c.Flags().Changed("timeout") {
		budget = f.timeout
		if budget > 120*time.Second {
			budget = 120 * time.Second
		}
	}
	bounded, done := context.WithTimeout(ctx, budget)
	return bounded, func() { done(); cancel() }
}
func writeDiscovery(c *cobra.Command, f *rootFlags, r discovery.Result, limit int) error {
	if limit > 0 && len(r.Data) > limit {
		r.Data = r.Data[:limit]
		r.Meta["partial"] = true
		r.Meta["note"] = "session output capped; raise --limit to inspect more performances"
	}
	r.Meta["returned"] = len(r.Data)
	if source := r.Meta["source"]; source != nil {
		r.Meta["provider_surface"] = source
	}
	r.Meta["source"] = "live"
	r.Meta["cache_ttl_seconds"] = 120
	if w, ok := r.Meta["warnings"].([]string); ok {
		for _, s := range w {
			if !f.asJSON || strings.Contains(s, "failed") || strings.Contains(s, "changed") {
				fmt.Fprintln(c.ErrOrStderr(), "warning:", s)
			}
		}
	}
	// Domain records already carry only relevant planning facts. Preserve sale windows/terms.
	local := *f
	local.compact = false
	b, e := json.Marshal(map[string]any{"results": r.Data, "meta": r.Meta})
	if e != nil {
		return e
	}
	return printOutputWithFlagsMeta(c.OutOrStdout(), b, &local, map[string]any{"source": "live"})
}
func discoveryError(c *cobra.Command, f *rootFlags, e error) error {
	var rate *cliutil.RateLimitError
	if errors.As(e, &rate) {
		return &cliError{code: 7, err: e}
	}
	return classifyAPIError(c.OutOrStdout(), fmt.Errorf("eplus public discovery: %w; check the source URL, narrow filters or retry with --fresh", e), f)
}
func addSearchFlags(c *cobra.Command, o *discovery.SearchOptions, domestic bool) {
	c.Flags().StringVar(&o.Keyword, "keyword", "", "Event title keyword to send to public search")
	c.Flags().StringVar(&o.Artist, "artist", "", "Artist name matched by the public keyword search")
	c.Flags().StringVar(&o.Venue, "venue", "", "Venue name substring within the bounded scanned records")
	c.Flags().StringVar(&o.Location, "location", "", "Prefecture or venue substring within the scanned records")
	c.Flags().StringVar(&o.From, "from", "", "First service date inclusive, in YYYY-MM-DD JST")
	c.Flags().StringVar(&o.To, "to", "", "Last service date inclusive, in YYYY-MM-DD JST")
	categories := "concert, theatre, sports, event, classical, art, anime, film"
	if !domestic {
		categories = "concert, theatre, art, sports, culture, event, anime, festival"
	}
	c.Flags().StringVar(&o.Category, "category", "", "Category filter: "+categories)
	c.Flags().IntVar(&o.Limit, "limit", 10, "Maximum matching records to return, from 1 to 100")
	if domestic {
		c.Flags().StringVar(&o.Region, "region", "", "Source region: kanto, kansai, tokai, hokkaido-tohoku, hokushinetsu, chugoku-shikoku, kyushu-okinawa, overseas")
		c.Flags().IntVar(&o.Page, "page", 1, "First source page to read, from 1 to 50")
		c.Flags().IntVar(&o.Pages, "pages", 1, "Maximum source pages to scan, from 1 to 5")
	}
}

// pp:data-source live
func newEventsSearchAdapterCmd(f *rootFlags) *cobra.Command {
	var o discovery.SearchOptions
	var d discoveryFlags
	c := &cobra.Command{Use: "search", Short: "Search domestic performances with exact sale rounds", Example: "  eplus-pp-cli events search --artist Radiohead --limit 3 --agent\n  eplus-pp-cli events search --from 2026-11-01 --to 2026-11-07 --region kanto --category theatre --agent", Annotations: map[string]string{"pp:data-source": "live", "mcp:read-only": "true", "pp:method": "GET", "pp:path": "/sf/search", "pp:happy-args": "--artist=Radiohead;--limit=2"}, RunE: func(c *cobra.Command, a []string) error {
		if dryRunOK(f) {
			return writeDryRun(c.OutOrStdout(), f, "public domestic search GET")
		}
		if len(a) != 0 {
			return usageErr(fmt.Errorf("events search accepts flags; use --keyword or --artist"))
		}
		if e := discovery.ValidateSearch(o); e != nil {
			return usageErr(e)
		}
		client, e := newDiscoveryClient(f, d)
		if e != nil {
			return e
		}
		ctx, cancel := discoveryContext(c, f)
		defer cancel()
		r, e := client.DomesticSearch(ctx, o)
		if e != nil {
			return discoveryError(c, f, e)
		}
		return writeDiscovery(c, f, r, o.Limit)
	}}
	addSearchFlags(c, &o, true)
	addDiscoveryFlags(c, &d)
	return c
}

// pp:data-source live
func newInternationalSearchCmd(f *rootFlags) *cobra.Command {
	var o discovery.SearchOptions
	var d discoveryFlags
	var scan int
	c := &cobra.Command{Use: "search", Short: "Search international offerings with bounded detail reads", Example: "  eplus-pp-cli international search --category concert --limit 5 --agent\n  eplus-pp-cli international search --from 2026-11-01 --to 2026-11-30 --max-scan 3 --agent", Annotations: map[string]string{"pp:data-source": "live", "mcp:read-only": "true", "pp:happy-args": "--category=concert;--limit=2"}, RunE: func(c *cobra.Command, a []string) error {
		if dryRunOK(f) {
			return writeDryRun(c.OutOrStdout(), f, "public international catalog GET")
		}
		if len(a) != 0 {
			return usageErr(fmt.Errorf("international search accepts flags; use --keyword or --artist"))
		}
		if e := discovery.ValidateInternationalSearch(o, scan); e != nil {
			return usageErr(e)
		}
		client, e := newDiscoveryClient(f, d)
		if e != nil {
			return e
		}
		ctx, cancel := discoveryContext(c, f)
		defer cancel()
		r, e := client.InternationalSearch(ctx, o, scan)
		if e != nil {
			return discoveryError(c, f, e)
		}
		return writeDiscovery(c, f, r, o.Limit)
	}}
	addSearchFlags(c, &o, false)
	addDiscoveryFlags(c, &d)
	c.Flags().IntVar(&scan, "max-scan", 5, "Maximum catalog tours to examine, from 1 to 20")
	return c
}

// pp:data-source live
func newDiscoveryDetail(f *rootFlags, ib bool) *cobra.Command {
	var d discoveryFlags
	var limit int
	scope, example := "events", "4592490001-P0030001P021003"
	if ib {
		scope = "international"
		example = "7078"
	}
	c := &cobra.Command{Use: "detail [id]", Short: "Inspect performances, deadlines, terms and booking links", Example: "  eplus-pp-cli " + scope + " detail " + example + " --agent", Annotations: map[string]string{"pp:data-source": "live", "mcp:read-only": "true", "pp:happy-args": "id=" + example}, RunE: func(c *cobra.Command, a []string) error {
		if dryRunOK(f) {
			return writeDryRun(c.OutOrStdout(), f, "public "+scope+" detail GET")
		}
		if len(a) != 1 {
			return usageErr(fmt.Errorf("%s detail requires one source ID or public detail URL", scope))
		}
		if limit < 1 || limit > 100 {
			return usageErr(fmt.Errorf("--limit must be 1..100"))
		}
		var e error
		if ib {
			_, e = discovery.InternationalURL(a[0])
		} else {
			_, e = discovery.DomesticURL(a[0])
		}
		if e != nil {
			return usageErr(e)
		}
		client, e := newDiscoveryClient(f, d)
		if e != nil {
			return e
		}
		ctx, cancel := discoveryContext(c, f)
		defer cancel()
		var r discovery.Result
		if ib {
			r, e = client.InternationalDetail(ctx, a[0])
		} else {
			r, e = client.DomesticDetail(ctx, a[0])
		}
		if e != nil {
			return discoveryError(c, f, e)
		}
		return writeDiscovery(c, f, r, limit)
	}}
	addDiscoveryFlags(c, &d)
	c.Flags().IntVar(&limit, "limit", 20, "Maximum performance sessions to return, from 1 to 100")
	return c
}

// pp:data-source live
func newPolicies(f *rootFlags) *cobra.Command {
	var d discoveryFlags
	c := &cobra.Command{Use: "policies", Short: "Read current international eligibility and collection guidance", Example: "  eplus-pp-cli policies --agent", Annotations: map[string]string{"pp:data-source": "live", "mcp:read-only": "true"}, RunE: func(c *cobra.Command, a []string) error {
		if dryRunOK(f) {
			return writeDryRun(c.OutOrStdout(), f, "public international FAQ GET")
		}
		if len(a) > 0 {
			return usageErr(fmt.Errorf("policies accepts no positional arguments"))
		}
		client, e := newDiscoveryClient(f, d)
		if e != nil {
			return e
		}
		ctx, cancel := discoveryContext(c, f)
		defer cancel()
		r, e := client.Policies(ctx)
		if e != nil {
			return discoveryError(c, f, e)
		}
		return writeDiscovery(c, f, r, 0)
	}}
	addDiscoveryFlags(c, &d)
	return c
}

// pp:data-source live
func newCompare(f *rootFlags) *cobra.Command {
	var d discoveryFlags
	var source string
	var limit int
	c := &cobra.Command{Use: "compare [id...]", Short: "Compare sessions and sale rounds for two to four source IDs", Example: "  eplus-pp-cli compare 4592490001-P0030001P021003 4592490001-P0030001P021005 --agent\n  eplus-pp-cli compare 7078 7079 --source international --agent", Annotations: map[string]string{"pp:data-source": "live", "mcp:read-only": "true", "pp:happy-args": "first=7078;second=7079;--source=international"}, RunE: func(c *cobra.Command, a []string) error {
		if dryRunOK(f) {
			return writeDryRun(c.OutOrStdout(), f, "bounded public detail comparison GETs")
		}
		if len(a) < 2 || len(a) > 4 {
			return usageErr(fmt.Errorf("compare requires two to four public detail IDs"))
		}
		if source != "domestic" && source != "international" {
			return usageErr(fmt.Errorf("--source must be domestic or international"))
		}
		if limit < 1 || limit > 100 {
			return usageErr(fmt.Errorf("--limit must be 1..100"))
		}
		if limit < len(a) {
			return usageErr(fmt.Errorf("--limit must be at least the number of comparison inputs (%d)", len(a)))
		}
		for _, id := range a {
			var e error
			if source == "domestic" {
				_, e = discovery.DomesticURL(id)
			} else {
				_, e = discovery.InternationalURL(id)
			}
			if e != nil {
				return usageErr(e)
			}
		}
		client, e := newDiscoveryClient(f, d)
		if e != nil {
			return e
		}
		ctx, cancel := discoveryContext(c, f)
		defer cancel()
		groups := [][]discovery.Row{}
		partial := false
		obs := []discovery.Observation{}
		failures := []discovery.Row{}
		warnings := []string{}
		for _, id := range a {
			var r discovery.Result
			var err error
			if source == "domestic" {
				r, err = client.DomesticDetail(ctx, id)
			} else {
				r, err = client.InternationalDetail(ctx, id)
			}
			if err != nil {
				var rate *cliutil.RateLimitError
				if errors.As(err, &rate) {
					return discoveryError(c, f, err)
				}
				failures = append(failures, discovery.Row{"input_id": id, "error": err.Error()})
				continue
			}
			for _, row := range r.Data {
				row["input_id"] = id
			}
			groups = append(groups, r.Data)
			partial = partial || r.Meta["partial"] == true
			obs = append(obs, r.Meta["observations"].([]discovery.Observation)...)
			warnings = append(warnings, r.Meta["warnings"].([]string)...)
		}
		if len(failures) == len(a) {
			return discoveryError(c, f, fmt.Errorf("all comparison reads failed: %s", failures[0]["error"]))
		}
		if len(failures) > 0 {
			fmt.Fprintf(c.ErrOrStderr(), "warning: %d of %d comparison reads failed; %d succeeded\n", len(failures), len(a), len(a)-len(failures))
		}
		rows, truncated := balancedComparisonRows(groups, limit)
		meta := discovery.Row{"source": source, "partial": partial || truncated || len(failures) > 0, "fetch_failures": failures, "observations": obs, "stats": client.Stats(), "warnings": warnings}
		if truncated {
			meta["note"] = "comparison session output capped; raise --limit to inspect more performances"
		}
		return writeDiscovery(c, f, discovery.Result{Data: rows, Meta: meta}, limit)
	}}
	addDiscoveryFlags(c, &d)
	c.Flags().StringVar(&source, "source", "domestic", "Provider surface to compare: domestic or international")
	c.Flags().IntVar(&limit, "limit", 20, "Maximum combined sessions, at least the number of inputs (maximum 100)")
	return c
}

// balancedComparisonRows gives each successful input a turn before taking more sessions.
func balancedComparisonRows(groups [][]discovery.Row, limit int) ([]discovery.Row, bool) {
	rows := []discovery.Row{}
	total := 0
	for _, group := range groups {
		total += len(group)
	}
	for index := 0; len(rows) < limit; index++ {
		added := false
		for _, group := range groups {
			if index < len(group) && len(rows) < limit {
				rows = append(rows, group[index])
				added = true
			}
		}
		if !added {
			break
		}
	}
	return rows, len(rows) < total
}

func newEventsDetailAdapterCmd(f *rootFlags) *cobra.Command { return newDiscoveryDetail(f, false) }
