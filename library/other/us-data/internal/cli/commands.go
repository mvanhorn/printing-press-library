// Copyright 2026 Dhilip Subramanian. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

func newCPICmd(flags *rootFlags) *cobra.Command {
	var series string
	var years int
	cmd := &cobra.Command{
		Use:   "cpi",
		Short: "Fetch a BLS CPI snapshot",
		Example: "  us-data-pp-cli cpi --agent\n" +
			"  us-data-pp-cli cpi --series CUUR0000SA0 --years 3 --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := commandContext(cmd, flags)
			defer cancel()
			result, err := fetchBLSSeries(ctx, series, "Consumer Price Index for All Urban Consumers: All items in U.S. city average", years)
			if err != nil {
				return err
			}
			return printResult(cmd, flags, result)
		},
	}
	cmd.Flags().StringVar(&series, "series", "CUUR0000SA0", "BLS CPI series ID")
	cmd.Flags().IntVar(&years, "years", 3, "Years of observations to request")
	return cmd
}

func newUnemploymentCmd(flags *rootFlags) *cobra.Command {
	var series string
	var state string
	var years int
	cmd := &cobra.Command{
		Use:   "unemployment",
		Short: "Fetch a BLS unemployment snapshot",
		Example: "  us-data-pp-cli unemployment --agent\n" +
			"  us-data-pp-cli unemployment --series LNS14000000 --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if state != "" && series == "LNS14000000" {
				return printResult(cmd, flags, GuidanceResult{
					Kind:   "source_guidance",
					Status: "needs_series_mapping",
					Title:  "State unemployment lookup",
					Messages: []string{
						"State unemployment requires LA/LAS BLS series mapping by state and area.",
						"Pass an explicit --series for now, or use the national default with no --state.",
					},
					Sources: []string{"https://www.bls.gov/developers/", "https://www.bls.gov/lau/"},
				})
			}
			ctx, cancel := commandContext(cmd, flags)
			defer cancel()
			result, err := fetchBLSSeries(ctx, series, "National unemployment rate", years)
			if err != nil {
				return err
			}
			return printResult(cmd, flags, result)
		},
	}
	cmd.Flags().StringVar(&series, "series", "LNS14000000", "BLS unemployment series ID")
	cmd.Flags().StringVar(&state, "state", "", "Optional state code; requires explicit --series in this first print")
	cmd.Flags().IntVar(&years, "years", 3, "Years of observations to request")
	return cmd
}

func newPopulationCmd(flags *rootFlags) *cobra.Command {
	var place string
	cmd := &cobra.Command{
		Use:     "population",
		Short:   "Fetch Census population for a supported place",
		Example: "  us-data-pp-cli population --place \"Austin, TX\" --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if place == "" {
				return usageErr("--place is required")
			}
			ctx, cancel := commandContext(cmd, flags)
			defer cancel()
			result, err := fetchPopulation(ctx, place)
			var guidance guidanceError
			if errors.As(err, &guidance) {
				return printResult(cmd, flags, guidance.result)
			}
			if err != nil {
				return err
			}
			return printResult(cmd, flags, result)
		},
	}
	cmd.Flags().StringVar(&place, "place", "", "Place label, for example Austin, TX")
	return cmd
}

func newWagesCmd(flags *rootFlags) *cobra.Command {
	var occupation string
	var state string
	cmd := &cobra.Command{
		Use:     "wages",
		Short:   "Explain source-backed wage lookup status",
		Example: "  us-data-pp-cli wages --occupation \"software developer\" --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = state
			return printResult(cmd, flags, unsupportedWagesGuidance(occupation))
		},
	}
	cmd.Flags().StringVar(&occupation, "occupation", "", "Occupation name or SOC code")
	cmd.Flags().StringVar(&state, "state", "", "Optional state code")
	return cmd
}

