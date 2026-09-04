// Copyright 2026 Paul Gradeff and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: land-use relevance filter.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// landUseHit is a notice that matched the land-use filter, with the reason.
type landUseHit struct {
	pmnNotice
	LandUseReason string `json:"landUseReason"`
}

// pp:data-source live
func newNovelLanduseCmd(flags *rootFlags) *cobra.Command {
	var flagLocation string
	var flagDays int
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "landuse",
		Short: "Keep only meetings whose body or agenda involves a land-use approval",
		Long: "Filter notices to meetings whose body or agenda involves zoning, subdivisions, " +
			"rezones, CUPs, variances, annexations, or plats. With --location, scans one ZIP/city; " +
			"without it, sweeps all Millard County towns.",
		Example:     "  utah-pmn-pp-cli landuse --location Delta --days 60 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would filter notices to land-use approval items")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			start, end := dateWindow(flagDays)
			var notices []pmnNotice
			if flagLocation != "" {
				notices, err = fetchNotices(ctx, c, flagLocation, start, end, flagLimit)
				sortNoticesByDate(notices)
			} else {
				notices, err = sweepLocations(ctx, c, millardCityNames(), start, end, flagLimit)
			}
			if err != nil {
				return classifyAPIError(err, flags)
			}
			hits := make([]landUseHit, 0, len(notices))
			for _, n := range notices {
				reason := ""
				if matched, kw := agendaHasLandUse(n); matched {
					reason = "agenda: " + kw
				} else if bodyLooksLandUse(n) {
					reason = "body: " + n.PublicBodyName
				}
				if reason != "" {
					hits = append(hits, landUseHit{pmnNotice: n, LandUseReason: reason})
				}
			}
			b, err := json.Marshal(hits)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
		},
	}
	cmd.Flags().StringVar(&flagLocation, "location", "", "ZIP or city to scan (default: all Millard County towns)")
	cmd.Flags().IntVar(&flagDays, "days", 60, "Days ahead to scan from today")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Max notices per location before filtering")
	return cmd
}
