// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source auto
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/japan47go/internal/service"
	"github.com/spf13/cobra"
)

func newServicesInspectCmd(f *rootFlags) *cobra.Command {
	var refresh bool
	c := &cobra.Command{Use: "inspect [id]", Short: "Inspect a local service UUID for published deadlines, fees, durations and participant rules", Long: "Read one canonical JAPAN47GO record and save only normalized decision evidence. Multiple deadlines stay ambiguous, staffing/guide ages/personal contacts are discarded. Source closed=false does not prove current opening or availability.", Example: "  japan47go-pp-cli services inspect 2980022e-ef99-4115-95e5-be5227cdc74e --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "id=2980022e-ef99-4115-95e5-be5227cdc74e"}}
	c.RunE = func(c *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(c.OutOrStdout(), f, "services inspect")
		}
		if len(args) == 0 && c.Flags().NFlag() == 0 {
			return c.Help()
		}
		if len(args) != 1 {
			return serviceError(c, f, usageErr(fmt.Errorf("services inspect requires one source UUID or canonical detail URL")))
		}
		id, e := service.ID(args[0])
		if e != nil {
			return serviceError(c, f, usageErr(e))
		}
		ctx, cancel := boundCtx(c.Context(), f)
		defer cancel()
		s, e := serviceGet(ctx, service.NewClient(f.rateLimit), id, f, refresh)
		if e != nil {
			return serviceError(c, f, e)
		}
		serviceWarn(c, s)
		return serviceOutput(c, f, map[string]any{"service": s, "source_boundary": "Published JAPAN47GO facts at observed_at; request acceptance, current operation and live availability remain unknown."})
	}
	c.Flags().BoolVar(&refresh, "refresh", false, "Force a live source recheck without local fallback")
	return c
}
