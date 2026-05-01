// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// api_hpn_research.go: `api hpn research` and `api hpn research get`.
// Research costs 1 credit per call on COMPLETED. Default --budget is 5
// (deeper than search because dossier surfacing tempts users into
// chained calls that add up). Default behavior is to block until the
// async job converges; --no-wait short-circuits and returns the id only.

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/client"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/happenstance/api"
)

// CreditCostPerResearch is the documented cost of one /v1/research call
// on COMPLETED. Surfaced in the cost preview before the call goes out.
const CreditCostPerResearch = 1

// DefaultResearchBudget mirrors the plan's "research costs add up"
// rationale: search defaults --budget 0 (unlimited because 2 credits is
// cheap) but research defaults --budget 5 because deep dossier chains
// blow the credit balance otherwise. Override with --budget 0 to opt
// out, --budget N to set a per-call ceiling.
const DefaultResearchBudget = 5

// hpnResearchEnvelope is the JSON envelope `api hpn research` (and
// `api hpn research get`) emit on stdout. Carries both the raw research
// profile (so jq pipelines can introspect employment/education arrays)
// AND the normalized client.Person projection (so the same downstream
// renderers used by coverage / hp people work without branching on
// source).
type hpnResearchEnvelope struct {
	ResearchID string                `json:"research_id"`
	URL        string                `json:"url,omitempty"`
	Subject    string                `json:"subject"`
	Status     string                `json:"status"`
	Source     string                `json:"source"`
	Completed  bool                  `json:"completed"`
	Profile    *api.ResearchProfile  `json:"profile,omitempty"`
	Person     *client.Person        `json:"person,omitempty"`
}

func newAPIHpnResearchCmd(flags *rootFlags) *cobra.Command {
	var (
		noWait bool
		budget int
	)

	cmd := &cobra.Command{
		Use:   "research <description>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Run a Happenstance deep-research dossier (costs 1 credit on completion)",
		Long: `Run a deep-research dossier against the Happenstance public API.

Pass a natural-language description that names the subject and gives
the model some grounding context. Costs 1 credit per call on COMPLETED.
The default --budget is ` + fmt.Sprintf("%d", DefaultResearchBudget) + ` (override with --budget 0 to disable
the gate or --budget N to set a per-call ceiling).

The flow is asynchronous: the CLI calls POST /v1/research and by default
polls GET /v1/research/{id} until the status is COMPLETED, FAILED, or
FAILED_AMBIGUOUS. Pass --no-wait to skip polling and return the id
only — useful for agents that want to fan out many research calls and
gather them later via ` + "`api hpn research get <id>`" + `.`,
		Example: `  contact-goat-pp-cli api hpn research "Brian Chesky, CEO at Airbnb" --yes
  contact-goat-pp-cli api hpn research "Patrick Collison" --no-wait --yes
  contact-goat-pp-cli api hpn research "..." --dry-run`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			description := strings.TrimSpace(strings.Join(args, " "))
			if description == "" {
				return usageErr(fmt.Errorf("research description is empty — pass a natural-language prose subject"))
			}

			c, err := flags.newHappenstanceAPIClient()
			if err != nil {
				return err
			}

			if !flags.dryRun {
				if blocked, msg := checkSearchBudget(budget, CreditCostPerResearch); blocked {
					if flags.asJSON {
						_ = flags.printJSON(cmd, map[string]any{
							"status":      "skipped",
							"reason":      msg,
							"would_spend": CreditCostPerResearch,
							"budget":      budget,
						})
					} else {
						fmt.Fprintln(cmd.OutOrStdout(), msg)
					}
					return nil
				}
				if !flags.yes && !flags.noInput {
					fmt.Fprintf(cmd.ErrOrStderr(),
						"Will spend %d credit on completion. Re-run with --yes to proceed, --budget 0 to disable the prompt, or --dry-run to preview.\n",
						CreditCostPerResearch,
					)
					return usageErr(fmt.Errorf("confirmation required: pass --yes to proceed"))
				}
			}

			env, err := runHpnResearch(cmd.Context(), c, description, noWait)
			if err != nil {
				return classifyHpnError(err)
			}
			return emitHpnResearchEnvelope(cmd, flags, env, description)
		},
	}

	cmd.Flags().BoolVar(&noWait, "no-wait", false, "Return the research id immediately without polling for completion")
	cmd.Flags().IntVar(&budget, "budget", DefaultResearchBudget, "Max credits to spend per call. 0 disables the budget gate.")

	cmd.AddCommand(newAPIHpnResearchGetCmd(flags))
	return cmd
}

func newAPIHpnResearchGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <research_id>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Re-fetch an existing research dossier by id (free)",
		Long: `Calls GET /v1/research/{id} and renders the dossier in the same shape
as ` + "`api hpn research`" + `. Free probe — no credits spent.`,
		Example: `  contact-goat-pp-cli api hpn research get rsh_abc123`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			researchID := strings.TrimSpace(args[0])
			if researchID == "" {
				return usageErr(fmt.Errorf("research_id is empty"))
			}
			c, err := flags.newHappenstanceAPIClient()
			if err != nil {
				return err
			}
			env, err := c.GetResearch(cmd.Context(), researchID)
			if err != nil {
				return classifyHpnError(err)
			}
			// We don't know the original subject prose; surface the id as the
			// subject so the JSON envelope's required field is non-empty.
			subject := researchID
			return emitHpnResearchEnvelope(cmd, flags, env, subject)
		},
	}
	return cmd
}

// runHpnResearch is the POST + (optional) poll loop, factored out so
// tests can drive it directly against an httptest fixture without going
// through cobra.
func runHpnResearch(ctx context.Context, c *api.Client, description string, noWait bool) (api.ResearchEnvelope, error) {
	created, err := c.Research(ctx, description)
	if err != nil {
		return api.ResearchEnvelope{}, err
	}
	if created.Id == "" || noWait {
		return created, nil
	}
	final, err := c.PollResearch(ctx, created.Id, nil)
	if err != nil {
		return api.ResearchEnvelope{}, err
	}
	if final.URL == "" {
		final.URL = created.URL
	}
	if final.Id == "" {
		final.Id = created.Id
	}
	return final, nil
}

// emitHpnResearchEnvelope renders an api.ResearchEnvelope into the
// canonical JSON envelope (when --json) or a human-readable summary
// (otherwise). FAILED / FAILED_AMBIGUOUS statuses surface as exit code
// 5 so the caller can distinguish "the research finished and produced
// nothing useful" from "the API call itself errored".
func emitHpnResearchEnvelope(cmd *cobra.Command, flags *rootFlags, env api.ResearchEnvelope, subject string) error {
	out := hpnResearchEnvelope{
		ResearchID: env.Id,
		URL:        env.URL,
		Subject:    subject,
		Status:     env.Status,
		Source:     "api",
		Completed:  env.Status == api.StatusCompleted,
		Profile:    env.Profile,
	}
	if env.Profile != nil {
		p := api.ToClientPersonFromResearch(*env.Profile, subject)
		out.Person = &p
	}
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		if err := flags.printJSON(cmd, out); err != nil {
			return err
		}
	} else {
		printHpnResearchSummary(cmd, out)
	}
	// FAILED / FAILED_AMBIGUOUS surfaces verbatim and exits 5 so callers
	// can distinguish a server-side failure from a transport error.
	switch env.Status {
	case api.StatusFailed, api.StatusFailedAmbiguous:
		return apiErr(fmt.Errorf("research %s ended with status %s", env.Id, env.Status))
	}
	return nil
}

func printHpnResearchSummary(cmd *cobra.Command, env hpnResearchEnvelope) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s (%s)\n", env.Subject, env.Status)
	if env.URL != "" {
		fmt.Fprintf(w, "  url: %s\n", env.URL)
	}
	if env.ResearchID != "" {
		fmt.Fprintf(w, "  id: %s\n", env.ResearchID)
	}
	if env.Profile == nil {
		return
	}
	p := env.Profile
	if len(p.Employment) > 0 {
		first := p.Employment[0]
		fmt.Fprintf(w, "  current: %s @ %s\n", first.Title, first.Company)
	}
	if p.Summary != "" {
		fmt.Fprintf(w, "  summary: %s\n", truncate(p.Summary, 200))
	}
	if len(p.Hobbies) > 0 {
		fmt.Fprintf(w, "  hobbies: %s\n", strings.Join(p.Hobbies, ", "))
	}
}
