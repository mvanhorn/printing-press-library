// Copyright 2026 zjsng. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/jr-east-status/internal/jreast"
	"github.com/spf13/cobra"
	"time"
)

func newNovelStatusCmd(flags *rootFlags) *cobra.Command {
	var region, line, age string
	var limit int
	cmd := &cobra.Command{Use: "status", Short: "Inspect disruptions, directions, sections and bilingual source evidence", Example: "  jr-east-status-pp-cli status --line sobuline --region kanto --agent", Annotations: jrAnnotations("--line=sobuline;--region=kanto;--agent=true", "live")}
	cmd.Flags().StringVar(&region, "region", "kanto", "Source area; a region prefix in --line selects the area")
	cmd.Flags().StringVar(&line, "line", "", "Native ID, exact bilingual name or region:line ID; resolve IDs using lines")
	cmd.Flags().StringVar(&age, "max-source-age", "15m", "Maximum accepted source page age; supports 1s through 24h")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum notice facts for the selected line, between 1 and 20")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "inspect line status")
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
		if e := jrLimit(limit, 20); e != nil {
			return e
		}
		d, e := jrAge(age)
		if e != nil {
			return e
		}
		r, id, e := jrRegionLine(region, line, cmd.Flags().Changed("region"))
		if e != nil {
			return e
		}
		ctx, cancel := jrContext(cmd, flags)
		defer cancel()
		now := time.Now()
		c := jreast.NewClient(4, flags.rateLimit)
		s, e := c.Region(ctx, r, now, d)
		if e != nil {
			return jrSourceError(e)
		}
		m := jrMeta(c, now)
		m.Sources = s.Sources
		out := make([]jreast.Line, 0)
		if len(s.Lines) == 0 && jrSnapshotClosed(s) {
			l, e := jreast.FindLine(jreast.ReferenceLines(r), id)
			if e != nil {
				return notFoundErr(e)
			}
			m.Note = "Reporting is closed. Identity comes from the bundled source catalogue; no line-specific operational conclusion is available."
			return flags.printJSON(cmd, jreast.Envelope[jreast.Line]{Meta: m, Results: []jreast.Line{l}})
		}
		l, e := jreast.FindLine(s.Lines, id)
		if e != nil {
			return notFoundErr(e)
		}
		if r.ID == "express" && !jrSnapshotClosed(s) {
			detail, e := jrExpress(ctx, c, l, now, d)
			if e != nil {
				return jrSourceError(e)
			}
			if len(detail.Lines) > 0 {
				l.Notices = detail.Lines[0].Notices
				l.NoticeFactCount = len(l.Notices)
				l.Statuses = detail.Lines[0].Statuses
				l.LanguageConflict = detail.Lines[0].LanguageConflict
				s.Sources = append(s.Sources, detail.Sources...)
				l.Assessment = jreast.Assessment(l, s.Sources, now)
				m.Sources = s.Sources
			}
		}
		if len(l.Notices) > limit {
			l.Notices = l.Notices[:limit]
			m.Truncated = true
		}
		out = append(out, l)
		m.RequestCount = c.RequestCount()
		m.Note = "Affected direction comes from notice text; source_groups are navigation categories. Actual individual train delay remains unknown. English notices may use AI translation."
		return flags.printJSON(cmd, jreast.Envelope[jreast.Line]{Meta: m, Results: out})
	}
	return cmd
}
