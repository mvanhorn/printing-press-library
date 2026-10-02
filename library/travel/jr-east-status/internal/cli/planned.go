// Copyright 2026 zjsng. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/jr-east-status/internal/jreast"
	"github.com/spf13/cobra"
	"time"
)

func newNovelPlannedCmd(flags *rootFlags) *cobra.Command {
	var region, line string
	var limit int
	cmd := &cobra.Command{Use: "planned", Short: "Read selected construction facts and official schedule handoffs", Example: "  jr-east-status-pp-cli planned --line koumiline --region kanto --agent", Annotations: jrAnnotations("--line=koumiline;--region=kanto;--agent=true", "live")}
	cmd.Flags().StringVar(&region, "region", "kanto", "Source region used to resolve the exact native line identity")
	cmd.Flags().StringVar(&line, "line", "", "Native ID or exact name; scheduled work is distinct from current suspensions")
	cmd.Flags().IntVar(&limit, "limit", 4, "Maximum matching planned notices, between 1 and 6")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "read planned construction facts")
		}
		if e := jrLiveMode(flags); e != nil {
			return e
		}
		if e := jrNoPositionals(args); e != nil {
			return e
		}
		if line == "" {
			return jrMissing(cmd, flags, "line")
		}
		if e := jrLimit(limit, 6); e != nil {
			return e
		}
		r, id, e := jrRegionLine(region, line, cmd.Flags().Changed("region"))
		if e != nil {
			return e
		}
		ctx, cancel := jrContext(cmd, flags)
		defer cancel()
		now := time.Now()
		c := jreast.NewClient(3, flags.rateLimit)
		s, e := c.Region(ctx, r, now, 15*time.Minute)
		if e != nil {
			return jrSourceError(e)
		}
		if len(s.Lines) == 0 && jrSnapshotClosed(s) {
			s.Lines = jreast.ReferenceLines(r)
		}
		l, e := jreast.FindLine(s.Lines, id)
		if e != nil {
			return notFoundErr(e)
		}
		body, e := c.Get(ctx, jreast.PlannedURL)
		if e != nil {
			return jrSourceError(e)
		}
		out, clipped, e := jreast.ParsePlanned(body, l)
		if e != nil {
			return apiErr(e)
		}
		m := jrMeta(c, now)
		m.Truncated = clipped
		m.Sources = s.Sources
		m.Sources = append(m.Sources, jreast.SourceState{URL: jreast.PlannedURL, Language: "ja", ObservedAt: now.In(jreast.JST).Format(time.RFC3339), Freshness: "missing_timestamp", ReportingState: "planned_notice_page"})
		if len(out) > limit {
			out = out[:limit]
			m.Truncated = true
		}
		m.Note = "Dates/times are source expressions in Japan local time. Calendar year stays unknown when absent; current service and scheduled work are separate. Complex schedules and changes require the official handoff."
		if len(out) == 0 {
			m.Note = "No matching construction heading on this bounded source page; no absence-of-disruption conclusion is established. Check " + jreast.PlannedURL
		}
		return flags.printJSON(cmd, jreast.Envelope[jreast.Planned]{Meta: m, Results: out})
	}
	return cmd
}
