// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
	"github.com/spf13/cobra"
)

type shopxExpiringRow struct {
	ListingID       int64  `json:"listing_id"`
	Title           string `json:"title"`
	EndingTimestamp int64  `json:"ending_timestamp"`
	EndingAt        string `json:"ending_at"`
	DaysLeft        int    `json:"days_left"`
	ShouldAutoRenew *bool  `json:"should_auto_renew"`
	State           string `json:"state"`
}

func shopxExpiringListings(db *store.Store, shop string, now time.Time, window time.Duration) ([]shopxExpiringRow, []string) {
	if shop == "" {
		return []shopxExpiringRow{}, []string{shopxNoteShopUnset}
	}
	notes := make([]string, 0)
	listings, dropped := shopxActiveListings(db, shop)
	if dropped > 0 {
		notes = append(notes, shopxForeignNote(dropped))
	}
	rows := make([]shopxExpiringRow, 0)
	end := now.Add(window)
	for _, listing := range listings {
		ending, ok := shopxTS(listing["ending_timestamp"])
		if !ok || ending.Before(now) || ending.After(end) {
			continue
		}
		var renew *bool
		if value, present := shopxBool(listing["should_auto_renew"]); present {
			renew = &value
		}
		rows = append(rows, shopxExpiringRow{
			ListingID: shopxListingID(listing), Title: shopxStr(listing["title"]),
			EndingTimestamp: ending.Unix(), EndingAt: ending.UTC().Format(time.RFC3339),
			DaysLeft: int(ending.Sub(now) / (24 * time.Hour)), ShouldAutoRenew: renew,
			State: shopxStr(listing["state"]),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].EndingTimestamp == rows[j].EndingTimestamp {
			return rows[i].ListingID < rows[j].ListingID
		}
		return rows[i].EndingTimestamp < rows[j].EndingTimestamp
	})
	return rows, notes
}

func newNovelListingsExpiringCmd(flags *rootFlags) *cobra.Command {
	var flagWithin, dbPath string
	cmd := &cobra.Command{
		Use:         "expiring",
		Short:       "List active listings expiring soon and whether they auto-renew.",
		Long:        "Use this command for expiry/renewal timing. Do NOT use it for tag/photo/GPSR gaps; use 'listings audit' instead.",
		Example:     "  etsy-pp-cli listings expiring --within 30d --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "listings expiring")
			}
			window, err := shopxParseWindow("within", flagWithin)
			if err != nil {
				return err
			}
			db, err := shopxOpenDB(cmd, dbPath)
			if err != nil {
				return err
			}
			if db != nil {
				defer db.Close()
			}
			familyNotes := shopxCommandNotes(db, "listings")
			rows, notes := shopxExpiringListings(db, shopxShopID(cmd), shopxNow(), window)
			notes = append(familyNotes, notes...)
			return shopxEmit(cmd, flags, rows, notes)
		},
	}
	cmd.Flags().StringVar(&flagWithin, "within", "30d", "Include listings expiring within this duration")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}
