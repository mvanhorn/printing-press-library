// Copyright 2026 Victor Wibisono and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/agoda/internal/store"
)

// newWatchAddCmd registers a (destination, dates, occupancy) tuple to re-price.
// Without it `watch run` has nothing to do, so the two ship together.
func newWatchAddCmd(flags *rootFlags) *cobra.Command {
	sf := &searchFlags{}
	var dbPath string

	cmd := &cobra.Command{
		Use:   "add [destination]",
		Short: "Track a destination and date range for price drops",
		Long: `Register a search to re-price on every 'watch run'.

A watch is a destination plus dates plus occupancy. Each run prices it, appends
the result to local history, and reports properties that fell below their
trailing median. Adding the same watch twice is a no-op.`,
		Example: "  agoda-pp-cli watch add Tokyo --checkin 2026-10-15 --nights 2",
		Annotations: map[string]string{
			"mcp:read-only": "false",
			"pp:happy-args": "destination=Tokyo;--nights=2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "watch add")
			}
			dest := ""
			if len(args) > 0 {
				dest = args[0]
			}
			if dest == "" && sf.cityID <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a destination argument or --city-id is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c := newAgodaClient(flags)
			d, err := resolveCity(ctx, c, dest, sf.cityID)
			if err != nil {
				return err
			}
			opts, err := sf.searchOptions(d.CityID, false)
			if err != nil {
				return err
			}
			if dbPath == "" {
				dbPath = defaultDBPath("agoda-pp-cli")
			}
			st, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer st.Close()
			if err := store.EnsureAgodaSchema(ctx, st.DB()); err != nil {
				return err
			}
			if _, err := st.DB().ExecContext(ctx, `
                INSERT OR IGNORE INTO price_watches
                    (city_id, destination, checkin, nights, adults, rooms, currency, created_at)
                VALUES (?,?,?,?,?,?,?,?)`,
				d.CityID, displayDestination(d, dest), opts.CheckIn, opts.Nights,
				opts.Adults, opts.Rooms, opts.Currency,
				time.Now().UTC().Format(time.RFC3339)); err != nil {
				return fmt.Errorf("saving watch: %w", err)
			}

			result := map[string]any{
				"watched":     true,
				"destination": displayDestination(d, dest),
				"city_id":     d.CityID,
				"checkin":     opts.CheckIn,
				"nights":      opts.Nights,
				"adults":      opts.Adults,
				"rooms":       opts.Rooms,
				"currency":    opts.Currency,
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"Watching %s (city %d) for check-in %s, %d night(s), %d adult(s), in %s.\nRe-price with: agoda-pp-cli watch run\n",
				displayDestination(d, dest), d.CityID, opts.CheckIn, opts.Nights, opts.Adults, opts.Currency)
			return nil
		},
	}
	bindSearchFlags(cmd, sf)
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local price-history database")
	return cmd
}

// newWatchListCmd shows configured watches and how much history each has.
func newWatchListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List configured price watches and their recorded history",
		Example: "  agoda-pp-cli watch list --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "watch list")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if dbPath == "" {
				dbPath = defaultDBPath("agoda-pp-cli")
			}
			type watchView struct {
				Destination  string `json:"destination"`
				CityID       int    `json:"city_id"`
				CheckIn      string `json:"checkin"`
				Nights       int    `json:"nights"`
				Adults       int    `json:"adults"`
				Currency     string `json:"currency"`
				Observations int    `json:"observations"`
			}
			views := make([]watchView, 0)

			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"no local price history at %s\nrun: agoda-pp-cli watch add <destination> --checkin <date>\n", dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), views, flags)
				}
				return nil
			}
			st, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer st.Close()
			if err := store.EnsureAgodaSchema(ctx, st.DB()); err != nil {
				return err
			}
			watches, err := loadWatches(ctx, st.DB())
			if err != nil {
				return err
			}
			// Counts are gathered after the watch rows are fully drained:
			// SQLite serves one connection, so querying inside the scan loop
			// would deadlock against the open result set.
			for _, w := range watches {
				var n int
				// Match the full watch identity. Filtering on only city,
				// check-in, nights, and adults counted observations belonging
				// to a different watch that differed solely by room count or
				// currency, inflating the history figure shown here.
				row := st.DB().QueryRowContext(ctx, `
                    SELECT COUNT(*) FROM price_observations
                    WHERE city_id = ? AND checkin = ? AND nights = ? AND adults = ?
                      AND rooms = ? AND currency = ?`,
					w.CityID, w.CheckIn, w.Nights, w.Adults, w.Rooms, w.Currency)
				if err := row.Scan(&n); err != nil {
					n = 0
				}
				views = append(views, watchView{
					Destination: w.Destination, CityID: w.CityID, CheckIn: w.CheckIn,
					Nights: w.Nights, Adults: w.Adults, Currency: w.Currency, Observations: n,
				})
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), views, flags)
			}
			if len(views) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(),
					"No watches configured. Add one with: agoda-pp-cli watch add <destination> --checkin <date>")
				return nil
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "DESTINATION\tCHECK-IN\tNIGHTS\tADULTS\tCURRENCY\tOBSERVATIONS")
			for _, v := range views {
				fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%s\t%d\n",
					v.Destination, v.CheckIn, v.Nights, v.Adults, v.Currency, v.Observations)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local price-history database")
	return cmd
}
