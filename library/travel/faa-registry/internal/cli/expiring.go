// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/faa-registry/internal/registrydb"
)

func newNovelExpiringCmd(flags *rootFlags) *cobra.Command {
	var flagWithin string
	var flagState string
	var flagOwner string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "expiring",
		Short: "List registrations expiring within a window, soonest first",
		Long: `Query the local registry for registrations whose expiration date falls within
the next N days, optionally filtered by owner or state. US registrations run
on a 7-year cycle and the FAA notifies by paper mail; this is the scripted
version. Requires a prior sync.`,
		Example:     "  faa-registry-pp-cli expiring --within 365 --state WA\n  faa-registry-pp-cli expiring --within 365 --owner \"NETJETS SALES\" --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			days := 90
			if flagWithin != "" {
				w := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(flagWithin)), "d")
				n, err := strconv.Atoi(w)
				if err != nil || n <= 0 {
					return fmt.Errorf("--within must be a positive number of days, got %q", flagWithin)
				}
				days = n
			}
			db, err := openRegistryDB(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			emitRegistryStaleHint(cmd, db, flags)
			res, err := db.Expiring(cmd.Context(), days, flagOwner, flagState, flagLimit)
			if err != nil {
				return err
			}
			if res == nil {
				res = []registrydb.ExpiringAircraft{}
			}
			if len(res) == 0 {
				// FAA renewals cluster at month-ends far out; an empty window
				// is common and correct. Say when the next one lands.
				if soonest, serr := db.SoonestExpiration(cmd.Context(), flagOwner, flagState); serr == nil && soonest != "" {
					fmt.Fprintf(cmd.ErrOrStderr(), "no registrations expire within %d days; the soonest matching expiration is %s\n", days, soonest)
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), res, flags)
		},
	}
	cmd.Flags().StringVar(&flagWithin, "within", "90", "Window in days (e.g. 90 or 90d)")
	cmd.Flags().StringVar(&flagState, "state", "", "Filter by two-letter state code, e.g. WA")
	cmd.Flags().StringVar(&flagOwner, "owner", "", "Filter by owner name (prefix match incl. co-owners)")
	cmd.Flags().IntVar(&flagLimit, "limit", 100, "Maximum rows to return (0 = no limit)")
	return cmd
}
