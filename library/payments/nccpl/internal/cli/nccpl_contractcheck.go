package cli

// pp:data-source live

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/nccpl/internal/cliutil"
)

type nccplContractResult struct {
	Resource   string `json:"resource"`
	DateFormat string `json:"date_format"`
	Envelope   string `json:"expected_envelope"`
	LatestDate string `json:"latest_date,omitempty"`
	Rows       int    `json:"rows"`
	Status     string `json:"status"`
	Detail     string `json:"detail,omitempty"`
}

type nccplContractView struct {
	Results []nccplContractResult `json:"results"`
	Passed  int                   `json:"passed"`
	Failed  int                   `json:"failed"`
	Note    string                `json:"note,omitempty"`
}

func nccplModeLabel(m nccplDateMode) string {
	switch m {
	case nccplRangeDMY:
		return "DD/MM/YYYY range"
	case nccplRangeISO:
		return "YYYY-MM-DD range"
	default:
		return "YYYY-MM-DD single"
	}
}

func newNCCPLContractCheckCmd(flags *rootFlags) *cobra.Command {
	var (
		resourcesCSV string
		exitCode     bool
	)

	cmd := &cobra.Command{
		Use:   "contract-check",
		Short: "Assert every endpoint family still answers correctly against the live API",
		Long: strings.Trim(`
For each resource, resolve its own latest published date and then POST for exactly that
date, asserting a non-empty row array comes back under the expected envelope key.

This API encodes dates three different ways and returns rows under five different
envelope keys. Getting either wrong yields an empty array with HTTP 200 rather than an
error, so a silent-empty result is indistinguishable from a quiet day unless something
asserts otherwise. A date the API itself just reported as its latest MUST have rows, so
zero rows here is unambiguously a defect: an expired session, a changed request
contract, or envelope drift.

Run this when results look empty, before blaming the data.
`, "\n"),
		Example: strings.Trim(`
  nccpl-pp-cli contract-check
  nccpl-pp-cli contract-check --resources fipi,fipi-normal,lipi-normal --json
  nccpl-pp-cli contract-check --exit-code
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--resources=fipi",
			"pp:typed-exit-codes": "0,3,4",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "contract-check")
			}
			selected, err := nccplSelectResources(resourcesCSV)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// Fail fast when no session has been captured. Without a Cloudflare
			// clearance cookie every request lands on a challenge page, which reads
			// as a slow timeout rather than the real problem: nothing is configured.
			if !nccplHasSession(flags.configPath) {
				return authErr(fmt.Errorf("no NCCPL session configured; run 'nccpl-pp-cli auth login --chrome' first (this API has no API key)"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Live dogfood runs this against the real API under a flat per-command
			// timeout: probe one representative resource per date-format family
			// rather than all twenty.
			if cliutil.IsDogfoodEnv() && resourcesCSV == "" {
				selected = nccplRepresentativeResources()
			}

			view := nccplContractView{Results: make([]nccplContractResult, 0, len(selected))}
			for _, r := range selected {
				out := nccplContractResult{
					Resource:   r.Name,
					DateFormat: nccplModeLabel(r.Mode),
					Envelope:   r.Envelope,
				}
				latest, err := nccplLatestDate(ctx, c, r)
				if err != nil {
					out.Status, out.Detail = "FAIL", err.Error()
					view.Failed++
					view.Results = append(view.Results, out)
					continue
				}
				out.LatestDate = latest

				rows, _, err := nccplFetchDate(ctx, c, r, latest)
				switch {
				case err != nil:
					out.Status, out.Detail = "FAIL", err.Error()
					view.Failed++
				case len(rows) == 0:
					out.Status = "FAIL"
					out.Detail = "latest-date reported " + latest + " but the data endpoint returned zero rows; expected non-empty"
					view.Failed++
				default:
					out.Status, out.Rows = "PASS", len(rows)
					view.Passed++
				}
				view.Results = append(view.Results, out)
			}
			if view.Failed > 0 {
				view.Note = "a failing family usually means an expired session (re-run 'auth login --chrome'), a changed date encoding, or envelope drift"
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-18s %-10s %-12s %-6s %s\n",
					"RESOURCE", "DATE FORMAT", "ENVELOPE", "LATEST", "ROWS", "STATUS")
				for _, r := range view.Results {
					fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-18s %-10s %-12s %-6d %s\n",
						r.Resource, r.DateFormat, r.Envelope, r.LatestDate, r.Rows, r.Status)
					if r.Detail != "" {
						fmt.Fprintf(cmd.ErrOrStderr(), "  %s: %s\n", r.Resource, r.Detail)
					}
				}
				fmt.Fprintf(cmd.OutOrStdout(), "\n%d passed, %d failed\n", view.Passed, view.Failed)
				if view.Note != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\n", view.Note)
				}
			}

			if exitCode && view.Failed > 0 {
				return notFoundErr(fmt.Errorf("%d endpoint family/families failed the contract check", view.Failed))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&resourcesCSV, "resources", "", "Comma-separated resources to check; empty means all")
	cmd.Flags().BoolVar(&exitCode, "exit-code", false, "Exit 3 when any family fails, for pipeline gating")
	return cmd
}

// nccplRepresentativeResources returns one resource per (date format, envelope key)
// combination, which is the smallest set that still exercises every request encoding
// and every response shape this API uses.
func nccplRepresentativeResources() []nccplResource {
	seen := map[string]bool{}
	out := make([]nccplResource, 0)
	for _, r := range nccplResources {
		k := fmt.Sprintf("%d|%s", r.Mode, r.Envelope)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, r)
	}
	return out
}
