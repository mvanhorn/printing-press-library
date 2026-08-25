// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// pp:data-source auto

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/pexels/internal/config"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/pexels/internal/pexels"
)

type quotaForecast struct {
	Limit           int64    `json:"limit"`
	Remaining       int64    `json:"remaining"`
	ResetUnix       int64    `json:"reset_unix"`
	ResetETA        string   `json:"reset_eta"`
	PlannedRequests int      `json:"planned_requests"`
	MaxPages        int      `json:"max_pages"`
	Resources       []string `json:"resources"`
	Fits            bool     `json:"fits"`
	Note            string   `json:"note"`
}

func newNovelQuotaForecastCmd(flags *rootFlags) *cobra.Command {
	var flagResources string
	var flagMaxPages int
	var flagPerPage int

	cmd := &cobra.Command{
		Use:         "forecast",
		Short:       "Check before a bulk pull whether it fits your remaining hourly/monthly Pexels quota, with a reset ETA.",
		Example:     "--resources photos,videos --max-pages 10 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--max-pages=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would forecast Pexels quota against your remaining rate-limit budget")
				return nil
			}

			resources := splitResources(flagResources, "photos")
			planned := flagMaxPages * len(resources)

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// Best-effort live refresh to populate the rate ledger, unless the
			// user pinned --data-source local.
			if flags.dataSource != "local" {
				cfg, _ := config.Load(flags.configPath)
				key := ""
				if cfg != nil {
					key = cfg.PexelsApiKey
				}
				client := pexels.New(key)
				_, _, _ = client.Get(ctx, "/curated", map[string]string{"per_page": "1"})
			}

			snap, err := pexels.LoadRate()
			if err != nil {
				if errors.Is(err, pexels.ErrNoRateLedger) {
					out := quotaForecast{
						PlannedRequests: planned,
						MaxPages:        flagMaxPages,
						Resources:       resources,
						Fits:            false,
						Note:            "no rate data recorded yet; run a live command (e.g. `pexels-pp-cli photos curated`) first",
					}
					return emitQuotaForecast(cmd, flags, out)
				}
				return err
			}

			out := quotaForecast{
				Limit:           snap.Limit,
				Remaining:       snap.Remaining,
				ResetUnix:       snap.Reset,
				PlannedRequests: planned,
				MaxPages:        flagMaxPages,
				Resources:       resources,
			}
			if snap.Known() {
				out.Fits = snap.Remaining >= int64(planned)
				out.ResetETA = humanResetETA(snap.Reset)
				if out.Fits {
					out.Note = fmt.Sprintf("%d requests planned, %d remaining — fits", planned, snap.Remaining)
				} else {
					out.Note = fmt.Sprintf("%d requests planned but only %d remaining — does not fit; quota resets in %s", planned, snap.Remaining, out.ResetETA)
				}
			} else {
				out.Note = "rate snapshot has no limit data; quota unknown"
			}
			return emitQuotaForecast(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&flagResources, "resources", "photos", "comma-separated resource types to forecast (e.g. photos,videos)")
	cmd.Flags().IntVar(&flagMaxPages, "max-pages", 5, "pages you intend to fetch per resource")
	cmd.Flags().IntVar(&flagPerPage, "per-page", 80, "results per page (informational)")
	return cmd
}

func emitQuotaForecast(cmd *cobra.Command, flags *rootFlags, out quotaForecast) error {
	stdout := cmd.OutOrStdout()
	if flags.asJSON || flags.agent || !isTerminal(stdout) {
		return printJSONFiltered(stdout, out, flags)
	}
	fmt.Fprintf(stdout, "Quota forecast: %s\n", out.Note)
	if out.Limit > 0 {
		fmt.Fprintf(stdout, "  remaining %d / %d, planned %d requests, fits=%v\n", out.Remaining, out.Limit, out.PlannedRequests, out.Fits)
	}
	return nil
}

func splitResources(s, def string) []string {
	out := make([]string, 0)
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 && def != "" {
		out = append(out, def)
	}
	return out
}

func humanResetETA(resetUnix int64) string {
	if resetUnix <= 0 {
		return ""
	}
	d := time.Until(time.Unix(resetUnix, 0))
	if d <= 0 {
		return "now"
	}
	return d.Round(time.Second).String()
}
