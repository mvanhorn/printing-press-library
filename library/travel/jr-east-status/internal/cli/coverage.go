// Copyright 2026 zjsng. Licensed under Apache-2.0.
// pp:data-source computed
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/jr-east-status/internal/jreast"
	"github.com/spf13/cobra"
	"time"
)

func newNovelCoverageCmd(flags *rootFlags) *cobra.Command {
	var at string
	cmd := &cobra.Command{Use: "coverage", Short: "Explain reporting windows, thresholds and certificate limits", Example: "  jr-east-status-pp-cli coverage --at 2026-10-03T01:30:00+09:00 --agent", Annotations: jrAnnotations("--at=2026-10-03T01:30:00+09:00", "computed")}
	cmd.Flags().StringVar(&at, "at", "", "Optional RFC3339 time with timezone; classifies JST reporting service day")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "explain reporting coverage")
		}
		if e := jrNoPositionals(args); e != nil {
			return e
		}
		now := time.Now()
		if at != "" {
			v, e := time.Parse(time.RFC3339, at)
			if e != nil {
				return usageErr(fmt.Errorf("--at must be RFC3339 with timezone, such as 2026-10-03T01:30:00+09:00"))
			}
			now = v
		}
		c := jreast.NewClient(0, flags.rateLimit)
		m := jrMeta(c, now)
		m.Source = "computed"
		m.Note = "Observed source policy, not a live delay check. Page timestamps differ from incident update times. English uses AI translation. Certificates are route maxima and do not prove boarding."
		return flags.printJSON(cmd, jreast.Envelope[jreast.Coverage]{Meta: m, Results: []jreast.Coverage{jreast.ReportingCoverage(now)}})
	}
	return cmd
}
