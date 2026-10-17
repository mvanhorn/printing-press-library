// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/investorpaisa/internal/cliutil"
	"github.com/spf13/cobra"
)

type conditionDiff struct {
	Entered []string `json:"entered"`
	Left    []string `json:"left"`
}

type changesResult struct {
	Source       string                   `json:"source"`
	AsOf         string                   `json:"as_of,omitempty"`
	BaselineAsOf string                   `json:"baseline_as_of,omitempty"`
	Since        string                   `json:"since"`
	Sector       string                   `json:"sector,omitempty"`
	Conditions   map[string]conditionDiff `json:"conditions"`
	Note         string                   `json:"note,omitempty"`
}

// ipConditions evaluates the named conditions for one screener row.
func ipConditions(r ipRow, rsThreshold float64) map[string]bool {
	c, cok := ipNum(r, "close")
	out := map[string]bool{}
	if rs, ok := ipNum(r, "rs"); ok {
		out["rs_above_threshold"] = rs >= rsThreshold
	}
	if s, ok := ipNum(r, "sma50"); ok && cok {
		out["above_50dma"] = c > s
	}
	if s, ok := ipNum(r, "sma200"); ok && cok {
		out["above_200dma"] = c > s
	}
	if v, ok := ipNum(r, "newhi"); ok {
		out["new_52w_high"] = v >= 1
	}
	if v, ok := ipNum(r, "fromlo52"); ok {
		out["at_52w_low"] = v <= 0
	}
	if v, ok := ipNum(r, "volratio"); ok {
		out["volume_spike_2x"] = v >= 2
	}
	return out
}

// diffSnapshots lists, per condition, the symbols that entered or left it
// between a baseline and the current rows. Symbols missing from either side
// are ignored so index reshuffles do not look like crossings.
func diffSnapshots(base, cur []ipRow, rsThreshold float64, sector string) map[string]conditionDiff {
	baseBy := map[string]map[string]bool{}
	for _, r := range base {
		baseBy[ipStrField(r, "symbol")] = ipConditions(r, rsThreshold)
	}
	out := map[string]conditionDiff{}
	for _, name := range []string{"rs_above_threshold", "above_50dma", "above_200dma", "new_52w_high", "at_52w_low", "volume_spike_2x"} {
		out[name] = conditionDiff{Entered: []string{}, Left: []string{}}
	}
	for _, r := range cur {
		if sector != "" && !strings.EqualFold(ipStrField(r, "sector"), sector) {
			continue
		}
		sym := ipStrField(r, "symbol")
		prev, ok := baseBy[sym]
		if !ok {
			continue
		}
		for name, now := range ipConditions(r, rsThreshold) {
			was, had := prev[name]
			if !had || was == now {
				continue
			}
			d := out[name]
			if now {
				d.Entered = append(d.Entered, sym)
			} else {
				d.Left = append(d.Left, sym)
			}
			out[name] = d
		}
	}
	for k, d := range out {
		sort.Strings(d.Entered)
		sort.Strings(d.Left)
		out[k] = d
	}
	return out
}

var sinceRe = regexp.MustCompile(`^(\d+)([dwh])$`)

