// `drift <pid>` returns the price/title timeline captured in listing_snapshots.
// The store appends a snapshot on every UpsertListing, so the timeline grows
// across sync runs and powers the "this listing was $50, now $35" narration
// other Craigslist tools cannot offer because they don't keep history.

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// driftPoint is the typed shape we emit per snapshot.
type driftPoint struct {
	ObservedAt int64  `json:"observedAt"`
	Price      int    `json:"price"`
	Title      string `json:"title"`
}

func newDriftCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "drift [pid]",
		Short:       "Show the price/title timeline for a listing",
		Long:        "Read every snapshot we have captured for a posting ID and emit a timeline. Empty when sync has only seen the listing once.",
		Example:     "  craigslist-pp-cli drift 7915891289 --json\n  craigslist-pp-cli drift 7915891289 --json --select observedAt,price",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			pid, err := strconv.ParseInt(strings.TrimSpace(args[0]), 10, 64)
			if err != nil {
				return fmt.Errorf("invalid pid %q: %w", args[0], err)
			}
			ctx := cmd.Context()
			db, err := openCLStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			snaps, err := db.GetSnapshots(ctx, pid)
			if err != nil {
				return err
			}
			out := make([]driftPoint, 0, len(snaps))
			for _, s := range snaps {
				out = append(out, driftPoint{ObservedAt: s.ObservedAt, Price: s.Price, Title: s.Title})
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				items := make([]map[string]any, 0, len(out))
				for _, p := range out {
					items = append(items, map[string]any{"observedAt": p.ObservedAt, "price": p.Price, "title": p.Title})
				}
				return printAutoTable(cmd.OutOrStdout(), items)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}
