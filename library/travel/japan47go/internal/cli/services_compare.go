// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source auto
package cli

import (
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/japan47go/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/japan47go/internal/service"
	"github.com/spf13/cobra"
	"strings"
	"time"
)

func newNovelServicesCompareCmd(f *rootFlags) *cobra.Command {
	var ids, on, asOf string
	var party int
	var free, refresh bool
	c := &cobra.Command{Use: "compare [ids...]", Short: "Compare local services against request dates, party size and explicit zero-cost evidence", Long: "Compare 2..5 exact source records against explicit requirements. --ids is a comma-separated MCP-compatible alternative to positional UUIDs. Calendar deadlines are derived from one unambiguous rule; old hiatus and office closures never establish tour operation. Supported means published-rule compatibility; availability remains unknown.", Example: "  japan47go-pp-cli services compare --ids 2980022e-ef99-4115-95e5-be5227cdc74e,0ad62a4e-2987-4e83-af63-7a6dd69e0d98 --on 2026-11-01 --as-of 2026-10-25 --party 1 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "--ids=2980022e-ef99-4115-95e5-be5227cdc74e,0ad62a4e-2987-4e83-af63-7a6dd69e0d98;--on=2026-11-01;--as-of=2026-10-25;--party=1"}}
	c.RunE = func(c *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(c.OutOrStdout(), f, "services compare")
		}
		if len(args) == 0 && c.Flags().NFlag() == 0 {
			return c.Help()
		}
		if ids != "" {
			if len(args) > 0 {
				return serviceError(c, f, usageErr(fmt.Errorf("use positional IDs or --ids, not both")))
			}
			args = strings.Split(ids, ",")
		}
		if len(args) < 2 || len(args) > 5 {
			return serviceError(c, f, usageErr(fmt.Errorf("services compare requires 2..5 source IDs")))
		}
		seen := map[string]bool{}
		resolved := []string{}
		for _, v := range args {
			id, e := service.ID(strings.TrimSpace(v))
			if e != nil {
				return serviceError(c, f, usageErr(e))
			}
			if seen[id] {
				return serviceError(c, f, usageErr(fmt.Errorf("repeated source ID %s", id)))
			}
			seen[id] = true
			resolved = append(resolved, id)
		}
		if asOf == "" {
			asOf = time.Now().In(time.FixedZone("Asia/Tokyo", 9*3600)).Format("2006-01-02")
		}
		q := service.Constraints{On: on, AsOf: asOf, Party: party, RequireFree: free}
		if e := service.ValidateConstraints(q); e != nil {
			return serviceError(c, f, usageErr(e))
		}
		if c.Flags().Changed("party") && party == 0 {
			return serviceError(c, f, usageErr(fmt.Errorf("--party must be 1..10000 when provided")))
		}
		ctx, cancel := boundCtx(c.Context(), f)
		defer cancel()
		client := service.NewClient(f.rateLimit)
		out := []service.Comparison{}
		failures := []map[string]string{}
		var last error
		for _, id := range resolved {
			s, e := serviceGet(ctx, client, id, f, refresh)
			if e != nil {
				var rate *cliutil.RateLimitError
				if errors.As(e, &rate) {
					return serviceError(c, f, e)
				}
				failures = append(failures, map[string]string{"id": id, "error": e.Error()})
				last = e
				continue
			}
			serviceWarn(c, s)
			out = append(out, service.Compare(s, q))
		}
		if len(out) == 0 {
			return serviceError(c, f, fmt.Errorf("all %d source reads failed: %w", len(resolved), last))
		}
		if len(failures) > 0 {
			fmt.Fprintf(c.ErrOrStderr(), "warning: %d of %d source reads failed; comparison contains %d successful records\n", len(failures), len(resolved), len(out))
		}
		return serviceOutput(c, f, map[string]any{"comparisons": out, "constraints": q, "fetch_failures": failures, "coverage": map[string]any{"requested": len(resolved), "inspected": len(out), "complete": len(failures) == 0}, "source_boundary": "Derived calendar/party/fee compatibility only; availability and daily operation remain unknown. Missing fee units and conditional expenses cannot be totaled."})
	}
	c.Flags().StringVar(&ids, "ids", "", "Comma-separated source UUIDs or canonical URLs,2..5 distinct records")
	c.Flags().StringVar(&on, "on", "", "Requested visit date as exact YYYY-MM-DD for notice checks")
	c.Flags().StringVar(&asOf, "as-of", "", "Request date as exact YYYY-MM-DD; defaults to current Japan date")
	c.Flags().IntVar(&party, "party", 0, "Participant count 1..10000; omit to leave party requirement unrequested")
	c.Flags().BoolVar(&free, "require-free", false, "Require unambiguous source evidence of zero cost")
	c.Flags().BoolVar(&refresh, "refresh", false, "Force live source rechecks without local fallback")
	return c
}