// parseSinceDays converts 1d, 3d, 1w or 24h into whole days (minimum 1).
func parseSinceDays(s string) (int, error) {
	m := sinceRe.FindStringSubmatch(strings.ToLower(strings.TrimSpace(s)))
	if m == nil {
		return 0, fmt.Errorf("invalid --since %q; use forms like 1d, 3d, 1w or 24h", s)
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n > 3650 {
		return 0, fmt.Errorf("--since %q is too large; use at most 3650 days", s)
	}
	switch m[2] {
	case "w":
		n *= 7
	case "h":
		n = (n + 23) / 24
	}
	if n < 1 {
		n = 1
	}
	return n, nil
}

func newNovelChangesCmd(flags *rootFlags) *cobra.Command {
	var since, sector, dbPath string
	var rsThreshold float64

	cmd := &cobra.Command{
		Use:   "changes",
		Short: "Which stocks entered or left conditions (52-week-high signal, RS crossing 70, DMA crossings) since an earlier snapshot.",
		Long: `Diff two stored daily screener snapshots and list the stocks that entered or left each condition:
RS above a threshold, above the 50 and 200 day averages, a new 52-week high, at a 52-week low, and a 2x volume spike.
Snapshots are stored every time a command fetches the screener table, so run 'sync' on different days to build history.

Use this to list which stocks entered or left conditions between two stored snapshots.
Do NOT use it for market-level trend; use 'breadth' instead.
Do NOT use it for today's snapshot ranking; use 'movers' instead.`,
		Example: strings.Trim(`
  investorpaisa-pp-cli changes --since 1d --agent
  investorpaisa-pp-cli changes --since 1w --sector IT --rs-threshold 60`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "--since=1d"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "changes")
			}
			days, err := parseSinceDays(since)
			if err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			loader := newIPLoader(cmd, flags, dbPath)
			defer loader.close()
			rows, src, err := loader.screenerRows(ctx)
			if err != nil {
				return err
			}
			db, err := loader.store(ctx)
			if err != nil {
				return err
			}
			snaps, err := ipLatestSnapshots(ctx, db, ipKindScreener, 60)
			if err != nil {
				return err
			}
			res := changesResult{Source: src.Source, AsOf: src.AsOf, Since: since, Sector: sector, Conditions: map[string]conditionDiff{}}
			cur, perr := time.Parse("2006-01-02", src.AsOf)
			if perr != nil {
				return fmt.Errorf("snapshot has no valid date: %q", src.AsOf)
			}
			target := cur.AddDate(0, 0, -days).Format("2006-01-02")
			var base *ipSnapshot
			for i := range snaps {
				if snaps[i].AsOf < src.AsOf && snaps[i].AsOf <= target {
					base = &snaps[i]
					break
				}
			}
			if base == nil {
				for i := range snaps { // fall back to the oldest earlier snapshot
					if snaps[i].AsOf < src.AsOf {
						base = &snaps[i]
					}
				}
			}
			if base == nil {
				res.Note = "only one snapshot is stored so there is nothing to compare; run 'investorpaisa-pp-cli sync' on another trading day"
				res.Conditions = diffSnapshots(rows, rows, rsThreshold, sector)
			} else {
				baseRows, berr := ipParseRows(base.Body)
				if berr != nil {
					return berr
				}
				res.BaselineAsOf = base.AsOf
				res.Conditions = diffSnapshots(baseRows, rows, rsThreshold, sector)
				if base.AsOf > target {
					res.Note = fmt.Sprintf("no snapshot as old as --since %s; compared against the oldest stored one (%s)", since, base.AsOf)
				}
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "Changes %s -> %s\n", cliutil.ScrubTerminal(res.BaselineAsOf), cliutil.ScrubTerminal(res.AsOf))
				names := make([]string, 0, len(res.Conditions))
				for k := range res.Conditions {
					names = append(names, k)
				}
				sort.Strings(names)
				for _, k := range names {
					d := res.Conditions[k]
					fmt.Fprintf(cmd.OutOrStdout(), "%-20s +%d %v  -%d %v\n", k, len(d.Entered), ipScrubList(d.Entered), len(d.Left), ipScrubList(d.Left))
				}
				if res.Note != "" {
					fmt.Fprintln(cmd.ErrOrStderr(), res.Note)
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), res, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "1d", "How far back to compare: 1d, 3d, 1w, 24h")
	cmd.Flags().StringVar(&sector, "sector", "", "Only stocks in this sector")
	cmd.Flags().Float64Var(&rsThreshold, "rs-threshold", 70, "Relative-strength level for the rs_above_threshold condition")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local SQLite store")
	return cmd
}
