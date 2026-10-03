package cli

// pp:data-source auto
import (
	"context"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/kurumatabi/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/kurumatabi/internal/parks"
	"github.com/spf13/cobra"
	"strings"
	"time"
)

var parkClientFactory = parks.NewClient

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		group, _, err := root.Find([]string{"parks"})
		if err != nil || group == root {
			return
		}
		group.Short = "Discover and evaluate official Japanese overnight vehicle parks"
		group.Long = "Search and inspect the RV Park / Kurumatabi ecosystem with per-record evidence. Vehicle overnight stays, outdoor camping permission, dated vacancy and total prices are separate questions."
		addNovelCommandIfAbsent(group, newParkSearchCmd(flags))
		addNovelCommandIfAbsent(group, newParkDetailCmd(flags))
		addNovelCommandIfAbsent(group, newParkFiltersCmd(flags))
		addNovelCommandIfAbsent(group, newParkHandoffCmd(flags))
	})
}
func parkAnnotation(source, happy string) map[string]string {
	return map[string]string{"mcp:read-only": "true", "pp:data-source": source, "pp:happy-args": happy, "pp:typed-exit-codes": "0,2,3,5,7"}
}
func parkError(err error) error {
	var rate *cliutil.RateLimitError
	if errors.As(err, &rate) {
		return rateLimitErr(err)
	}
	return apiErr(err)
}
func parkCacheHint(cmd *cobra.Command, f *rootFlags, db string, ps []parks.Park) {
	if len(ps) == 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "no cached park observations; run parks search or parks detail --db %s\n", db)
		return
	}
	if f.maxAge <= 0 {
		return
	}
	stale := 0
	for _, p := range ps {
		t, e := time.Parse(time.RFC3339, p.ObservedAt)
		if e != nil || time.Since(t) > f.maxAge {
			stale++
		}
	}
	if stale > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d cached observations exceed --max-age; refresh parks detail for current evidence\n", stale)
	}
}
func parkRead(ctx context.Context, cmd *cobra.Command, f *rootFlags, id, db string) (parks.Park, string, error) {
	return parkReadWithClient(ctx, cmd, f, id, db, nil)
}
func parkReadWithClient(ctx context.Context, cmd *cobra.Command, f *rootFlags, id, db string, c *parks.Client) (parks.Park, string, error) {
	id, e := parks.NormalizeID(id)
	if e != nil {
		return parks.Park{}, "", usageErr(e)
	}
	if f.dataSource == "local" {
		ps, e := parks.Load(ctx, db)
		if e != nil {
			return parks.Park{}, "", e
		}
		parkCacheHint(cmd, f, db, ps)
		for _, p := range ps {
			if p.ID == id {
				if p.SourceLevel != "detail" {
					return parks.Park{}, "local", notFoundErr(fmt.Errorf("park %s has only a cached %s observation; run parks detail %s --data-source live to cache per-record conditions", id, p.SourceLevel, id))
				}
				return p, "local", nil
			}
		}
		return parks.Park{}, "local", notFoundErr(fmt.Errorf("park %s is not cached; run parks detail %s --data-source live --db %s", id, id, db))
	}
	if c == nil {
		c = parkClientFactory("", f.rateLimit)
	}
	p, e := c.Detail(ctx, id)
	if e != nil {
		var rate *cliutil.RateLimitError
		if f.dataSource == "auto" && !f.noCache && !errors.As(e, &rate) {
			cached, ce := parks.Load(ctx, db)
			if ce != nil {
				return parks.Park{}, "", ce
			}
			for _, old := range cached {
				if old.ID == id && old.SourceLevel == "detail" {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: live detail failed (%v); returning cached observation from %s\n", e, old.ObservedAt)
					parkCacheHint(cmd, f, db, []parks.Park{old})
					return old, "local", nil
				}
			}
		}
		return parks.Park{}, "live", parkError(e)
	}
	if !f.noCache {
		if e = parks.Save(ctx, db, []parks.Park{p}); e != nil {
			return parks.Park{}, "live", e
		}
	}
	return p, "live", nil
}
func parkEmit(cmd *cobra.Command, f *rootFlags, source string, rows any, note string) error {
	return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"meta": map[string]any{"source": source, "note": note, "observed_pool_only": source == "local", "live_vacancy": "unknown"}, "results": rows}, f)
}
func newParkFiltersCmd(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "filters", Short: "List verified source filter labels and wire values", Example: "  kurumatabi-pp-cli parks filters --json", Annotations: parkAnnotation("computed", ""), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "parks filters")
		}
		if len(args) > 0 {
			return usageErr(fmt.Errorf("parks filters takes no positional arguments"))
		}
		return printJSONFiltered(cmd.OutOrStdout(), parks.Filters(), f)
	}}
	return cmd
}
func newParkDetailCmd(f *rootFlags) *cobra.Command {
	var db string
	cmd := &cobra.Command{Use: "detail [park-id]", Short: "Read published park dimensions, tariffs and facility evidence", Long: "Read a canonical park observation. Auto and live fetch public detail; local reads only the observed SQLite cache. Tariffs are published cases, not dated totals; unknown facilities remain unknown.", Example: "  kurumatabi-pp-cli parks detail rvpark/1086 --agent\n  kurumatabi-pp-cli parks detail yypark/213 --data-source local --json", Annotations: parkAnnotation("auto", "park-id=rvpark/1086"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "parks detail")
		}
		if len(args) == 0 && !hasChangedLocalFlags(cmd) && !f.asJSON && !f.agent {
			return cmd.Help()
		}
		if len(args) != 1 {
			return usageErr(fmt.Errorf("provide one park-id, for example rvpark/1086"))
		}
		db = parkResolveDB(db)
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		p, src, e := parkRead(ctx, cmd, f, args[0], db)
		if e != nil {
			return e
		}
		return parkEmit(cmd, f, src, []parks.Park{p}, "Confirm current opening and vehicle acceptance directly with the host.")
	}}
	cmd.Flags().StringVar(&db, "db", "", "SQLite path for observations; default follows runtime CLI data paths")
	return cmd
}
func newParkSearchCmd(f *rootFlags) *cobra.Command {
	var q parks.Query
	var fs, db string
	cmd := &cobra.Command{Use: "search", Short: "Search bounded provider pages or cached park observations", Long: "Provider search uses the native public POST filter and an ephemeral cookie jar for GET pagination. --limit bounds returned rows; --max-scan-pages bounds provider work. Local mode searches only cached observations and never implies full nationwide coverage.", Example: "  kurumatabi-pp-cli parks search --prefecture nagano --type rvpark --vehicle van --facility electricity,pets --limit 5 --agent\n  kurumatabi-pp-cli parks search --keyword 黒姫 --data-source local --json", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "--prefecture=nagano;--type=rvpark;--limit=3", "pp:typed-exit-codes": "0,2,3,5,7"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "parks search")
		}
		if len(args) > 0 {
			return usageErr(fmt.Errorf("use --keyword for park names or places"))
		}
		q.Facilities = []string{}
		for _, item := range strings.Split(fs, ",") {
			if item = strings.TrimSpace(item); item != "" {
				q.Facilities = append(q.Facilities, item)
			}
		}
		if _, e := parks.QueryValues(q); e != nil {
			return usageErr(e)
		}
		db = parkResolveDB(db)
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		if f.dataSource == "local" {
			ps, e := parks.Load(ctx, db)
			if e != nil {
				return e
			}
			parkCacheHint(cmd, f, db, ps)
			matches := []parks.Park{}
			values, _ := parks.QueryValues(q)
			label, e := parks.VehicleLabel(q.Vehicle)
			if e != nil {
				return usageErr(e)
			}
			typ := map[string]string{"2": "rvpark", "3": "yypark", "4": "camp3000", "5": "campjrva", "7": "gourmet", "8": "minpark", "9": "train", "10": "kurumatabipark"}[values.Get("category[]")]
			pref := q.Prefecture
			for _, x := range parks.Filters().Groups["prefectures"] {
				if x.Value == values.Get("area_pref") {
					pref = x.Label
				}
			}
			period, _ := parks.PeriodLabel(q.Period)
			if strings.HasPrefix(pref, "--") {
				return usageErr(fmt.Errorf("local search requires a prefecture, not a broad region; choose a prefecture from parks filters"))
			}
			for _, p := range ps {
				if period != "" && !containsParkString(p.AvailabilityPeriods, period) {
					continue
				}
				if q.Keyword != "" && !strings.Contains(strings.ToLower(p.Name+" "+p.Address), strings.ToLower(q.Keyword)) {
					continue
				}
				if typ != "" && p.Type != typ {
					continue
				}
				if pref != "" && pref != "全国" && !strings.Contains(p.Address, pref) {
					continue
				}
				if label != "" && !containsParkString(p.Vehicles, label) {
					continue
				}
				ok := true
				for _, k := range q.Facilities {
					key, err := parks.FacilityKey(strings.TrimSpace(k))
					if err != nil {
						return usageErr(err)
					}
					if p.Facilities[key].Status != "yes" {
						ok = false
					}
				}
				if !ok {
					continue
				}
				matches = append(matches, p)
			}
			truncated := len(matches) > q.Limit
			if truncated {
				matches = matches[:q.Limit]
			}
			out := parks.SearchResult{Meta: parks.Meta{Source: "local", SourceURL: parks.Origin + "/park/search.php", ScannedRecords: len(ps), ReturnedRecords: len(matches), OutputTruncated: truncated, Note: "Cached observed pool only; no provider total or current vacancy is known."}, Results: matches}
			return printJSONFiltered(cmd.OutOrStdout(), out, f)
		}
		if cliutil.IsDogfoodEnv() && q.MaxPages > 1 {
			q.MaxPages = 1
		}
		out, e := parkClientFactory("", f.rateLimit).Search(ctx, q)
		if e != nil {
			var rate *cliutil.RateLimitError
			if f.dataSource == "auto" && !f.noCache && !errors.As(e, &rate) {
				cached, ce := parks.Load(ctx, db)
				if ce != nil {
					return ce
				}
				if len(cached) > 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: live search failed (%v); using only cached observed candidates\n", e)
					old := f.dataSource
					oldContext := cmd.Context()
					cmd.SetContext(ctx)
					defer cmd.SetContext(oldContext)
					f.dataSource = "local"
					defer func() { f.dataSource = old }()
					return cmd.RunE(cmd, args)
				}
			}
			return parkError(e)
		}
		if !f.noCache {
			if e = parks.Save(ctx, db, out.Observations); e != nil {
				return e
			}
		}
		return printJSONFiltered(cmd.OutOrStdout(), out, f)
	}}
	cmd.Flags().StringVar(&q.Prefecture, "prefecture", "", "Prefecture wire code or Japanese label; see parks filters")
	cmd.Flags().StringVar(&q.Type, "type", "", "Park type slug, Japanese label or source value")
	cmd.Flags().StringVar(&q.Keyword, "keyword", "", "Facility name or place text, up to 100 characters")
	cmd.Flags().StringVar(&q.Vehicle, "vehicle", "", "Vehicle category such as van, cab, trailer or kei")
	cmd.Flags().StringVar(&fs, "facility", "", "Comma-separated source facility filters, such as electricity,pets")
	cmd.Flags().StringVar(&q.Period, "period", "", "Broad provider opening-period value; not date-specific availability")
	cmd.Flags().IntVar(&q.MaxPages, "max-scan-pages", 1, "Maximum provider pages to scan, 1..5, twenty cards per page")
	cmd.Flags().IntVar(&q.Limit, "limit", 10, "Maximum returned parks, 1..100, separate from scan effort")
	cmd.Flags().StringVar(&db, "db", "", "SQLite path for observations; default follows runtime CLI data paths")
	return cmd
}
func containsParkString(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
func newParkHandoffCmd(f *rootFlags) *cobra.Command {
	var db string
	cmd := &cobra.Command{Use: "handoff [park-id]", Short: "Prepare contact links and host-confirmation questions", Long: "Return public booking/contact links and unresolved confirmation questions. This command does not contact the host or create a reservation.", Example: "  kurumatabi-pp-cli parks handoff rvpark/1086 --agent", Annotations: parkAnnotation("auto", "park-id=rvpark/1086"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "parks handoff")
		}
		if len(args) == 0 && !f.agent && !f.asJSON {
			return cmd.Help()
		}
		if len(args) != 1 {
			return usageErr(fmt.Errorf("provide one park-id for booking preparation"))
		}
		db = parkResolveDB(db)
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		p, src, e := parkRead(ctx, cmd, f, args[0], db)
		if e != nil {
			return e
		}
		questions := []string{"Confirm actual date-specific vacancy, current opening, applicable tariff and all extra fees.", "Confirm acceptance of the actual vehicle, pitch, arrival time and requested outdoor activities."}
		if p.Membership.Status == "unknown" {
			questions = append(questions, "Ask whether nonmembers may use this facility.")
		}
		if p.Dimensions.LengthM == nil || p.Dimensions.WidthM == nil || p.Dimensions.HeightM == nil && !p.Dimensions.HeightUnrestricted {
			questions = append(questions, "Ask for missing vehicle/pitch dimension limits in metres.")
		}
		row := map[string]any{"id": p.ID, "name": p.Name, "url": p.URL, "booking": p.Booking, "membership": p.Membership, "tariffs": p.Tariffs, "opening": p.Sections["利用可能期間"], "observed_at": p.ObservedAt, "source_updated_date_jst": p.SourceUpdated, "questions": questions}
		return parkEmit(cmd, f, src, []map[string]any{row}, "Read-only handoff; no booking or message sent.")
	}}
	cmd.Flags().StringVar(&db, "db", "", "SQLite path for observations; default follows runtime CLI data paths")
	return cmd
}

func parkResolveDB(db string) string {
	if db != "" {
		return db
	}
	return defaultDBPath("kurumatabi-pp-cli")
}
