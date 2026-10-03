// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"fmt"
	hw "github.com/mvanhorn/printing-press-library/library/travel/hostelworld/internal/hostelworld"
	"github.com/spf13/cobra"
	"strings"
)

func newNovelHostelsDatesCmd(flags *rootFlags) *cobra.Command {
	var o stayOptions
	var starts string
	cmd := &cobra.Command{Use: "dates [property-id]", Short: "Request each alternative stay window independently", Example: "  " + "hostelworld-pp-cli hostels dates 67481 --starts 2026-11-10,2026-11-11 --nights 3 --guests 2 --agent", Annotations: map[string]string{"mcp:read-only": "false", "mcp:local-write": "true", "pp:live-happy-path": "true", "pp:data-source": "live", "pp:happy-args": "id=67481;--starts=2026-10-04,2026-10-05;--nights=3;--guests=2"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 || !hw.ValidID(args[0]) {
			return usageErr(fmt.Errorf("dates requires one numeric property ID"))
		}
		if cmd.Flags().Changed("check-in") || cmd.Flags().Changed("check-out") {
			return usageErr(fmt.Errorf("dates uses --starts and --nights; --check-in/--check-out are unsupported for alternative windows"))
		}
		dates := strings.Split(starts, ",")
		if starts == "" || len(dates) > 5 {
			return usageErr(fmt.Errorf("--starts requires 1–5 comma-separated check-in dates"))
		}
		queries := []hw.Query{}
		seen := map[string]bool{}
		for _, date := range dates {
			date = strings.TrimSpace(date)
			if seen[date] {
				return usageErr(fmt.Errorf("--starts contains duplicate dates"))
			}
			seen[date] = true
			copyOpts := o
			copyOpts.start = date
			copyOpts.end = ""
			q, err := copyOpts.query()
			if err != nil {
				return usageErr(err)
			}
			queries = append(queries, q)
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		p, err := propertyObject(ctx, c, args[0])
		if err != nil {
			return classifyAPIError(cmd.OutOrStdout(), err, flags)
		}
		out := []any{}
		failures := []any{}
		for _, q := range queries {
			v, err := offersObject(ctx, c, args[0], p, q, o)
			if err != nil {
				if fatalRate(err) {
					return classifyAPIError(cmd.OutOrStdout(), err, flags)
				}
				f := map[string]any{"query": q, "status": "source_error", "error": err.Error()}
				out = append(out, f)
				failures = append(failures, f)
				continue
			}
			limitPlans(v, o.limit)
			out = append(out, v)
		}
		if len(failures) > 0 {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d date windows failed; %d have live responses\n", len(failures), len(queries), len(queries)-len(failures))
		}
		if len(failures) == len(queries) {
			return fmt.Errorf("all requested date windows failed")
		}
		return outputPlanning(cmd, flags, map[string]any{"property_id": args[0], "results": out, "requested_windows": len(queries), "fetch_failures": failures, "coverage": "each window has its own dated source request; missing inventory and fetch errors are distinct"}, o.save)
	}}
	decoratePlanning(cmd, flags)
	stayFlags(cmd, &o)
	cmd.Flags().Lookup("nights").Usage = "Stay length of 1–30 nights for each requested start date"
	cmd.Flags().StringVar(&starts, "starts", "", "Comma-separated explicit alternative check-in dates, up to five")
	cmd.Flags().Lookup("check-in").Hidden = true
	cmd.Flags().Lookup("check-out").Hidden = true
	return cmd
}
