package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/dispatch"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/regions"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/sourcetypes"
)

// newCoverageCmd: per-tier row counts + which v1 sources are missing for
// a synced city. Pure offline: reads the regions table and the dispatcher
// registry, no live calls.
func newCoverageCmd(flags *rootFlags) *cobra.Command {
	var country string
	cmd := &cobra.Command{
		Use:   "coverage [city]",
		Short: "Per-region source coverage report (real impl vs stubs)",
		Long: `coverage shows which Stage-2 sources are wired for the given country (or
city → country). Includes per-source slug, locale, real-vs-stub status, and
the stub reason for sources that are deferred. Useful for confirming what
will actually get queried by near/goat for a given trip.`,
		Example: strings.Trim(`
  wanderlust-goat-pp-cli coverage --country JP --json
  wanderlust-goat-pp-cli coverage Seoul --country KR
  wanderlust-goat-pp-cli coverage --country US        # falls back to English-Reddit only
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			city := ""
			if len(args) > 0 {
				city = strings.TrimSpace(args[0])
			}
			cc := strings.ToUpper(strings.TrimSpace(country))
			if cc == "" {
				cc = "*"
			}
			region := regions.Lookup(cc)
			reg := dispatch.DefaultRegistry()

			report := CoverageReport{City: city, Country: cc, Region: region}
			for _, slug := range region.LocalReviewSites {
				cli := reg.Get(slug)
				if cli == nil {
					report.NotRegistered = append(report.NotRegistered, slug)
					continue
				}
				row := CoverageRow{
					Slug:   slug,
					Locale: cli.Locale(),
					Stub:   cli.IsStub(),
				}
				if cli.IsStub() {
					row.StubReason = sourcetypes.StubReason(cli)
					report.Stubbed = append(report.Stubbed, row)
				} else {
					report.Real = append(report.Real, row)
				}
			}
			report.Forums = region.LocalForums
			report.GoogleTLD = region.GoogleTLD

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			renderCoverage(cmd, report)
			return nil
		},
	}
	cmd.Flags().StringVar(&country, "country", "", "ISO 3166-1 alpha-2 country code (e.g. JP, KR, FR)")
	return cmd
}

type CoverageReport struct {
	City          string         `json:"city,omitempty"`
	Country       string         `json:"country"`
	Region        regions.Region `json:"region"`
	Real          []CoverageRow  `json:"real_sources"`
	Stubbed       []CoverageRow  `json:"stubbed_sources"`
	NotRegistered []string       `json:"not_registered,omitempty"`
	Forums        []string       `json:"forums"`
	GoogleTLD     string         `json:"google_tld"`
}

type CoverageRow struct {
	Slug       string `json:"slug"`
	Locale     string `json:"locale"`
	Stub       bool   `json:"stub"`
	StubReason string `json:"stub_reason,omitempty"`
}

func renderCoverage(cmd *cobra.Command, r CoverageReport) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s coverage for country %s\n", bold("wanderlust-goat"), r.Country)
	fmt.Fprintf(w, "  primary lang: %s, languages: %s, google.%s\n", r.Region.PrimaryLanguage, strings.Join(r.Region.Languages, ","), r.Region.GoogleTLD)

	if len(r.Real) > 0 {
		fmt.Fprintf(w, "  %s sources (Stage-2 real impl):\n", green("wired"))
		sort.Slice(r.Real, func(i, j int) bool { return r.Real[i].Slug < r.Real[j].Slug })
		for _, row := range r.Real {
			fmt.Fprintf(w, "    - %s (%s)\n", row.Slug, row.Locale)
		}
	}
	if len(r.Stubbed) > 0 {
		fmt.Fprintf(w, "  %s sources (deferred to v2.x):\n", yellow("stubs"))
		sort.Slice(r.Stubbed, func(i, j int) bool { return r.Stubbed[i].Slug < r.Stubbed[j].Slug })
		for _, row := range r.Stubbed {
			fmt.Fprintf(w, "    - %s (%s) — %s\n", row.Slug, row.Locale, row.StubReason)
		}
	}
	if len(r.NotRegistered) > 0 {
		fmt.Fprintf(w, "  %s — slug present in regions table but no client:\n", red("not registered"))
		for _, slug := range r.NotRegistered {
			fmt.Fprintf(w, "    - %s\n", slug)
		}
	}
	fmt.Fprintf(w, "  forums: %s\n", strings.Join(r.Forums, ", "))
}
