// Copyright 2026 ganes-j and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored `limit` command: manage the per-release max price that turns a
// wantlist entry into a standing limit order for `fills`.
//
// pp:data-source local

package cli

import (
	"database/sql"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

func newLimitCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "limit",
		Short: "Set, list, or remove the max price you'd pay for a release (drives 'fills').",
		Long: "Manage limit prices per release. A release with a limit becomes a standing " +
			"limit order that 'fills' checks against the live marketplace lowest asking.",
	}
	cmd.AddCommand(newLimitSetCmd(flags))
	cmd.AddCommand(newLimitListCmd(flags))
	cmd.AddCommand(newLimitRmCmd(flags))
	return cmd
}

func newLimitSetCmd(flags *rootFlags) *cobra.Command {
	var currency string
	var note string
	cmd := &cobra.Command{
		Use:         "set <release_id> <max_price>",
		Short:       "Set the max price you'd pay for a release.",
		Example:     "  discogs-pp-cli limit set 249504 25.00 --currency USD",
		Annotations: map[string]string{"pp:data-source": "local", "pp:happy-args": "release_id=249504;max_price=25.00"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := checkDataSource(flags, "local"); err != nil {
				return err
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("both <release_id> and <max_price> are required"))
			}
			releaseID, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("release_id must be a number, got %q", args[0]))
			}
			price, err := strconv.ParseFloat(args[1], 64)
			if err != nil {
				return usageErr(fmt.Errorf("max_price must be a number, got %q", args[1]))
			}
			st, err := openDiscogsStore(cmd.Context(), flags)
			if err != nil {
				return err
			}
			defer st.Close()
			if err := st.SetWantlistLimit(cmd.Context(), releaseID, price, currency, note); err != nil {
				return fmt.Errorf("saving limit: %w", err)
			}
			out := map[string]any{"release_id": releaseID, "max_price": price, "currency": currency}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "limit set: release %d <= %.2f %s\n", releaseID, price, currency)
			return nil
		},
	}
	cmd.Flags().StringVar(&currency, "currency", "USD", "Currency for the limit price")
	cmd.Flags().StringVar(&note, "note", "", "Optional note")
	return cmd
}

func newLimitRmCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "rm <release_id>",
		Short:       "Remove the limit on a release.",
		Example:     "  discogs-pp-cli limit rm 249504",
		Annotations: map[string]string{"pp:data-source": "local", "pp:happy-args": "release_id=249504"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := checkDataSource(flags, "local"); err != nil {
				return err
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<release_id> is required"))
			}
			releaseID, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("release_id must be a number, got %q", args[0]))
			}
			st, err := openDiscogsStore(cmd.Context(), flags)
			if err != nil {
				return err
			}
			defer st.Close()
			removed, err := st.DeleteWantlistLimit(cmd.Context(), releaseID)
			if err != nil {
				return fmt.Errorf("removing limit: %w", err)
			}
			if !removed {
				fmt.Fprintf(cmd.ErrOrStderr(), "no limit set for release %d\n", releaseID)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "removed limit for release %d\n", releaseID)
			}
			return nil
		},
	}
	return cmd
}

type limitRow struct {
	ReleaseID    int64    `json:"release_id"`
	MaxPrice     float64  `json:"max_price"`
	Currency     string   `json:"currency"`
	Note         string   `json:"note,omitempty"`
	SetAt        string   `json:"set_at"`
	LatestLowest *float64 `json:"latest_lowest,omitempty"`
}

func newLimitListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List all limits you've set, with the latest recorded lowest asking.",
		Example:     "  discogs-pp-cli limit list --agent",
		Annotations: map[string]string{"pp:data-source": "local", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := checkDataSource(flags, "local"); err != nil {
				return err
			}
			st, err := openDiscogsStore(cmd.Context(), flags)
			if err != nil {
				return err
			}
			defer st.Close()
			rows, err := st.DB().QueryContext(cmd.Context(),
				`SELECT release_id, max_price, COALESCE(currency,''), COALESCE(note,''), set_at
				 FROM wantlist_limits ORDER BY set_at DESC`)
			if err != nil {
				return fmt.Errorf("reading limits: %w", err)
			}
			out := make([]limitRow, 0)
			for rows.Next() {
				var r limitRow
				var cur, note sql.NullString
				if err := rows.Scan(&r.ReleaseID, &r.MaxPrice, &cur, &note, &r.SetAt); err != nil {
					continue
				}
				r.Currency = cur.String
				r.Note = note.String
				out = append(out, r)
			}
			_ = rows.Err()
			_ = rows.Close()
			// Enrich with the latest snapshot (follow-up queries are safe now that rows is closed).
			for i := range out {
				if snap, ok := latestSnapshot(cmd.Context(), st, out[i].ReleaseID); ok && snap.Lowest.Valid {
					v := snap.Lowest.Float64
					out[i].LatestLowest = &v
				}
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no limits set — use 'limit set <release_id> <max_price>'")
				return nil
			}
			for _, r := range out {
				latest := "—"
				if r.LatestLowest != nil {
					latest = fmt.Sprintf("%.2f", *r.LatestLowest)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "release %-10d limit %.2f %s   latest lowest %s\n", r.ReleaseID, r.MaxPrice, r.Currency, latest)
			}
			return nil
		},
	}
	return cmd
}
