// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.
// episode info — cheap dispatcher-trace preview. Shows which sources can
// fetch the URL, estimated per-source cost, and which would fire by default.
// No transcript fetch. No spend recorded. Lets users (and agents) say
// "what's this URL worth fetching" before committing to a paid call.

package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/dispatch"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/source"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/transcript"
)

// infoRow is one available-sources row in the info output.
type infoRow struct {
	Tier         string  `json:"tier"`
	Adapter      string  `json:"adapter"`
	Matches      bool    `json:"matches"`
	GatedReason  string  `json:"gated_reason,omitempty"`
	EstimatedUSD float64 `json:"estimated_usd"`
	WouldFire    bool    `json:"would_fire"`
}

// infoReport is the structured shape returned by `episode info --json`.
type infoReport struct {
	URL         string    `json:"url"`
	FiredBy     string    `json:"fired_by"`
	Sources     []infoRow `json:"sources"`
	HasCached   bool      `json:"has_cached"`
	CachedTitle string    `json:"cached_title,omitempty"`
	CachedShow  string    `json:"cached_show,omitempty"`
}

func newEpisodeInfoCmd(flags *rootFlags) *cobra.Command {
	var (
		flagPaid     bool
		flagProvider string
	)
	cmd := &cobra.Command{
		Use:   "info [url]",
		Short: "Preview which sources can fetch a URL and the estimated cost — no transcript fetch, no spend",
		Long: `Runs the dispatcher in trace mode and surfaces:
  - Every adapter, its tier, and whether its URL pattern matches
  - For matching adapters: whether they would fire (vs gated behind --paid),
    and the estimated per-call cost
  - Whether the episode is already cached locally (title + show pulled from store)

Use this before committing to a paid fetch. Cheap (no network beyond
existing-cache lookup), agent-friendly with --json.`,
		Example: `  podcast-goat-pp-cli episode info https://lexfridman.com/sam-altman-2/
  podcast-goat-pp-cli episode info <url> --paid --json
  podcast-goat-pp-cli episode info <url> --provider spoken,taddy`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			url := args[0]

			providers := []string{}
			if flagProvider != "" {
				for _, p := range strings.Split(flagProvider, ",") {
					if t := strings.TrimSpace(p); t != "" {
						providers = append(providers, t)
					}
				}
			}
			opts := dispatch.Options{
				AllowPaid:        flagPaid,
				AllowedProviders: providers,
				DryRun:           true, // critical: no actual fetch
				Explain:          true,
			}

			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would preview %s (verify mode short-circuit)\n", url)
				return nil
			}

			report := infoReport{URL: url}

			// Cached lookup: is this URL already in the store?
			if ps, perr := openPodcastStore(cmd.Context()); perr == nil {
				if existing, gErr := ps.GetTranscript(cmd.Context(), url); gErr == nil && existing != nil {
					report.HasCached = true
					report.CachedTitle = existing.Title
					report.CachedShow = existing.Show
				}
			}

			// Dispatcher trace. We tolerate dispatcher errors here — `info`
			// is a preview command, so "no adapter matched" or "paid tier
			// gated" should still produce useful output (which adapters
			// match the URL, what they'd cost), not abort with an error.
			res, _ := dispatch.Dispatch(cmd.Context(), url, opts)
			if res != nil {
				report.FiredBy = res.FiredBy
			}

			// Build sources rows from the trace + adapter registry.
			adapters := dispatch.Registered()
			for _, a := range adapters {
				row := infoRow{Adapter: a.Name(), Tier: string(a.Tier())}
				matched := a.Match(url)
				row.Matches = matched
				if !matched {
					row.GatedReason = "URL pattern does not match"
				} else if a.Tier() == transcript.TierPaid && !flagPaid {
					row.GatedReason = "paid tier; needs --paid to fire"
				}
				row.EstimatedUSD = estimatedCostUSD(a)
				row.WouldFire = (a.Name() == res.FiredBy)
				report.Sources = append(report.Sources, row)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			renderInfo(cmd.OutOrStdout(), report)
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagPaid, "paid", false, "Include paid-tier adapters in the would-fire calculation")
	cmd.Flags().StringVar(&flagProvider, "provider", "", "Restrict adapter consideration to these (CSV)")
	return cmd
}

// estimatedCostUSD returns a coarse per-call USD estimate for each adapter.
// Free adapters return 0; paid adapters use their typical per-transcript cost.
// Whisper providers are per-minute and the duration is unknown without a
// metadata fetch, so we return a placeholder — accurate enough for triage.
func estimatedCostUSD(a source.Adapter) float64 {
	switch a.Name() {
	case "spoken":
		return 0.10
	case "taddy":
		return 0.40
	case "whisperapi":
		// ~$0.004/min × ~60min typical episode = ~$0.24
		return 0.24
	default:
		return 0
	}
}

func renderInfo(w io.Writer, r infoReport) {
	fmt.Fprintf(w, "URL: %s\n", r.URL)
	if r.HasCached {
		fmt.Fprintf(w, "Cached: yes\n")
		if r.CachedTitle != "" {
			fmt.Fprintf(w, "  title: %s\n", r.CachedTitle)
		}
		if r.CachedShow != "" {
			fmt.Fprintf(w, "  show: %s\n", r.CachedShow)
		}
	} else {
		fmt.Fprintf(w, "Cached: no\n")
	}
	fmt.Fprintf(w, "Would fire: %s\n", emptyDash(r.FiredBy))
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Available sources:")
	for _, s := range r.Sources {
		marker := "  -"
		if s.WouldFire {
			marker = "  *"
		}
		if !s.Matches {
			continue // hide non-matching for brevity in human output
		}
		costStr := "$0.00 (free)"
		if s.EstimatedUSD > 0 {
			costStr = fmt.Sprintf("$%.2f", s.EstimatedUSD)
		}
		reason := ""
		if s.GatedReason != "" {
			reason = " — " + s.GatedReason
		}
		fmt.Fprintf(w, "%s %s/%-12s  est: %s%s\n", marker, s.Tier, s.Adapter, costStr, reason)
	}
}

func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// context import kept for compile alignment with other cli files.
var _ = context.Background
