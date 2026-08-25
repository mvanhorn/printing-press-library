// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

// health.go implements the `health` command: a live reachability + latency probe
// of every data source in the intelligence fallback chain. The primary
// ClinicalTrials.gov registry plus each secondary enrichment source (RxNorm,
// openFDA, PubMed, OpenAlex, MeSH, EU CTIS) gets a cheap probe so an operator can
// tell "the API is down" apart from "this query has no data" before trusting a run.
package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/clinical-trials/internal/source"

	"github.com/spf13/cobra"
)

// perProbeTimeout bounds each individual source probe. Probes run
// sequentially under one shared --timeout ctx; without a per-probe cap, two
// fully-timed-out probes (30s HTTP client timeout each) exhaust the default
// 1-minute budget and every remaining source reads as a false "down".
const perProbeTimeout = 15 * time.Second

// sourceHealth is one probed source's status.
type sourceHealth struct {
	Name      string `json:"name"`
	Role      string `json:"role"` // "primary" or "secondary"
	Status    string `json:"status"`
	LatencyMS int64  `json:"latency_ms"`
	Detail    string `json:"detail,omitempty"`
}

// healthView is the JSON shape `health` emits.
type healthView struct {
	OverallStatus string         `json:"overall_status"`
	Healthy       int            `json:"healthy"`
	Total         int            `json:"total"`
	Sources       []sourceHealth `json:"sources"`
	Note          string         `json:"note,omitempty"`
}

// pp:data-source live
func newNovelHealthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Show live status, latency, and failure rate for every data source in the fallback chain.",
		Long: "Probe every data source the intelligence commands depend on — the primary\n" +
			"ClinicalTrials.gov registry plus the secondary enrichment sources (RxNorm,\n" +
			"openFDA, PubMed, OpenAlex, MeSH, EU CTIS) — and report each one's reachability\n" +
			"and latency, so you can distinguish an outage from an empty result set.",
		Example: "  clinical-trials-pp-cli health --json\n" +
			"  clinical-trials-pp-cli health",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would probe every data source in the fallback chain")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			src := flags.sourceClient()

			probes := []struct {
				name string
				role string
				fn   func(context.Context) error
			}{
				{"ClinicalTrials.gov", "primary", func(ctx context.Context) error {
					_, e := ctgovCount(ctx, c, ctgovParams("term", "cancer"))
					return e
				}},
				{"RxNorm", "secondary", func(ctx context.Context) error {
					_, e := source.Normalize(ctx, src, "aspirin")
					return e
				}},
				{"openFDA", "secondary", func(ctx context.Context) error {
					_, e := source.AdverseEvents(ctx, src, "aspirin", 1, openFDAKey())
					return e
				}},
				{"PubMed", "secondary", func(ctx context.Context) error {
					_, e := source.SearchPubMed(ctx, src, "NCT04280705", 1, ncbiKey())
					return e
				}},
				{"OpenAlex", "secondary", func(ctx context.Context) error {
					_, e := source.SearchOpenAlex(ctx, src, "NCT04280705", 1, openAlexMailto())
					return e
				}},
				{"MeSH", "secondary", func(ctx context.Context) error {
					_, e := source.LookupMeSH(ctx, src, "diabetes", 1)
					return e
				}},
				{"EU CTIS", "secondary", func(ctx context.Context) error {
					_, e := source.ProbeCTIS(ctx, src)
					return e
				}},
			}

			view := healthView{Total: len(probes)}
			for _, p := range probes {
				h := sourceHealth{Name: p.name, Role: p.role}
				start := time.Now()
				// Each probe gets its own sub-deadline so a slow or blocked
				// source can't exhaust the shared ctx and mark every later
				// source "down" with a misleading "context deadline exceeded".
				probeCtx, probeCancel := context.WithTimeout(ctx, perProbeTimeout)
				perr := p.fn(probeCtx)
				probeCancel()
				h.LatencyMS = time.Since(start).Milliseconds()
				if perr == nil {
					h.Status = "ok"
					view.Healthy++
				} else {
					h.Status = "down"
					h.Detail = truncate(perr.Error(), 160)
				}
				view.Sources = append(view.Sources, h)
			}

			view.OverallStatus = deriveOverallStatus(view.Sources)
			if view.OverallStatus != "ok" {
				view.Note = "one or more sources are unreachable; secondary outages only affect enrichment (safety, evidence, cross-registry)"
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return renderHealthHuman(cmd, view)
		},
	}
	return cmd
}

// deriveOverallStatus rolls per-source results into one verdict: "down" when the
// primary registry is unreachable (intelligence commands can't run at all),
// "degraded" when only secondary enrichment sources failed, "ok" otherwise.
func deriveOverallStatus(sources []sourceHealth) string {
	overall := "ok"
	for _, h := range sources {
		if h.Status == "ok" {
			continue
		}
		if h.Role == "primary" {
			return "down"
		}
		overall = "degraded"
	}
	return overall
}

func renderHealthHuman(cmd *cobra.Command, v healthView) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Data-source health: %s (%d/%d healthy)\n\n", upperStatus(v.OverallStatus), v.Healthy, v.Total)
	for _, h := range v.Sources {
		mark := green("ok")
		if h.Status != "ok" {
			mark = red(h.Status)
		}
		fmt.Fprintf(w, "  %-20s %-6s %5dms  %s\n", h.Name, mark, h.LatencyMS, h.Detail)
	}
	if v.Note != "" {
		fmt.Fprintf(w, "\nnote: %s\n", v.Note)
	}
	return nil
}

func upperStatus(s string) string {
	switch s {
	case "ok":
		return green("OK")
	case "degraded":
		return yellow("DEGRADED")
	case "down":
		return red("DOWN")
	default:
		return s
	}
}
