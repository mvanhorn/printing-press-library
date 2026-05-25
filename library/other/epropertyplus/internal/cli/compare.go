// Copyright 2026 startos00. Licensed under Apache-2.0. See LICENSE.

// Hand-built novel command: `compare --instances a,b[,c]` — summary stats per
// instance (total, structures, lots, structure_pct, avg_asking_price),
// preferring the synced store and otherwise sampling a live hydrate.

package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/epropertyplus/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/epropertyplus/internal/config"
	"github.com/mvanhorn/printing-press-library/library/other/epropertyplus/internal/landbank"
	"github.com/mvanhorn/printing-press-library/library/other/epropertyplus/internal/store"

	"github.com/spf13/cobra"
)

// compareSampleCap bounds the live hydrate per instance when no synced data is
// available, keeping `compare` fast and polite while still estimating the
// structure ratio.
const compareSampleCap = 60

type instanceStats struct {
	Instance       string   `json:"instance"`
	Source         string   `json:"source"`
	Total          int      `json:"total"`
	Structures     int      `json:"structures"`
	Lots           int      `json:"lots"`
	StructurePct   float64  `json:"structure_pct"`
	AvgAskingPrice *float64 `json:"avg_asking_price"`
	IndexSize      int      `json:"index_size,omitempty"`
	Sampled        int      `json:"sampled,omitempty"`
}

func newCompareCmd(flags *rootFlags) *cobra.Command {
	var instances string

	cmd := &cobra.Command{
		Use:   "compare --instances a,b[,c]",
		Short: "Compare inventory summary stats across two or more land-bank instances",
		Long: strings.Trim(`
Compute and compare per-instance summary stats: total, structures, lots,
structure_pct, and avg_asking_price.

For each instance, the synced local store is used when it has data; otherwise a
sample of up to 60 properties is hydrated live to estimate the structure ratio
(in which case index_size and sampled are reported alongside the estimate).

Human output is a table; --json / --agent emit structured stats.
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  epropertyplus-pp-cli compare --instances kclb --json
  epropertyplus-pp-cli compare --instances kclb,detroit
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			slugs := splitCSV(instances)
			if len(slugs) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			stats := make([]instanceStats, 0, len(slugs))
			for _, slug := range slugs {
				s, err := computeInstanceStats(cmd.Context(), flags, slug)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s: %v\n", slug, err)
					continue
				}
				stats = append(stats, s)
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), stats, flags)
			}

			headers := []string{"INSTANCE", "SOURCE", "TOTAL", "STRUCTURES", "LOTS", "STRUCTURE_PCT", "AVG_ASKING"}
			rows := make([][]string, 0, len(stats))
			for _, s := range stats {
				avg := "-"
				if s.AvgAskingPrice != nil {
					avg = fmt.Sprintf("%.0f", *s.AvgAskingPrice)
				}
				rows = append(rows, []string{
					s.Instance, s.Source,
					fmt.Sprintf("%d", s.Total),
					fmt.Sprintf("%d", s.Structures),
					fmt.Sprintf("%d", s.Lots),
					fmt.Sprintf("%.1f%%", s.StructurePct),
					avg,
				})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}
	cmd.Flags().StringVar(&instances, "instances", "", "Comma-separated instance slugs to compare (e.g. kclb,detroit)")
	return cmd
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// computeInstanceStats gathers stats for one instance slug. It prefers the
// synced store; otherwise it samples a live hydrate (capped) and scales the
// total to the index size.
func computeInstanceStats(ctx context.Context, flags *rootFlags, slug string) (instanceStats, error) {
	st := instanceStats{Instance: slug}

	// Store-first.
	dbPath := defaultDBPath("epropertyplus-pp-cli")
	if _, err := os.Stat(dbPath); err == nil {
		db, derr := store.OpenWithContext(ctx, dbPath)
		if derr == nil {
			defer db.Close()
			if n, cerr := db.CountFullProperties(slug); cerr == nil && n > 0 {
				rows, lerr := db.ListFullProperties(slug, 0)
				if lerr == nil {
					return statsFromLoaded(slug, "local", rowsToLoaded(rows), 0, 0), nil
				}
			}
		}
	}

	// Live sample. Build a dedicated client pointed at this slug rather than
	// the active --instance, so compare can span instances in one invocation.
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return st, err
	}
	cfg.BaseURL = landbank.BaseURLForSlug(slug)
	c := client.New(cfg, flags.timeout, flags.rateLimit)
	c.NoCache = flags.noCache

	idx, err := fetchIndex(c)
	if err != nil {
		return st, err
	}
	var loaded []loadedProperty
	sampled := 0
	for _, row := range idx.Rows {
		if sampled >= compareSampleCap {
			break
		}
		p, raw, herr := hydrateProperty(c, row.ID.String())
		if herr != nil {
			continue
		}
		sampled++
		loaded = append(loaded, loadedProperty{Prop: p, Raw: raw})
		time.Sleep(politeDelay)
	}
	return statsFromLoaded(slug, "live", loaded, idx.Size, sampled), nil
}

func rowsToLoaded(rows []store.PropertyRow) []loadedProperty {
	out := make([]loadedProperty, 0, len(rows))
	for _, r := range rows {
		p, err := landbank.ParseProperty(r.Data)
		if err != nil {
			continue
		}
		out = append(out, loadedProperty{Prop: p, Raw: r.Data})
	}
	return out
}

// statsFromLoaded computes summary stats from a set of hydrated properties.
// When indexSize/sampled are non-zero (live sampling), Total is scaled to the
// full index size from the sampled structure ratio; otherwise Total is the
// exact loaded count (synced store).
func statsFromLoaded(slug, source string, loaded []loadedProperty, indexSize, sampled int) instanceStats {
	st := instanceStats{Instance: slug, Source: source}
	var priceSum float64
	var priceN int
	for _, lp := range loaded {
		if lp.Prop.IsStructure() {
			st.Structures++
		} else {
			st.Lots++
		}
		if lp.Prop.AskingPrice != nil {
			priceSum += *lp.Prop.AskingPrice
			priceN++
		}
	}
	measured := st.Structures + st.Lots
	if measured > 0 {
		st.StructurePct = float64(st.Structures) / float64(measured) * 100
	}
	if priceN > 0 {
		avg := priceSum / float64(priceN)
		st.AvgAskingPrice = &avg
	}

	if indexSize > 0 && sampled > 0 {
		// Live sample: report the full index size as the total and surface the
		// sample provenance.
		st.Total = indexSize
		st.IndexSize = indexSize
		st.Sampled = sampled
	} else {
		st.Total = measured
	}
	return st
}
