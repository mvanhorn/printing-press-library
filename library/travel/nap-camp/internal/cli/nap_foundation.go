// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/nap-camp/internal/napcamp"
	"github.com/spf13/cobra"
	"strconv"
	"strings"
	"time"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		camp := &cobra.Command{Use: "campsite", Short: "Discover public campervan-entry campsites and inspect facilities."}
		camp.Example = "  nap-camp-pp-cli campsite discover --region kanto --limit 2 --json\n  nap-camp-pp-cli campsite inspect 11007 --json"
		camp.AddCommand(newNapDiscoverCmd(flags), newNapCampsiteCmd(flags))
		pitch := &cobra.Command{Use: "pitch", Short: "Inspect specific pitch rules and public dated acceptance."}
		pitch.Example = "  nap-camp-pp-cli pitch inspect 11007 20005062 --json\n  nap-camp-pp-cli pitch calendar 11007 20005062 --month 2026-10 --limit 7 --json"
		pitch.AddCommand(newNapPitchCmd(flags), newNapCalendarCmd(flags))
		root.AddCommand(camp, pitch, newNapCatalogCmd(flags))
		for _, group := range root.Commands() {
			if group.Name() == "planner" {
				group.Example = "  nap-camp-pp-cli planner fit 11007 20005062 --people 2 --power --json\n  nap-camp-pp-cli planner windows 11007 20005062 --month 2026-10 --nights 2 --limit 3 --json"
			}
		}

	})
}

func napAPI(flags *rootFlags) (napcamp.API, error) {
	c, e := flags.newClient()
	if e != nil {
		return napcamp.API{}, e
	}
	return napcamp.API{Reader: c}, nil
}
func napIDs(args []string, n int) error {
	if len(args) != n {
		return usageErr(fmt.Errorf("expected %d numeric campsite/plan IDs; see --help", n))
	}
	for _, id := range args {
		if e := napcamp.ValidateID(id); e != nil {
			return usageErr(e)
		}
	}
	return nil
}
func napLimit(n, max int) error {
	if n < 1 || n > max {
		return usageErr(fmt.Errorf("--limit must be 1..%d", max))
	}
	return nil
}
func napMonth(m string) error {
	_, e := napcamp.ValidateMonth(m)
	if e != nil {
		return usageErr(e)
	}
	return nil
}
func napNowMonth() string { return time.Now().In(time.FixedZone("JST", 9*3600)).Format("2006-01") }

