// pp:data-source live
package cli

import (
	"fmt"
	"github.com/spf13/cobra"
	"strings"
	"time"
)

func newNovelStayDatesCmd(f *rootFlags) *cobra.Command {
	s := &stayFlags{}
	var checkIns string
	var nights int
	cmd := &cobra.Command{Use: "dates <property-id> <room-id> <plan-id>", Short: "Inspect every requested date for one exact selection, preserving sold-out and failed rows", Example: "  ikyu-pp-cli stay dates 00002889 10193741 11055986 --check-ins 2026-10-18,2026-10-19 --nights 1 --adults 2 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "property=00002889;room=10193741;plan=11055986;--check-ins=2026-10-18,2026-10-19;--nights=1;--adults=2"}}
	bindStayFlags(cmd, s, true, false, false)
	cmd.Flags().Lookup("check-in").Hidden = true
	cmd.Flags().Lookup("check-out").Hidden = true
	cmd.Flags().StringVar(&checkIns, "check-ins", "", "Comma-separated explicit check-in dates in Japan (1–7 distinct dates)")
	cmd.Flags().IntVar(&nights, "nights", 1, "Identical stay length for all alternatives, from 1 to 30 nights")
	addFieldsAlias(cmd, f)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return stayDryRun(cmd, f, map[string]any{"operation": "exact offer date alternatives", "selection": args, "check_ins": checkIns, "nights": nights, "party": s.stay, "concurrency": 2, "maximum_dates": 7})
		}
		if e := stayArg(args, 3, "use stay dates <property-id> <room-id> <plan-id> --check-ins YYYY-MM-DD,... --nights 1"); e != nil {
			return e
		}
		if cmd.Flags().Changed("check-in") || cmd.Flags().Changed("check-out") {
			return usageErr(fmt.Errorf("dates uses --check-ins and --nights, not single-stay check-in/check-out flags"))
		}
		selection, e := parseSelection(strings.Join(args, ":"))
		if e != nil {
			return e
		}
		dates := strings.Split(checkIns, ",")
		if len(dates) < 1 || len(dates) > 7 || checkIns == "" || nights < 1 || nights > 30 {
			return usageErr(fmt.Errorf("--check-ins requires 1–7 dates and --nights must be 1–30"))
		}
		jobs := []offerJob{}
		seen := map[string]bool{}
		for _, raw := range dates {
			date := strings.TrimSpace(raw)
			if seen[date] {
				return usageErr(fmt.Errorf("duplicate check-in date %s", date))
			}
			seen[date] = true
			parsed, e := time.Parse("2006-01-02", date)
			if e != nil {
				return usageErr(fmt.Errorf("--check-ins contains an invalid YYYY-MM-DD date"))
			}
			stay := s.stay
			stay.CheckIn = date
			stay.CheckOut = parsed.AddDate(0, 0, nights).Format("2006-01-02")
			if e := stayValidate(stay); e != nil {
				return e
			}
			jobs = append(jobs, offerJob{Key: date, Selection: selection, Stay: stay})
		}
		ctx, cancel := stayContext(cmd, f)
		defer cancel()
		c, e := stayClient(cmd, f, s)
		if e != nil {
			return e
		}
		rows, failures, results := fetchOfferRows(ctx, c, jobs)
		reportOfferFailures(cmd, failures, len(jobs))
		stats := c.Stats()
		result := map[string]any{"data": rows, "selection": selection.String(), "nights": nights, "fetch_failures": failures, "partial": len(failures) > 0, "successful": len(results), "requested": len(jobs), "note": "Alternative dates are separate observations; they are not same-stay equivalent offers."}
		if e := stayEmit(cmd, f, result, &stats); e != nil {
			return e
		}
		if len(results) == 0 {
			return allOfferFailures(failures, len(jobs))
		}
		return nil
	}
	return cmd
}
