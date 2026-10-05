// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/store"
	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/uberjobs"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newNovelFacetsCmd(flags))
	})
}

// facetRow is one filter value the careers site offers.
type facetRow struct {
	ID    string   `json:"id"`
	Facet string   `json:"facet"`
	Value string   `json:"value"`
	ISO3  *string  `json:"iso3"`
	Teams []string `json:"teams"`
}

// facetNames maps accepted --facet spellings to the row facet name.
var facetNames = map[string]string{
	"country": "country", "countries": "country",
	"team": "team", "teams": "team",
	"sub_team": "sub_team", "sub-team": "sub_team", "subteam": "sub_team", "sub_teams": "sub_team", "subteams": "sub_team",
	"contract_type": "contract_type", "contract-type": "contract_type", "contract_types": "contract_type",
	"work_pattern": "work_pattern", "work-pattern": "work_pattern", "work_patterns": "work_pattern",
}

func newNovelFacetsCmd(flags *rootFlags) *cobra.Command {
	var dbPath, facet string
	cmd := &cobra.Command{
		Use:   "facets",
		Short: "List the countries, teams, subteams, contract types and work patterns the careers site currently offers",
		Long: strings.Trim(`
List the exact filter values the Uber careers site offers, one row per value

The values come from the facet lists the site embeds in its job list page (one
request), the only exact source of the team, subteam, and country spellings
that --team, --sub-team, and --country match. Country rows carry their ISO3
code; meta.unmapped_countries lists any site country with no ISO3 mapping.
Sub-team rows carry the teams the site maps them to.

With --data-source local the newest snapshot saved by sync answers instead.
Exit codes: 2 usage, 5 API or content error, 6 DNS or transport failure,
7 refused (403, 429, or a challenge; never retried).`, "\n"),
		Example: strings.Trim(`
  uber-jobs-pp-cli facets --json
  uber-jobs-pp-cli facets --facet country --agent
  uber-jobs-pp-cli facets --facet sub_team --data-source local`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "auto",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) > 0 {
				return usageErr(fmt.Errorf("facets takes no arguments; use --facet to pick one list"))
			}
			ds, err := resolveDataSource(cmd, flags, "auto", "live", "local")
			if err != nil {
				return err
			}
			want := ""
			if strings.TrimSpace(facet) != "" {
				var ok bool
				want, ok = facetNames[strings.ToLower(strings.TrimSpace(facet))]
				if !ok {
					return usageErr(fmt.Errorf("--facet %q is not a facet: use country, team, sub_team, contract_type, or work_pattern", facet))
				}
			}
			if dryRunOK(flags) {
				if ds == "local" {
					return writeDryRun(cmd.OutOrStdout(), flags, "read the newest facets snapshot from the local store at "+uberDBPath(dbPath))
				}
				return writeDryRun(cmd.OutOrStdout(), flags, "GET "+strings.TrimRight(baseURLFor(flags), "/")+"/en/jobs/ (text/html) and parse the embedded facet lists")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			env, err := readFacets(ctx, flags, ds, dbPath)
			if err != nil {
				return err
			}
			all, _ := env.Results.([]facetRow)
			rows := make([]facetRow, 0, len(all))
			for _, r := range all {
				if want == "" || r.Facet == want {
					rows = append(rows, r)
				}
			}
			env.Scanned = len(all)
			env.Hits, env.Returned = len(rows), len(rows)
			env.Results = rows
			return printEnvelope(cmd, flags, env, func(w io.Writer) error {
				tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
				fmt.Fprintln(tw, "FACET\tVALUE\tISO3")
				for _, r := range rows {
					fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Facet, r.Value, deref(r.ISO3, "-"))
				}
				if err := tw.Flush(); err != nil {
					return err
				}
				fmt.Fprintf(w, "\n%d values (source: %s)\n", len(rows), env.Meta.Source)
				if len(env.Meta.Unmapped) > 0 {
					fmt.Fprintln(w, "countries with no ISO3 mapping:", strings.Join(env.Meta.Unmapped, ", "))
				}
				if env.Meta.Note != "" {
					fmt.Fprintln(w, "note:", env.Meta.Note)
				}
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&facet, "facet", "", "Only one list: country, team, sub_team, contract_type, or work_pattern")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI data dir)")
	addDataSourceFlag(cmd, "auto", "Where to read: auto (live, then the newest local snapshot if the site is unreachable), live, or local")
	return cmd
}

// readFacets returns an envelope whose Results is []facetRow.
func readFacets(ctx context.Context, flags *rootFlags, ds, dbPath string) (uberjobs.Envelope, error) {
	env := uberjobs.NewEnvelope("facets", "", []facetRow{}, 0)
	if ds == "local" {
		return localFacets(ctx, dbPath, env)
	}
	c, err := newUberClient(flags)
	if err != nil {
		return env, err
	}
	f, ferr := c.FetchFacets(ctx)
	if ferr != nil {
		if ds == "auto" && (uberjobs.IsRefusal(ferr) || uberjobs.IsTransport(ferr)) {
			if lenv, lerr := localFacets(ctx, dbPath, env); lerr == nil && lenv.Meta.Extra != nil {
				lenv.Meta.Fallback = true
				lenv.Meta.FallbackFrom = ferr.Error()
				lenv.Meta.Note = "the live read failed, so these values come from the newest local snapshot"
				return lenv, nil
			}
		}
		return env, uberErr(ferr)
	}
	env.Meta.Source = uberjobs.SourceSite
	env.Meta.Complete = true
	env.Meta.Requests = c.Requests()
	env.Meta.Unmapped = f.UnmappedCountries
	env.Meta.Extra = map[string]any{"total_jobs": f.TotalJobs}
	env.Results = facetRows(f)
	return env, nil
}

// localFacets answers from the newest snapshot, read-only; a missing or
// empty store is an empty result with a note, never an error, and is never
// created. Meta.Extra is set only when a snapshot exists.
func localFacets(ctx context.Context, dbPath string, env uberjobs.Envelope) (uberjobs.Envelope, error) {
	var f *uberjobs.Facets
	var at string
	_, err := withStoreRO(ctx, dbPath, func(s *store.Store) error {
		var err error
		f, at, err = uberjobs.LatestFacets(ctx, s.DB())
		return err
	})
	if err != nil && !isMissingTable(err) {
		return env, err
	}
	env.Meta.Source = uberjobs.SourceLocal
	if f == nil {
		env.Meta.Note = "no facets snapshot in the local store; run: uber-jobs-pp-cli sync"
		env.Results = []facetRow{}
		return env, nil
	}
	env.Meta.Complete = true
	env.Meta.Unmapped = f.UnmappedCountries
	env.Meta.Extra = map[string]any{"total_jobs": f.TotalJobs, "fetched_at": at}
	env.Meta.Note = "from the facets snapshot saved at " + at
	env.Results = facetRows(f)
	return env, nil
}

// facetRows flattens the facet lists into rows in a fixed order.
func facetRows(f *uberjobs.Facets) []facetRow {
	parents := map[string][]string{}
	for team, subs := range f.TeamSubTeams {
		for _, s := range subs {
			parents[s] = append(parents[s], team)
		}
	}
	rows := make([]facetRow, 0, len(f.Countries)+len(f.Teams)+len(f.SubTeams)+len(f.ContractTypes)+len(f.WorkPatterns))
	add := func(facet string, values []string, fill func(*facetRow)) {
		for _, v := range values {
			r := facetRow{ID: facet + ":" + v, Facet: facet, Value: v}
			if fill != nil {
				fill(&r)
			}
			rows = append(rows, r)
		}
	}
	add("country", f.Countries, func(r *facetRow) {
		if iso, ok := uberjobs.CountryNameToISO3(r.Value); ok {
			r.ISO3 = &iso
		}
	})
	add("team", f.Teams, nil)
	add("sub_team", f.SubTeams, func(r *facetRow) {
		teams := append([]string{}, parents[r.Value]...)
		sort.Strings(teams)
		r.Teams = teams
	})
	add("contract_type", f.ContractTypes, nil)
	add("work_pattern", f.WorkPatterns, nil)
	return rows
}
