// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/fandango/internal/cliutil"

	"github.com/spf13/cobra"
)

func newNovelStartingSoonCmd(flags *rootFlags) *cobra.Command {
	var flagZipCode string
	var flagWithin string

	cmd := &cobra.Command{
		Use:         "starting-soon",
		Short:       "Find nearby screenings beginning within an immediate time window.",
		Example:     "  fandango-pp-cli starting-soon --zip-code 10001 --within 90m --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would find official Fandango showtimes starting soon")
				return nil
			}
			if flagZipCode == "" {
				return usageErr(fmt.Errorf("--zip-code is required"))
			}
			window, err := cliutil.ParseDurationLoose(defaultString(flagWithin, "90m"))
			if err != nil || window <= 0 {
				return usageErr(fmt.Errorf("--within must be a positive duration such as 90m"))
			}
			now := time.Now()
			rows, err := fetchFandangoShowtimes(cmd, flags, map[string]string{
				"ZipCode": flagZipCode, "Radius": "25", "StartDateTime": now.Format(time.RFC3339),
				"EndDateTime": now.Add(window).Format(time.RFC3339), "Limit": "100",
			})
			if err != nil {
				return err
			}
			rows = filterFandangoWindow(rows, now, now.Add(window))
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"zip_code": flagZipCode, "within": window.String(), "count": len(rows), "showtimes": rows,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&flagZipCode, "zip-code", "", "Postal code for nearby showtimes")
	cmd.Flags().StringVar(&flagWithin, "within", "90m", "How soon a showtime must start (for example 90m)")
	return cmd
}
