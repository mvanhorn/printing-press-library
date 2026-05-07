package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/sources"
)

func newCoverageCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "coverage <city-slug>",
		Short: "Per-tier row counts, last-sync ages, country-match boost, and which v1 sources are missing for a synced city.",
		Long: `Local-store aggregation that tells you whether a 'near' or 'goat' query
on this city is running on thin data. For each registered v1 source, prints
the row count from the last sync, the last-synced timestamp, the trust
weight, and (for regional sources) whether the country-match boost applies.
Missing sources surface as zero rows so an agent can decide to call
sync-city before querying.`,
		Example: strings.Trim(`
  # Quick coverage check for Tokyo
  wanderlust-goat-pp-cli coverage tokyo

  # Pipe to jq for agent consumption
  wanderlust-goat-pp-cli coverage paris --json --select coverage.source,coverage.row_count`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			slug := strings.ToLower(strings.TrimSpace(strings.Join(args, " ")))
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			store, err := openGoatStore(cmd, flags)
			if err != nil {
				return err
			}
			defer store.Close()
			rows, err := store.Coverage(ctx, slug)
			if err != nil {
				return err
			}
			byName := map[string]int{}
			lastSyncByName := map[string]string{}
			for _, r := range rows {
				byName[r.Source] = r.RowCount
				if r.LastSyncedAt.Valid {
					lastSyncByName[r.Source] = r.LastSyncedAt.String
				}
			}
			country := ""
			if c := lookupCity(slug); c != nil {
				country = string(c.Country)
			}
			report := coverageReport{City: slug, Country: country}
			for _, s := range sources.Registry {
				report.Coverage = append(report.Coverage, coverageEntry{
					Source:            s.Slug,
					Tier:              tierName(s.Tier),
					Trust:             s.Trust,
					CountryMatchBoost: boostMaybe(s, country),
					RowCount:          byName[s.Slug],
					LastSyncedAt:      lastSyncByName[s.Slug],
					Stub:              s.Stub,
					StubReason:        s.StubReason,
				})
			}
			report.Summary = summarize(report)
			if report.Summary.TotalRows == 0 && lookupCity(slug) == nil {
				_ = printJSONFiltered(cmd.OutOrStdout(), report, flags)
				return notFoundErr(fmt.Errorf("no synced data for %q (unknown city slug); run 'sync-city <slug>'", slug))
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	return cmd
}

type coverageReport struct {
	City     string          `json:"city"`
	Country  string          `json:"country,omitempty"`
	Coverage []coverageEntry `json:"coverage"`
	Summary  coverageSummary `json:"summary"`
}

type coverageEntry struct {
	Source            string  `json:"source"`
	Tier              string  `json:"tier"`
	Trust             float64 `json:"trust"`
	CountryMatchBoost float64 `json:"country_match_boost"`
	RowCount          int     `json:"row_count"`
	LastSyncedAt      string  `json:"last_synced_at,omitempty"`
	Stub              bool    `json:"stub,omitempty"`
	StubReason        string  `json:"stub_reason,omitempty"`
}

type coverageSummary struct {
	TotalRows  int `json:"total_rows"`
	WithData   int `json:"sources_with_data"`
	Empty      int `json:"sources_empty"`
	StubsTotal int `json:"stubs_total"`
}

func summarize(r coverageReport) coverageSummary {
	s := coverageSummary{}
	for _, e := range r.Coverage {
		s.TotalRows += e.RowCount
		if e.RowCount > 0 {
			s.WithData++
		} else {
			s.Empty++
		}
		if e.Stub {
			s.StubsTotal++
		}
	}
	return s
}

func boostMaybe(s sources.Source, country string) float64 {
	if s.CountryMatchBoost > 0 && string(s.Country) == country {
		return s.CountryMatchBoost
	}
	return 0
}

func tierName(t sources.Tier) string {
	switch t {
	case sources.TierFoundation:
		return "foundation"
	case sources.TierEditorial:
		return "editorial"
	case sources.TierRegional:
		return "regional"
	case sources.TierHidden:
		return "hidden"
	case sources.TierCrowd:
		return "crowd"
	}
	return "unknown"
}
