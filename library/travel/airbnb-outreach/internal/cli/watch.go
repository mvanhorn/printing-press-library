// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/airbnb-outreach/internal/airbnb"
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb-outreach/internal/store"
	"github.com/spf13/cobra"
)

// watchRecord is one watched listing plus its latest observed price.
type watchRecord struct {
	ID          string    `json:"id"`
	Checkin     string    `json:"checkin,omitempty"`
	Checkout    string    `json:"checkout,omitempty"`
	Adults      int       `json:"adults,omitempty"`
	LastPrice   string    `json:"last_price,omitempty"`
	LastChecked time.Time `json:"last_checked,omitempty"`
	AddedAt     time.Time `json:"added_at"`
}

func newWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Track listings' prices over time and detect drops",
		Long: `Watch listings you care about. 'watch check' re-quotes each and reports
price changes since the last check — impossible on the website, which has no
price history.`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newWatchAddCmd(flags))
	cmd.AddCommand(newWatchListCmd(flags))
	cmd.AddCommand(newWatchRemoveCmd(flags))
	cmd.AddCommand(newWatchCheckCmd(flags))
	return cmd
}

func newWatchAddCmd(flags *rootFlags) *cobra.Command {
	var checkin, checkout string
	var adults int
	cmd := &cobra.Command{
		Use:     "add [listing-id]",
		Short:   "Add a listing to your price watchlist",
		Example: "  airbnb-outreach-pp-cli watch add 400704 --checkin 2026-08-10 --checkout 2026-08-14 --adults 2",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			id := airbnb.NumericID(args[0])
			db, err := store.Open(defaultDBPath("airbnb-outreach-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			rec := watchRecord{ID: id, Checkin: checkin, Checkout: checkout, Adults: adults, AddedAt: time.Now()}
			data, _ := json.Marshal(rec)
			if err := db.Upsert("watch", id, data); err != nil {
				return err
			}
			if flags.asJSON {
				return flags.printJSON(cmd, rec)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s Watching listing %s.\n", green("✓"), id)
			return nil
		},
	}
	cmd.Flags().StringVar(&checkin, "checkin", "", "Check-in date used when re-quoting (YYYY-MM-DD)")
	cmd.Flags().StringVar(&checkout, "checkout", "", "Check-out date used when re-quoting (YYYY-MM-DD)")
	cmd.Flags().IntVar(&adults, "adults", 1, "Number of adults")
	return cmd
}

func newWatchListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "Show your watchlist",
		Example: "  airbnb-outreach-pp-cli watch list --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			recs, err := loadWatchRecords()
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, recs)
		},
	}
}

func newWatchRemoveCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "remove [listing-id]",
		Short: "Remove a listing from your watchlist",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			id := airbnb.NumericID(args[0])
			db, err := store.Open(defaultDBPath("airbnb-outreach-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			if _, err := db.DB().Exec(`DELETE FROM resources WHERE resource_type = 'watch' AND id = ?`, id); err != nil {
				return err
			}
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{"status": "removed", "id": id})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed listing %s from watchlist.\n", id)
			return nil
		},
	}
}

func newWatchCheckCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Re-quote every watched listing and report price changes",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			recs, err := loadWatchRecords()
			if err != nil {
				return err
			}
			db, err := store.Open(defaultDBPath("airbnb-outreach-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			c := newAirbnbClient(flags)
			type change struct {
				ID       string `json:"id"`
				Previous string `json:"previous_price"`
				Current  string `json:"current_price"`
				Changed  bool   `json:"changed"`
				Note     string `json:"note,omitempty"`
			}
			var changes []change
			for _, rec := range recs {
				ch := change{ID: rec.ID, Previous: rec.LastPrice}
				quote, err := c.Quote(airbnb.QuoteParams{ListingID: rec.ID, Checkin: rec.Checkin, Checkout: rec.Checkout, Adults: rec.Adults})
				if err != nil {
					ch.Note = err.Error()
					changes = append(changes, ch)
					continue
				}
				price := extractPriceString(quote)
				ch.Current = price
				ch.Changed = price != "" && rec.LastPrice != "" && price != rec.LastPrice
				rec.LastPrice = price
				rec.LastChecked = time.Now()
				data, _ := json.Marshal(rec)
				_ = db.Upsert("watch", rec.ID, data)
				changes = append(changes, ch)
			}
			return flags.printJSON(cmd, changes)
		},
	}
}

func loadWatchRecords() ([]watchRecord, error) {
	db, err := store.Open(defaultDBPath("airbnb-outreach-pp-cli"))
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.List("watch", 500)
	if err != nil {
		return nil, err
	}
	recs := make([]watchRecord, 0, len(rows))
	for _, r := range rows {
		var rec watchRecord
		if json.Unmarshal(r, &rec) == nil {
			recs = append(recs, rec)
		}
	}
	return recs, nil
}

// extractPriceString does a bounded search for a formatted price in a quote
// response. Airbnb nests the total under varying section keys, so this looks
// for the first plausible formatted-amount string.
func extractPriceString(data json.RawMessage) string {
	var v any
	if json.Unmarshal(data, &v) != nil {
		return ""
	}
	return findStringByKeys(v, 0, "totalPrice", "priceString", "amountFormatted", "total", "price")
}

func findStringByKeys(v any, depth int, keys ...string) string {
	if depth > 8 {
		return ""
	}
	switch t := v.(type) {
	case map[string]any:
		for _, k := range keys {
			if s, ok := t[k].(string); ok && s != "" {
				return s
			}
		}
		for _, child := range t {
			if s := findStringByKeys(child, depth+1, keys...); s != "" {
				return s
			}
		}
	case []any:
		for _, child := range t {
			if s := findStringByKeys(child, depth+1, keys...); s != "" {
				return s
			}
		}
	}
	return ""
}
