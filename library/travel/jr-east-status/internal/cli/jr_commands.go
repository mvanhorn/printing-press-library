// Copyright 2026 zjsng. Licensed under Apache-2.0.
package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/travel/jr-east-status/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/jr-east-status/internal/jreast"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newJRAreasCmd(flags))
		addNovelCommandIfAbsent(root, newJRLinesCmd(flags))
		// Typed HTML handlers do not apply the CLI's link extraction. The spec
		// hides that endpoint; expose this preserved CLI command as its bounded
		// MCP mirror instead, without changing the generated registration code.
		for _, command := range root.Commands() {
			if command.Name() == "sources" {
				delete(command.Annotations, "pp:endpoint")
			}
		}
	})
}

func jrAnnotations(happy string, source string) map[string]string {
	// The installed Press happy-argument parser renders key=value as two argv
	// tokens, which is valid for value flags but not Cobra boolean switches.
	// Happy examples remain machine-formatted automatically when captured.
	happy = strings.Trim(strings.ReplaceAll(happy, "--agent=true", ""), ";")
	return map[string]string{"mcp:read-only": "true", "pp:data-source": source, "pp:happy-args": happy}
}
func jrContext(cmd *cobra.Command, flags *rootFlags) (context.Context, context.CancelFunc) {
	base, cb := boundCtx(cmd.Context(), flags)
	ctx, c := context.WithTimeout(base, 30*time.Second)
	return ctx, func() { c(); cb() }
}
func jrMeta(c *jreast.Client, now time.Time) jreast.Meta {
	return jreast.Meta{Source: "live", ObservedAt: now.In(jreast.JST).Format(time.RFC3339), Coverage: jreast.ReportingCoverage(now), RequestCount: c.RequestCount(), MaxRequests: c.MaxRequests, Sources: make([]jreast.SourceState, 0), FetchFailures: make([]jreast.Failure, 0)}
}
func jrSourceError(e error) error {
	var rate *cliutil.RateLimitError
	if errors.As(e, &rate) {
		return rateLimitErr(e)
	}
	return apiErr(e)
}
func jrLiveMode(flags *rootFlags) error {
	if flags.dataSource == "local" {
		return usageErr(fmt.Errorf("this operational command requires live source reads; --data-source local is unavailable"))
	}
	return nil
}

func jrSnapshotClosed(s jreast.Snapshot) bool {
	for _, source := range s.Sources {
		if source.ReportingState == "outside_reporting_hours" {
			return true
		}
	}
	return false
}

func jrAge(v string) (time.Duration, error) {
	d, e := cliutil.ParseDurationLoose(v)
	if e != nil || d < time.Second || d > 24*time.Hour {
		return 0, usageErr(fmt.Errorf("--max-source-age must be between 1s and 24h, for example 15m"))
	}
	return d, nil
}
func jrRegionLine(region, line string, explicit bool) (jreast.Region, string, error) {
	if p := strings.SplitN(line, ":", 2); len(p) == 2 {
		if explicit && region != p[0] {
			return jreast.Region{}, "", usageErr(fmt.Errorf("--region conflicts with the region prefix in --line"))
		}
		region = p[0]
		line = p[1]
	}
	r, e := jreast.RegionByID(region)
	if e != nil {
		return r, "", usageErr(e)
	}
	return r, line, nil
}
func jrLimit(n, max int) error {
	if n < 1 || n > max {
		return usageErr(fmt.Errorf("--limit must be between 1 and %d", max))
	}
	return nil
}
func jrNoPositionals(a []string) error {
	if len(a) > 0 {
		return usageErr(fmt.Errorf("unexpected positional argument; use the documented --line or --lines flag"))
	}
	return nil
}
func jrMissing(cmd *cobra.Command, flags *rootFlags, name string) error {
	if !hasChangedLocalFlags(cmd) && !flags.agent && !flags.asJSON && !flags.noInput {
		return cmd.Help()
	}
	return usageErr(fmt.Errorf("--%s is required; discover native IDs with lines", name))
}

