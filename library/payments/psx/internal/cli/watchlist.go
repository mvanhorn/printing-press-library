// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/payments/psx/internal/cliutil"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/psx/internal/store"
)

// newNovelWatchlistCmd gives PSX a concept of "you". The portal has no user
// accounts of any kind, so a saved symbol set can only live locally.
func newNovelWatchlistCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watchlist",
		Short: "Keep a saved symbol set and price it on demand, with change measured from the day you added each name.",
		Long: "Use this command to manage and price your saved symbol set.\n" +
			"Do NOT use it for a one-off price on a symbol you do not track; use 'quote' instead.\n" +
			"Do NOT use it for the whole-market table; use 'market watch' instead.",
		Example:     "  psx-pp-cli watchlist prices --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newWatchlistAddCmd(flags), newWatchlistRemoveCmd(flags), newWatchlistShowCmd(flags))
	return cmd
}

// watchlistSymbols reads the saved set. Callers must not hold another result
// set open across this call: SQLite serves one connection.
func watchlistSymbols(ctx context.Context, s *store.Store) ([]string, map[string]float64, error) {
	rows, err := s.DB().QueryContext(ctx, `SELECT symbol, added_price FROM psx_watchlist ORDER BY symbol`)
	if err != nil {
		return nil, nil, fmt.Errorf("reading watchlist: %w", err)
	}
	syms := make([]string, 0)
	base := make(map[string]float64)
	for rows.Next() {
		var sym sql.NullString
		var price sql.NullFloat64
		if err := rows.Scan(&sym, &price); err != nil {
			_ = rows.Close()
			return nil, nil, fmt.Errorf("scanning watchlist: %w", err)
		}
		if sym.String == "" {
			continue
		}
		syms = append(syms, sym.String)
		if price.Valid {
			base[sym.String] = price.Float64
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, nil, err
	}
	return syms, base, rows.Close()
}

func newWatchlistAddCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "track <symbol>...",
		Short:       "Add one or more instruments to the watchlist",
		Example:     "  psx-pp-cli watchlist track OGDC LUCK ENGRO",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:happy-args": "symbol=OGDC"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "watchlist track")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("at least one symbol is required, e.g. OGDC"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			s, _, err := openLocalStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer s.Close()

			// Price each name at add time so "change since I added it" is
			// answerable later. A pricing failure must not block the add.
			priced := map[string]float64{}
			if t, err := fetchTable(ctx, psxClient(flags), "/market-watch", "", "symbol", "current"); err == nil {
				for _, row := range t.Rows {
					if v, ok := parseNum(row["current"]); ok {
						priced[strings.ToUpper(row["symbol"])] = v
					}
				}
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not price symbols at add time: %v\n", err)
			}

			now := time.Now().UTC().Format(time.RFC3339)
			type added struct {
				Symbol     string  `json:"symbol"`
				AddedAt    string  `json:"added_at"`
				AddedPrice float64 `json:"added_price,omitempty"`
			}
			out := make([]added, 0, len(args))
			for _, raw := range args {
				sym := strings.ToUpper(strings.TrimSpace(raw))
				if sym == "" {
					continue
				}
				p := priced[sym]
				// Keep the original cost basis so "since added" stays meaningful
				// across re-adds, but backfill it when the first add could not
				// price the symbol. Read back what was actually stored rather
				// than echoing the freshly fetched price.
				if _, err := s.DB().ExecContext(ctx,
					`INSERT INTO psx_watchlist (symbol, added_at, added_price) VALUES (?, ?, ?)
					 ON CONFLICT(symbol) DO UPDATE SET
					   added_at = excluded.added_at,
					   added_price = COALESCE(psx_watchlist.added_price, excluded.added_price)`,
					sym, now, nullableFloat(p)); err != nil {
					return fmt.Errorf("adding %s: %w", sym, err)
				}
				var stored sql.NullFloat64
				if err := s.DB().QueryRowContext(ctx,
					`SELECT added_price FROM psx_watchlist WHERE symbol = ?`, sym).Scan(&stored); err != nil {
					return fmt.Errorf("reading back %s: %w", sym, err)
				}
				out = append(out, added{Symbol: sym, AddedAt: now, AddedPrice: stored.Float64})
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			for _, a := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "added %s\n", a.Symbol)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "database path")
	return cmd
}