func newIndustryCmd(flags *rootFlags) *cobra.Command {
	var naics string
	var industry string
	var state string
	cmd := &cobra.Command{
		Use:     "industry",
		Short:   "Fetch or explain BEA industry/regional data",
		Example: "  us-data-pp-cli industry --naics 541511 --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if env("US_DATA_BEA_API_KEY") == "" {
				return printResult(cmd, flags, beaSetupGuidance())
			}
			ctx, cancel := commandContext(cmd, flags)
			defer cancel()
			result, err := fetchBEAIndustry(ctx, naics, industry, state)
			if err != nil {
				return err
			}
			return printResult(cmd, flags, result)
		},
	}
	cmd.Flags().StringVar(&naics, "naics", "", "NAICS code")
	cmd.Flags().StringVar(&industry, "industry", "", "Industry label")
	cmd.Flags().StringVar(&state, "state", "", "Optional state code")
	return cmd
}

func newCompareRegionsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "compare-regions <left> <right>",
		Short:   "Build an agent-readable comparison shell for two regions",
		Args:    cobra.ExactArgs(2),
		Example: "  us-data-pp-cli compare-regions \"Seattle, WA\" \"Austin, TX\" --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			result := CompareResult{
				Kind:  "region_comparison",
				Left:  CompareSide{Region: args[0]},
				Right: CompareSide{Region: args[1]},
				Sources: []string{
					"Census Data API ACS profile",
					"BLS Public Data API",
					"BEA API",
				},
			}
			if env("US_DATA_CENSUS_API_KEY") == "" {
				result.Notices = append(result.Notices, "Population comparison needs US_DATA_CENSUS_API_KEY because Census data queries require an API key.")
			} else {
				ctx, cancel := commandContext(cmd, flags)
				defer cancel()
				if left, err := fetchPopulation(ctx, args[0]); err == nil {
					result.Left.Population = &left
				} else {
					result.Notices = append(result.Notices, "Left population lookup: "+err.Error())
				}
				if right, err := fetchPopulation(ctx, args[1]); err == nil {
					result.Right.Population = &right
				} else {
					result.Notices = append(result.Notices, "Right population lookup: "+err.Error())
				}
			}
			if env("US_DATA_BEA_API_KEY") == "" {
				result.Notices = append(result.Notices, "BEA regional economic facts need US_DATA_BEA_API_KEY.")
			}
			result.Notices = append(result.Notices, "BLS labor comparisons need explicit series mappings for local/state labor series in this first print.")
			return printResult(cmd, flags, result)
		},
	}
	return cmd
}

func newSourcesCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "sources",
		Short: "Show source coverage and auth requirements",
		RunE: func(cmd *cobra.Command, args []string) error {
			return printResult(cmd, flags, map[string]any{
				"kind": "source_coverage",
				"sources": []map[string]any{
					{
						"name":                     "BLS Public Data API",
						"env":                      "US_DATA_BLS_API_KEY",
						"required_for_first_print": false,
						"notes":                    []string{"Keyless v1 timeseries is used for CPI and national unemployment.", "Registered v2 access supports higher limits and richer metadata."},
					},
					{
						"name":                     "Census Data API",
						"env":                      "US_DATA_CENSUS_API_KEY",
						"required_for_first_print": true,
						"notes":                    []string{"Census data queries currently require an API key."},
					},
					{
						"name":                     "BEA API",
						"env":                      "US_DATA_BEA_API_KEY",
						"required_for_first_print": true,
						"notes":                    []string{"BEA requests require a registered UserID."},
					},
				},
			})
		},
	}
}

func newDoctorCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check local auth and source readiness",
		RunE: func(cmd *cobra.Command, args []string) error {
			return printResult(cmd, flags, map[string]any{
				"kind":                  "doctor",
				"status":                "ok",
				"bls_key_configured":    env("US_DATA_BLS_API_KEY") != "",
				"census_key_configured": env("US_DATA_CENSUS_API_KEY") != "",
				"bea_key_configured":    env("US_DATA_BEA_API_KEY") != "",
				"notes": []string{
					"BLS CPI and national unemployment use keyless public v1 timeseries by default.",
					"Census and BEA commands return setup guidance until their keys are configured.",
				},
			})
		},
	}
}
