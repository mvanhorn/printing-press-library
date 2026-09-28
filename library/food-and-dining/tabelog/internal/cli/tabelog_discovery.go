// pp:data-source live
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tabelog/internal/domain"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tabelog/internal/notebook"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tabelog/internal/source"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		root.AddCommand(newTabelogFindCmd(flags), newTabelogShowCmd(flags), newTabelogAreasCmd(flags), newTabelogCuisinesCmd(flags))
	})
}

func newTabelogFindCmd(flags *rootFlags) *cobra.Command {
	o := source.FindOptions{Limit: 5, MaxPages: 1}
	cmd := &cobra.Command{Use: "find", Short: "Find ranked restaurants within explicit location and meal criteria", Example: "  tabelog-pp-cli find --area tokyo --cuisine bar --limit 5 --agent\n  tabelog-pp-cli find --area https://tabelog.com/en/tokyo/A1301/A130101/rstLst/ --meal lunch --budget-max 2000", Annotations: tripReadAnnotations("--area=tokyo;--cuisine=bar;--limit=5")}
	cmd.Long = cmd.Short + fmt.Sprintf(".\n\nSupported JPY budget thresholds: %v. Zero disables a bound. Budget filters require --meal lunch or dinner and refer to source average-price brackets.", source.BudgetThresholds())
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return usageErr(fmt.Errorf("find accepts criteria as flags; use --area LOCATION"))
		}
		check := o
		if flags.dryRun && check.Area == "" {
			check.Area = "planned-required-area"
		}
		if e := check.Validate(); e != nil {
			return tripError(e)
		}
		if dryRunOK(flags) {
			return tripPlan(cmd, flags, "find", map[string]any{"area": o.Area, "cuisine": o.Cuisine, "meal": o.Meal, "budget_min": o.BudgetMin, "budget_max": o.BudgetMax, "keyword": o.Keyword, "limit": o.Limit, "max_pages": o.MaxPages})
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		c, e := tripClient(flags)
		if e != nil {
			return tripError(e)
		}
		items, all, meta, e := c.Find(ctx, o)
		if e != nil {
			return tripSourceFailure(cmd, flags, e)
		}
		db, nb, e := tripOpen(ctx, flags)
		if e != nil {
			return e
		}
		defer db.Close()
		snapshots := map[string]json.RawMessage{}
		for _, r := range all {
			old, e := tripSnapshot(ctx, nb, r.ID)
			if e == nil && old.Surface == "detail" {
				continue
			}
			if e != nil && !errors.Is(e, notebook.ErrSnapshotNotFound) {
				return e
			}
			raw, e := json.Marshal(r)
			if e != nil {
				return e
			}
			snapshots[r.ID] = raw
		}
		if len(snapshots) > 0 {
			if e = nb.ReplaceSnapshots(ctx, snapshots); e != nil {
				return e
			}
		}
		return tripPrint(cmd, flags, items, meta)
	}
	cmd.Flags().StringVar(&o.Area, "area", "", "Required prefecture, typed location selector, or English area URL")
	cmd.Flags().StringVar(&o.Cuisine, "cuisine", "", "Verified cuisine slug or taxonomy choice (bar is broad category)")
	cmd.Flags().StringVar(&o.Meal, "meal", "", "Source average-price meal: lunch or dinner")
	cmd.Flags().IntVar(&o.BudgetMin, "budget-min", 0, "Source-supported minimum average-price threshold in JPY")
	cmd.Flags().IntVar(&o.BudgetMax, "budget-max", 0, "Source-supported maximum average-price threshold in JPY")
	cmd.Flags().StringVar(&o.Keyword, "keyword", "", "Free text within the verified source geography")
	cmd.Flags().IntVar(&o.Limit, "limit", 5, "Maximum candidates to return (1–50); no detail fan-out")
	cmd.Flags().IntVar(&o.MaxPages, "max-pages", 1, "Maximum source pages to fetch (1–5), independently of result limit")
	return cmd
}

var numericID = regexp.MustCompile(`^[0-9]{7,10}$`)

func newTabelogShowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "show [URL_OR_CACHED_ID]", Short: "Inspect one restaurant's source facts and practical details", Example: "  tabelog-pp-cli show https://tabelog.com/en/tokyo/A1301/A130103/13294162/ --agent\n  tabelog-pp-cli show 13294162 --data-source local", Annotations: tripReadAnnotations("restaurant=https://tabelog.com/en/tokyo/A1301/A130103/13294162/")}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) > 1 {
			return usageErr(fmt.Errorf("show accepts one URL or previously fetched restaurant ID"))
		}
		input := ""
		if len(args) > 0 {
			input = args[0]
		}
		canonical, id := "", input
		if input != "" && !numericID.MatchString(input) {
			var e error
			canonical, id, _, e = source.RestaurantURL(input)
			if e != nil {
				return tripError(e)
			}
		}
		if dryRunOK(flags) {
			return tripPlan(cmd, flags, "show", map[string]any{"restaurant": input, "detail_url": canonical, "cached_id_lookup": canonical == ""})
		}
		if input == "" {
			return usageErr(fmt.Errorf("show requires a canonical English URL or an already fetched ID"))
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		db, nb, e := tripOpen(ctx, flags)
		if e != nil {
			return e
		}
		defer db.Close()
		saved, foundErr := tripSnapshot(ctx, nb, id)
		if foundErr != nil && !errors.Is(foundErr, notebook.ErrSnapshotNotFound) {
			return foundErr
		}
		if canonical == "" {
			if foundErr != nil {
				return notFoundErr(fmt.Errorf("restaurant ID%s has not been fetched; use show with its canonical English URL first", id))
			}
			canonical = saved.URL
		}
		if foundErr == nil && saved.URL != canonical && flags.dataSource == "local" {
			return notFoundErr(fmt.Errorf("cached restaurant ID %s has a different canonical URL; fetch the requested URL with --data-source live", id))
		}
		if foundErr == nil && saved.URL == canonical && (flags.dataSource == "local" || (flags.dataSource == "auto" && !flags.noCache && saved.Surface == "detail" && time.Since(saved.FetchedAt) <= 6*time.Hour)) {
			meta := domain.Meta{Source: "cache", SourceSurface: saved.Surface, SourceURL: saved.SourceURL, FetchedAt: saved.FetchedAt, AgeSeconds: int64(time.Since(saved.FetchedAt).Seconds()), Stale: time.Since(saved.FetchedAt) > 6*time.Hour, Returned: 1, Scanned: 1, Coverage: "cached_snapshot"}
			if flags.dataSource == "local" {
				meta.Source = "local"
			}
			return tripPrint(cmd, flags, []domain.Restaurant{saved}, meta)
		}
		c, e := tripClient(flags)
		if e != nil {
			return tripError(e)
		}
		r, e := c.FetchDetail(ctx, canonical)
		if e != nil {
			return tripSourceFailure(cmd, flags, e)
		}
		if e = tripReplace(ctx, nb, r); e != nil {
			return e
		}
		meta := c.Meta()
		meta.SourceURL = r.SourceURL
		meta.SourceSurface = r.Surface
		meta.FetchedAt = r.FetchedAt
		meta.AgeSeconds = int64(time.Since(r.FetchedAt).Seconds())
		meta.Returned = 1
		meta.Scanned = 1
		meta.Pages = 1
		meta.Coverage = "detail_page"
		return tripPrint(cmd, flags, []domain.Restaurant{r}, meta)
	}
	return cmd
}

func newTabelogAreasCmd(flags *rootFlags) *cobra.Command {
	var kind string
	var limit int
	cmd := &cobra.Command{Use: "areas [QUERY]", Short: "Resolve typed prefecture, area and station choices", Example: "  tabelog-pp-cli areas Shinjuku --agent\n  tabelog-pp-cli areas Ginza --kind station --agent", Annotations: tripReadAnnotations("query=Shinjuku")}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) > 1 {
			return usageErr(fmt.Errorf("areas accepts one quoted location query"))
		}
		q := ""
		if len(args) > 0 {
			q = strings.TrimSpace(args[0])
		}
		if kind != "" && kind != "area" && kind != "station" && kind != "prefecture" {
			return usageErr(fmt.Errorf("--kind must be area, station, or prefecture"))
		}
		if limit < 1 || limit > 100 {
			return usageErr(fmt.Errorf("--limit must be between1 and100"))
		}
		if dryRunOK(flags) {
			return tripPlan(cmd, flags, "areas", map[string]any{"query": q, "kind": kind, "limit": limit})
		}
		if q == "" {
			return usageErr(fmt.Errorf("areas requires QUERY; use --kind to narrow location types"))
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		c, e := tripClient(flags)
		if e != nil {
			return tripError(e)
		}
		items, e := c.Lookup(ctx, q, kind)
		if e != nil {
			return tripSourceFailure(cmd, flags, e)
		}
		meta := c.Meta()
		meta.Scanned = len(items)
		if len(items) > limit {
			items = items[:limit]
			meta.HasMore = true
		}
		meta.Returned = len(items)
		meta.Coverage = "source_location_choices"
		meta.Criteria = map[string]any{"query": q, "kind": kind}
		return tripPrint(cmd, flags, items, meta)
	}
	cmd.Flags().StringVar(&kind, "kind", "", "Filter choices by area, station, or prefecture")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum typed source choices to return (1–100)")
	return cmd
}

func newTabelogCuisinesCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{Use: "cuisines [QUERY]", Short: "Browse verified English cuisine and bar taxonomy choices", Example: "  tabelog-pp-cli cuisines bar --agent\n  tabelog-pp-cli cuisines sushi --agent", Annotations: tripReadAnnotations("query=bar")}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) > 1 {
			return usageErr(fmt.Errorf("cuisines accepts one quoted cuisine query"))
		}
		q := ""
		if len(args) > 0 {
			q = strings.TrimSpace(args[0])
		}
		if limit < 1 || limit > 100 {
			return usageErr(fmt.Errorf("--limit must be between1 and100"))
		}
		if dryRunOK(flags) {
			return tripPlan(cmd, flags, "cuisines", map[string]any{"query": q, "limit": limit})
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		c, e := tripClient(flags)
		if e != nil {
			return tripError(e)
		}
		items, e := c.Cuisines(ctx, q)
		if e != nil {
			return tripSourceFailure(cmd, flags, e)
		}
		meta := c.Meta()
		if meta.Requests == 0 {
			meta.Source = "catalogue"
		}
		meta.Scanned = len(items)
		if len(items) > limit {
			items = items[:limit]
			meta.HasMore = true
		}
		meta.Returned = len(items)
		meta.Coverage = "verified_category_choices"
		meta.Criteria = map[string]any{"query": q}
		return tripPrint(cmd, flags, items, meta)
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum cuisine/taxonomy choices to return (1–100)")
	return cmd
}