func nullableFloat(v float64) any {
	if v == 0 {
		return nil
	}
	return v
}

func newWatchlistRemoveCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "untrack <symbol>...",
		Short:       "Remove one or more instruments from the watchlist",
		Example:     "  psx-pp-cli watchlist untrack OGDC",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:happy-args": "symbol=OGDC", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "watchlist untrack")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("at least one symbol is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if mirrorMissing(dbPath) {
				return writeMirrorHint(cmd, flags, orDefaultDB(dbPath), "watchlist")
			}
			s, _, err := openLocalStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			removed := make([]string, 0, len(args))
			for _, raw := range args {
				sym := strings.ToUpper(strings.TrimSpace(raw))
				res, err := s.DB().ExecContext(ctx, `DELETE FROM psx_watchlist WHERE symbol = ?`, sym)
				if err != nil {
					return fmt.Errorf("removing %s: %w", sym, err)
				}
				if n, _ := res.RowsAffected(); n > 0 {
					removed = append(removed, sym)
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), removed, flags)
			}
			if len(removed) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No matching symbols on the watchlist.")
				return nil
			}
			for _, sym := range removed {
				fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", sym)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "database path")
	return cmd
}

func newWatchlistShowCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "prices",
		Short:       "Price the saved symbol set, with change since each name was added",
		Example:     "  psx-pp-cli watchlist prices --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "watchlist prices")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if mirrorMissing(dbPath) {
				return writeMirrorHint(cmd, flags, orDefaultDB(dbPath), "watchlist")
			}
			s, _, err := openLocalStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			syms, base, err := watchlistSymbols(ctx, s)
			if err != nil {
				return err
			}
			type entry struct {
				Symbol        string  `json:"symbol"`
				Current       float64 `json:"current,omitempty"`
				ChangePct     string  `json:"change_pct,omitempty"`
				AddedPrice    float64 `json:"added_price,omitempty"`
				SinceAddedPct float64 `json:"since_added_pct,omitempty"`
				Missing       bool    `json:"missing,omitempty"`
			}
			out := make([]entry, 0, len(syms))
			if len(syms) == 0 {
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), out, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Watchlist is empty. Add names with: psx-pp-cli watchlist track OGDC")
				return nil
			}
			t, err := fetchTable(ctx, psxClient(flags), "/market-watch", "", "symbol", "current")
			if err != nil {
				return err
			}
			live := map[string]map[string]string{}
			for _, row := range t.Rows {
				live[strings.ToUpper(row["symbol"])] = row
			}
			for _, sym := range syms {
				row, ok := live[sym]
				if !ok {
					out = append(out, entry{Symbol: sym, Missing: true, AddedPrice: base[sym]})
					continue
				}
				e := entry{Symbol: sym, ChangePct: row["change_pct"], AddedPrice: base[sym]}
				if v, ok := parseNum(row["current"]); ok {
					e.Current = v
					if b, has := base[sym]; has && b != 0 {
						e.SinceAddedPct = (v - b) / b * 100
					}
				}
				out = append(out, e)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %10s %10s %12s\n", "SYMBOL", "CURRENT", "CHANGE", "SINCE ADDED")
			for _, e := range out {
				if e.Missing {
					fmt.Fprintf(cmd.OutOrStdout(), "%-12s %10s %10s %12s\n", e.Symbol, "-", "-", "not in snapshot")
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %10.2f %10s %11.2f%%\n", cliutil.ScrubTerminal(e.Symbol), e.Current, cliutil.ScrubTerminal(e.ChangePct), e.SinceAddedPct)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "database path")
	return cmd
}
