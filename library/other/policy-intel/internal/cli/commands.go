// Copyright 2026 Dhilip Subramanian and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newFederalRegisterCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "federal-register",
		Short: "Search FederalRegister.gov documents",
	}
	cmd.AddCommand(newFederalRegisterSearchCmd(flags))
	return cmd
}

func newFederalRegisterSearchCmd(flags *rootFlags) *cobra.Command {
	var agency string
	var since string
	var limit int
	cmd := &cobra.Command{
		Use:     "search <term>",
		Short:   "Search Federal Register documents",
		Args:    cobra.ExactArgs(1),
		Example: `  policy-intel-pp-cli federal-register search "artificial intelligence" --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := commandContext(cmd, flags)
			defer cancel()
			result, err := fetchFederalRegister(ctx, federalRegisterOptions{
				Term:   args[0],
				Agency: agency,
				Since:  since,
				Limit:  limit,
			})
			if err != nil {
				return err
			}
			return printResult(cmd, flags, result)
		},
	}
	cmd.Flags().StringVar(&agency, "agency", "", "FederalRegister.gov agency slug, such as federal-trade-commission")
	cmd.Flags().StringVar(&since, "since", "", "Earliest publication date in YYYY-MM-DD")
	cmd.Flags().IntVar(&limit, "limit", 5, "Number of documents to return")
	return cmd
}

func newRulesCmd(flags *rootFlags) *cobra.Command {
	var agency string
	var since string
	var limit int
	cmd := &cobra.Command{
		Use:     "rules <topic>",
		Short:   "Find recent rules and proposed rules",
		Args:    cobra.ExactArgs(1),
		Example: `  policy-intel-pp-cli rules "artificial intelligence" --agency federal-trade-commission --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := commandContext(cmd, flags)
			defer cancel()
			result, err := fetchFederalRegister(ctx, federalRegisterOptions{
				Term:   args[0],
				Agency: agency,
				Since:  since,
				Types:  []string{"RULE", "PRORULE"},
				Limit:  limit,
				Kind:   "federal_register_rules",
				Source: "FederalRegister.gov API",
			})
			if err != nil {
				return err
			}
			return printResult(cmd, flags, result)
		},
	}
	cmd.Flags().StringVar(&agency, "agency", "", "FederalRegister.gov agency slug, such as federal-trade-commission")
	cmd.Flags().StringVar(&since, "since", "", "Earliest publication date in YYYY-MM-DD")
	cmd.Flags().IntVar(&limit, "limit", 5, "Number of documents to return")
	return cmd
}

func newDocketCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "docket <docket-id>",
		Short:   "Fetch Regulations.gov docket details",
		Args:    cobra.ExactArgs(1),
		Example: `  policy-intel-pp-cli docket EPA-HQ-OPPT-2018-0462 --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := commandContext(cmd, flags)
			defer cancel()
			result, err := fetchRegulationsDocket(ctx, args[0])
			if err != nil {
				return err
			}
			return printResult(cmd, flags, result)
		},
	}
	return cmd
}

func newCommentsCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:     "comments <docket-id>",
		Short:   "List public comments for a Regulations.gov docket",
		Args:    cobra.ExactArgs(1),
		Example: `  policy-intel-pp-cli comments EPA-HQ-OPPT-2018-0462 --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := commandContext(cmd, flags)
			defer cancel()
			result, err := fetchRegulationsComments(ctx, args[0], limit)
			if err != nil {
				return err
			}
			return printResult(cmd, flags, result)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 5, "Number of comments to return; Regulations.gov minimum is 5")
	return cmd
}

func newDeadlinesCmd(flags *rootFlags) *cobra.Command {
	var agency string
	var from string
	var limit int
	cmd := &cobra.Command{
		Use:     "deadlines <topic>",
		Short:   "Find open comment deadlines for a topic",
		Args:    cobra.ExactArgs(1),
		Example: `  policy-intel-pp-cli deadlines "water" --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if from == "" {
				from = todayDate()
			}
			if _, err := time.Parse("2006-01-02", from); err != nil {
				return fmt.Errorf("--from must use YYYY-MM-DD: %w", err)
			}
			ctx, cancel := commandContext(cmd, flags)
			defer cancel()
			result, err := fetchRegulationsDocuments(ctx, regulationsListOptions{
				Kind:         "regulations_deadlines",
				Term:         args[0],
				Agency:       agency,
				CommentEndGE: from,
				Sort:         "commentEndDate",
				Limit:        limit,
			})
			if err != nil {
				return err
			}
			return printResult(cmd, flags, result)
		},
	}
	cmd.Flags().StringVar(&agency, "agency", "", "Regulations.gov agency ID, such as EPA, FTC, FDA")
	cmd.Flags().StringVar(&from, "from", "", "Earliest comment deadline date in YYYY-MM-DD; defaults to today UTC")
	cmd.Flags().IntVar(&limit, "limit", 5, "Number of documents to return; Regulations.gov minimum is 5")
	return cmd
}

func newSourcesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sources",
		Short: "Show source coverage and auth requirements",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return printResult(cmd, flags, SourcesResult{
				Kind: "policy_intel_sources",
				Sources: []SourceStatus{
					{
						Name:   "FederalRegister.gov",
						Status: "ready",
						Auth:   "none",
						Notes: []string{
							"Search and rules commands use this source without credentials.",
							"FederalRegister.gov says its APIs do not require API keys.",
						},
						SourceURLs: []string{"https://www.federalregister.gov/developers/documentation/api/v1"},
					},
					{
						Name:    "Regulations.gov",
						Status:  regulationsStatus(),
						Auth:    "optional_api_key",
						EnvVars: []string{"POLICY_INTEL_REGULATIONS_API_KEY"},
						Notes: []string{
							"Docket, comment, and deadline commands use this source.",
							"If no env var is set, the CLI uses api.data.gov DEMO_KEY for small smoke-test calls.",
						},
						SourceURLs: []string{"https://open.gsa.gov/api/regulationsgov/"},
					},
				},
			})
		},
	}
	return cmd
}

func newDoctorCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check local policy-intel readiness",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return printResult(cmd, flags, map[string]any{
				"kind":                                 "policy_intel_doctor",
				"federal_register_ready":               true,
				"regulations_api_key_configured":       env("POLICY_INTEL_REGULATIONS_API_KEY") != "",
				"regulations_falls_back_to_demo_key":   env("POLICY_INTEL_REGULATIONS_API_KEY") == "",
				"regulations_page_size_minimum":        5,
				"non_goals":                            []string{"comment submission", "legal advice", "compliance certification"},
				"recommended_agent_output_flag":        "--agent",
				"regulations_api_key_environment_name": "POLICY_INTEL_REGULATIONS_API_KEY",
			})
		},
	}
	return cmd
}

func regulationsStatus() string {
	if env("POLICY_INTEL_REGULATIONS_API_KEY") != "" {
		return "ready"
	}
	return "demo_key"
}