func newNapDiscoverCmd(flags *rootFlags) *cobra.Command {
	var region, keyword, in, out string
	var pref, area, page, limit int
	var power, shower, laundry, garbage, pets bool
	cmd := &cobra.Command{Use: "discover", Short: "Find campervan-entry facilities; category membership is not selected-pitch permission.", Example: "  " + "nap-camp-pp-cli campsite discover --region kanto --power --limit 3 --json", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--region=kanto;--limit=3"}}
	napBind(flags, cmd, "live", func(cmd *cobra.Command, args []string) error {
		if len(args) != 0 {
			return usageErr(fmt.Errorf("discover takes flags, not positional arguments"))
		}
		if e := napLimit(limit, 50); e != nil {
			return e
		}
		if page < 1 || page > 1000 {
			return usageErr(fmt.Errorf("--page must be 1..1000"))
		}
		if pref < 0 || pref > 47 || area < 0 || area > 10000 {
			return usageErr(fmt.Errorf("--prefecture-id must be 0..47 and --area-id 0..10000"))
		}
		if e := napcamp.ValidateDates(in, out); e != nil {
			return usageErr(e)
		}
		rid := 0
		regions := map[string]int{"hokkaido_tohoku": 1, "kanto": 2, "hokuriku_koshinetsu": 3, "tokai": 4, "kansai": 5, "chugoku_shikoku": 6, "kyushu_okinawa": 7}
		if region != "" {
			var ok bool
			rid, ok = regions[region]
			if !ok {
				n, e := strconv.Atoi(region)
				if e != nil || n < 1 || n > 7 {
					return usageErr(fmt.Errorf("--region must be a source region slug from catalog or ID 1..7"))
				}
				rid = n
			}
		}
		filters := make([]int, 0)
		for _, f := range []struct {
			want bool
			id   int
		}{{power, 43}, {shower, 48}, {laundry, 50}, {garbage, 58}, {pets, 47}} {
			if f.want {
				filters = append(filters, f.id)
			}
		}
		a, e := napAPI(flags)
		if e != nil {
			return e
		}
		v, e := a.Discover(cmd.Context(), napcamp.Query{RegionID: rid, PrefectureID: pref, AreaID: area, Page: page, Limit: limit, Filters: filters, Keyword: keyword, CheckIn: in, CheckOut: out})
		if e != nil {
			return classifyAPIErrorOnly(e)
		}
		return flags.printJSON(cmd, v)
	})
	cmd.Flags().StringVar(&region, "region", "", "Source region slug, e.g. kanto, or numeric ID 1..7")
	cmd.Flags().IntVar(&pref, "prefecture-id", 0, "Source prefecture ID from catalog prefectures; 0 means unspecified")
	cmd.Flags().IntVar(&area, "area-id", 0, "Source area ID from source locations; 0 means unspecified")
	cmd.Flags().IntVar(&page, "page", 1, "Explicit source page; query one page at a time")
	cmd.Flags().IntVar(&limit, "limit", 5, "Maximum returned facilities and requested source page size (1..50)")
	cmd.Flags().StringVar(&keyword, "keyword", "", "Source Japanese or Latin freeword campsite search")
	cmd.Flags().StringVar(&in, "check-in", "", "JST arrival date YYYY-MM-DD; pair with --check-out")
	cmd.Flags().StringVar(&out, "check-out", "", "JST departure YYYY-MM-DD; dated membership is not vacancy")
	cmd.Flags().BoolVar(&power, "power", false, "Require facility-level source AC-power category; inspect specific pitch")
	cmd.Flags().BoolVar(&shower, "shower", false, "Require facility-level source shower category; fees unknown")
	cmd.Flags().BoolVar(&laundry, "laundry", false, "Require facility-level source laundry category; fees unknown")
	cmd.Flags().BoolVar(&garbage, "garbage", false, "Require facility-level source garbage-disposal category; terms unknown")
	cmd.Flags().BoolVar(&pets, "pets", false, "Require facility-level source pet category; inspect pitch restrictions")
	return cmd
}
func newNapCampsiteCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{Use: "inspect <campsite-id>", Short: "Read campsite facilities, season, fees and a bounded plan list.", Example: "  " + "nap-camp-pp-cli campsite inspect 11007 --limit 5 --json", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "campsite=11007;--limit=3"}}
	napBind(flags, cmd, "live", func(cmd *cobra.Command, args []string) error {
		if e := napIDs(args, 1); e != nil {
			return e
		}
		if e := napLimit(limit, 25); e != nil {
			return e
		}
		a, e := napAPI(flags)
		if e != nil {
			return e
		}
		c, e := a.Campsite(cmd.Context(), args[0])
		if e != nil {
			return classifyAPIErrorOnly(e)
		}
		p, e := a.Plans(cmd.Context(), args[0], c, limit)
		if e != nil {
			return classifyAPIErrorOnly(e)
		}
		return flags.printJSON(cmd, napcamp.Object{"observed_at": napcamp.Now(), "campsite": c, "plans": p["plans"], "source_plan_count": p["source_count"], "truncated": p["truncated"], "qualification": "Facility categories and starting prices require selected-pitch, dimension and dated confirmation."})
	})
	cmd.Flags().IntVar(&limit, "limit", 5, "Maximum plan previews (1..25); source count is reported")
	return cmd
}
func newNapPitchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "inspect <campsite-id> <plan-id>", Short: "Read the specific pitch's vehicle memo, capacity, power, rules and starting price.", Example: "  " + "nap-camp-pp-cli pitch inspect 11007 20005062 --json", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "campsite=11007;plan=20005062"}}
	napBind(flags, cmd, "live", func(cmd *cobra.Command, args []string) error {
		if e := napIDs(args, 2); e != nil {
			return e
		}
		a, e := napAPI(flags)
		if e != nil {
			return e
		}
		s, e := a.Snapshot(cmd.Context(), args[0], args[1], "")
		if e != nil {
			return classifyAPIErrorOnly(e)
		}
		return flags.printJSON(cmd, s)
	})
	return cmd
}

