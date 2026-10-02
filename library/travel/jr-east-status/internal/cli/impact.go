// Copyright 2026 zjsng. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/jr-east-status/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/jr-east-status/internal/jreast"
	"github.com/spf13/cobra"
	"strings"
	"time"
)

func newNovelImpactCmd(flags *rootFlags) *cobra.Command {
	var lineCSV, age string
	cmd := &cobra.Command{Use: "impact", Short: "Compare up to eight itinerary lines with source uncertainty", Long: "Compare the supplied line components. This does not check timetables, connections or route alternatives. Affected sections and source timing remain per line; normal labels never establish zero delay.", Example: "  jr-east-status-pp-cli impact --lines kanto:yamanoteline,kanto:sobuline --agent", Annotations: jrAnnotations("--lines=kanto:yamanoteline,kanto:sobuline;--agent=true", "live")}
	cmd.Flags().StringVar(&lineCSV, "lines", "", "Comma-separated region:native-line IDs, up to eight itinerary components")
	cmd.Flags().StringVar(&age, "max-source-age", "15m", "Maximum accepted source page age; supports 1s through 24h")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "compare itinerary impacts")
		}
		if e := jrLiveMode(flags); e != nil {
			return e
		}
		if e := jrNoPositionals(args); e != nil {
			return e
		}
		if lineCSV == "" {
			return jrMissing(cmd, flags, "lines")
		}
		inputs := strings.Split(lineCSV, ",")
		if len(inputs) > 8 {
			return usageErr(fmt.Errorf("--lines accepts at most eight itinerary components"))
		}
		d, e := jrAge(age)
		if e != nil {
			return e
		}
		for _, v := range inputs {
			p := strings.SplitN(strings.TrimSpace(v), ":", 2)
			if len(p) != 2 || p[1] == "" {
				return usageErr(fmt.Errorf("--lines components must be region:native-line-id"))
			}
			if _, e := jreast.RegionByID(p[0]); e != nil {
				return usageErr(e)
			}
		}
		ctx, cancel := jrContext(cmd, flags)
		defer cancel()
		now := time.Now()
		c := jreast.NewClient(18, flags.rateLimit)
		m := jrMeta(c, now)
		out := make([]jreast.Line, 0)
		cache := map[string]jreast.Snapshot{}
		failed := map[string]error{}
		for _, v := range inputs {
			p := strings.SplitN(strings.TrimSpace(v), ":", 2)
			r, _ := jreast.RegionByID(p[0])
			s, ok := cache[r.ID]
			if !ok && failed[r.ID] == nil {
				s, e = c.Region(ctx, r, now, d)
				if e != nil {
					var rate *cliutil.RateLimitError
					if errors.As(e, &rate) {
						return rateLimitErr(e)
					}
					failed[r.ID] = e
					m.FetchFailures = append(m.FetchFailures, jreast.Failure{Source: r.ID, Error: e.Error()})
				} else {
					cache[r.ID] = s
					m.Sources = append(m.Sources, s.Sources...)
				}
			}
			if failed[r.ID] != nil {
				out = append(out, jreast.Line{ID: strings.TrimSpace(v), SourceID: p[1], Region: r.ID, Assessment: "source_error", Statuses: make([]string, 0), Groups: make([]jreast.Group, 0)})
				continue
			}
			if len(s.Lines) == 0 && jrSnapshotClosed(s) {
				s.Lines = jreast.ReferenceLines(r)
			}
			l, e := jreast.FindLine(s.Lines, p[1])
			if e != nil {
				m.FetchFailures = append(m.FetchFailures, jreast.Failure{Source: strings.TrimSpace(v), Error: e.Error()})
				out = append(out, jreast.Line{ID: strings.TrimSpace(v), SourceID: p[1], Region: r.ID, Assessment: "line_not_found", Statuses: make([]string, 0), Groups: make([]jreast.Group, 0)})
				continue
			}
			if r.ID == "express" && !jrSnapshotClosed(s) {
				detail, e := jrExpress(ctx, c, l, now, d)
				if e != nil {
					var rate *cliutil.RateLimitError
					if errors.As(e, &rate) {
						return rateLimitErr(e)
					}
					m.FetchFailures = append(m.FetchFailures, jreast.Failure{Source: l.ID, Error: e.Error()})
					l.Assessment = "detail_source_error"
				} else if len(detail.Lines) > 0 {
					l.Notices = detail.Lines[0].Notices
					l.NoticeFactCount = len(l.Notices)
					l.Statuses = detail.Lines[0].Statuses
					l.LanguageConflict = detail.Lines[0].LanguageConflict
					m.Sources = append(m.Sources, detail.Sources...)
					l.Assessment = jreast.Assessment(l, append(s.Sources, detail.Sources...), now)
				}
			}
			if len(l.Notices) > 8 {
				l.Notices = l.Notices[:8]
				m.Truncated = true
			}
			out = append(out, l)
		}
		m.RequestCount = c.RequestCount()
		m.Note = "Line-level comparison only; sections may cover part of a line. Normal labels, stale sources and absent notices cannot establish connection feasibility or zero delay."
		if len(m.FetchFailures) > 0 {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d source/component failures across %d requested components; failed components remain explicit unknowns\n", len(m.FetchFailures), len(inputs))
		}
		if e := flags.printJSON(cmd, jreast.Envelope[jreast.Line]{Meta: m, Results: out}); e != nil {
			return e
		}
		verified := 0
		for _, l := range out {
			if l.Assessment != "source_error" && l.Assessment != "detail_source_error" && l.Assessment != "line_not_found" {
				verified++
			}
		}
		if verified == 0 && len(m.FetchFailures) > 0 {
			return apiErr(fmt.Errorf("no requested component was verified; inspect meta.fetch_failures"))
		}
		return nil
	}
	return cmd
}
