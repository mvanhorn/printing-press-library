// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/iclasspro/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/iclasspro/internal/icp"
	"github.com/mvanhorn/printing-press-library/library/productivity/iclasspro/internal/store"

	"github.com/spf13/cobra"
)

type fillRateReport struct {
	Account   string      `json:"account"`
	Since     string      `json:"since"`
	Trends    []icp.Trend `json:"trends"`
	Samples   int         `json:"samples_read"`
	Entities  int         `json:"entities_with_history"`
	Direction string      `json:"filter_direction,omitempty"`
	Note      string      `json:"note,omitempty"`
}

func newNovelFillRateCmd(flags *rootFlags) *cobra.Command {
	var (
		programs  string
		dbPath    string
		since     string
		limit     int
		direction string
	)

	cmd := &cobra.Command{
		Use:   "fill-rate [account]",
		Short: "Show how fast classes are filling over time, by class or by program.",
		Long: strings.Trim(`
Compute the direction and velocity of fill from recorded openings history.

Use this command for how fast classes are filling over time. Do NOT use it for a
point-in-time list of what changed; use 'drift' instead.

A single API call can only ever report the current openings count. This reads
every observation 'sync' has recorded and reports the trend, plus a projected
full date when a class is filling steadily. Entities with fewer than two
observations are omitted rather than reported as flat, because one sample is not
a trend.`, "\n"),
		Example:     "  iclasspro-pp-cli fill-rate scaq --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<account>=scaq;--since=90d"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "fill-rate")
			}
			account, err := icpRequireAccount(args)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}

			cutoff := icpNow().AddDate(0, 0, -90)
			if strings.TrimSpace(since) != "" {
				d, perr := cliutil.ParseDurationLoose(since)
				if perr != nil {
					return usageErr(fmt.Errorf("--since %q is not a duration (try 7d, 24h, 1w): %w", since, perr))
				}
				cutoff = icpNow().Add(-d)
			}
			switch strings.ToLower(strings.TrimSpace(direction)) {
			case "", "any", "filling", "emptying", "flat":
			default:
				return usageErr(fmt.Errorf("--direction accepts any, filling, emptying, or flat"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			path := icpDBPath(dbPath)
			report := fillRateReport{
				Account: account, Since: cutoff.Format(time.RFC3339),
				Trends: make([]icp.Trend, 0), Direction: direction,
			}
			s, ok, err := icpOpenStoreForRead(ctx, path)
			if err != nil {
				return err
			}
			if !ok {
				report.Note = fmt.Sprintf("no local mirror yet; run 'iclasspro-pp-cli sync %s'", account)
				return icpNoLocalData(cmd, flags, report, account)
			}
			defer func() { _ = s.Close() }()
			icpStaleHint(ctx, cmd.ErrOrStderr(), s, flags, account)

			rows, err := s.ICPHistory(ctx, account, cutoff)
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				report.Note = fmt.Sprintf("no recorded observations for %s", account)
				return icpNoLocalData(cmd, flags, report, account)
			}
			report.Samples = len(rows)

			// Program filtering needs the entity records, which carry programId;
			// history rows do not. Resolve the allowed entity ids first.
			allowed, err := icpProgramFilter(ctx, s, account, programs)
			if err != nil {
				return usageErr(err)
			}

			samples := make([]icp.Sample, 0, len(rows))
			seen := map[string]bool{}
			for _, r := range rows {
				key := fmt.Sprintf("%s/%s/%d", r.Account, r.Kind, r.EntityID)
				if allowed != nil && !allowed[key] {
					continue
				}
				seen[key] = true
				samples = append(samples, icp.Sample{
					Key: key, Account: r.Account, Kind: r.Kind, EntityID: r.EntityID,
					Name: r.Name, Openings: r.Openings, ObservedAt: r.ObservedAt,
				})
			}
			report.Entities = len(seen)
			trends := icp.FillRates(samples)

			if d := strings.ToLower(strings.TrimSpace(direction)); d != "" && d != "any" {
				filtered := make([]icp.Trend, 0, len(trends))
				for _, t := range trends {
					if t.Direction == d {
						filtered = append(filtered, t)
					}
				}
				trends = filtered
			}
			if limit > 0 && len(trends) > limit {
				trends = trends[:limit]
			}
			report.Trends = trends

			if len(trends) == 0 {
				report.Note = fmt.Sprintf(
					"no entity has two or more observations in this window; run 'iclasspro-pp-cli sync %s' again after some time has passed",
					account)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			if len(trends) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), report.Note)
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "DIRECTION\tPER DAY\tFIRST\tLAST\tSAMPLES\tPROJECTED FULL\tNAME")
			for _, t := range trends {
				fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%d\t%s\t%s\n",
					t.Direction, strconv.FormatFloat(t.PerDay, 'f', -1, 64),
					t.First, t.Last, t.Samples, orDash(t.ProjectedETA), truncate(t.Name, 44))
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&programs, "programs", "", "Comma-separated program ids to include")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path")
	cmd.Flags().StringVar(&since, "since", "90d", "Only use observations newer than this (e.g. 7d, 24h, 1w)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum trends to return (0 = all)")
	cmd.Flags().StringVar(&direction, "direction", "", "Only show trends in this direction: filling, emptying, flat")
	return cmd
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

// icpProgramFilter resolves a comma-separated program-id list into the set of
// entity keys belonging to those programs. Returns nil when no filter is set.
func icpProgramFilter(ctx context.Context, s *store.Store, account, programs string) (map[string]bool, error) {
	if strings.TrimSpace(programs) == "" {
		return nil, nil
	}
	want := map[int]bool{}
	for _, p := range strings.Split(programs, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		id, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("--programs %q is not a numeric program id", p)
		}
		want[id] = true
	}
	if len(want) == 0 {
		return nil, nil
	}
	ents, err := icpLatestEntities(ctx, s, account)
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{}
	for _, e := range ents {
		if want[e.ProgramID] {
			allowed[e.Key()] = true
		}
	}
	return allowed, nil
}