// pp:data-source live
func newJRAreasCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "areas", Short: "Read JR East region/service summaries and canonical source links", Example: "  jr-east-status-pp-cli areas --agent", Annotations: jrAnnotations("--timeout=30s", "live")}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "read regional summaries")
		}
		if e := jrLiveMode(flags); e != nil {
			return e
		}
		if e := jrNoPositionals(args); e != nil {
			return e
		}
		ctx, cancel := jrContext(cmd, flags)
		defer cancel()
		now := time.Now()
		c := jreast.NewClient(1, flags.rateLimit)
		body, e := c.Get(ctx, jreast.Origin+"/train_info/e/infotop.aspx")
		if e != nil {
			return jrSourceError(e)
		}
		areas, state, e := jreast.ParseAreas(body, now)
		if e != nil {
			return apiErr(e)
		}
		m := jrMeta(c, now)
		m.Sources = append(m.Sources, state)
		m.Note = "Regional aggregate labels; source script exposes no update timestamp. Use line status for affected sections and dated source evidence."
		return flags.printJSON(cmd, jreast.Envelope[jreast.Area]{Meta: m, Results: areas})
	}
	return cmd
}

// pp:data-source live
func newJRLinesCmd(flags *rootFlags) *cobra.Command {
	var region, query, age string
	var limit int
	cmd := &cobra.Command{Use: "lines", Short: "Discover native line IDs and bilingual names in one source region", Example: "  jr-east-status-pp-cli lines --region kanto --query Narita --agent", Annotations: jrAnnotations("--region=kanto;--query=Yamanote;--agent=true", "live")}
	cmd.Flags().StringVar(&region, "region", "kanto", "JR East service area: kanto, tohoku, shinetsu, express or shinkansen")
	cmd.Flags().StringVar(&query, "query", "", "Case-insensitive substring of native ID, bilingual name or line code")
	cmd.Flags().StringVar(&age, "max-source-age", "15m", "Maximum source page age; missing or future timestamps remain unknown")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum returned lines (1–100); each source page scans at most 512 rows")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "discover source lines")
		}
		if e := jrLiveMode(flags); e != nil {
			return e
		}
		if e := jrNoPositionals(args); e != nil {
			return e
		}
		if e := jrLimit(limit, 100); e != nil {
			return e
		}
		d, e := jrAge(age)
		if e != nil {
			return e
		}
		r, e := jreast.RegionByID(region)
		if e != nil {
			return usageErr(e)
		}
		ctx, cancel := jrContext(cmd, flags)
		defer cancel()
		now := time.Now()
		c := jreast.NewClient(2, flags.rateLimit)
		s, e := c.Region(ctx, r, now, d)
		if e != nil {
			return jrSourceError(e)
		}
		if len(s.Lines) == 0 && jrSnapshotClosed(s) {
			s.Lines = jreast.ReferenceLines(r)
			s.Warnings = append(s.Warnings, "Reporting is closed; line identities come from the bundled source catalogue and no operational labels are supplied.")
		}
		out := make([]jreast.Line, 0)
		q := jreast.Normalize(query)
		total := 0
		for _, l := range s.Lines {
			if !strings.Contains(jreast.Normalize(l.ID+" "+l.NameJA+" "+l.NameEN+" "+l.Code), q) {
				continue
			}
			total++
			if len(out) < limit {
				l.Notices = nil
				out = append(out, l)
			}
		}
		m := jrMeta(c, now)
		m.Sources = s.Sources
		m.Truncated = total > len(out)
		if len(out) == 0 {
			m.Note = "No matching line in this source region; choose another --region or --query. Absence is not a no-delay conclusion."
		}
		if len(s.Warnings) > 0 {
			m.Note = strings.Join(s.Warnings, "; ")
		}
		return flags.printJSON(cmd, jreast.Envelope[jreast.Line]{Meta: m, Results: out})
	}
	return cmd
}

func jrExpress(ctx context.Context, c *jreast.Client, l jreast.Line, now time.Time, d time.Duration) (jreast.Snapshot, error) {
	r, _ := jreast.RegionByID("express")
	ja, e := c.Get(ctx, l.SourceJA)
	if e != nil {
		return jreast.Snapshot{}, e
	}
	jp, e := jreast.ParseRegion(ja, r, "ja", l.SourceJA, now, d)
	if e != nil {
		return jreast.Snapshot{}, e
	}
	en, e := c.Get(ctx, l.SourceEN)
	if e != nil {
		return jreast.Snapshot{}, e
	}
	ep, e := jreast.ParseRegion(en, r, "en", l.SourceEN, now, d)
	if e != nil {
		return jreast.Snapshot{}, e
	}
	return jreast.JoinRegions(jp, ep, now), nil
}