func newNapCalendarCmd(flags *rootFlags) *cobra.Command {
	var month string
	var limit int
	cmd := &cobra.Command{Use: "calendar <campsite-id> <plan-id>", Short: "Read requested-month source acceptance and starting prices; vacancy stays unknown.", Example: "  " + "nap-camp-pp-cli pitch calendar 11007 20005062 --month 2026-10 --limit 7 --json", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "campsite=11007;plan=20005062;--month=2026-10;--limit=7"}}
	napBind(flags, cmd, "live", func(cmd *cobra.Command, args []string) error {
		if e := napIDs(args, 2); e != nil {
			return e
		}
		if e := napMonth(month); e != nil {
			return e
		}
		if e := napLimit(limit, 62); e != nil {
			return e
		}
		a, e := napAPI(flags)
		if e != nil {
			return e
		}
		c, e := a.Campsite(cmd.Context(), args[0])
		if e != nil {
			return classifyAPIErrorOnly(e)
		}
		days, e := a.Calendar(cmd.Context(), args[0], args[1], month)
		if e != nil {
			return classifyAPIErrorOnly(e)
		}
		v := napcamp.CalendarView(days, month, limit)
		v["campsite_id"] = args[0]
		v["plan_id"] = args[1]
		v["canonical_url"] = fmt.Sprint(c["canonical_url"]) + "/plans/" + args[1]
		v["source_url"] = napcamp.Origin + "/api/campsite/" + args[0] + "/plans/" + args[1] + "/reservation?month=" + month
		return flags.printJSON(cmd, v)
	})
	cmd.Flags().StringVar(&month, "month", napNowMonth(), "Requested JST month YYYY-MM; following month source rows are filtered")
	cmd.Flags().IntVar(&limit, "limit", 31, "Maximum day rows (1..62); request --limit 31 for a full month")
	return cmd
}
func newNapCatalogCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{Use: "catalog [regions|prefectures|filters]", Short: "Look up stable source region and facility-category IDs with Japanese labels.", Example: "  " + "nap-camp-pp-cli catalog regions --json", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "kind=regions"}}
	napBind(flags, cmd, "live", func(cmd *cobra.Command, args []string) error {
		kind := "regions"
		if len(args) > 1 {
			return usageErr(fmt.Errorf("catalog expects one kind"))
		}
		if len(args) == 1 {
			kind = strings.ToLower(args[0])
		}
		if kind != "regions" && kind != "prefectures" && kind != "filters" {
			return usageErr(fmt.Errorf("catalog kind must be regions, prefectures or filters"))
		}
		if e := napLimit(limit, 200); e != nil {
			return e
		}
		a, e := napAPI(flags)
		if e != nil {
			return e
		}
		v, e := a.Catalog(cmd.Context(), kind, limit)
		if e != nil {
			return classifyAPIErrorOnly(e)
		}
		return flags.printJSON(cmd, v)
	})
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum returned source IDs and labels (1..200)")
	return cmd
}

func napBind(flags *rootFlags, cmd *cobra.Command, strategy string, run func(*cobra.Command, []string) error) {
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, cmd.CommandPath())
		}
		if flags.dataSource == "local" && strategy == "live" {
			return usageErr(fmt.Errorf("%s has no local data source; use planner changes for saved observations", cmd.CommandPath()))
		}
		if flags.dataSource == "live" && strategy == "local" {
			return usageErr(fmt.Errorf("%s has no live equivalent; use planner snapshot for fresh evidence", cmd.CommandPath()))
		}
		return run(cmd, args)
	}
}
