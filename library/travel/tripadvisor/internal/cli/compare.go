// Copyright 2026 David Bryson and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored transcendence command. Preserved across regen.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type compareView struct {
	LocationIDs   []string         `json:"location_ids"`
	Count         int              `json:"compared"`
	Results       []taDetail       `json:"results"`
	FetchFailures []taFetchFailure `json:"fetch_failures"`
}

func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	var (
		language string
		currency string
	)

	cmd := &cobra.Command{
		Use:   "compare <locationId> <locationId> [locationId...]",
		Short: "Compare 2-5 locations side by side: rating, review count, ranking, subratings, trip-type mix",
		Long: "Fetch details for several location IDs and emit one structured comparison so an agent can " +
			"choose between specific places it already shortlisted, without joining multiple calls itself.",
		Example: "  tripadvisor-pp-cli compare 93450 258705 --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "<locationId>=89575;<locationId>=89620",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("compare needs at least 2 location IDs"))
			}
			if len(args) > 5 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("compare accepts at most 5 location IDs (got %d)", len(args)))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// maxScan == len(args): compare never silently drops an explicit ID.
			details, failures, _ := taFetchDetailsBounded(cmd.Context(), c, args, language, currency, len(args))
			if len(details) == 0 {
				return classifyAPIError(fmt.Errorf("all %d detail fetches failed", len(args)), flags)
			}
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d locations could not be fetched and are omitted\n", len(failures), len(args))
			}
			view := compareView{
				LocationIDs:   args,
				Count:         len(details),
				Results:       details,
				FetchFailures: failures,
			}
			return emitTANovel(cmd, flags, view, view.Results)
		},
	}

	cmd.Flags().StringVar(&language, "language", "en", "Language code")
	cmd.Flags().StringVar(&currency, "currency", "USD", "ISO 4217 currency for price fields")
	return cmd
}
