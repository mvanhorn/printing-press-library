// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/apartments/internal/apt"

	"github.com/spf13/cobra"
)

type historyEntry struct {
	ObservedAt  string  `json:"observed_at"`
	MaxRent     int     `json:"max_rent,omitempty"`
	Beds        int     `json:"beds,omitempty"`
	Baths       float64 `json:"baths,omitempty"`
	AvailableAt string  `json:"available_at,omitempty"`
	FetchStatus int     `json:"fetch_status,omitempty"`
}

func newHistoryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "history <url-or-id>",
		Short:       "Time-series of every observation of one listing.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  apartments-pp-cli history https://www.apartments.com/the-domain-austin-tx/abc123/ --json
  apartments-pp-cli history the-domain-austin-tx
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			arg := args[0]
			listingURL := arg
			if !strings.HasPrefix(arg, "http") {
				listingURL = "https://www.apartments.com/" + strings.Trim(arg, "/") + "/"
			}
			db, err := openAptStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			rows, err := apt.SnapshotsForURL(db.DB(), listingURL)
			if err != nil {
				return err
			}
			out := make([]historyEntry, 0, len(rows))
			for _, r := range rows {
				out = append(out, historyEntry{
					ObservedAt:  r.ObservedAt.Format(time.RFC3339),
					MaxRent:     r.MaxRent,
					Beds:        r.Beds,
					Baths:       r.Baths,
					AvailableAt: r.AvailableAt,
					FetchStatus: r.FetchStatus,
				})
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}
